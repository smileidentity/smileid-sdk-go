package smileid

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestConfigRejectsUnknownEnvironment(t *testing.T) {
	_, err := NewClient(Config{PartnerID: "1234", APIKey: "test-key", Environment: Environment("prod")})
	assertValidationError(t, err)
}

func TestConfigRejectsInsecureBaseURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"http scheme", "http://testapi.smileidentity.com"},
		{"no scheme", "testapi.smileidentity.com"},
		{"relative", "/v3"},
		{"empty host", "https://"},
		{"query", "https://testapi.smileidentity.com?a=1"},
		{"fragment", "https://testapi.smileidentity.com#frag"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(Config{PartnerID: "1234", APIKey: "test-key", BaseURL: tt.url})
			assertValidationError(t, err)
		})
	}
}

func TestConfigRejectsInsecureDefaultCallbackURL(t *testing.T) {
	_, err := NewClient(Config{
		PartnerID:          "1234",
		APIKey:             "test-key",
		DefaultCallbackURL: "http://partner.example.com/callback",
	})
	assertValidationError(t, err)
}

func TestEntryRejectsInsecureCallbackURLBeforeSend(t *testing.T) {
	c := failingClient(t)
	_, err := c.EnhancedKYC.Verify(context.Background(), EnhancedKYCParams{
		Country: "NG", IDType: "NIN", IDNumber: "12345678901",
		UserDetails: validUserDetails(), Consent: validConsent(),
		CallbackURL: String("http://partner.example.com/cb"),
	})
	assertValidationError(t, err)
}

func TestWithCallbackURLOptionRejectsInsecureURL(t *testing.T) {
	c := failingClient(t)
	_, err := c.EnhancedKYC.Verify(context.Background(), EnhancedKYCParams{
		Country: "NG", IDType: "NIN", IDNumber: "12345678901",
		UserDetails: validUserDetails(), Consent: validConsent(),
	}, WithCallbackURL("http://partner.example.com/cb"))
	assertValidationError(t, err)
}

func TestReplayRejectsInsecureCallbackURLBeforeSend(t *testing.T) {
	c := failingClient(t)
	_, err := c.Verifications.Replay(context.Background(), "job_01h8x9y2z3a4b5c6d7e8f9g0h1",
		ReplayParams{CallbackURL: String("http://partner.example.com/cb")})
	assertValidationError(t, err)
}

func TestSuccessBodyNotJSONObjectIsUnexpectedResponseError(t *testing.T) {
	bodies := []struct {
		name string
		body string
	}{
		{"html", "<html>intermediary error page</html>"},
		{"array", `[{"status":"complete"}]`},
		{"string", `"complete"`},
		{"null", "null"},
		{"empty", ""},
	}
	for _, tt := range bodies {
		t.Run(tt.name, func(t *testing.T) {
			c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v3/token" {
					serveToken(w)
					return
				}
				w.Header().Set("X-Request-Id", "req-9")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, tt.body)
			})

			_, err := c.Verifications.Retrieve(context.Background(), "job_x")
			var unexpected *UnexpectedResponseError
			if !errors.As(err, &unexpected) {
				t.Fatalf("err = %T %v, want *UnexpectedResponseError", err, err)
			}
			if unexpected.StatusCode != http.StatusOK {
				t.Errorf("StatusCode = %d", unexpected.StatusCode)
			}
			if unexpected.RawBody != tt.body {
				t.Errorf("RawBody = %q", unexpected.RawBody)
			}
			if unexpected.RequestID != "req-9" {
				t.Errorf("RequestID = %q", unexpected.RequestID)
			}
		})
	}
}

