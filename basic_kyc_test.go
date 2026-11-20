package usesmileid

import (
	"context"
	"net/http"
	"testing"
)

const basicKYCAcceptedBody = `{"status":"accepted","message":"Request accepted and queued for processing.","job_id":"job_01h8x9y2z3a4b5c6d7e8f9g0h1","user_id":"user_01h8x9y2z3a4b5c6d7e8f9g0h1","created_at":"2026-03-10T12:00:00.000Z"}`

const basicKYCResultBody = `{
  "status": "attention",
  "message": "Verification completed: partial match.",
  "reason": "partial_match",
  "created_at": "2026-03-10T12:00:00.000Z",
  "completed_at": "2026-03-10T12:00:04.000Z",
  "product": "basic_kyc",
  "partner_params": {"order_id": "ORD-12345"},
  "match_results": {
    "overall": "partial_match",
    "names": "partial_match",
    "first_name": "exact_match",
    "last_name": "transposed",
    "other_names": "not_provided",
    "date_of_birth": "no_match",
    "gender": "not_provided",
    "phone_number": "exact_match",
    "secondary_id_number": "not_provided"
  },
  "user_provided_info": {"given_names": "John", "last_name": "Doe", "email": "john@example.com", "phone_number": "+2348012345678", "id_number": "12345678901"},
  "consent": {"granted": true},
  "antifraud": {"risk_score": 0.1},
  "device_signals": {"ip": "203.0.113.7"},
  "kyc_receipt": "https://portal.usesmileid.com/api/v3/001/job_result_summary/job_01/generate_v3_pdf/live?token=REDACTED"
}`

const basicKYCErroredBody = `{"status":"error","message":"Verification failed.","reason":"provider_error","created_at":"2026-03-10T12:00:00.000Z","product":"basic_kyc","partner_params":null,"match_results":null,"user_provided_info":{"given_names":"John","last_name":"Doe","email":null,"phone_number":null,"id_number":null},"consent":{},"antifraud":null,"device_signals":null,"kyc_receipt":null}`

func basicKYCParams() BasicKYCParams {
	return BasicKYCParams{
		Country: "NG", IDType: "NIN", IDNumber: "12345678901",
		UserDetails: validUserDetails(),
		Consent:     validConsent(),
	}
}

func TestBasicKYCVerifyGoldenRequest(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, basicKYCAcceptedBody))

	p := basicKYCParams()
	p.CallbackURL = String("https://example.com/callback")
	p.BankCode = String("044")
	p.Operator = String("MTN")
	p.PartnerParams = map[string]string{"order_id": "ORD-12345"}
	p.Metadata = []MetadataEntry{{Name: "k", Value: "v"}}
	resp, err := c.BasicKYC.Verify(context.Background(), p)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	if cap.method != "POST" || cap.path != "/v3/basic_kyc" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.header.Get("SmileID-Token") == "" {
		t.Error("missing SmileID-Token")
	}
	if cap.header.Get("SmileID-Partner-ID") != "" || cap.header.Get("User-ID") != "" {
		t.Error("basic_kyc must send neither SmileID-Partner-ID nor User-ID")
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "country", "NG")
	assertScalar(t, parts, "id_type", "NIN")
	assertScalar(t, parts, "id_number", "12345678901")
	assertScalar(t, parts, "callback_url", "https://example.com/callback")
	assertScalar(t, parts, "bank_code", "044")
	assertScalar(t, parts, "operator", "MTN")
	assertJSONPart(t, parts, "partner_params", `{"order_id":"ORD-12345"}`)
	assertJSONPart(t, parts, "metadata", `[{"name":"k","value":"v"}]`)
	onePart(t, parts, "user_details")
	onePart(t, parts, "consent")

	if resp.Status != "accepted" || resp.JobID != "job_01h8x9y2z3a4b5c6d7e8f9g0h1" {
		t.Errorf("response = %+v", resp)
	}
}

func TestBasicKYCVerifyMinimalRequest(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, basicKYCAcceptedBody))

	if _, err := c.BasicKYC.Verify(context.Background(), basicKYCParams(), WithCallbackURL("https://example.com/stored")); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "callback_url", "https://example.com/stored")
	for _, name := range []string{"bank_code", "operator", "partner_params", "metadata"} {
		assertNoPart(t, parts, name)
	}
}

