package usesmileid

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// enhancedKYCSyncBody is the 200 body of POST /v3/enhanced_kyc/sync for the
// Amberford persona in the submitEnhancedKycSync fixtures.
const enhancedKYCSyncBody = `{
  "job_id": "job_body_01h8x9y2z3a4b5c6d7e8f9g0h1",
  "user_id": "user_body_01h8x9y2z3a4b5c6d7e8f9g0h1",
  "status": "attention",
  "reason": "medium_risk",
  "message": "Verification successful",
  "kyc_receipt": "https://portal.usesmileid.com/api/v3/001/job_result_summary/job_01h8x9y2z3a4b5c6d7e8f9g0h1/generate_v3_pdf/live?token=REDACTED",
  "created_at": "2026-03-10T12:00:00.000Z",
  "partner_params": {"order_id": "ORD-12345"}
}`

// enhancedKYCSyncClearBody is the 200 body for the Clearwater persona.
const enhancedKYCSyncClearBody = `{"job_id":"job_clear_01","user_id":"user_clear_01","status":"clear","reason":null,"message":"Verification successful","kyc_receipt":null}`

func enhancedKYCSyncParams() EnhancedKYCParams {
	return EnhancedKYCParams{
		Country: "NG", IDType: "NIN", IDNumber: "12345678901",
		UserDetails: validUserDetails(),
		Consent:     validConsent(),
	}
}

func TestEnhancedKYCVerifySyncGoldenRequestAndResult(t *testing.T) {
	var cap captured
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/token" {
			// Lower-case names, as an API Gateway may send them.
			w.Header()["smileid-job-id"] = []string{"job_01h8x9y2z3a4b5c6d7e8f9g0h1"}
			w.Header()["user-id"] = []string{"user_01h8x9y2z3a4b5c6d7e8f9g0h1"}
			w.Header()["response-signature"] = []string{"c2lnbmF0dXJl"}
			w.Header()["response-timestamp"] = []string{"2026-03-10T12:00:04.000Z"}
		}
		captureHandler(&cap, http.StatusOK, enhancedKYCSyncBody)(w, r)
	})

	p := enhancedKYCSyncParams()
	p.UserID = String("user_01h8x9y2z3a4b5c6d7e8f9g0h1")
	p.BankCode = String("058")
	p.Operator = String("MTN")
	p.PartnerParams = map[string]string{"order_id": "ORD-12345"}
	p.Metadata = []MetadataEntry{{Name: "channel", Value: "web"}}
	res, err := c.EnhancedKYC.VerifySync(context.Background(), p)
	if err != nil {
		t.Fatalf("VerifySync: %v", err)
	}

	if cap.method != "POST" || cap.path != "/v3/enhanced_kyc/sync" || cap.query != "" {
		t.Errorf("request = %s %s?%s", cap.method, cap.path, cap.query)
	}
	if cap.header.Get("SmileID-Token") == "" {
		t.Error("missing SmileID-Token")
	}
	if cap.header.Get("SmileID-Idempotency-Key") != "" {
		t.Error("enhanced_kyc/sync has no idempotency key")
	}
	if cap.header.Get("SmileID-Partner-ID") != "" {
		t.Error("enhanced_kyc/sync must not send SmileID-Partner-ID")
	}
	if got := cap.header.Get("User-ID"); got != "user_01h8x9y2z3a4b5c6d7e8f9g0h1" {
		t.Errorf("User-ID header = %q", got)
	}
	if !strings.HasPrefix(cap.contentType, "multipart/form-data; boundary=") {
		t.Errorf("Content-Type = %q", cap.contentType)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	if len(parts) != 9 {
		t.Errorf("parts = %d, want 9", len(parts))
	}
	assertScalar(t, parts, "country", "NG")
	assertScalar(t, parts, "id_type", "NIN")
	assertScalar(t, parts, "id_number", "12345678901")
	assertScalar(t, parts, "bank_code", "058")
	assertScalar(t, parts, "operator", "MTN")
	assertJSONPart(t, parts, "partner_params", `{"order_id":"ORD-12345"}`)
	assertJSONPart(t, parts, "metadata", `[{"name":"channel","value":"web"}]`)
	assertNoPart(t, parts, "callback_url")
	assertNoPart(t, parts, "user_id")
	assertBasicKYCCommon(t, &cap, parts)

	if res.JobID != "job_01h8x9y2z3a4b5c6d7e8f9g0h1" {
		t.Errorf("JobID = %q, want the SmileID-Job-ID header", res.JobID)
	}
	if res.UserID != "user_01h8x9y2z3a4b5c6d7e8f9g0h1" {
		t.Errorf("UserID = %q, want the User-ID header", res.UserID)
	}
	if res.Status != "attention" || res.Reason == nil || *res.Reason != "medium_risk" || res.Message != "Verification successful" {
		t.Errorf("status/reason/message = %q %v %q", res.Status, res.Reason, res.Message)
	}
	if res.KYCReceipt == nil || !strings.HasPrefix(*res.KYCReceipt, "https://portal.usesmileid.com/") {
		t.Errorf("kyc_receipt = %v", res.KYCReceipt)
	}
	if res.ResponseSignature != "c2lnbmF0dXJl" || res.ResponseTimestamp != "2026-03-10T12:00:04.000Z" {
		t.Errorf("signature/timestamp = %q %q", res.ResponseSignature, res.ResponseTimestamp)
	}
	// Fields with no typed field are kept in Raw, and Raw holds the body's
	// own job_id, not the header's.
	if res.Raw["created_at"] != "2026-03-10T12:00:00.000Z" || res.Raw["job_id"] != "job_body_01h8x9y2z3a4b5c6d7e8f9g0h1" {
		t.Errorf("raw = %v", res.Raw)
	}
	if pp, ok := res.Raw["partner_params"].(map[string]interface{}); !ok || pp["order_id"] != "ORD-12345" {
		t.Errorf("raw partner_params = %v", res.Raw["partner_params"])
	}
}

