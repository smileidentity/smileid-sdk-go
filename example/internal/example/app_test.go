package example

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServicesCommandListsReferenceData(t *testing.T) {
	server := newFakeSmileServer(t)

	var out bytes.Buffer
	err := Run(context.Background(), []string{
		"--base-url", server.URL,
		"services",
		"--country", "NG",
	}, testEnv(server), &out, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got servicesOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if got.Country != "NG" {
		t.Fatalf("Country = %q, want NG", got.Country)
	}
	if got.BankCodes[0].Code != "001" {
		t.Fatalf("BankCodes[0].Code = %q, want 001", got.BankCodes[0].Code)
	}
	if got.IDTypes[0].Type != "NIN" {
		t.Fatalf("IDTypes[0].Type = %q, want NIN", got.IDTypes[0].Type)
	}
	if got.Documents[0].Country.Code != "NG" {
		t.Fatalf("Documents[0].Country.Code = %q, want NG", got.Documents[0].Country.Code)
	}
	if server.tokenRequests != 0 {
		t.Fatalf("services command made %d token requests, want 0", server.tokenRequests)
	}
}

func TestEnhancedKYCCommandSubmitsVerification(t *testing.T) {
	server := newFakeSmileServer(t)

	var out bytes.Buffer
	err := Run(context.Background(), []string{
		"--base-url", server.URL,
		"--callback-url", "https://example.com/smile-callback",
		"enhanced-kyc",
		"--country", "NG",
		"--id-type", "NIN",
		"--id-number", "12345678901",
		"--given-names", "Amina",
		"--last-name", "Okafor",
		"--email", "amina@example.com",
		"--privacy-url", "https://example.com/privacy",
	}, testEnv(server), &out, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got acceptedOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if got.JobID != "job_enhanced_123" || !got.Accepted {
		t.Fatalf("accepted output = %+v, want accepted job_enhanced_123", got)
	}
	if server.tokenRequests != 1 {
		t.Fatalf("tokenRequests = %d, want 1", server.tokenRequests)
	}
	assertMultipartField(t, server.enhancedKYCForm, "country", "NG")
	assertMultipartField(t, server.enhancedKYCForm, "id_type", "NIN")
	assertMultipartField(t, server.enhancedKYCForm, "id_number", "12345678901")
	assertMultipartField(t, server.enhancedKYCForm, "callback_url", "https://example.com/smile-callback")
	assertMultipartContains(t, server.enhancedKYCForm, "user_details", `"given_names":"Amina"`)
	assertMultipartContains(t, server.enhancedKYCForm, "consent", `"granted":true`)
}

func TestStatusCommandRetrievesJob(t *testing.T) {
	server := newFakeSmileServer(t)

	var out bytes.Buffer
	err := Run(context.Background(), []string{
		"--base-url", server.URL,
		"status",
		"--job-id", "job_enhanced_123",
	}, testEnv(server), &out, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got statusOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if got.Status != "complete" || got.Message != "clear" {
		t.Fatalf("status output = %+v, want complete clear", got)
	}
}

func TestReplayCommandRequestsCallbackReplay(t *testing.T) {
	server := newFakeSmileServer(t)

	var out bytes.Buffer
	err := Run(context.Background(), []string{
		"--base-url", server.URL,
		"replay",
		"--job-id", "job_enhanced_123",
		"--callback-url", "https://example.com/replay-callback",
	}, testEnv(server), &out, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var got replayOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode output: %v\n%s", err, out.String())
	}
	if got.Status != "success" || got.JobID != "job_enhanced_123" {
		t.Fatalf("replay output = %+v, want success job_enhanced_123", got)
	}
	if server.replayCallbackURL != "https://example.com/replay-callback" {
		t.Fatalf("replay callback URL = %q", server.replayCallbackURL)
	}
}

func TestRunReturnsHelpfulValidationErrors(t *testing.T) {
	var out, stderr bytes.Buffer
	err := Run(context.Background(), []string{"services"}, func(string) string { return "" }, &out, &stderr)
	if err == nil {
		t.Fatal("Run() error = nil, want missing credentials")
	}
	if !strings.Contains(err.Error(), "SMILE_PARTNER_ID") {
		t.Fatalf("error = %q, want SMILE_PARTNER_ID guidance", err)
	}
}

func TestHelpDoesNotRequireCredentials(t *testing.T) {
	var out bytes.Buffer
	err := Run(context.Background(), []string{"help"}, func(string) string { return "" }, &out, io.Discard)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("help output = %q, want Usage", out.String())
	}
}

type fakeSmileServer struct {
	*httptest.Server
	tokenRequests     int
	enhancedKYCForm   map[string]string
	replayCallbackURL string
}

func newFakeSmileServer(t *testing.T) *fakeSmileServer {
	t.Helper()
	fake := &fakeSmileServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v3/token", func(w http.ResponseWriter, r *http.Request) {
		fake.tokenRequests++
		if r.Header.Get("smileid-partner-id") != "12345" {
			t.Errorf("smileid-partner-id = %q", r.Header.Get("smileid-partner-id"))
		}
		if r.Header.Get("smileid-api-key") != "test-api-key" {
			t.Errorf("smileid-api-key = %q", r.Header.Get("smileid-api-key"))
		}
		writeJSON(t, w, http.StatusOK, map[string]string{"token": "test-token"})
	})
	mux.HandleFunc("/v3/services/bank_codes", func(w http.ResponseWriter, r *http.Request) {
		assertQuery(t, r, "country", "NG")
		writeJSON(t, w, http.StatusOK, map[string]any{"bank_codes": []map[string]string{{
			"code": "001", "country": "NG", "name": "Example Bank",
		}}})
	})
	mux.HandleFunc("/v3/services/supported_id_types", func(w http.ResponseWriter, r *http.Request) {
		assertQuery(t, r, "country", "NG")
		writeJSON(t, w, http.StatusOK, map[string]any{"id_types": []map[string]any{{
			"country": "NG", "label": "National Identification Number", "regex": "^\\d{11}$", "required_fields": []string{"id_number"}, "type": "NIN",
		}}})
	})
	mux.HandleFunc("/v3/services/supported_documents", func(w http.ResponseWriter, r *http.Request) {
		assertQuery(t, r, "country_code", "NG")
		writeJSON(t, w, http.StatusOK, map[string]any{"valid_documents": []map[string]any{{
			"country": map[string]string{"code": "NG", "name": "Nigeria", "continent": "Africa"},
			"id_types": []map[string]any{{
				"code": "PASSPORT", "name": "Passport", "example": []string{"A12345678"}, "has_back": false,
			}},
		}}})
	})
	mux.HandleFunc("/v3/enhanced_kyc", func(w http.ResponseWriter, r *http.Request) {
		assertBearerToken(t, r)
		form, err := parseMultipart(r)
		if err != nil {
			t.Errorf("parse multipart: %v", err)
		}
		fake.enhancedKYCForm = form
		writeJSON(t, w, http.StatusAccepted, map[string]string{
			"status": "Accepted", "message": "submitted", "job_id": "job_enhanced_123", "user_id": "user_123",
		})
	})
	mux.HandleFunc("/v3/status/job_enhanced_123", func(w http.ResponseWriter, r *http.Request) {
		assertBearerToken(t, r)
		writeJSON(t, w, http.StatusOK, map[string]string{
			"status": "complete", "job_id": "job_enhanced_123", "user_id": "user_123", "message": "clear",
		})
	})
	mux.HandleFunc("/v3/replay/job_enhanced_123", func(w http.ResponseWriter, r *http.Request) {
		assertBearerToken(t, r)
		var body struct {
			CallbackURL string `json:"callback_url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode replay body: %v", err)
		}
		fake.replayCallbackURL = body.CallbackURL
		writeJSON(t, w, http.StatusOK, map[string]string{
			"status": "success", "job_id": "job_enhanced_123", "user_id": "user_123", "message": "replayed",
		})
	})
	fake.Server = httptest.NewTLSServer(mux)
	t.Cleanup(fake.Close)
	return fake
}

func testEnv(server *fakeSmileServer) func(string) string {
	return func(key string) string {
		switch key {
		case "SMILE_PARTNER_ID":
			return "12345"
		case "SMILE_API_KEY":
			return "test-api-key"
		case "SMILE_TIMEOUT":
			return "5s"
		case "SMILE_EXAMPLE_INSECURE_TLS":
			return "1"
		default:
			return ""
		}
	}
}

func assertBearerToken(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Header.Get("SmileID-Token") != "test-token" {
		t.Fatalf("SmileID-Token = %q, want test-token", r.Header.Get("SmileID-Token"))
	}
}

func assertQuery(t *testing.T, r *http.Request, key, want string) {
	t.Helper()
	if got := r.URL.Query().Get(key); got != want {
		t.Fatalf("%s query = %q, want %q", key, got, want)
	}
}

func assertMultipartField(t *testing.T, form map[string]string, key, want string) {
	t.Helper()
	if got := form[key]; got != want {
		t.Fatalf("multipart %s = %q, want %q", key, got, want)
	}
}

func assertMultipartContains(t *testing.T, form map[string]string, key, want string) {
	t.Helper()
	if got := form[key]; !strings.Contains(got, want) {
		t.Fatalf("multipart %s = %q, want substring %q", key, got, want)
	}
}

func parseMultipart(r *http.Request) (map[string]string, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return values, nil
		}
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(part)
		if err != nil {
			return nil, err
		}
		values[part.FormName()] = string(body)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write JSON: %v", err)
	}
}

var _ = multipart.ErrMessageTooLarge
