package usesmileid

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

const amlAcceptedBody = `{"status":"accepted","message":"Job submitted successfully","job_id":"job_01h8x9y2z3a4b5c6d7e8f9g0h1","user_id":"user_01h8x9y2z3a4b5c6d7e8f9g0h1"}`

const amlMonitoringAcceptedBody = `{"status":"accepted","message":"Request accepted and queued for processing.","job_id":"job_01h8x9y2z3a4b5c6d7e8f9g0h1","user_id":"user_01h8x9y2z3a4b5c6d7e8f9g0h1","created_at":"2026-03-10T12:00:00.000Z"}`

const amlSyncBody = `{
  "status": "block",
  "message": "Screening completed: potential matches identified.",
  "reason": "aml_match_found",
  "created_at": "2026-07-20T12:00:00.000Z",
  "completed_at": "2026-07-20T12:00:01.000Z",
  "product": "one_time_aml",
  "partner_params": {"order_id": "ORD-12345"},
  "kyc_receipt": "https://receipts.smileidentity.com/abc",
  "strict_match": true,
  "no_of_persons_found": 1,
  "people": [{
    "name": "Gbenga Afolabi",
    "addresses": [],
    "aliases": [{"name": "G. Afolabi"}],
    "associations": [],
    "dates_of_birth": [],
    "enforcement_actions": [],
    "nationalities": [],
    "news_summary": [{"news_category": "financial_crime", "news_category_label": "Financial crime", "number_of_articles": 3}],
    "sanctions": [{"list": "OFAC SDN"}],
    "pep": {"pep_level": 2, "political_positions": ["Minister"], "sources": [{"name": "src"}]}
  }],
  "user_provided_info": {"given_names": "Nkechi", "last_name": "Okafor", "email": "nkechi.okafor@example.com", "phone_number": null},
  "consent": {},
  "antifraud": null,
  "device_signals": null
}`

const amlClearBody = `{"status":"clear","message":"Screening completed: no matches found.","reason":null,"created_at":"2026-07-20T12:00:00.000Z","product":"one_time_aml","partner_params":null,"kyc_receipt":null,"strict_match":true,"no_of_persons_found":0,"people":[],"user_provided_info":{"given_names":null,"last_name":null,"email":null,"phone_number":null},"consent":{},"antifraud":null,"device_signals":null}`

const amlNewsBody = `{"job_id":"a1b2c3d4e5","product":"aml_monitoring_news","user_id":"user_123","meta":{"current_page":1,"limit":25,"total_count":1},"news":[{"body":"Sample adverse-media content.","published_date":"2026-01-15T12:00:00.000Z","publisher":"Example News Publisher","publisher_country_code":"NG","title":"Sample AML Monitoring News Item","url":"https://example.com/news/sample-aml-monitoring"}]}`

func oneTimeAMLParams() OneTimeAMLParams {
	return OneTimeAMLParams{
		Countries:   []string{"NG", "KE"},
		FullName:    "John Doe",
		UserDetails: validUserDetails(),
		Consent:     validConsent(),
	}
}

func assertJSONPart(t *testing.T, parts []capturedPart, name, want string) {
	t.Helper()
	p := onePart(t, parts, name)
	if p.contentType != "application/json" {
		t.Errorf("%s content type = %q", name, p.contentType)
	}
	if string(p.content) != want {
		t.Errorf("%s = %s, want %s", name, p.content, want)
	}
}

func assertNoPart(t *testing.T, parts []capturedPart, name string) {
	t.Helper()
	if got := partsByName(parts, name); len(got) != 0 {
		t.Errorf("part %q should be omitted", name)
	}
}

