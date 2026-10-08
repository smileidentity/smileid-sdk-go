package usesmileid

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

const jobStatusBody = `{"status":"clear","job_id":"job_x","message":"Job completed"}`

// tokenRecorder serves the token endpoint, recording each token request, and
// delegates every other request to op.
type tokenRecorder struct {
	mu     sync.Mutex
	claims []map[string]string
	bodies []int
}

func (tr *tokenRecorder) handler(t *testing.T, op http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/token" {
			op(w, r)
			return
		}
		got := map[string]string{}
		if body := readBody(r); len(body) > 0 {
			for _, p := range parseMultipart(t, r.Header.Get("Content-Type"), body) {
				got[p.name] = string(p.content)
			}
			tr.mu.Lock()
			tr.bodies = append(tr.bodies, len(body))
			tr.mu.Unlock()
		}
		tr.mu.Lock()
		tr.claims = append(tr.claims, got)
		tr.mu.Unlock()
		serveToken(w)
	}
}

func (tr *tokenRecorder) last() map[string]string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.claims[len(tr.claims)-1]
}

func (tr *tokenRecorder) count() int {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return len(tr.claims)
}

func assertClaims(t *testing.T, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("token claims = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("token claims = %v, want %v", got, want)
		}
	}
}

func statusOK(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, jobStatusBody)
}

func accepted(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprint(w, `{"status":"Accepted","job_id":"job_x","user_id":"user-1"}`)
}

func TestEachRequestMintsItsOwnToken(t *testing.T) {
	var tr tokenRecorder
	c := testClient(t, tr.handler(t, statusOK))

	for i := 0; i < 3; i++ {
		if _, err := c.Verifications.Retrieve(context.Background(), "job_x"); err != nil {
			t.Fatalf("Retrieve: %v", err)
		}
	}
	if got := tr.count(); got != 3 {
		t.Errorf("token minted %d times, want 3", got)
	}
}

func TestUnclaimedRequestSendsNoTokenBody(t *testing.T) {
	var tr tokenRecorder
	c := testClient(t, tr.handler(t, statusOK))

	if _, err := c.Verifications.Retrieve(context.Background(), "job_x"); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	assertClaims(t, tr.last(), map[string]string{})
	if len(tr.bodies) != 0 {
		t.Errorf("token request carried a body: %v", tr.bodies)
	}
}

func TestTokenBindsUserAndProduct(t *testing.T) {
	var tr tokenRecorder
	c := testClient(t, tr.handler(t, accepted))

	_, err := c.EnhancedKYC.Verify(context.Background(), EnhancedKYCParams{
		Country: "NG", IDType: "NIN", IDNumber: "1",
		UserDetails: validUserDetails(),
		Consent:     validConsent(),
		UserID:      String("user-1"),
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	assertClaims(t, tr.last(), map[string]string{"user_id": "user-1", "product": "enhanced_kyc"})
}

func TestTokenBindsBodyUserIDForAuthentication(t *testing.T) {
	var tr tokenRecorder
	c := testClient(t, tr.handler(t, accepted))

	_, err := c.Biometric.Authenticate(context.Background(), AuthenticationParams{
		UserID:         "user-1",
		SelfieImage:    FromBytes(jpegBytes, "s.jpg"),
		LivenessImages: livenessSet(6),
		UserDetails:    validUserDetails(),
		Consent:        validConsent(),
	})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	assertClaims(t, tr.last(), map[string]string{"user_id": "user-1", "product": "smart_selfie_authentication"})
}

func TestUnauthorizedIsNotRetried(t *testing.T) {
	var tr tokenRecorder
	var opCalls int32
	c := testClient(t, tr.handler(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&opCalls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"status":"Unauthorized","message":"invalid token"}`)
	}))

	_, err := c.Verifications.Retrieve(context.Background(), "job_x")
	var authErr *AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("err = %T %v, want *AuthenticationError", err, err)
	}
	if authErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d", authErr.StatusCode)
	}
	if got := atomic.LoadInt32(&opCalls); got != 1 {
		t.Errorf("op called %d times, want 1", got)
	}
	if got := tr.count(); got != 1 {
		t.Errorf("token minted %d times, want 1", got)
	}
}

func TestIdempotentRetriesReuseTheRequestToken(t *testing.T) {
	var tr tokenRecorder
	var opCalls int32
	var mu sync.Mutex
	var tokens []string
	c := testClient(t, tr.handler(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		tokens = append(tokens, r.Header.Get("SmileID-Token"))
		mu.Unlock()
		if atomic.AddInt32(&opCalls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"status":"error","message":"busy"}`)
			return
		}
		statusOK(w, r)
	}))

	if _, err := c.Verifications.Retrieve(context.Background(), "job_x"); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if got := atomic.LoadInt32(&opCalls); got != 2 {
		t.Errorf("op called %d times, want 2", got)
	}
	if got := tr.count(); got != 1 {
		t.Errorf("token minted %d times, want 1", got)
	}
	if len(tokens) != 2 || tokens[0] == "" || tokens[0] != tokens[1] {
		t.Errorf("tokens across retries = %v, want one repeated token", tokens)
	}
}

func TestEnrolledImageGrantIsBoundToTheJobToken(t *testing.T) {
	var tr tokenRecorder
	var cap captured
	c := testClient(t, tr.handler(t, func(w http.ResponseWriter, r *http.Request) {
		cap.body = readBody(r)
		cap.contentType = r.Header.Get("Content-Type")
		accepted(w, r)
	}))

	_, err := c.BiometricKYC.Verify(context.Background(), BiometricKYCParams{
		UseEnrolledImage: true,
		UserID:           String("user-1"),
		Country:          "NG", IDType: "NIN", IDNumber: "1",
		UserDetails: validUserDetails(),
		Consent:     validConsent(),
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	assertClaims(t, tr.last(), map[string]string{
		"user_id":            "user-1",
		"product":            "biometric_kyc",
		"use_enrolled_image": "true",
	})
	parts := parseMultipart(t, cap.contentType, cap.body)
	for _, name := range []string{"use_enrolled_image", "selfie_image", "liveness_images"} {
		if got := partsByName(parts, name); len(got) != 0 {
			t.Errorf("job body carried %q", name)
		}
	}
}

func TestGrantDoesNotReachLaterRequests(t *testing.T) {
	var tr tokenRecorder
	c := testClient(t, tr.handler(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/biometric_kyc" {
			accepted(w, r)
			return
		}
		statusOK(w, r)
	}))

	_, err := c.BiometricKYC.Verify(context.Background(), BiometricKYCParams{
		UseEnrolledImage: true,
		UserID:           String("user-1"),
		Country:          "NG", IDType: "NIN", IDNumber: "1",
		UserDetails: validUserDetails(),
		Consent:     validConsent(),
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if _, err := c.Verifications.Retrieve(context.Background(), "job_x"); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if got := tr.count(); got != 2 {
		t.Fatalf("token minted %d times, want 2", got)
	}
	assertClaims(t, tr.last(), map[string]string{})
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
		statusOK(w, r)
	})

	if _, err := c.Verifications.Retrieve(context.Background(), "job_x"); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
}