// TestEnhancedKYCVerifySyncFallsBackToBodyIDs checks that without the
// SmileID-Job-ID and User-ID headers the result keeps the body's IDs.
func TestEnhancedKYCVerifySyncFallsBackToBodyIDs(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, enhancedKYCSyncClearBody))

	res, err := c.EnhancedKYC.VerifySync(context.Background(), enhancedKYCSyncParams())
	if err != nil {
		t.Fatalf("VerifySync: %v", err)
	}
	if cap.header.Get("User-ID") != "" {
		t.Error("User-ID header sent without a UserID")
	}
	if res.JobID != "job_clear_01" || res.UserID != "user_clear_01" {
		t.Errorf("JobID/UserID = %q %q", res.JobID, res.UserID)
	}
	if res.Status != "clear" || res.Reason != nil || res.KYCReceipt != nil || res.ResponseSignature != "" || res.ResponseTimestamp != "" {
		t.Errorf("result = %+v", res)
	}
	if len(res.Raw) != 6 || res.Raw["status"] != "clear" {
		t.Errorf("raw = %v", res.Raw)
	}
}

// TestEnhancedKYCVerifySyncBlankHeadersFallBackToBody checks that blank
// SmileID-Job-ID and User-ID headers count as absent.
func TestEnhancedKYCVerifySyncBlankHeadersFallBackToBody(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/token" {
			w.Header()["smileid-job-id"] = []string{" "}
			w.Header()["user-id"] = []string{""}
		}
		captureHandler(&captured{}, http.StatusOK, enhancedKYCSyncClearBody)(w, r)
	})

	res, err := c.EnhancedKYC.VerifySync(context.Background(), enhancedKYCSyncParams())
	if err != nil {
		t.Fatalf("VerifySync: %v", err)
	}
	if res.JobID != "job_clear_01" || res.UserID != "user_clear_01" {
		t.Errorf("JobID/UserID = %q %q", res.JobID, res.UserID)
	}
}

// TestEnhancedKYCVerifySyncWithoutIDs checks that a body without job_id or
// user_id and a response without the headers leave both IDs empty.
func TestEnhancedKYCVerifySyncWithoutIDs(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusOK, `{"status":"clear","message":"Verification successful","reason":null}`))

	res, err := c.EnhancedKYC.VerifySync(context.Background(), enhancedKYCSyncParams())
	if err != nil {
		t.Fatalf("VerifySync: %v", err)
	}
	if res.JobID != "" || res.UserID != "" || res.Status != "clear" {
		t.Errorf("result = %+v", res)
	}
}

// TestEnhancedKYCVerifySyncShapeMismatch checks that a body whose typed
// fields have the wrong JSON type is an *UnexpectedResponseError.
func TestEnhancedKYCVerifySyncShapeMismatch(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusOK, `{"status":5}`))

	_, err := c.EnhancedKYC.VerifySync(context.Background(), enhancedKYCSyncParams())
	var unexpected *UnexpectedResponseError
	if !errors.As(err, &unexpected) {
		t.Fatalf("err = %T %v, want *UnexpectedResponseError", err, err)
	}
}

