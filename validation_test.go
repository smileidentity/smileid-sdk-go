package usesmileid

import (
	"context"
	"errors"
	"testing"
)

// failingClient points at an unresolvable host so any request that is
// actually sent fails the test with a non-validation error, proving
// validation fires before sending.
func failingClient(t *testing.T) *Client {
	t.Helper()
	c, err := NewClient(Config{PartnerID: "1234", APIKey: "test-key", BaseURL: "https://invalid.invalid"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func assertValidationError(t *testing.T, err error) {
	t.Helper()
	var v *ValidationError
	if !errors.As(err, &v) {
		t.Fatalf("err = %T %v, want *ValidationError", err, err)
	}
}

func TestUserDetailsRequiresEmailOrPhone(t *testing.T) {
	c := failingClient(t)
	_, err := c.EnhancedKYC.Verify(context.Background(), EnhancedKYCParams{
		Country: "NG", IDType: "NIN", IDNumber: "1",
		UserDetails: UserDetails{GivenNames: "John", LastName: "Doe"},
		Consent:     validConsent(),
	})
	assertValidationError(t, err)
}

func TestUserDetailsWithPhoneOnlyIsValid(t *testing.T) {
	ud := UserDetails{GivenNames: "John", LastName: "Doe", PhoneNumber: String("+2348012345678")}
	if err := validateUserDetails(ud); err != nil {
		t.Errorf("phone-only user details should be valid: %v", err)
	}
}

func TestVerifyEnhancedRequiresIDType(t *testing.T) {
	c := failingClient(t)
	_, err := c.Documents.VerifyEnhanced(context.Background(), DocumentVerificationParams{
		Country:        "NG",
		SelfieImage:    FromBytes(jpegBytes, "s.jpg"),
		LivenessImages: livenessSet(6),
		Document:       FromBytes(jpegBytes, "d.jpg"),
		UserDetails:    validUserDetails(),
		Consent:        validConsent(),
	})
	assertValidationError(t, err)
}

func TestAuthenticationRequiresImagesUnlessEnrolled(t *testing.T) {
	c := failingClient(t)
	_, err := c.Biometric.Authenticate(context.Background(), AuthenticationParams{
		UserID:      "user_1",
		UserDetails: validUserDetails(),
		Consent:     validConsent(),
	})
	assertValidationError(t, err)
}

func TestAuthenticationWithEnrolledImageSkipsImageCheck(t *testing.T) {
	err := validateAuthentication(AuthenticationParams{
		UserID:           "user_1",
		UseEnrolledImage: Bool(true),
		UserDetails:      validUserDetails(),
		Consent:          validConsent(),
	})
	if err != nil {
		t.Errorf("use_enrolled_image should skip the image requirement: %v", err)
	}
}

func TestReportFraudConditionalRules(t *testing.T) {
	tests := []struct {
		name    string
		params  ReportFraudParams
		wantErr bool
	}{
		{"flag needs reason", ReportFraudParams{IsFraud: true, ReportedBy: "r@e.com"}, true},
		{"flag with valid reason ok", ReportFraudParams{IsFraud: true, Reason: String(ReasonAccountTakeover), ReportedBy: "r@e.com"}, false},
		{"flag with invalid reason", ReportFraudParams{IsFraud: true, Reason: String("NOPE"), ReportedBy: "r@e.com"}, true},
		{"reason OTHER needs notes", ReportFraudParams{IsFraud: true, Reason: String(ReasonOther), ReportedBy: "r@e.com"}, true},
		{"reason OTHER with notes ok", ReportFraudParams{IsFraud: true, Reason: String(ReasonOther), Notes: String("n"), ReportedBy: "r@e.com"}, false},
		{"clear needs notes", ReportFraudParams{IsFraud: false, ReportedBy: "r@e.com"}, true},
		{"clear with notes ok", ReportFraudParams{IsFraud: false, Notes: String("cleared"), ReportedBy: "r@e.com"}, false},
		{"reported_by required", ReportFraudParams{IsFraud: false, Notes: String("n")}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReportFraud(tt.params)
			if tt.wantErr && err == nil {
				t.Error("expected validation error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}