func TestAMLScreenGoldenRequest(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, amlAcceptedBody))

	p := oneTimeAMLParams()
	p.BirthYear = String("1990")
	p.StrictMatch = Bool(false)
	p.Aliases = []string{"Johnny Doe"}
	p.SearchExistingUser = Bool(true)
	p.UserID = String("user-1234")
	p.CallbackURL = String("https://example.com/callback")
	p.PartnerParams = map[string]string{"order_id": "ORD-12345"}
	p.Metadata = []MetadataEntry{{Name: "k", Value: "v"}}

	resp, err := c.AML.Screen(context.Background(), p)
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}
	if !resp.IsAccepted() || resp.JobID != "job_01h8x9y2z3a4b5c6d7e8f9g0h1" || resp.UserID != "user_01h8x9y2z3a4b5c6d7e8f9g0h1" {
		t.Errorf("response = %+v", resp)
	}

	if cap.method != "POST" || cap.path != "/v3/aml" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.header.Get("SmileID-Token") == "" {
		t.Error("missing SmileID-Token")
	}
	if got := cap.header.Get("SmileID-Source-SDK"); got != "go" {
		t.Errorf("SmileID-Source-SDK = %q", got)
	}
	if cap.header.Get("SmileID-Partner-ID") != "" || cap.header.Get("User-ID") != "" || cap.header.Get("SmileID-Idempotency-Key") != "" {
		t.Error("aml must send no Partner-ID, User-ID or idempotency header")
	}

	parts := parseMultipart(t, cap.contentType, cap.body)
	assertJSONPart(t, parts, "countries", `["NG","KE"]`)
	assertScalar(t, parts, "full_name", "John Doe")
	assertScalar(t, parts, "birth_year", "1990")
	assertScalar(t, parts, "strict_match", "false")
	assertJSONPart(t, parts, "aliases", `["Johnny Doe"]`)
	assertScalar(t, parts, "search_existing_user", "true")
	assertScalar(t, parts, "user_id", "user-1234")
	assertScalar(t, parts, "callback_url", "https://example.com/callback")
	assertJSONPart(t, parts, "partner_params", `{"order_id":"ORD-12345"}`)
	assertJSONPart(t, parts, "metadata", `[{"name":"k","value":"v"}]`)
	assertJSONPart(t, parts, "user_details", `{"given_names":"John","last_name":"Doe","email":"john@example.com"}`)
	if !strings.Contains(string(onePart(t, parts, "consent").content), `"granted":true`) {
		t.Error("consent part missing grant")
	}
}

func TestAMLScreenMinimalRequestUsesClientDefaultCallback(t *testing.T) {
	var cap captured
	srv := captureHandler(&cap, http.StatusAccepted, amlAcceptedBody)
	c := testClient(t, srv)
	c.cfg.defaultCallbackURL = "https://example.com/default"

	if _, err := c.AML.Screen(context.Background(), oneTimeAMLParams(), WithCallbackURL("https://example.com/override")); err != nil {
		t.Fatalf("Screen: %v", err)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "callback_url", "https://example.com/override")
	for _, name := range []string{"birth_year", "strict_match", "aliases", "search_existing_user", "user_id", "partner_params", "metadata"} {
		assertNoPart(t, parts, name)
	}
}

