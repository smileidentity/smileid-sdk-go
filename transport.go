package smileid

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/smileidentity/smileid-sdk-go/v12/generated/models"
	"github.com/smileidentity/smileid-sdk-go/v12/generated/operations"
)

const (
	sdkName    = "go"
	sdkVersion = "12.0.0"

	timestampLayout = "2006-01-02T15:04:05.000Z"
)

// transport is the single HTTP layer. It builds the URL, attaches auth and
// telemetry headers, optionally signs the request, serializes the body,
// applies the retry policy, and turns failures into typed errors.
type transport struct {
	cfg  config
	http *http.Client
	auth *tokenCache
}

func newTransport(cfg config, hc *http.Client) *transport {
	t := &transport{cfg: cfg, http: hc}
	t.auth = &tokenCache{fetch: t.fetchToken}
	return t
}

func userAgent() string {
	return fmt.Sprintf("smileid-sdk-%s/%s (go/%s)", sdkName, sdkVersion, strings.TrimPrefix(runtime.Version(), "go"))
}

// fetchToken calls POST /v3/token with the documented lowercase headers and no
// body. The token request is idempotent and unauthenticated.
func (t *transport) fetchToken(ctx context.Context) (string, error) {
	var resp struct {
		Token string `json:"token"`
	}
	req := &operations.Request{
		Method:     "POST",
		Path:       "/v3/token",
		Idempotent: true,
		Headers: map[string]string{
			"smileid-partner-id": t.cfg.partnerID,
			"smileid-api-key":    t.cfg.apiKey,
		},
	}
	if err := t.Do(ctx, req, &resp); err != nil {
		return "", err
	}
	if resp.Token == "" {
		return "", &AuthenticationError{&smileIDError{Message: "token endpoint returned an empty token"}}
	}
	return resp.Token, nil
}

