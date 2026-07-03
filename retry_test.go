package smileid

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name  string
		value string
		check func(time.Duration) bool
	}{
		{"seconds", "30", func(d time.Duration) bool { return d == 30*time.Second }},
		{"seconds capped at 60", "120", func(d time.Duration) bool { return d == 60*time.Second }},
		{"zero", "0", func(d time.Duration) bool { return d == 0 }},
		{"invalid", "soon", func(d time.Duration) bool { return d == 0 }},
		{"empty", "", func(d time.Duration) bool { return d == 0 }},
		{"http-date", time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat),
			func(d time.Duration) bool { return d > time.Second && d <= 6*time.Second }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRetryAfter(tt.value); !tt.check(got) {
				t.Errorf("parseRetryAfter(%q) = %v", tt.value, got)
			}
		})
	}
}

func TestIdempotentGETRetriedOn500(t *testing.T) {
	var opCalls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		if atomic.AddInt32(&opCalls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"status":"Internal Server Error","message":"boom"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"complete","job_id":"job_x"}`)
	})

	js, err := c.Verifications.Retrieve(context.Background(), "job_x")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if js.Status != "complete" {
		t.Errorf("status = %q", js.Status)
	}
	if got := atomic.LoadInt32(&opCalls); got != 2 {
		t.Errorf("op called %d times, want 2 (retried once)", got)
	}
}

func TestConflictNotRetried(t *testing.T) {
	var opCalls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		atomic.AddInt32(&opCalls, 1)
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"status":"Conflict","message":"still processing"}`)
	})

	_, err := c.Verifications.Retrieve(context.Background(), "job_x")
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %T, want *ConflictError", err)
	}
	if got := atomic.LoadInt32(&opCalls); got != 1 {
		t.Errorf("409 was retried: op called %d times, want 1", got)
	}
}

func TestEntryPostNotRetried(t *testing.T) {
	var opCalls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		atomic.AddInt32(&opCalls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"status":"Internal Server Error","message":"boom"}`)
	})

	_, err := c.EnhancedKYC.Verify(context.Background(), EnhancedKYCParams{
		Country: "NG", IDType: "NIN", IDNumber: "12345678901",
		UserDetails: validUserDetails(), Consent: validConsent(),
	})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T, want *APIError", err)
	}
	if got := atomic.LoadInt32(&opCalls); got != 1 {
		t.Errorf("entry POST was retried: op called %d times, want 1", got)
	}
}

type flakyRoundTripper struct {
	mu        sync.Mutex
	remaining int
	base      http.RoundTripper
}

func (f *flakyRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	fail := f.remaining > 0
	if fail {
		f.remaining--
	}
	f.mu.Unlock()
	if fail {
		return nil, errors.New("dial tcp: connection refused")
	}
	return f.base.RoundTrip(r)
}

func TestConnectionErrorRetriedForIdempotent(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"bank_codes":[{"code":"044","country":"NG","name":"Access Bank"}]}`)
	})
	// Wrap the client's transport so the first attempt fails to connect. The
	// wrapped base is the TLS test server's own transport, which trusts its
	// certificate.
	c.transport.http = &http.Client{Transport: &flakyRoundTripper{remaining: 1, base: c.transport.http.Transport}}

	resp, err := c.Services.BankCodes(context.Background(), BankCodesParams{})
	if err != nil {
		t.Fatalf("BankCodes after one connection failure: %v", err)
	}
	if len(resp.BankCodes) != 1 {
		t.Errorf("bank_codes = %+v", resp.BankCodes)
	}
}

// writeTruncatedBody declares a Content-Length larger than what it writes,
// then returns: the server closes the connection early and the client's body
// read fails mid-stream.
func writeTruncatedBody(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Length", "500")
	w.WriteHeader(status)
	fmt.Fprint(w, `{"status":`)
}

func TestTruncatedBodyRetriedForIdempotent(t *testing.T) {
	var opCalls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		if atomic.AddInt32(&opCalls, 1) == 1 {
			writeTruncatedBody(w, http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"complete","job_id":"job_x"}`)
	})

	js, err := c.Verifications.Retrieve(context.Background(), "job_x")
	if err != nil {
		t.Fatalf("Retrieve after one truncated body: %v", err)
	}
	if js.Status != "complete" {
		t.Errorf("status = %q", js.Status)
	}
	if got := atomic.LoadInt32(&opCalls); got != 2 {
		t.Errorf("op called %d times, want 2 (retried once)", got)
	}
}

func TestTruncatedBodyIsConnectionErrorForEntryPost(t *testing.T) {
	var opCalls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		atomic.AddInt32(&opCalls, 1)
		writeTruncatedBody(w, http.StatusAccepted)
	})

	_, err := c.EnhancedKYC.Verify(context.Background(), EnhancedKYCParams{
		Country: "NG", IDType: "NIN", IDNumber: "12345678901",
		UserDetails: validUserDetails(), Consent: validConsent(),
	})
	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("err = %T %v, want *ConnectionError", err, err)
	}
	if got := atomic.LoadInt32(&opCalls); got != 1 {
		t.Errorf("entry POST was retried after a truncated body: op called %d times, want 1", got)
	}
}