func TestAMLScreenSyncGoldenRequestAndResult(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, amlSyncBody))

	res, err := c.AML.ScreenSync(context.Background(), OneTimeAMLSyncParams{
		OneTimeAMLParams: oneTimeAMLParams(),
		IdempotencyKey:   String("kuda-onboarding-8f14e45fceea167a"),
	})
	if err != nil {
		t.Fatalf("ScreenSync: %v", err)
	}

	if cap.method != "POST" || cap.path != "/v3/aml/sync" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if got := cap.header.Get("SmileID-Idempotency-Key"); got != "kuda-onboarding-8f14e45fceea167a" {
		t.Errorf("SmileID-Idempotency-Key = %q", got)
	}
	if cap.header.Get("SmileID-Token") == "" {
		t.Error("missing SmileID-Token")
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertJSONPart(t, parts, "countries", `["NG","KE"]`)
	assertScalar(t, parts, "full_name", "John Doe")
	assertNoPart(t, parts, "callback_url")

	if res.Status != "block" || res.Reason == nil || *res.Reason != "aml_match_found" {
		t.Errorf("status/reason = %q %v", res.Status, res.Reason)
	}
	if res.Product != "one_time_aml" || !res.StrictMatch || res.NoOfPersonsFound != 1 || res.CompletedAt != "2026-07-20T12:00:01.000Z" {
		t.Errorf("result = %+v", res)
	}
	if res.KYCReceipt == nil || *res.KYCReceipt != "https://receipts.smileidentity.com/abc" {
		t.Errorf("kyc_receipt = %v", res.KYCReceipt)
	}
	if res.PartnerParams["order_id"] != "ORD-12345" {
		t.Errorf("partner_params = %v", res.PartnerParams)
	}
	if len(res.People) != 1 {
		t.Fatalf("people = %+v", res.People)
	}
	person := res.People[0]
	if person.Name != "Gbenga Afolabi" || person.Sanctions[0]["list"] != "OFAC SDN" || person.Aliases[0]["name"] != "G. Afolabi" {
		t.Errorf("person = %+v", person)
	}
	if len(person.NewsSummary) != 1 || person.NewsSummary[0].NewsCategory != "financial_crime" || person.NewsSummary[0].NumberOfArticles != 3 {
		t.Errorf("news_summary = %+v", person.NewsSummary)
	}
	if person.PEP == nil || *person.PEP.PEPLevel != 2 || person.PEP.PoliticalPositions[0] != "Minister" || person.PEP.Sources[0]["name"] != "src" {
		t.Errorf("pep = %+v", person.PEP)
	}
	info := res.UserProvidedInfo
	if *info.GivenNames != "Nkechi" || *info.LastName != "Okafor" || *info.Email != "nkechi.okafor@example.com" || info.PhoneNumber != nil {
		t.Errorf("user_provided_info = %+v", info)
	}
	if res.Consent == nil || len(res.Consent) != 0 || res.Antifraud != nil || res.DeviceSignals != nil {
		t.Errorf("consent/antifraud/device_signals = %v %v %v", res.Consent, res.Antifraud, res.DeviceSignals)
	}
}

func TestAMLScreenSyncClearResult(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, amlClearBody))

	res, err := c.AML.ScreenSync(context.Background(), OneTimeAMLSyncParams{
		OneTimeAMLParams: oneTimeAMLParams(),
	}, WithCallbackURL("https://example.com/stored"))
	if err != nil {
		t.Fatalf("ScreenSync: %v", err)
	}
	if cap.header.Get("SmileID-Idempotency-Key") != "" {
		t.Error("idempotency header sent without a key")
	}
	assertScalar(t, parseMultipart(t, cap.contentType, cap.body), "callback_url", "https://example.com/stored")
	if res.Status != "clear" || res.Reason != nil || res.KYCReceipt != nil || res.PartnerParams != nil || res.NoOfPersonsFound != 0 || len(res.People) != 0 {
		t.Errorf("result = %+v", res)
	}
	if res.UserProvidedInfo.GivenNames != nil {
		t.Errorf("user_provided_info = %+v", res.UserProvidedInfo)
	}
}

func TestAMLScreenSyncRetriesWithIdempotencyKey(t *testing.T) {
	calls := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			w.WriteHeader(http.StatusRequestTimeout)
			_, _ = w.Write([]byte(`{"status":"Request Timeout","message":"Provider did not answer in time."}`))
			return
		}
		_, _ = w.Write([]byte(amlClearBody))
	})
	c.cfg.maxRetries = 1

	res, err := c.AML.ScreenSync(context.Background(), OneTimeAMLSyncParams{
		OneTimeAMLParams: oneTimeAMLParams(),
		IdempotencyKey:   String("retry-key-0123456789"),
	})
	if err != nil {
		t.Fatalf("ScreenSync: %v", err)
	}
	if calls != 2 || res.Status != "clear" {
		t.Errorf("calls = %d, status = %q", calls, res.Status)
	}
}

