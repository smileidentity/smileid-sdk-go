package usesmileid

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

const (
	smileLinkCreatedBody = `{"link":"https://links.usesmileid.com/1234/abc","message":"Smile Link created","ref_id":"1f0d3f0e-6f2a-4a39-9d63-7c2f5b6c9a10","user_id":null}`

	smileLinkBody = `{
  "ref_id": "1f0d3f0e-6f2a-4a39-9d63-7c2f5b6c9a10",
  "link": "https://links.usesmileid.com/1234/abc",
  "name": "Onboarding",
  "company_name": "Acme",
  "description": "KYC for new customers",
  "id_types": [{"country": "NG", "id_type": "NIN", "verification_method": "enhanced_kyc"}],
  "is_single_use": true,
  "callback_url": "https://example.com/callback",
  "notice_privacy_policy_url": "https://example.com/privacy",
  "logo_url": "https://example.com/logo.png",
  "redirect_url": null,
  "theme_color": "#0a0a0a",
  "expires_at": "2027-01-01T00:00:00.000Z",
  "disabled_at": null,
  "partner_params": {"order_id": "ORD-1"},
  "user_id": "user_01",
  "token_provided_fields": ["consent", "country"],
  "token_provided_values": {"country": "NG", "consent": {"granted": true, "granted_at": "2026-03-06T12:00:00.000Z", "notice_language": "EN", "notice_privacy_policy_url": "https://example.com/privacy"}},
  "created_at": "2026-03-10T12:00:00.000Z",
  "updated_at": "2026-03-11T12:00:00.000Z"
}`

	smileLinkListBody = `{
  "links": [
    {"ref_id": "r1", "link": "https://links.usesmileid.com/1234/r1", "name": "One", "redirect_url": null, "expires_at": null, "partner_params": null, "user_id": null,
     "clear": 3, "block": 1, "attention": 2, "error": 0, "successful_jobs": 5, "failed_jobs": 1, "total_jobs": 6},
    {"ref_id": "r2", "link": "https://links.usesmileid.com/1234/r2", "name": "Two",
     "clear": 0, "block": 0, "attention": 0, "error": 0, "successful_jobs": 0, "failed_jobs": 0, "total_jobs": 0}
  ],
  "meta": {"current_page": 2, "limit": 20, "total_count": 22, "total_pages": 2}
}`
)

func smileLinkIDTypes() []SmileLinkIDType {
	return []SmileLinkIDType{{Country: "NG", IDType: "NIN", VerificationMethod: "enhanced_kyc"}}
}

func createSmileLinkParams() CreateSmileLinkParams {
	return CreateSmileLinkParams{
		IDTypes:                smileLinkIDTypes(),
		Name:                   "Onboarding",
		NoticePrivacyPolicyURL: "https://example.com/privacy",
	}
}

func TestSmileLinksCreateGoldenRequest(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusCreated, smileLinkCreatedBody))

	p := createSmileLinkParams()
	p.CompanyName = String("Acme")
	p.Description = String("KYC for new customers")
	p.IsSingleUse = true
	p.CallbackURL = String("https://example.com/callback")
	p.ExpiresAt = String("2027-01-01T00:00:00Z")
	p.LogoURL = String("https://example.com/logo.png")
	p.RedirectURL = String("https://example.com/done")
	p.ThemeColor = String("#0a0a0a")
	p.PartnerParams = map[string]interface{}{"order_id": "ORD-1"}
	p.UserID = String("user_01")
	res, err := c.SmileLinks.Create(context.Background(), p)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if res.Link != "https://links.usesmileid.com/1234/abc" || res.RefID != "1f0d3f0e-6f2a-4a39-9d63-7c2f5b6c9a10" || res.Message != "Smile Link created" || res.UserID != nil {
		t.Errorf("result = %+v", res)
	}

	if cap.method != "POST" || cap.path != "/v3/smile_links" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.header.Get("SmileID-Token") == "" || cap.header.Get("SmileID-Partner-ID") != "" || cap.header.Get("User-ID") != "" {
		t.Errorf("headers = %v", cap.header)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertJSONPart(t, parts, "id_types", `[{"country":"NG","id_type":"NIN","verification_method":"enhanced_kyc"}]`)
	assertScalar(t, parts, "name", "Onboarding")
	assertScalar(t, parts, "company_name", "Acme")
	assertScalar(t, parts, "description", "KYC for new customers")
	assertScalar(t, parts, "notice_privacy_policy_url", "https://example.com/privacy")
	assertScalar(t, parts, "is_single_use", "true")
	assertScalar(t, parts, "callback_url", "https://example.com/callback")
	assertScalar(t, parts, "expires_at", "2027-01-01T00:00:00Z")
	assertScalar(t, parts, "logo_url", "https://example.com/logo.png")
	assertScalar(t, parts, "redirect_url", "https://example.com/done")
	assertScalar(t, parts, "theme_color", "#0a0a0a")
	assertJSONPart(t, parts, "partner_params", `{"order_id":"ORD-1"}`)
	assertScalar(t, parts, "user_id", "user_01")
}

