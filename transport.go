package usesmileid

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/smileidentity/smileid-sdk-go/v12/generated/models"
	"github.com/smileidentity/smileid-sdk-go/v12/generated/operations"
)

const (
	sdkName    = "go"
	sdkVersion = "12.2.0"

	timestampLayout = "2006-01-02T15:04:05.000Z"
)

// transport is the single HTTP layer. It builds the URL, attaches auth and
// telemetry headers, serializes the body, applies the retry policy, and turns
// failures into typed errors.
type transport struct {
	cfg  config
	http *http.Client
}

func newTransport(cfg config, hc *http.Client) *transport {
	return &transport{cfg: cfg, http: hc}
}

func userAgent() string {
	return fmt.Sprintf("smileid-sdk-%s/%s (go/%s)", sdkName, sdkVersion, strings.TrimPrefix(runtime.Version(), "go"))
}

// productByPath mirrors the API authorizer: a token bound to a product is
// accepted only on that product's endpoint. Paths absent here mint a token
// with no claims.
var productByPath = map[string]string{
	"/v3/enhanced_kyc":                    "enhanced_kyc",
	"/v3/document_verification":           "document_verification",
	"/v3/enhanced_document_verification":  "enhanced_document_verification",
	"/v3/residency_document_verification": "residency_document_verification",
	"/v3/biometric_kyc":                   "biometric_kyc",
	"/v3/registration":                    "smart_selfie_registration",
	"/v3/authentication":                  "smart_selfie_authentication",
	"/v3/compare":                         "smart_selfie_compare",
}

// jobUserID returns the user a job names, from its User-ID header or its
// user_id body part. The API trims the header but not the token's user_id
// claim, so a padded id would mint a token for a different user than the job
// names.
func jobUserID(job *operations.Request) (*string, error) {
	userID := job.UserIDHeader
	if userID == nil {
		for _, p := range job.Parts {
			if p.Kind == operations.PartScalar && p.Name == "user_id" {
				v := p.Scalar
				userID = &v
				break
			}
		}
	}
	if userID != nil && (*userID == "" || *userID != strings.TrimSpace(*userID)) {
		return nil, validationErrorf("user_id must not be empty or padded with whitespace")
	}
	return userID, nil
}

// mintToken calls POST /v3/token with the documented lowercase headers and
// the claims of the request it authorizes. Tokens are never shared between
// requests: a grant authorizes a captureless job for one user and must not
// reach any other call. The token request is idempotent and unauthenticated,
// and runs under the job's context so the caller's timeout and cancellation
// apply to it.
func (t *transport) mintToken(ctx context.Context, job *operations.Request) (string, error) {
	product, mapped := productByPath[job.Path]
	var userID *string
	if mapped {
		var err error
		if userID, err = jobUserID(job); err != nil {
			return "", err
		}
	}
	grant := enrolledImageGranted(ctx)
	if grant && userID == nil {
		return "", validationErrorf("user_id is required when use_enrolled_image is true")
	}

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
	if userID != nil {
		req.Parts = append(req.Parts, scalarPart("user_id", *userID))
	}
	if mapped {
		req.Parts = append(req.Parts, scalarPart("product", product))
	}
	if grant {
		req.Parts = append(req.Parts, scalarPart("use_enrolled_image", "true"))
	}
	if len(req.Parts) > 0 {
		req.BodyKind = operations.BodyMultipart
	}
	if err := t.Do(ctx, req, &resp); err != nil {
		return "", err
	}
	if resp.Token == "" {
		return "", &UnexpectedResponseError{&smileIDError{Message: "token endpoint returned no token"}}
	}
	return resp.Token, nil
}

func scalarPart(name, value string) operations.Part {
	return operations.Part{Name: name, Kind: operations.PartScalar, Scalar: value}
}

// Do implements operations.Doer. It materializes the body once, mints the
// request's token if it is authenticated, then runs the attempt loop with
// automatic retries for idempotent operations. Retries reuse the token.
func (t *transport) Do(ctx context.Context, req *operations.Request, out interface{}) error {
	body, contentType, err := t.buildBody(req)
	if err != nil {
		return err
	}

	var token string
	if req.Authenticated {
		if token, err = t.mintToken(ctx, req); err != nil {
			return err
		}
	}

	for attempt := 0; ; attempt++ {
		httpReq, err := t.newHTTPRequest(ctx, req, body, contentType, token)
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
			if out == nil {
				return nil
			}
			return decodeJSONObject(resp.StatusCode, respBody, requestID(resp), out)
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

func (t *transport) newHTTPRequest(ctx context.Context, req *operations.Request, body []byte, contentType string, token string) (*http.Request, error) {
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
	if token != "" {
		hr.Header.Set("SmileID-Token", token)
	}
	if req.NeedsPartnerIDHeader {
		hr.Header.Set("SmileID-Partner-ID", t.cfg.partnerID)
	}
	if req.UserIDHeader != nil {
		hr.Header.Set("User-ID", *req.UserIDHeader)
	}

	return hr, nil
}

func (t *transport) buildBody(req *operations.Request) ([]byte, string, error) {
	switch req.BodyKind {
	case operations.BodyMultipart:
		return buildMultipart(req)
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

// mediaTypePattern accepts a bare RFC 6838 type/subtype (token characters
// only). It deliberately rejects parameters, whitespace and control bytes so a
// caller-supplied content type can never inject extra MIME header lines.
var mediaTypePattern = regexp.MustCompile(`^[0-9A-Za-z!#$&^_.+-]+/[0-9A-Za-z!#$&^_.+-]+$`)

func writeBinaryPart(w *multipart.Writer, field string, b *models.BinaryInput) error {
	if b == nil {
		return nil
	}
	data, err := b.Bytes()
	if err != nil {
		return err
	}
	ct := b.ContentTypeFor(field, data)
	if !mediaTypePattern.MatchString(ct) {
		return validationErrorf("invalid content type %q for multipart field %s", ct, field)
	}
	h := textproto.MIMEHeader{}
	// The %q verb escapes quotes, CR and LF, so a hostile filename cannot
	// break out of the Content-Disposition header.
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, field, b.Filename()))
	h.Set("Content-Type", ct)
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

// decodeJSONObject decodes a success (2xx) body into out. The body must be a
// JSON object; anything else (HTML, an array, a bare string, null, an empty
// body, or a shape mismatch) returns an *UnexpectedResponseError.
func decodeJSONObject(status int, body []byte, requestID string, out interface{}) error {
	var probe map[string]json.RawMessage
	if json.Unmarshal(body, &probe) != nil || probe == nil {
		return unexpectedResponseError(status, body, requestID)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return unexpectedResponseError(status, body, requestID)
	}
	return nil
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
