package usesmileid

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

const (
	phoneAcceptedBody   = `{"status":"accepted","message":"Job submitted successfully","job_id":"job_phone_01","user_id":"user_phone_01","created_at":"2026-03-10T12:00:00.000Z"}`
	addressAcceptedBody = `{"status":"accepted","message":"Job submitted successfully","job_id":"job_addr_01","user_id":"user_addr_01"}`
)

func phoneParams() PhoneVerificationParams {
	return PhoneVerificationParams{
		Country:     "NG",
		PhoneNumber: "+2348012345678",
		UserDetails: validUserDetails(),
		Consent:     validConsent(),
	}
}

func addressParams() AddressVerificationParams {
	return AddressVerificationParams{
		Address:         "1 Marina Road, Lagos",
		Country:         "NG",
		UtilityNumber:   String("04123456789"),
		UtilityProvider: String("ikeja-electric"),
		UtilityType:     String("PrePaid"),
		UserDetails:     validUserDetails(),
		Consent:         validConsent(),
	}
}

func TestPhoneVerifyGoldenRequest(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, phoneAcceptedBody))

	p := phoneParams()
	p.Country = "GH"
	p.PhoneNumber = "+233201234567"
	p.Operator = String("MTN")
	p.IDNumber = String("GHA-123456789-0")
	p.UserID = String("user_phone_01")
	p.CallbackURL = String("https://example.com/callback")
	p.PartnerParams = map[string]string{"order_id": "ORD-1"}
	p.Metadata = []MetadataEntry{{Name: "k", Value: "v"}}
	resp, err := c.Phone.Verify(context.Background(), p)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !resp.IsAccepted() || resp.JobID != "job_phone_01" || resp.UserID != "user_phone_01" || resp.CreatedAt != "2026-03-10T12:00:00.000Z" {
		t.Errorf("response = %+v", resp)
	}

	if cap.method != "POST" || cap.path != "/v3/phone" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.header.Get("SmileID-Token") == "" || cap.header.Get("SmileID-Partner-ID") != "" || cap.header.Get("User-ID") != "" {
		t.Errorf("headers = %v", cap.header)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "country", "GH")
	assertScalar(t, parts, "phone_number", "+233201234567")
	assertScalar(t, parts, "operator", "MTN")
	assertScalar(t, parts, "id_number", "GHA-123456789-0")
	assertScalar(t, parts, "user_id", "user_phone_01")
	assertScalar(t, parts, "callback_url", "https://example.com/callback")
	assertJSONPart(t, parts, "partner_params", `{"order_id":"ORD-1"}`)
	assertJSONPart(t, parts, "metadata", `[{"name":"k","value":"v"}]`)
	onePart(t, parts, "user_details")
	onePart(t, parts, "consent")
}

func TestPhoneVerifyRequiredFieldsAndDefaultCallback(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, phoneAcceptedBody))
	c.cfg.defaultCallbackURL = "https://example.com/default"

	if _, err := c.Phone.Verify(context.Background(), phoneParams()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "callback_url", "https://example.com/default")
	for _, name := range []string{"operator", "id_number", "user_id", "partner_params", "metadata"} {
		assertNoPart(t, parts, name)
	}

	if _, err := c.Phone.Verify(context.Background(), phoneParams(), WithCallbackURL("http://example.com/insecure")); err == nil {
		t.Error("an insecure callback URL was accepted")
	}
}

func TestPhoneVerifyValidation(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusAccepted, phoneAcceptedBody))
	cases := []struct {
		name   string
		mutate func(*PhoneVerificationParams)
		want   string
	}{
		{"lowercase country", func(p *PhoneVerificationParams) { p.Country = "ng" }, "country"},
		{"not E.164", func(p *PhoneVerificationParams) { p.PhoneNumber = "08012345678" }, "phone_number"},
		{"unknown operator", func(p *PhoneVerificationParams) { p.Operator = String("GLO") }, "operator"},
		{"Ghana without operator", func(p *PhoneVerificationParams) { p.Country = "GH" }, "operator is required"},
		{"blank id number", func(p *PhoneVerificationParams) { p.IDNumber = String(" ") }, "id_number"},
		{"Kenya without id number", func(p *PhoneVerificationParams) { p.Country = "KE" }, "id_number is required"},
		{"no given names", func(p *PhoneVerificationParams) { p.UserDetails.GivenNames = "" }, "given_names"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := phoneParams()
			tt.mutate(&p)
			_, err := c.Phone.Verify(context.Background(), p)
			assertValidationMessage(t, err, tt.want)
		})
	}
}

func TestPhoneVerifySurfacesAPIErrors(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusTooManyRequests, `{"status":"Too Many Requests","message":"Slow down."}`))
	_, err := c.Phone.Verify(context.Background(), phoneParams())
	var limited *RateLimitError
	if !errors.As(err, &limited) {
		t.Fatalf("err = %T %v, want *RateLimitError", err, err)
	}
}