func TestSmileLinksCreateRequiredFieldsAndCallbacks(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusCreated, smileLinkCreatedBody))

	if _, err := c.SmileLinks.Create(context.Background(), createSmileLinkParams()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "is_single_use", "false")
	for _, name := range []string{"company_name", "description", "callback_url", "expires_at", "logo_url", "redirect_url", "theme_color", "partner_params", "user_id"} {
		assertNoPart(t, parts, name)
	}

	// The client's default callback URL is used when the call sets none, and
	// WithCallbackURL replaces the one in the params.
	c.cfg.defaultCallbackURL = "https://example.com/default"
	if _, err := c.SmileLinks.Create(context.Background(), createSmileLinkParams()); err != nil {
		t.Fatalf("Create: %v", err)
	}
	assertScalar(t, parseMultipart(t, cap.contentType, cap.body), "callback_url", "https://example.com/default")
	if _, err := c.SmileLinks.Create(context.Background(), createSmileLinkParams(), WithCallbackURL("https://example.com/option")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	assertScalar(t, parseMultipart(t, cap.contentType, cap.body), "callback_url", "https://example.com/option")
}

func TestSmileLinksCreateValidation(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusCreated, smileLinkCreatedBody))
	cases := []struct {
		name   string
		mutate func(*CreateSmileLinkParams)
		want   string
	}{
		{"no id types", func(p *CreateSmileLinkParams) { p.IDTypes = nil }, "id_types"},
		{"incomplete id type", func(p *CreateSmileLinkParams) { p.IDTypes[0].VerificationMethod = " " }, "each id_types entry"},
		{"blank name", func(p *CreateSmileLinkParams) { p.Name = "" }, "name"},
		{"insecure privacy policy", func(p *CreateSmileLinkParams) { p.NoticePrivacyPolicyURL = "http://example.com" }, "notice_privacy_policy_url"},
		{"long company name", func(p *CreateSmileLinkParams) { p.CompanyName = String(strings.Repeat("a", 256)) }, "company_name"},
		{"long theme color", func(p *CreateSmileLinkParams) { p.ThemeColor = String(strings.Repeat("a", 33)) }, "theme_color"},
		{"bad expiry", func(p *CreateSmileLinkParams) { p.ExpiresAt = String("tomorrow") }, "expires_at"},
		{"insecure logo", func(p *CreateSmileLinkParams) { p.LogoURL = String("http://example.com/l.png") }, "logo_url"},
		{"insecure redirect", func(p *CreateSmileLinkParams) { p.RedirectURL = String("http://example.com") }, "redirect_url"},
		{"padded user id", func(p *CreateSmileLinkParams) { p.UserID = String(" u ") }, "user_id"},
		{"insecure callback", func(p *CreateSmileLinkParams) { p.CallbackURL = String("http://example.com/cb") }, "callback_url"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := createSmileLinkParams()
			tt.mutate(&p)
			_, err := c.SmileLinks.Create(context.Background(), p)
			assertValidationMessage(t, err, tt.want)
		})
	}
}

