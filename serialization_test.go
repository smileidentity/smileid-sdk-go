package smileid

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const acceptedBody = `{"status":"Accepted","message":"Request accepted and queued for processing.","job_id":"job_01h8x9y2z3a4b5c6d7e8f9g0h1","user_id":"user_01h8x9y2z3a4b5c6d7e8f9g0h1"}`

func TestEnhancedKYCGoldenRequest(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, acceptedBody))

	resp, err := c.EnhancedKYC.Verify(context.Background(), EnhancedKYCParams{
		Country:     "NG",
		IDType:      "NIN",
		IDNumber:    "12345678901",
		UserDetails: validUserDetails(),
		Consent:     validConsent(),
		UserID:      String("user_01h8x9y2z3a4b5c6d7e8f9g0h1"),
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if resp.JobID != "job_01h8x9y2z3a4b5c6d7e8f9g0h1" {
		t.Errorf("job_id = %q", resp.JobID)
	}

	if cap.path != "/v3/enhanced_kyc" {
		t.Errorf("path = %q", cap.path)
	}
	// Telemetry headers, always sent.
	if got := cap.header.Get("SmileID-Source-SDK"); got != "go" {
		t.Errorf("SmileID-Source-SDK = %q", got)
	}
	if got := cap.header.Get("SmileID-Source-SDK-Version"); got != "12.0.0" {
		t.Errorf("SmileID-Source-SDK-Version = %q", got)
	}
	if ua := cap.header.Get("User-Agent"); !strings.HasPrefix(ua, "smileid-sdk-go/12.0.0 (go/") {
		t.Errorf("User-Agent = %q", ua)
	}
	// Auth token injected; no Partner-ID header for enhanced_kyc.
	if cap.header.Get("SmileID-Token") == "" {
		t.Error("missing SmileID-Token")
	}
	if cap.header.Get("SmileID-Partner-ID") != "" {
		t.Error("enhanced_kyc must not send SmileID-Partner-ID")
	}
	// user_id routed to the User-ID header.
	if got := cap.header.Get("User-ID"); got != "user_01h8x9y2z3a4b5c6d7e8f9g0h1" {
		t.Errorf("User-ID header = %q", got)
	}

	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "country", "NG")
	assertScalar(t, parts, "id_type", "NIN")
	assertScalar(t, parts, "id_number", "12345678901")

	ud := onePart(t, parts, "user_details")
	if ud.contentType != "application/json" {
		t.Errorf("user_details content type = %q", ud.contentType)
	}
	var udMap map[string]any
	if err := json.Unmarshal(ud.content, &udMap); err != nil {
		t.Fatalf("user_details json: %v", err)
	}
	if udMap["given_names"] != "John" || udMap["email"] != "john@example.com" {
		t.Errorf("user_details = %s", ud.content)
	}
	if _, ok := udMap["phone_number"]; ok {
		t.Error("phone_number should be omitted when nil")
	}

	consent := onePart(t, parts, "consent")
	if consent.contentType != "application/json" {
		t.Errorf("consent content type = %q", consent.contentType)
	}
	if !strings.Contains(string(consent.content), `"granted":true`) {
		t.Errorf("consent = %s", consent.content)
	}
}

func TestDocumentVerificationMultipart(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, `{"status":"accepted","job_id":"job_x","user_id":"user_x"}`))

	idType := "PASSPORT"
	_, err := c.Documents.Verify(context.Background(), DocumentVerificationParams{
		Country:        "NG",
		IDType:         &idType,
		SelfieImage:    FromBytes(jpegBytes, "selfie.jpg"),
		LivenessImages: livenessSet(7),
		Document:       FromBytes(jpegBytes, "doc.jpg"),
		UserDetails:    UserDetails{GivenNames: "John", LastName: "Doe", PhoneNumber: String("+2348012345678")},
		Consent:        validConsent(),
		UserID:         String("user_x"),
	})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}

	// Partner-ID header required for document verification.
	if cap.header.Get("SmileID-Partner-ID") != "1234" {
		t.Errorf("SmileID-Partner-ID = %q", cap.header.Get("SmileID-Partner-ID"))
	}

	parts := parseMultipart(t, cap.contentType, cap.body)

	// liveness_images must be repeated parts, never CSV or indexed.
	liveness := partsByName(parts, "liveness_images")
	if len(liveness) != 7 {
		t.Fatalf("expected 7 liveness_images parts, got %d", len(liveness))
	}
	for _, p := range liveness {
		if p.contentType != "image/jpeg" {
			t.Errorf("liveness content type = %q", p.contentType)
		}
		if p.filename == "" {
			t.Error("liveness part missing filename")
		}
	}

	selfie := onePart(t, parts, "selfie_image")
	if selfie.contentType != "image/jpeg" || selfie.filename != "selfie.jpg" {
		t.Errorf("selfie = %+v", selfie)
	}
	doc := onePart(t, parts, "document")
	if doc.contentType != "image/jpeg" {
		t.Errorf("document content type = %q", doc.contentType)
	}
	assertScalar(t, parts, "id_type", "PASSPORT")
	if onePart(t, parts, "user_details").contentType != "application/json" {
		t.Error("user_details must be a JSON part")
	}
}