func TestBasicKYCVerifySyncGoldenRequestAndResult(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, basicKYCResultBody))

	res, err := c.BasicKYC.VerifySync(context.Background(), basicKYCParams())
	if err != nil {
		t.Fatalf("VerifySync: %v", err)
	}

	if cap.method != "POST" || cap.path != "/v3/basic_kyc/sync" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.header.Get("SmileID-Token") == "" {
		t.Error("missing SmileID-Token")
	}
	if cap.header.Get("SmileID-Idempotency-Key") != "" {
		t.Error("basic_kyc/sync has no idempotency key")
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "country", "NG")
	assertScalar(t, parts, "id_number", "12345678901")
	assertNoPart(t, parts, "callback_url")

	if res.Status != "attention" || res.Reason == nil || *res.Reason != "partial_match" || res.Product != "basic_kyc" {
		t.Errorf("status/reason/product = %q %v %q", res.Status, res.Reason, res.Product)
	}
	if res.CreatedAt != "2026-03-10T12:00:00.000Z" || res.CompletedAt != "2026-03-10T12:00:04.000Z" {
		t.Errorf("timestamps = %q %q", res.CreatedAt, res.CompletedAt)
	}
	if res.PartnerParams["order_id"] != "ORD-12345" {
		t.Errorf("partner_params = %v", res.PartnerParams)
	}
	want := BasicKYCMatchResults{
		Overall: "partial_match", Names: "partial_match", FirstName: "exact_match", LastName: "transposed",
		OtherNames: "not_provided", DateOfBirth: "no_match", Gender: "not_provided",
		PhoneNumber: "exact_match", SecondaryIDNumber: "not_provided",
	}
	if res.MatchResults == nil || *res.MatchResults != want {
		t.Errorf("match_results = %+v", res.MatchResults)
	}
	info := res.UserProvidedInfo
	if *info.GivenNames != "John" || *info.LastName != "Doe" || *info.Email != "john@example.com" || *info.PhoneNumber != "+2348012345678" || *info.IDNumber != "12345678901" {
		t.Errorf("user_provided_info = %+v", info)
	}
	if res.Consent["granted"] != true || res.Antifraud["risk_score"] != 0.1 || res.DeviceSignals["ip"] != "203.0.113.7" {
		t.Errorf("consent/antifraud/device_signals = %v %v %v", res.Consent, res.Antifraud, res.DeviceSignals)
	}
	if res.KYCReceipt == nil || *res.KYCReceipt == "" {
		t.Errorf("kyc_receipt = %v", res.KYCReceipt)
	}
}

func TestBasicKYCVerifySyncErroredResult(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, basicKYCErroredBody))

	res, err := c.BasicKYC.VerifySync(context.Background(), basicKYCParams(), WithCallbackURL("https://example.com/stored"))
	if err != nil {
		t.Fatalf("VerifySync: %v", err)
	}
	assertScalar(t, parseMultipart(t, cap.contentType, cap.body), "callback_url", "https://example.com/stored")
	if res.Status != "error" || res.MatchResults != nil || res.PartnerParams != nil || res.KYCReceipt != nil || res.CompletedAt != "" {
		t.Errorf("result = %+v", res)
	}
	if res.UserProvidedInfo.Email != nil || *res.UserProvidedInfo.GivenNames != "John" {
		t.Errorf("user_provided_info = %+v", res.UserProvidedInfo)
	}
}

func TestBasicKYCVerifySyncIsNotRetried(t *testing.T) {
	for _, status := range []int{http.StatusRequestTimeout, http.StatusFailedDependency, http.StatusInternalServerError} {
		calls := 0
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v3/token" {
				serveToken(w)
				return
			}
			calls++
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("SmileID-Job-ID", "job_01h8x9y2z3a4b5c6d7e8f9g0h1")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"status":"Request Timeout","message":"The ID authority did not respond in time."}`))
		})
		c.cfg.maxRetries = 2

		_, err := c.BasicKYC.VerifySync(context.Background(), basicKYCParams())
		if err == nil {
			t.Errorf("status %d: want error", status)
		}
		if calls != 1 {
			t.Errorf("status %d: calls = %d, want 1 (a retry would bill a second lookup)", status, calls)
		}
	}
}

func TestBasicKYCValidation(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusAccepted, basicKYCAcceptedBody))
	ctx := context.Background()
	cases := []struct {
		name   string
		mutate func(*BasicKYCParams)
		want   string
	}{
		{"no given names", func(p *BasicKYCParams) { p.UserDetails.GivenNames = " " }, "given_names"},
		{"no contact", func(p *BasicKYCParams) { p.UserDetails.Email = nil }, "email or phone_number"},
		{"insecure callback", func(p *BasicKYCParams) { p.CallbackURL = String("http://example.com/cb") }, "callback_url"},
	}
	for _, tt := range cases {
		t.Run("verify "+tt.name, func(t *testing.T) {
			p := basicKYCParams()
			tt.mutate(&p)
			_, err := c.BasicKYC.Verify(ctx, p)
			assertValidationMessage(t, err, tt.want)
		})
		t.Run("sync "+tt.name, func(t *testing.T) {
			p := basicKYCParams()
			tt.mutate(&p)
			_, err := c.BasicKYC.VerifySync(ctx, p)
			assertValidationMessage(t, err, tt.want)
		})
	}
}

func TestBasicKYCOperationsSurfaceAPIErrors(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusBadRequest, `{"status":"Bad Request","message":"bad"}`))
	if _, err := c.BasicKYC.Verify(context.Background(), basicKYCParams()); err == nil {
		t.Error("Verify: want error")
	}
}

func TestBasicKYCTokensCarryTheSpecProduct(t *testing.T) {
	calls := map[string]func(c *Client) error{
		"verify": func(c *Client) error {
			_, err := c.BasicKYC.Verify(context.Background(), basicKYCParams())
			return err
		},
		"verify_sync": func(c *Client) error {
			_, err := c.BasicKYC.VerifySync(context.Background(), basicKYCParams())
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			var tr tokenRecorder
			c := testClient(t, tr.handler(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(basicKYCResultBody))
			}))
			if err := call(c); err != nil {
				t.Fatalf("call: %v", err)
			}
			assertClaims(t, tr.lastClaims(t), map[string]string{"product": "basic_kyc"})
		})
	}
}