func TestPathParamsEscapedAsSingleSegment(t *testing.T) {
	var rawURI string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		rawURI = r.RequestURI
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"status":"accepted","message":"ok","user_id":"x"}`)
	})

	_, err := c.Users.ClearFraud(context.Background(), "../v3/token?x=1#f", ClearFraudParams{
		Notes: "n", ReportedBy: "r@e.com",
	})
	if err != nil {
		t.Fatalf("ClearFraud: %v", err)
	}
	want := "/v3/users/" + "..%2Fv3%2Ftoken%3Fx=1%23f" + "/report_fraud"
	if rawURI != want {
		t.Errorf("request URI = %q, want %q (hostile user_id must stay one path segment)", rawURI, want)
	}
}

func TestGoldenJobIDStaysByteIdentical(t *testing.T) {
	var rawURI string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		rawURI = r.RequestURI
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"complete","job_id":"job_01h8x9y2z3a4b5c6d7e8f9g0h1"}`)
	})

	if _, err := c.Verifications.Retrieve(context.Background(), "job_01h8x9y2z3a4b5c6d7e8f9g0h1"); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if rawURI != "/v3/status/job_01h8x9y2z3a4b5c6d7e8f9g0h1" {
		t.Errorf("request URI = %q, want the golden id byte-identical", rawURI)
	}
}

func TestHostileFilenameCannotInjectHeaders(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, acceptedBody))

	hostile := "evil\".jpg\r\nX-Injected: pwned\r\n\r\nbody"
	_, err := c.Biometric.Enroll(context.Background(), RegistrationParams{
		SelfieImage:    FromBytes(jpegBytes, hostile),
		LivenessImages: livenessSet(6),
		UserDetails:    validUserDetails(),
		Consent:        validConsent(),
	})
	if err != nil {
		t.Fatalf("Enroll with hostile filename: %v", err)
	}

	// The raw body must not contain an actual CRLF followed by the injected
	// header: %q escaping turns the CR/LF into literal backslash sequences.
	if bytes.Contains(cap.body, []byte("\r\nX-Injected")) {
		t.Error("hostile filename injected a raw header line into the multipart body")
	}
	// The multipart payload still parses and carries the selfie part.
	parts := parseMultipart(t, cap.contentType, cap.body)
	selfie := onePart(t, parts, "selfie_image")
	if selfie.contentType != "image/jpeg" {
		t.Errorf("selfie content type = %q", selfie.contentType)
	}
	if strings.Contains(selfie.filename, "\r") || strings.Contains(selfie.filename, "\n") {
		t.Errorf("parsed filename contains raw CR/LF: %q", selfie.filename)
	}
}

func TestHostileExplicitContentTypeRejected(t *testing.T) {
	// An empty override is not in this list: WithContentType("") means "no
	// override" and falls back to the safe default.
	hostiles := []string{
		"image/jpeg\r\nX-Evil: 1",
		"image/jpeg; boundary=x",
		"image jpeg",
		"image",
	}
	for _, ct := range hostiles {
		t.Run(fmt.Sprintf("ct_%q", ct), func(t *testing.T) {
			c := failingClient(t)
			_, err := c.Biometric.Enroll(context.Background(), RegistrationParams{
				SelfieImage:    FromBytes(jpegBytes, "selfie.jpg").WithContentType(ct),
				LivenessImages: livenessSet(6),
				UserDetails:    validUserDetails(),
				Consent:        validConsent(),
			})
			assertValidationError(t, err)
		})
	}
}

func TestBenignExplicitContentTypeAccepted(t *testing.T) {
	var cap captured
	c := testClient(t, captureHandler(&cap, http.StatusAccepted, acceptedBody))

	_, err := c.Biometric.Enroll(context.Background(), RegistrationParams{
		SelfieImage:    FromBytes(jpegBytes, "selfie.bin").WithContentType("application/octet-stream"),
		LivenessImages: livenessSet(6),
		UserDetails:    validUserDetails(),
		Consent:        validConsent(),
	})
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	parts := parseMultipart(t, cap.contentType, cap.body)
	if got := onePart(t, parts, "selfie_image").contentType; got != "application/octet-stream" {
		t.Errorf("selfie content type = %q", got)
	}
}
