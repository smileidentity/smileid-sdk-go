package usesmileid

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smileidentity/smileid-sdk-go/v12/generated/operations"
)

const jobStatusBody = `{"status":"clear","job_id":"job_x","message":"Job completed"}`

// tokenRecorder serves the token endpoint, recording each token request and
// minting token-1, token-2, ... so each request's token is identifiable. It
// delegates every other request to op.
type tokenRecorder struct {
	mu       sync.Mutex
	requests []tokenRequest
}

type tokenRequest struct {
	contentType string
	body        []byte
}

func (tr *tokenRecorder) handler(op http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/token" {
			op(w, r)
			return
		}
		body := readBody(r)
		tr.mu.Lock()
		tr.requests = append(tr.requests, tokenRequest{contentType: r.Header.Get("Content-Type"), body: body})
		n := len(tr.requests)
		tr.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"token":"token-%d"}`, n)
	}
}

func (tr *tokenRecorder) last(t *testing.T) tokenRequest {
	t.Helper()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if len(tr.requests) == 0 {
		t.Fatal("no token request recorded")
	}
	return tr.requests[len(tr.requests)-1]
}

func (tr *tokenRecorder) lastClaims(t *testing.T) map[string]string {
	t.Helper()
	req := tr.last(t)
	got := map[string]string{}
	if len(req.body) == 0 {
		return got
	}
	for _, p := range parseMultipart(t, req.contentType, req.body) {
		got[p.name] = string(p.content)
	}
	return got
}

func (tr *tokenRecorder) count() int {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return len(tr.requests)
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

func enhancedKYCParams(userID *string) EnhancedKYCParams {
	return EnhancedKYCParams{
		Country: "NG", IDType: "NIN", IDNumber: "1",
		UserDetails: validUserDetails(),
		Consent:     validConsent(),
		UserID:      userID,
	}
}

func enrolledKYCParams() BiometricKYCParams {
	return BiometricKYCParams{
		UserID:  String("user-1"),
		Country: "NG", IDType: "NIN", IDNumber: "1",
		UserDetails: validUserDetails(),
		Consent:     validConsent(),
	}
}

func selfieKYCParams() BiometricKYCParams {
	p := enrolledKYCParams()
	p.SelfieImage = FromBytes(jpegBytes, "s.jpg")
	p.LivenessImages = livenessSet(6)
	return p
}

func TestJobTokenBindsItsUserAndProduct(t *testing.T) {
	user := String("user-1")
	jobs := []struct {
		product string
		call    func(c *Client) error
	}{
		{"enhanced_kyc", func(c *Client) error {
			_, err := c.EnhancedKYC.Verify(context.Background(), enhancedKYCParams(user))
			return err
		}},
		{"document_verification", func(c *Client) error {
			_, err := c.Documents.Verify(context.Background(), DocumentVerificationParams{
				Country:        "NG",
				SelfieImage:    FromBytes(jpegBytes, "s.jpg"),
				LivenessImages: livenessSet(6),
				Document:       FromBytes(jpegBytes, "d.jpg"),
				UserDetails:    validUserDetails(),
				Consent:        validConsent(),
				UserID:         user,
			})
			return err
		}},
		{"enhanced_document_verification", func(c *Client) error {
			_, err := c.Documents.VerifyEnhanced(context.Background(), DocumentVerificationParams{
				Country:        "NG",
				IDType:         String("PASSPORT"),
				SelfieImage:    FromBytes(jpegBytes, "s.jpg"),
				LivenessImages: livenessSet(6),
				Document:       FromBytes(jpegBytes, "d.jpg"),
				UserDetails:    validUserDetails(),
				Consent:        validConsent(),
				UserID:         user,
			})
			return err
		}},
		{"residency_document_verification", func(c *Client) error {
			p := residencyParams()
			p.UserID = user
			_, err := c.Documents.VerifyResidency(context.Background(), p)
			return err
		}},
		{"biometric_kyc", func(c *Client) error {
			_, err := c.BiometricKYC.Verify(context.Background(), selfieKYCParams())
			return err
		}},
		{"smart_selfie_registration", func(c *Client) error {
			_, err := c.Biometric.Enroll(context.Background(), RegistrationParams{
				SelfieImage:    FromBytes(jpegBytes, "s.jpg"),
				LivenessImages: livenessSet(6),
				UserDetails:    validUserDetails(),
				Consent:        validConsent(),
				UserID:         user,
			})
			return err
		}},
		{"smart_selfie_authentication", func(c *Client) error {
			_, err := c.Biometric.Authenticate(context.Background(), AuthenticationParams{
				UserID:         "user-1",
				SelfieImage:    FromBytes(jpegBytes, "s.jpg"),
				LivenessImages: livenessSet(6),
				UserDetails:    validUserDetails(),
				Consent:        validConsent(),
			})
			return err
		}},
		{"smart_selfie_compare", func(c *Client) error {
			_, err := c.Biometric.Compare(context.Background(), CompareParams{
				SelfieImage:         FromBytes(jpegBytes, "s.jpg"),
				ComparisonImage:     FromBytes(jpegBytes, "c.jpg"),
				ComparisonImageType: "id_card",
				UserDetails:         validUserDetails(),
				Consent:             validConsent(),
				UserID:              user,
			})
			return err
		}},
	}
	for _, job := range jobs {
		t.Run(job.product, func(t *testing.T) {
			var tr tokenRecorder
			c := testClient(t, tr.handler(accepted))
			if err := job.call(c); err != nil {
				t.Fatalf("call: %v", err)
			}
			assertClaims(t, tr.lastClaims(t), map[string]string{"user_id": "user-1", "product": job.product})
		})
	}
}

func TestNonJobTokenBindsNothing(t *testing.T) {
	calls := []struct {
		name string
		call func(c *Client) error
	}{
		{"status", func(c *Client) error {
			_, err := c.Verifications.Retrieve(context.Background(), "job_x")
			return err
		}},
		{"replay", func(c *Client) error {
			_, err := c.Verifications.Replay(context.Background(), "job_x", ReplayParams{})
			return err
		}},
		{"report_fraud", func(c *Client) error {
			_, err := c.Users.FlagFraud(context.Background(), "user-1", FlagFraudParams{
				Reason: ReasonFirstPartyFraud, ReportedBy: "risk@partner.example",
			})
			return err
		}},
		{"id_status", func(c *Client) error {
			_, err := c.Services.IDStatus(context.Background(), IDStatusParams{Country: "NG", IDType: "NIN"})
			return err
		}},
	}
	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			var tr tokenRecorder
			c := testClient(t, tr.handler(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"status":"clear","job_id":"job_x","user_id":"user-1","message":"ok"}`)
			}))
			if err := tc.call(c); err != nil {
				t.Fatalf("call: %v", err)
			}
			if got := tr.count(); got != 1 {
				t.Fatalf("token minted %d times, want 1", got)
			}
			if body := tr.last(t).body; len(body) != 0 {
				t.Errorf("token request carried a body: %q", body)
			}
		})
	}
}