func TestAMLMonitorGoldenRequest(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, amlMonitoringAcceptedBody))

	resp, err := c.AML.Monitor(context.Background(), AMLMonitoringParams{
		Countries:          []string{"NG", "GH"},
		FullName:           String("Jane Doe"),
		UserDetails:        validUserDetails(),
		Consent:            validConsent(),
		BirthYear:          String("1990"),
		StrictMatch:        Bool(true),
		Aliases:            []string{"Jane A. Doe", "J. Doe"},
		SearchExistingUser: Bool(false),
		UserID:             String("user_1"),
		CallbackURL:        String("https://example.com/callback"),
		PartnerParams:      map[string]string{"order_id": "ORD-12345"},
		Metadata:           []MetadataEntry{{Name: "k", Value: "v"}},
	})
	if err != nil {
		t.Fatalf("Monitor: %v", err)
	}
	if !resp.IsAccepted() || resp.CreatedAt != "2026-03-10T12:00:00.000Z" || resp.JobID != "job_01h8x9y2z3a4b5c6d7e8f9g0h1" {
		t.Errorf("response = %+v", resp)
	}

	if cap.method != "POST" || cap.path != "/v3/aml/monitoring" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.header.Get("SmileID-Token") == "" {
		t.Error("missing SmileID-Token")
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertJSONPart(t, parts, "countries", `["NG","GH"]`)
	assertScalar(t, parts, "full_name", "Jane Doe")
	assertScalar(t, parts, "birth_year", "1990")
	assertScalar(t, parts, "strict_match", "true")
	assertJSONPart(t, parts, "aliases", `["Jane A. Doe","J. Doe"]`)
	assertScalar(t, parts, "search_existing_user", "false")
	assertScalar(t, parts, "user_id", "user_1")
	assertScalar(t, parts, "callback_url", "https://example.com/callback")
	assertJSONPart(t, parts, "partner_params", `{"order_id":"ORD-12345"}`)
	assertJSONPart(t, parts, "metadata", `[{"name":"k","value":"v"}]`)
	onePart(t, parts, "user_details")
	onePart(t, parts, "consent")
}

func TestAMLMonitorWithExistingUserOmitsName(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, amlMonitoringAcceptedBody))

	_, err := c.AML.Monitor(context.Background(), AMLMonitoringParams{
		Countries:          []string{"NG"},
		UserDetails:        validUserDetails(),
		Consent:            validConsent(),
		SearchExistingUser: Bool(true),
		UserID:             String("user_1"),
	})
	if err != nil {
		t.Fatalf("Monitor: %v", err)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertNoPart(t, parts, "full_name")
	assertScalar(t, parts, "search_existing_user", "true")
	assertScalar(t, parts, "user_id", "user_1")
	for _, name := range []string{"birth_year", "strict_match", "aliases", "callback_url", "partner_params", "metadata"} {
		assertNoPart(t, parts, name)
	}
}

func TestAMLMonitoringNewsGoldenRequestAndResult(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, amlNewsBody))

	res, err := c.AML.MonitoringNews(context.Background(), "a1b2c3d4e5", "ref_001", AMLMonitoringNewsParams{
		NewsCategory: "financial_crime",
		Page:         Ptr(2),
		Limit:        Ptr(50),
	})
	if err != nil {
		t.Fatalf("MonitoringNews: %v", err)
	}

	if cap.method != "GET" || cap.path != "/v3/aml/monitoring/a1b2c3d4e5/news/ref_001" {
		t.Errorf("request = %s %s", cap.method, cap.path)
	}
	if cap.query != "limit=50&news_category=financial_crime&page=2" {
		t.Errorf("query = %q", cap.query)
	}
	if cap.header.Get("SmileID-Token") == "" {
		t.Error("missing SmileID-Token")
	}
	if len(cap.body) != 0 || cap.contentType != "" {
		t.Errorf("GET sent a body: %q %q", cap.body, cap.contentType)
	}

	if res.JobID != "a1b2c3d4e5" || res.Product != "aml_monitoring_news" || res.UserID != "user_123" {
		t.Errorf("response = %+v", res)
	}
	if res.Meta != (AMLMonitoringNewsMeta{CurrentPage: 1, Limit: 25, TotalCount: 1}) {
		t.Errorf("meta = %+v", res.Meta)
	}
	want := AMLMonitoringNewsItem{
		Body:                 "Sample adverse-media content.",
		PublishedDate:        "2026-01-15T12:00:00.000Z",
		Publisher:            "Example News Publisher",
		PublisherCountryCode: "NG",
		Title:                "Sample AML Monitoring News Item",
		URL:                  "https://example.com/news/sample-aml-monitoring",
	}
	if len(res.News) != 1 || res.News[0] != want {
		t.Errorf("news = %+v", res.News)
	}
}