func TestSmileLinksCreateConflictIsNotRetried(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		calls++
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"status":"Conflict","message":"user_id is already in use by another link."}`))
	})
	p := createSmileLinkParams()
	p.UserID = String("user_01")
	_, err := c.SmileLinks.Create(context.Background(), p)
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("err = %T %v, want *ConflictError", err, err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestSmileLinksGetParsesLink(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, smileLinkBody))

	link, err := c.SmileLinks.Get(context.Background(), "1f0d3f0e-6f2a-4a39-9d63-7c2f5b6c9a10")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if cap.method != "GET" || cap.path != "/v3/smile_links/1f0d3f0e-6f2a-4a39-9d63-7c2f5b6c9a10" || len(cap.body) != 0 {
		t.Errorf("request = %s %s body %q", cap.method, cap.path, cap.body)
	}
	if link.RefID != "1f0d3f0e-6f2a-4a39-9d63-7c2f5b6c9a10" || link.Link != "https://links.usesmileid.com/1234/abc" ||
		link.Name != "Onboarding" || link.CompanyName != "Acme" || link.Description != "KYC for new customers" ||
		!link.IsSingleUse || link.CallbackURL != "https://example.com/callback" ||
		link.NoticePrivacyPolicyURL != "https://example.com/privacy" || link.LogoURL != "https://example.com/logo.png" ||
		link.ThemeColor != "#0a0a0a" || link.CreatedAt != "2026-03-10T12:00:00.000Z" || link.UpdatedAt != "2026-03-11T12:00:00.000Z" {
		t.Errorf("link = %+v", link)
	}
	if len(link.IDTypes) != 1 || link.IDTypes[0] != (SmileLinkIDType{Country: "NG", IDType: "NIN", VerificationMethod: "enhanced_kyc"}) {
		t.Errorf("id_types = %+v", link.IDTypes)
	}
	if link.RedirectURL != nil || link.DisabledAt != nil {
		t.Errorf("null fields = %v %v", link.RedirectURL, link.DisabledAt)
	}
	if link.ExpiresAt == nil || *link.ExpiresAt != "2027-01-01T00:00:00.000Z" || link.UserID == nil || *link.UserID != "user_01" {
		t.Errorf("expires_at/user_id = %v %v", link.ExpiresAt, link.UserID)
	}
	if link.PartnerParams["order_id"] != "ORD-1" || len(link.TokenProvidedFields) != 2 || link.TokenProvidedFields[1] != "country" {
		t.Errorf("partner params/token fields = %v %v", link.PartnerParams, link.TokenProvidedFields)
	}
	v := link.TokenProvidedValues
	if v.Country != "NG" || v.IDType != "" || v.Consent == nil || !v.Consent.Granted || v.Consent.NoticeLanguage != "EN" ||
		v.Consent.GrantedAt != "2026-03-06T12:00:00.000Z" || v.Consent.NoticePrivacyPolicyURL != "https://example.com/privacy" {
		t.Errorf("token values = %+v", v)
	}
}

func TestSmileLinksGetEscapesRefIDAndRejectsBlank(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, smileLinkBody))
	if _, err := c.SmileLinks.Get(context.Background(), "a b"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if cap.path != "/v3/smile_links/a b" {
		t.Errorf("path = %q", cap.path)
	}
	_, err := c.SmileLinks.Get(context.Background(), " ")
	assertValidationMessage(t, err, "ref_id")
}

func TestSmileLinksGetNotFound(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusNotFound, `{"status":"Not Found","message":"No link with this refId exists."}`))
	_, err := c.SmileLinks.Get(context.Background(), "missing")
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %T %v, want *NotFoundError", err, err)
	}
}

func TestSmileLinksListParsesLinksAndStats(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, smileLinkListBody))

	res, err := c.SmileLinks.List(context.Background(), ListSmileLinksParams{Limit: Ptr(20), Page: Ptr(2), SingleUse: Bool(true)})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if cap.method != "GET" || cap.path != "/v3/smile_links" || cap.query != "limit=20&page=2&single_use=true" {
		t.Errorf("request = %s %s ?%s", cap.method, cap.path, cap.query)
	}
	if res.Meta != (SmileLinkListMeta{CurrentPage: 2, Limit: 20, TotalCount: 22, TotalPages: 2}) {
		t.Errorf("meta = %+v", res.Meta)
	}
	if len(res.Links) != 2 {
		t.Fatalf("links = %d", len(res.Links))
	}
	first := res.Links[0]
	if first.RefID != "r1" || first.Name != "One" || first.RedirectURL != nil || first.ExpiresAt != nil || first.UserID != nil || first.PartnerParams != nil {
		t.Errorf("first link = %+v", first.SmileLink)
	}
	want := SmileLinkJobStats{Clear: 3, Block: 1, Attention: 2, Error: 0, SuccessfulJobs: 5, FailedJobs: 1, TotalJobs: 6}
	if first.SmileLinkJobStats != want {
		t.Errorf("stats = %+v", first.SmileLinkJobStats)
	}
	if res.Links[1].RefID != "r2" || res.Links[1].TotalJobs != 0 {
		t.Errorf("second link = %+v", res.Links[1])
	}
}

func TestSmileLinksListWithoutQuery(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, `{"links":[],"meta":{"current_page":1,"limit":20,"total_count":0,"total_pages":0}}`))
	res, err := c.SmileLinks.List(context.Background(), ListSmileLinksParams{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if cap.query != "" || len(res.Links) != 0 || res.Meta.Limit != 20 {
		t.Errorf("query = %q, result = %+v", cap.query, res)
	}
}

func TestSmileLinksListValidation(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusOK, smileLinkListBody))
	for name, tt := range map[string]struct {
		p    ListSmileLinksParams
		want string
	}{
		"limit too small": {ListSmileLinksParams{Limit: Ptr(0)}, "limit"},
		"limit too large": {ListSmileLinksParams{Limit: Ptr(101)}, "limit"},
		"page too small":  {ListSmileLinksParams{Page: Ptr(0)}, "page"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := c.SmileLinks.List(context.Background(), tt.p)
			assertValidationMessage(t, err, tt.want)
		})
	}
}

func TestSmileLinksUpdateGoldenRequest(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, smileLinkBody))

	link, err := c.SmileLinks.Update(context.Background(), "ref-1", UpdateSmileLinkParams{IDTypes: smileLinkIDTypes(), IsDisabled: Bool(true)})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if link.Name != "Onboarding" || len(link.IDTypes) != 1 {
		t.Errorf("link = %+v", link)
	}
	if cap.method != "PUT" || cap.path != "/v3/smile_links/ref-1" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertJSONPart(t, parts, "id_types", `[{"country":"NG","id_type":"NIN","verification_method":"enhanced_kyc"}]`)
	assertScalar(t, parts, "is_disabled", "true")

	if _, err := c.SmileLinks.Update(context.Background(), "ref-1", UpdateSmileLinkParams{IsDisabled: Bool(false)}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	parts = parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "is_disabled", "false")
	assertNoPart(t, parts, "id_types")

	if _, err := c.SmileLinks.Update(context.Background(), "ref-1", UpdateSmileLinkParams{IDTypes: smileLinkIDTypes()}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	parts = parseMultipart(t, cap.contentType, cap.body)
	assertNoPart(t, parts, "is_disabled")
}

func TestSmileLinksUpdateValidation(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusOK, smileLinkBody))
	cases := []struct {
		name  string
		refID string
		p     UpdateSmileLinkParams
		want  string
	}{
		{"blank ref id", " ", UpdateSmileLinkParams{IsDisabled: Bool(true)}, "ref_id"},
		{"nothing to update", "ref-1", UpdateSmileLinkParams{}, "id_types or is_disabled"},
		{"empty id types", "ref-1", UpdateSmileLinkParams{IDTypes: []SmileLinkIDType{}}, "id_types"},
		{"too many id types", "ref-1", UpdateSmileLinkParams{IDTypes: make([]SmileLinkIDType, 51)}, "at most 50"},
		{"incomplete id type", "ref-1", UpdateSmileLinkParams{IDTypes: []SmileLinkIDType{{Country: "NG"}}}, "each id_types entry"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.SmileLinks.Update(context.Background(), tt.refID, tt.p)
			assertValidationMessage(t, err, tt.want)
		})
	}
}

func TestSmileLinksUpdateSurfacesAPIErrors(t *testing.T) {
	for status, check := range map[int]func(error) bool{
		http.StatusConflict: func(err error) bool { var e *ConflictError; return errors.As(err, &e) },
		http.StatusGone:     func(err error) bool { var e *Error; return errors.As(err, &e) && e.StatusCode == http.StatusGone },
	} {
		c := testClient(t, captureHandler(&captured{}, status, `{"status":"error","message":"nope"}`))
		_, err := c.SmileLinks.Update(context.Background(), "ref-1", UpdateSmileLinkParams{IsDisabled: Bool(true)})
		if !check(err) {
			t.Errorf("status %d: err = %T %v", status, err, err)
		}
	}
}

// TestSmileLinkTokensCarryNoProduct checks that every Smile Link operation
// requires a token with no product claim, as the spec gives none.
func TestSmileLinkTokensCarryNoProduct(t *testing.T) {
	calls := map[string]func(c *Client) error{
		"create": func(c *Client) error {
			p := createSmileLinkParams()
			p.UserID = String("user_01")
			_, err := c.SmileLinks.Create(context.Background(), p)
			return err
		},
		"list": func(c *Client) error {
			_, err := c.SmileLinks.List(context.Background(), ListSmileLinksParams{})
			return err
		},
		"get": func(c *Client) error {
			_, err := c.SmileLinks.Get(context.Background(), "ref-1")
			return err
		},
		"update": func(c *Client) error {
			_, err := c.SmileLinks.Update(context.Background(), "ref-1", UpdateSmileLinkParams{IsDisabled: Bool(true)})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			var tr tokenRecorder
			c := testClient(t, tr.handler(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			if err := call(c); err != nil {
				t.Fatalf("call: %v", err)
			}
			assertClaims(t, tr.lastClaims(t), map[string]string{})
		})
	}
}

func TestSmileLinksListSurfacesAPIErrors(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusBadRequest, `{"status":"Bad Request","message":"limit is out of range."}`))
	_, err := c.SmileLinks.List(context.Background(), ListSmileLinksParams{})
	var invalid *InvalidRequestError
	if !errors.As(err, &invalid) {
		t.Fatalf("err = %T %v, want *InvalidRequestError", err, err)
	}
}