func TestJobWithoutUserBindsOnlyProduct(t *testing.T) {
	var tr tokenRecorder
	c := testClient(t, tr.handler(accepted))

	if _, err := c.EnhancedKYC.Verify(context.Background(), enhancedKYCParams(nil)); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	assertClaims(t, tr.lastClaims(t), map[string]string{"product": "enhanced_kyc"})
}

func TestEachJobSendsTheTokenMintedForIt(t *testing.T) {
	var tr tokenRecorder
	var mu sync.Mutex
	var sent []string
	c := testClient(t, tr.handler(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sent = append(sent, r.Header.Get("SmileID-Token"))
		mu.Unlock()
		accepted(w, r)
	}))

	for i := 0; i < 3; i++ {
		if _, err := c.EnhancedKYC.Verify(context.Background(), enhancedKYCParams(nil)); err != nil {
			t.Fatalf("Verify: %v", err)
		}
	}
	if got := tr.count(); got != 3 {
		t.Errorf("token minted %d times, want 3", got)
	}
	if want := []string{"token-1", "token-2", "token-3"}; !reflect.DeepEqual(sent, want) {
		t.Errorf("job tokens = %v, want %v", sent, want)
	}
}

func TestUnauthorizedIsNotRetried(t *testing.T) {
	var tr tokenRecorder
	var opCalls int32
	c := testClient(t, tr.handler(func(w http.ResponseWriter, r *http.Request) {
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
	c := testClient(t, tr.handler(func(w http.ResponseWriter, r *http.Request) {
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
	if want := []string{"token-1", "token-1"}; !reflect.DeepEqual(tokens, want) {
		t.Errorf("tokens across retries = %v, want %v", tokens, want)
	}
}

func TestFailedMintNeverSendsTheJob(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		assert func(t *testing.T, err error)
	}{
		{"401", http.StatusUnauthorized, `{"status":"Unauthorized","message":"bad key"}`, func(t *testing.T, err error) {
			var e *AuthenticationError
			if !errors.As(err, &e) {
				t.Errorf("err = %T %v, want *AuthenticationError", err, err)
			}
		}},
		{"503", http.StatusServiceUnavailable, `{"status":"error","message":"busy"}`, func(t *testing.T, err error) {
			var e *APIError
			if !errors.As(err, &e) {
				t.Errorf("err = %T %v, want *APIError", err, err)
			}
		}},
		{"empty token", http.StatusOK, `{"token":""}`, func(t *testing.T, err error) {
			var e *UnexpectedResponseError
			if !errors.As(err, &e) {
				t.Errorf("err = %T %v, want *UnexpectedResponseError", err, err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var jobCalls int32
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v3/token" {
					atomic.AddInt32(&jobCalls, 1)
					accepted(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})
			c.transport.cfg.maxRetries = 0

			_, err := c.EnhancedKYC.Verify(context.Background(), enhancedKYCParams(nil))
			if err == nil {
				t.Fatal("Verify succeeded, want an error")
			}
			tc.assert(t, err)
			if got := atomic.LoadInt32(&jobCalls); got != 0 {
				t.Errorf("job endpoint called %d times, want 0", got)
			}
		})
	}
}

func TestMintUsesTheRequestTimeout(t *testing.T) {
	var jobCalls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/token" {
			atomic.AddInt32(&jobCalls, 1)
			accepted(w, r)
			return
		}
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})

	start := time.Now()
	_, err := c.EnhancedKYC.Verify(context.Background(), enhancedKYCParams(nil), WithTimeout(50*time.Millisecond))
	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("err = %T %v, want *ConnectionError", err, err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("mint ignored the request timeout, took %v", elapsed)
	}
	if got := atomic.LoadInt32(&jobCalls); got != 0 {
		t.Errorf("job endpoint called %d times, want 0", got)
	}
}

func TestMintHonoursCallerCancellation(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.EnhancedKYC.Verify(ctx, enhancedKYCParams(nil))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestEmptyOrPaddedUserIDIsRejected(t *testing.T) {
	for _, id := range []string{"", " user-1", "user-1 "} {
		t.Run(fmt.Sprintf("%q", id), func(t *testing.T) {
			var calls int32
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
			})

			_, err := c.EnhancedKYC.Verify(context.Background(), enhancedKYCParams(String(id)))
			assertValidationError(t, err)
			if err == nil || !strings.Contains(err.Error(), "user_id") {
				t.Errorf("error should mention user_id: %v", err)
			}
			_, err = c.Biometric.Compare(context.Background(), CompareParams{
				SelfieImage:         FromBytes(jpegBytes, "s.jpg"),
				ComparisonImage:     FromBytes(jpegBytes, "c.jpg"),
				ComparisonImageType: "id_card",
				UserDetails:         validUserDetails(),
				Consent:             validConsent(),
				UserID:              String(id),
			})
			assertValidationError(t, err)
			if got := atomic.LoadInt32(&calls); got != 0 {
				t.Errorf("%d HTTP calls made, want 0", got)
			}
		})
	}
}

func TestEnrolledImageGrantIsBoundToTheJobToken(t *testing.T) {
	var tr tokenRecorder
	var cap captured
	c := testClient(t, tr.handler(func(w http.ResponseWriter, r *http.Request) {
		cap.body = readBody(r)
		cap.contentType = r.Header.Get("Content-Type")
		accepted(w, r)
	}))

	if _, err := c.BiometricKYC.Verify(context.Background(), enrolledKYCParams(), WithEnrolledImage()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	assertClaims(t, tr.lastClaims(t), map[string]string{
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
	var mu sync.Mutex
	headers := map[string]string{}
	c := testClient(t, tr.handler(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		headers[r.URL.Path] = r.Header.Get("SmileID-Token")
		mu.Unlock()
		if r.URL.Path == "/v3/biometric_kyc" {
			accepted(w, r)
			return
		}
		statusOK(w, r)
	}))

	if _, err := c.BiometricKYC.Verify(context.Background(), enrolledKYCParams(), WithEnrolledImage()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if _, err := c.Verifications.Retrieve(context.Background(), "job_x"); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if got := tr.count(); got != 2 {
		t.Fatalf("token minted %d times, want 2", got)
	}
	assertClaims(t, tr.lastClaims(t), map[string]string{})
	if got := headers["/v3/biometric_kyc"]; got != "token-1" {
		t.Errorf("KYC SmileID-Token = %q, want token-1", got)
	}
	if got := headers["/v3/status/job_x"]; got != "token-2" {
		t.Errorf("status SmileID-Token = %q, want token-2", got)
	}
}

func TestBiometricKYCWithoutGrantOmitsIt(t *testing.T) {
	var tr tokenRecorder
	c := testClient(t, tr.handler(accepted))

	if _, err := c.BiometricKYC.Verify(context.Background(), selfieKYCParams()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	assertClaims(t, tr.lastClaims(t), map[string]string{"user_id": "user-1", "product": "biometric_kyc"})
}

func TestTransportRefusesAGrantWithoutAUser(t *testing.T) {
	var calls int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	})
	req := &operations.Request{Method: "POST", Path: "/v3/biometric_kyc", Authenticated: true}

	err := c.transport.Do(withEnrolledImageGrant(context.Background()), req, nil)
	assertValidationError(t, err)
	if err == nil || !strings.Contains(err.Error(), "user_id is required") {
		t.Errorf("error should say user_id is required: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Errorf("%d HTTP calls made, want 0", got)
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
		statusOK(w, r)
	})

	if _, err := c.Verifications.Retrieve(context.Background(), "job_x"); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
}
