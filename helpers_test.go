package smileid

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// jpegBytes and pngBytes are minimal payloads carrying the right magic bytes.
var (
	jpegBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	pngBytes  = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01}
)

// fakeJWT builds an unsigned JWT whose payload carries the given exp claim.
func fakeJWT(exp time.Time) string {
	enc := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	header := enc([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := enc([]byte(fmt.Sprintf(`{"exp":%d}`, exp.Unix())))
	return header + "." + payload + ".sig"
}

// captured records the details of a request the test server received.
type captured struct {
	method      string
	path        string
	query       string
	header      http.Header
	contentType string
	body        []byte
}

// testClient builds a client pointed at the given handler. The server is TLS
// so every test exercises the https-only base URL rule; the server's client
// (which trusts the test certificate) is injected via Config.HTTPClient.
func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(Config{
		PartnerID:  "1234",
		APIKey:     "test-key",
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// serveToken writes a valid token response with an hour of life.
func serveToken(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"token":%q}`, fakeJWT(time.Now().Add(time.Hour)))
}

// captureHandler serves the token endpoint and records any other request,
// replying with the given status and body.
func captureHandler(cap *captured, status int, respBody string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		cap.method = r.Method
		cap.path = r.URL.Path
		cap.query = r.URL.RawQuery
		cap.header = r.Header.Clone()
		cap.contentType = r.Header.Get("Content-Type")
		cap.body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, respBody)
	}
}

type capturedPart struct {
	name        string
	filename    string
	contentType string
	content     []byte
}

func parseMultipart(t *testing.T, contentType string, body []byte) []capturedPart {
	t.Helper()
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type %q: %v", contentType, err)
	}
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	var parts []capturedPart
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		content, _ := io.ReadAll(p)
		parts = append(parts, capturedPart{
			name:        p.FormName(),
			filename:    p.FileName(),
			contentType: p.Header.Get("Content-Type"),
			content:     content,
		})
		_ = p.Close()
	}
	return parts
}

func partsByName(parts []capturedPart, name string) []capturedPart {
	var out []capturedPart
	for _, p := range parts {
		if p.name == name {
			out = append(out, p)
		}
	}
	return out
}

func onePart(t *testing.T, parts []capturedPart, name string) capturedPart {
	t.Helper()
	matches := partsByName(parts, name)
	if len(matches) != 1 {
		t.Fatalf("expected exactly one %q part, got %d", name, len(matches))
	}
	return matches[0]
}

// validUserDetails returns user details that pass client-side validation.
func validUserDetails() UserDetails {
	return UserDetails{GivenNames: "John", LastName: "Doe", Email: String("john@example.com")}
}

func validConsent() Consent {
	return GrantConsent(time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC), "EN", "https://example.com/privacy")
}

func livenessSet(n int) []*BinaryInput {
	imgs := make([]*BinaryInput, n)
	for i := range imgs {
		imgs[i] = FromBytes(jpegBytes, fmt.Sprintf("live%d.jpg", i))
	}
	return imgs
}