func TestDocumentPNGDetection(t *testing.T) {
	tests := []struct {
		name    string
		input   *BinaryInput
		field   string
		wantCT  string
		wantVia string
	}{
		{"document png by extension", FromBytes(jpegBytes, "doc.png"), "document", "image/png", "extension"},
		{"document png by magic bytes", FromBytes(pngBytes, "doc.jpg"), "document", "image/png", "magic"},
		{"document jpeg", FromBytes(jpegBytes, "doc.jpg"), "document", "image/jpeg", "default"},
		{"document_back png", FromBytes(pngBytes, "back.png"), "document_back", "image/png", "extension"},
		{"selfie png bytes stays jpeg", FromBytes(pngBytes, "selfie.png"), "selfie_image", "image/jpeg", "non-document"},
		{"explicit override wins", FromBytes(pngBytes, "doc.png").WithContentType("application/octet-stream"), "document", "application/octet-stream", "override"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := tt.input.Bytes()
			if got := tt.input.ContentTypeFor(tt.field, data); got != tt.wantCT {
				t.Errorf("ContentTypeFor(%q) = %q, want %q", tt.field, got, tt.wantCT)
			}
		})
	}
}

func TestAuthenticationRoutesUserIDToBody(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, acceptedBody))

	_, err := c.Biometric.Authenticate(context.Background(), AuthenticationParams{
		UserID:         "user_auth_001",
		SelfieImage:    FromBytes(jpegBytes, "selfie.jpg"),
		LivenessImages: livenessSet(6),
		UserDetails:    validUserDetails(),
		Consent:        validConsent(),
	})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	// user_id goes in the body, NOT the User-ID header.
	if cap.header.Get("User-ID") != "" {
		t.Error("authentication must not send the User-ID header")
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "user_id", "user_auth_001")
}

func TestReplayIsJSONNotMultipart(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted,
		`{"status":"accepted","job_id":"job_x","user_id":"test-user","message":"queued"}`))

	_, err := c.Verifications.Replay(context.Background(), "job_01h2xcejqtf2nbrexx3vqjhp41",
		ReplayParams{CallbackURL: String("https://partner.example.com/webhook")})
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}

	if cap.contentType != "application/json" {
		t.Errorf("replay content type = %q, want application/json", cap.contentType)
	}
	var body map[string]any
	if err := json.Unmarshal(cap.body, &body); err != nil {
		t.Fatalf("replay body json: %v", err)
	}
	if body["callback_url"] != "https://partner.example.com/webhook" {
		t.Errorf("replay body = %s", cap.body)
	}
}

func TestReportFraudMultipart(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted,
		`{"status":"accepted","message":"Fraud report accepted","user_id":"user-123"}`))

	_, err := c.Users.FlagFraud(context.Background(), "user-123", FlagFraudParams{
		Reason:     ReasonFirstPartyFraud,
		ReportedBy: "risk@partner.example",
	})
	if err != nil {
		t.Fatalf("FlagFraud: %v", err)
	}

	if cap.path != "/v3/users/user-123/report_fraud" {
		t.Errorf("path = %q", cap.path)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	assertScalar(t, parts, "is_fraud", "true")
	assertScalar(t, parts, "reason", "FIRST_PARTY_FRAUD")
	assertScalar(t, parts, "reported_by", "risk@partner.example")
}

func assertScalar(t *testing.T, parts []capturedPart, name, want string) {
	t.Helper()
	p := onePart(t, parts, name)
	if p.contentType != "" {
		t.Errorf("scalar %q has content type %q, want none", name, p.contentType)
	}
	if string(p.content) != want {
		t.Errorf("scalar %q = %q, want %q", name, p.content, want)
	}
}