// Do implements operations.Doer. It materializes the body once, then runs the
// attempt loop: automatic retries for idempotent operations, plus a single
// token refresh on a 401 for any authenticated operation.
func (t *transport) Do(ctx context.Context, req *operations.Request, out interface{}) error {
	body, contentType, err := t.buildBody(req)
	if err != nil {
		return err
	}

	authRefreshed := false
	for attempt := 0; ; attempt++ {
		httpReq, err := t.newHTTPRequest(ctx, req, body, contentType)
		if err != nil {
			return err
		}

		resp, err := t.http.Do(httpReq)
		if err != nil {
			if req.Idempotent && attempt < t.cfg.maxRetries && ctx.Err() == nil {
				if berr := sleepBackoff(ctx, attempt, 0); berr != nil {
					return connectionError(berr)
				}
				continue
			}
			return connectionError(err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			// A connection dropped mid-body is a transport failure, not a
			// malformed response: surface it as a retryable ConnectionError.
			if req.Idempotent && attempt < t.cfg.maxRetries && ctx.Err() == nil {
				if berr := sleepBackoff(ctx, attempt, 0); berr != nil {
					return connectionError(berr)
				}
				continue
			}
			return connectionError(readErr)
		}

		if resp.StatusCode == http.StatusNotFound && req.NotFoundReturnsBody {
			if out != nil {
				return json.Unmarshal(respBody, out)
			}
			return nil
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out != nil && len(respBody) > 0 {
				return json.Unmarshal(respBody, out)
			}
			return nil
		}

		// Single token refresh on 401 for any authenticated call.
		if resp.StatusCode == http.StatusUnauthorized && req.Authenticated && !authRefreshed {
			authRefreshed = true
			t.auth.invalidate()
			continue
		}

		// Retry policy: idempotent operations only, on retryable statuses.
		if req.Idempotent && attempt < t.cfg.maxRetries && isRetryableStatus(resp.StatusCode) {
			wait := parseRetryAfter(resp.Header.Get("Retry-After"))
			if berr := sleepBackoff(ctx, attempt, wait); berr != nil {
				return connectionError(berr)
			}
			continue
		}

		return parseError(resp.StatusCode, respBody, requestID(resp))
	}
}

func (t *transport) newHTTPRequest(ctx context.Context, req *operations.Request, body []byte, contentType string) (*http.Request, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	hr, err := http.NewRequestWithContext(ctx, req.Method, t.cfg.baseURL+req.Path, reader)
	if err != nil {
		return nil, err
	}
	if len(req.Query) > 0 {
		hr.URL.RawQuery = req.Query.Encode()
	}

	// Telemetry headers, always sent.
	hr.Header.Set("SmileID-Source-SDK", sdkName)
	hr.Header.Set("SmileID-Source-SDK-Version", sdkVersion)
	hr.Header.Set("User-Agent", userAgent())

	for k, v := range req.Headers {
		hr.Header.Set(k, v)
	}
	if contentType != "" {
		hr.Header.Set("Content-Type", contentType)
	}
	if req.Authenticated {
		tok, err := t.auth.token(ctx)
		if err != nil {
			return nil, err
		}
		hr.Header.Set("SmileID-Token", tok)
	}
	if req.NeedsPartnerIDHeader {
		hr.Header.Set("SmileID-Partner-ID", t.cfg.partnerID)
	}
	if req.UserIDHeader != "" {
		hr.Header.Set("User-ID", req.UserIDHeader)
	}

	// Optional HMAC signing runs last: it needs the final serialized body.
	if t.cfg.partnerSecret != "" {
		ts := time.Now().UTC().Format(timestampLayout)
		mac := hmac.New(sha256.New, []byte(t.cfg.partnerSecret))
		mac.Write([]byte(ts))
		mac.Write(body)
		hr.Header.Set("SmileID-Timestamp", ts)
		hr.Header.Set("SmileID-Request-Signature", hex.EncodeToString(mac.Sum(nil)))
	}

	return hr, nil
}

func (t *transport) buildBody(req *operations.Request) ([]byte, string, error) {
	switch req.BodyKind {
	case operations.BodyMultipart:
		return buildMultipart(req)
	case operations.BodyJSON:
		b, err := json.Marshal(req.JSONBody)
		if err != nil {
			return nil, "", err
		}
		return b, "application/json", nil
	default:
		return nil, "", nil
	}
}

func buildMultipart(req *operations.Request) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for _, part := range req.Parts {
		switch part.Kind {
		case operations.PartScalar:
			if err := w.WriteField(part.Name, part.Scalar); err != nil {
				return nil, "", err
			}
		case operations.PartJSON:
			jb, err := json.Marshal(part.JSON)
			if err != nil {
				return nil, "", err
			}
			h := textproto.MIMEHeader{}
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q`, part.Name))
			h.Set("Content-Type", "application/json")
			pw, err := w.CreatePart(h)
			if err != nil {
				return nil, "", err
			}
			if _, err := pw.Write(jb); err != nil {
				return nil, "", err
			}
		case operations.PartBinary:
			if err := writeBinaryPart(w, part.Name, part.Binary); err != nil {
				return nil, "", err
			}
		case operations.PartBinaryArray:
			for _, b := range part.Array {
				if err := writeBinaryPart(w, part.Name, b); err != nil {
					return nil, "", err
				}
			}
		}
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

func writeBinaryPart(w *multipart.Writer, field string, b *models.BinaryInput) error {
	if b == nil {
		return nil
	}
	data, err := b.Bytes()
	if err != nil {
		return err
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, b.Filename()))
	h.Set("Content-Type", b.ContentTypeFor(field, data))
	pw, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = pw.Write(data)
	return err
}

func connectionError(err error) error {
	return &ConnectionError{smileIDError: &smileIDError{Message: err.Error()}, Err: err}
}

func isRetryableStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, // 408
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// parseRetryAfter reads a Retry-After header in either RFC 7231 form
// (delay-seconds or HTTP-date), capping the result at 60s.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return capDuration(time.Duration(secs) * time.Second)
	}
	if tm, err := http.ParseTime(v); err == nil {
		return capDuration(time.Until(tm))
	}
	return 0
}

func capDuration(d time.Duration) time.Duration {
	const max = 60 * time.Second
	switch {
	case d < 0:
		return 0
	case d > max:
		return max
	default:
		return d
	}
}

// sleepBackoff waits before the next retry, honouring an explicit Retry-After
// or falling back to exponential backoff with jitter. It returns the context
// error if the context is cancelled while waiting.
func sleepBackoff(ctx context.Context, attempt int, retryAfter time.Duration) error {
	var wait time.Duration
	if retryAfter > 0 {
		wait = retryAfter
	} else {
		base := 500 * time.Millisecond
		wait = base*time.Duration(1<<uint(attempt)) + time.Duration(rand.Int63n(int64(base)))
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func requestID(resp *http.Response) string {
	for _, name := range []string{"X-Request-Id", "SmileID-Request-Id", "Request-Id"} {
		if v := resp.Header.Get(name); v != "" {
			return v
		}
	}
	return ""
}