func TestAddressVerifyGoldenRequest(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, addressAcceptedBody))

	p := addressParams()
	p.IDNumber = String("12345")
	p.UserID = String("user_addr_01")
	p.CallbackURL = String("https://example.com/callback")
	p.PartnerParams = map[string]string{"order_id": "ORD-1"}
	p.Metadata = []MetadataEntry{{Name: "k", Value: "v"}}
	resp, err := c.Address.Verify(context.Background(), p)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !resp.IsAccepted() || resp.JobID != "job_addr_01" || resp.UserID != "user_addr_01" || resp.CreatedAt != "" {
		t.Errorf("response = %+v", resp)
	}

	if cap.method != "POST" || cap.path != "/v3/address" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.header.Get("SmileID-Token") == "" || cap.header.Get("SmileID-Partner-ID") != "" {
		t.Errorf("headers = %v", cap.header)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "address", "1 Marina Road, Lagos")
	assertScalar(t, parts, "country", "NG")
	assertScalar(t, parts, "id_number", "12345")
	assertScalar(t, parts, "utility_number", "04123456789")
	assertScalar(t, parts, "utility_provider", "ikeja-electric")
	assertScalar(t, parts, "utility_type", "PrePaid")
	assertScalar(t, parts, "user_id", "user_addr_01")
	assertScalar(t, parts, "callback_url", "https://example.com/callback")
	assertJSONPart(t, parts, "partner_params", `{"order_id":"ORD-1"}`)
	assertJSONPart(t, parts, "metadata", `[{"name":"k","value":"v"}]`)
	onePart(t, parts, "user_details")
	onePart(t, parts, "consent")
}

func TestAddressVerifyRequiredFieldsAndDefaultCallback(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, addressAcceptedBody))
	c.cfg.defaultCallbackURL = "https://example.com/default"

	p := AddressVerificationParams{
		Address: "2 Main Street, Cape Town", Country: "ZA", IDNumber: String("8001015009087"),
		UserDetails: validUserDetails(), Consent: validConsent(),
	}
	if _, err := c.Address.Verify(context.Background(), p); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "callback_url", "https://example.com/default")
	for _, name := range []string{"utility_number", "utility_provider", "utility_type", "user_id"} {
		assertNoPart(t, parts, name)
	}

	if _, err := c.Address.Verify(context.Background(), p, WithCallbackURL("http://example.com/insecure")); err == nil {
		t.Error("an insecure callback URL was accepted")
	}
}

func TestAddressVerifyValidation(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusAccepted, addressAcceptedBody))
	long := make([]byte, 513)
	for i := range long {
		long[i] = 'a'
	}
	cases := []struct {
		name   string
		mutate func(*AddressVerificationParams)
		want   string
	}{
		{"blank address", func(p *AddressVerificationParams) { p.Address = " " }, "address"},
		{"address too long", func(p *AddressVerificationParams) { p.Address = string(long) }, "address"},
		{"unsupported country", func(p *AddressVerificationParams) { p.Country = "KE" }, "country"},
		{"no identifier", func(p *AddressVerificationParams) { p.UtilityNumber = nil }, "id_number or utility_number"},
		{"bad utility type", func(p *AddressVerificationParams) { p.UtilityType = String("Other") }, "utility_type"},
		{"South Africa without id number", func(p *AddressVerificationParams) { p.Country = "ZA" }, "id_number is required for South Africa"},
		{"Nigeria without provider", func(p *AddressVerificationParams) { p.UtilityProvider = nil }, "required for Nigeria"},
		{"Nigeria with blank provider", func(p *AddressVerificationParams) { p.UtilityProvider = String(" ") }, "required for Nigeria"},
		{"Nigeria without utility type", func(p *AddressVerificationParams) { p.UtilityType = nil }, "required for Nigeria"},
		{"no given names", func(p *AddressVerificationParams) { p.UserDetails.GivenNames = "" }, "given_names"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := addressParams()
			tt.mutate(&p)
			_, err := c.Address.Verify(context.Background(), p)
			assertValidationMessage(t, err, tt.want)
		})
	}
}

func TestAddressVerifySurfacesAPIErrors(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusPaymentRequired, `{"status":"Payment Required","message":"Insufficient balance."}`))
	_, err := c.Address.Verify(context.Background(), addressParams())
	var payment *PaymentRequiredError
	if !errors.As(err, &payment) {
		t.Fatalf("err = %T %v, want *PaymentRequiredError", err, err)
	}
}

func TestPhoneAndAddressTokensCarryTheSpecProduct(t *testing.T) {
	calls := []struct {
		name   string
		claims map[string]string
		call   func(c *Client) error
	}{
		{"phone", map[string]string{"user_id": "user-1", "product": "phone_number_verification"}, func(c *Client) error {
			p := phoneParams()
			p.UserID = String("user-1")
			_, err := c.Phone.Verify(context.Background(), p)
			return err
		}},
		{"address", map[string]string{"product": "address_verification"}, func(c *Client) error {
			_, err := c.Address.Verify(context.Background(), addressParams())
			return err
		}},
	}
	for _, tc := range calls {
		t.Run(tc.name, func(t *testing.T) {
			var tr tokenRecorder
			c := testClient(t, tr.handler(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(addressAcceptedBody))
			}))
			if err := tc.call(c); err != nil {
				t.Fatalf("call: %v", err)
			}
			assertClaims(t, tr.lastClaims(t), tc.claims)
		})
	}
}