// TestEnhancedKYCVerifySyncTimeoutThenRetrieve follows the README's advice for
// a 408: read the job ID from the error and read the outcome from the status
// endpoint, without resubmitting.
func TestEnhancedKYCVerifySyncTimeoutThenRetrieve(t *testing.T) {
	var syncCalls int
	var statusPath string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/v3/token":
			serveToken(w)
		case r.URL.Path == "/v3/enhanced_kyc/sync":
			syncCalls++
			w.Header().Set("SmileID-Job-ID", "job_ekyc_timeout_01")
			w.WriteHeader(http.StatusRequestTimeout)
			_, _ = w.Write([]byte(`{"status":"Request Timeout","message":"The ID authority did not respond in time. The job is still recorded — retrieve its result from /v3/status/{job_id}."}`))
		case strings.HasPrefix(r.URL.Path, "/v3/status/"):
			statusPath = r.URL.Path
			_, _ = w.Write([]byte(`{"status":"block","job_id":"job_ekyc_timeout_01","user_id":"user_01","message":"Verification failed: Unable to validate ID - Not Found"}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusTeapot)
		}
	})

	_, err := c.EnhancedKYC.VerifySync(context.Background(), enhancedKYCSyncParams())
	var timeout *Error
	if !errors.As(err, &timeout) || timeout.StatusCode != http.StatusRequestTimeout {
		t.Fatalf("err = %T %v, want *Error with status 408", err, err)
	}
	if timeout.JobID != "job_ekyc_timeout_01" {
		t.Fatalf("the 408 error's JobID = %q", timeout.JobID)
	}

	status, err := c.Verifications.Retrieve(context.Background(), timeout.JobID)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if statusPath != "/v3/status/job_ekyc_timeout_01" {
		t.Errorf("status request path = %q", statusPath)
	}
	if status.Status != "block" || status.JobID != "job_ekyc_timeout_01" || !status.IsComplete() {
		t.Errorf("status = %+v", status)
	}
	if syncCalls != 1 {
		t.Errorf("sync calls = %d, want 1", syncCalls)
	}
}

func TestEnhancedKYCVerifySyncNeverSendsDefaultCallback(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, enhancedKYCSyncBody))
	c.cfg.defaultCallbackURL = "https://example.com/default"

	if _, err := c.EnhancedKYC.VerifySync(context.Background(), enhancedKYCSyncParams()); err != nil {
		t.Fatalf("VerifySync: %v", err)
	}
	assertNoPart(t, parseMultipart(t, cap.contentType, cap.body), "callback_url")

	p := enhancedKYCSyncParams()
	p.CallbackURL = String("https://example.com/explicit")
	if _, err := c.EnhancedKYC.VerifySync(context.Background(), p); err != nil {
		t.Fatalf("VerifySync: %v", err)
	}
	assertScalar(t, parseMultipart(t, cap.contentType, cap.body), "callback_url", "https://example.com/explicit")

	if _, err := c.EnhancedKYC.VerifySync(context.Background(), enhancedKYCSyncParams(), WithCallbackURL("https://example.com/option")); err != nil {
		t.Fatalf("VerifySync: %v", err)
	}
	assertScalar(t, parseMultipart(t, cap.contentType, cap.body), "callback_url", "https://example.com/option")
}

func TestEnhancedKYCVerifySyncValidation(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusOK, enhancedKYCSyncBody))
	cases := []struct {
		name   string
		mutate func(*EnhancedKYCParams)
		want   string
	}{
		{"no given names", func(p *EnhancedKYCParams) { p.UserDetails.GivenNames = " " }, "given_names"},
		{"insecure callback", func(p *EnhancedKYCParams) { p.CallbackURL = String("http://example.com/cb") }, "callback_url"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := enhancedKYCSyncParams()
			tt.mutate(&p)
			_, err := c.EnhancedKYC.VerifySync(context.Background(), p)
			assertValidationMessage(t, err, tt.want)
		})
	}
}

func TestEnhancedKYCVerifySyncSurfacesAPIErrors(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusBadRequest, `{"status":"Bad Request","message":"Required field 'country' is missing or invalid."}`))
	_, err := c.EnhancedKYC.VerifySync(context.Background(), enhancedKYCSyncParams())
	var invalid *InvalidRequestError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %T %v, want *InvalidRequestError", err, err)
	}
	if invalid.StatusCode != http.StatusBadRequest || invalid.JobID != "" {
		t.Errorf("status/job ID = %d %q", invalid.StatusCode, invalid.JobID)
	}
}

func TestEnhancedKYCVerifySyncTokenCarriesTheSpecProduct(t *testing.T) {
	var tr tokenRecorder
	c := testClient(t, tr.handler(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(enhancedKYCSyncBody))
	}))
	if _, err := c.EnhancedKYC.VerifySync(context.Background(), enhancedKYCSyncParams()); err != nil {
		t.Fatalf("VerifySync: %v", err)
	}
	assertClaims(t, tr.lastClaims(t), map[string]string{"product": "enhanced_kyc"})
}
