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

func TestTokenCachedUntilExpiry(t *testing.T) {
	var tokenCalls, opCalls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			atomic.AddInt32(&tokenCalls, 1)
			serveToken(w)
			return
		}
		atomic.AddInt32(&opCalls, 1)
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"complete","job_id":"job_x"}`)
	})

	for i := 0; i < 3; i++ {
		if _, err := c.Verifications.Retrieve(context.Background(), "job_x"); err != nil {
			t.Fatalf("Retrieve: %v", err)
		}
	}
	if got := atomic.LoadInt32(&tokenCalls); got != 1 {
		t.Errorf("token fetched %d times, want 1 (cached)", got)
	}
	if got := atomic.LoadInt32(&opCalls); got != 3 {
		t.Errorf("op called %d times, want 3", got)
	}
}

func TestTokenRefreshOn401ThenSuccess(t *testing.T) {
	var tokenCalls, opCalls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			atomic.AddInt32(&tokenCalls, 1)
			serveToken(w)
			return
		}
		// First op attempt: 401. Second (after refresh): 200.
		if atomic.AddInt32(&opCalls, 1) == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"status":"Unauthorized","message":"token expired"}`)
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
		t.Errorf("op called %d times, want 2 (one retry)", got)
	}
	if got := atomic.LoadInt32(&tokenCalls); got != 2 {
		t.Errorf("token fetched %d times, want 2 (refreshed once)", got)
	}
}

func TestSecondUnauthorizedRaisesAuthenticationError(t *testing.T) {
	var opCalls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		atomic.AddInt32(&opCalls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"status":"Unauthorized","message":"invalid token"}`)
	})

	_, err := c.Verifications.Retrieve(context.Background(), "job_x")
	var authErr *AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("err = %T %v, want *AuthenticationError", err, err)
	}
	if authErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d", authErr.StatusCode)
	}
	// The op is attempted twice: original + one refresh retry.
	if got := atomic.LoadInt32(&opCalls); got != 2 {
		t.Errorf("op called %d times, want 2", got)
	}
}

func TestNoTokenForUnauthenticatedServices(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			t.Error("services.bank_codes must not fetch a token")
			serveToken(w)
			return
		}
		if r.Header.Get("SmileID-Token") != "" {
			t.Error("unauthenticated call carried a SmileID-Token")
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"bank_codes":[{"code":"044","country":"NG","name":"Access Bank"}]}`)
	})

	resp, err := c.Services.BankCodes(context.Background(), BankCodesParams{})
	if err != nil {
		t.Fatalf("BankCodes: %v", err)
	}
	if len(resp.BankCodes) != 1 || resp.BankCodes[0].Code != "044" {
		t.Errorf("bank_codes = %+v", resp.BankCodes)
	}
}

func TestTokenEndpointReceivesLowercaseCredentialHeaders(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			if r.Header.Get("smileid-partner-id") != "1234" {
				t.Errorf("smileid-partner-id = %q", r.Header.Get("smileid-partner-id"))
			}
			if r.Header.Get("smileid-api-key") != "test-key" {
				t.Errorf("smileid-api-key = %q", r.Header.Get("smileid-api-key"))
			}
			if r.Header.Get("SmileID-Token") != "" {
				t.Error("token request must not carry SmileID-Token")
			}
			serveToken(w)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"complete","job_id":"job_x"}`)
	})

	if _, err := c.Verifications.Retrieve(context.Background(), "job_x"); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
}

func TestConcurrentCallsDoNotStampedeToken(t *testing.T) {
	var tokenCalls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			atomic.AddInt32(&tokenCalls, 1)
			time.Sleep(10 * time.Millisecond) // widen the race window
			serveToken(w)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"complete","job_id":"job_x"}`)
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Verifications.Retrieve(context.Background(), "job_x"); err != nil {
				t.Errorf("Retrieve: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&tokenCalls); got != 1 {
		t.Errorf("token fetched %d times under concurrency, want 1", got)
	}
}