func TestAMLMonitoringNewsOmitsOptionalQuery(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusOK, amlNewsBody))

	if _, err := c.AML.MonitoringNews(context.Background(), "j", "r", AMLMonitoringNewsParams{NewsCategory: "other"}); err != nil {
		t.Fatalf("MonitoringNews: %v", err)
	}
	if cap.query != "news_category=other" {
		t.Errorf("query = %q", cap.query)
	}
}

func TestAMLValidation(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request to %s", r.URL.Path)
	})
	ctx := context.Background()

	screening := []struct {
		name   string
		mutate func(*OneTimeAMLParams)
		want   string
	}{
		{"no countries", func(p *OneTimeAMLParams) { p.Countries = nil }, "countries is required"},
		{"long country", func(p *OneTimeAMLParams) { p.Countries = []string{"NGA"} }, "two-letter"},
		{"no name", func(p *OneTimeAMLParams) { p.FullName = " " }, "full_name is required"},
		{"too many aliases", func(p *OneTimeAMLParams) { p.Aliases = make([]string, 21) }, "at most 20"},
		{"bad birth year", func(p *OneTimeAMLParams) { p.BirthYear = String("90") }, "birth_year"},
		{"search without user", func(p *OneTimeAMLParams) { p.SearchExistingUser = Bool(true) }, "user_id is required"},
		{"search with blank user", func(p *OneTimeAMLParams) {
			p.SearchExistingUser = Bool(true)
			p.UserID = String(" ")
		}, "user_id is required"},
		{"no contact", func(p *OneTimeAMLParams) { p.UserDetails.Email = nil }, "email or phone_number"},
		{"insecure callback", func(p *OneTimeAMLParams) { p.CallbackURL = String("http://example.com/cb") }, "callback_url"},
	}
	for _, tt := range screening {
		t.Run("screen "+tt.name, func(t *testing.T) {
			p := oneTimeAMLParams()
			tt.mutate(&p)
			_, err := c.AML.Screen(ctx, p)
			assertValidationMessage(t, err, tt.want)
		})
		t.Run("sync "+tt.name, func(t *testing.T) {
			p := oneTimeAMLParams()
			tt.mutate(&p)
			_, err := c.AML.ScreenSync(ctx, OneTimeAMLSyncParams{OneTimeAMLParams: p})
			assertValidationMessage(t, err, tt.want)
		})
	}

	t.Run("sync bad idempotency key", func(t *testing.T) {
		_, err := c.AML.ScreenSync(ctx, OneTimeAMLSyncParams{OneTimeAMLParams: oneTimeAMLParams(), IdempotencyKey: String("short")})
		assertValidationMessage(t, err, "idempotency_key")
	})

	monitoring := []struct {
		name   string
		mutate func(*AMLMonitoringParams)
		want   string
	}{
		{"no name", func(p *AMLMonitoringParams) { p.FullName = nil }, "full_name is required"},
		{"blank name", func(p *AMLMonitoringParams) { p.FullName = String("  ") }, "full_name is required"},
		{"no countries", func(p *AMLMonitoringParams) { p.Countries = nil }, "countries is required"},
		{"search without user", func(p *AMLMonitoringParams) { p.SearchExistingUser = Bool(true) }, "user_id is required"},
		{"no contact", func(p *AMLMonitoringParams) { p.UserDetails.Email = nil }, "email or phone_number"},
		{"insecure callback", func(p *AMLMonitoringParams) { p.CallbackURL = String("http://example.com/cb") }, "callback_url"},
	}
	for _, tt := range monitoring {
		t.Run("monitor "+tt.name, func(t *testing.T) {
			p := AMLMonitoringParams{
				Countries: []string{"NG"}, FullName: String("Jane Doe"),
				UserDetails: validUserDetails(), Consent: validConsent(),
			}
			tt.mutate(&p)
			_, err := c.AML.Monitor(ctx, p)
			assertValidationMessage(t, err, tt.want)
		})
	}

	news := []struct {
		name   string
		job    string
		ref    string
		params AMLMonitoringNewsParams
		want   string
	}{
		{"no job", " ", "r", AMLMonitoringNewsParams{NewsCategory: "other"}, "job_id"},
		{"no reference", "j", "", AMLMonitoringNewsParams{NewsCategory: "other"}, "reference"},
		{"bad category", "j", "r", AMLMonitoringNewsParams{NewsCategory: "gossip"}, "news_category"},
		{"page too low", "j", "r", AMLMonitoringNewsParams{NewsCategory: "other", Page: Ptr(0)}, "page"},
		{"page too high", "j", "r", AMLMonitoringNewsParams{NewsCategory: "other", Page: Ptr(101)}, "page"},
		{"limit too low", "j", "r", AMLMonitoringNewsParams{NewsCategory: "other", Limit: Ptr(0)}, "limit"},
		{"limit too high", "j", "r", AMLMonitoringNewsParams{NewsCategory: "other", Limit: Ptr(101)}, "limit"},
	}
	for _, tt := range news {
		t.Run("news "+tt.name, func(t *testing.T) {
			_, err := c.AML.MonitoringNews(ctx, tt.job, tt.ref, tt.params)
			assertValidationMessage(t, err, tt.want)
		})
	}
}

func TestAMLOperationsSurfaceAPIErrors(t *testing.T) {
	c := testClient(t, captureHandler(&captured{}, http.StatusNotFound, `{"status":"Not Found","message":"No AML job found for this partner"}`))
	_, err := c.AML.MonitoringNews(context.Background(), "j", "r", AMLMonitoringNewsParams{NewsCategory: "other"})
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err = %T %v, want *NotFoundError", err, err)
	}

	c = testClient(t, captureHandler(&captured{}, http.StatusBadRequest, `{"status":"Bad Request","message":"bad"}`))
	if _, err := c.AML.Screen(context.Background(), oneTimeAMLParams()); err == nil {
		t.Error("Screen: want error")
	}
	if _, err := c.AML.ScreenSync(context.Background(), OneTimeAMLSyncParams{OneTimeAMLParams: oneTimeAMLParams()}); err == nil {
		t.Error("ScreenSync: want error")
	}
	if _, err := c.AML.Monitor(context.Background(), AMLMonitoringParams{
		Countries: []string{"NG"}, FullName: String("J"), UserDetails: validUserDetails(), Consent: validConsent(),
	}); err == nil {
		t.Error("Monitor: want error")
	}
}

func TestAMLNewsCategoriesMatchSpec(t *testing.T) {
	if len(AMLNewsCategories) != 18 || AMLNewsCategories[0] != "arms_trafficking" || AMLNewsCategories[17] != "violent_crime" {
		t.Errorf("AMLNewsCategories = %v", AMLNewsCategories)
	}
	var out AMLMonitoringNewsResponse
	if err := json.Unmarshal([]byte(amlNewsBody), &out); err != nil {
		t.Fatal(err)
	}
}

func assertValidationMessage(t *testing.T, err error, want string) {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %T %v, want *ValidationError", err, err)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("err = %q, want it to contain %q", err, want)
	}
}
