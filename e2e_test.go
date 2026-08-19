package usesmileid

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestE2EEnhancedKYCSandbox submits a real Enhanced KYC job and polls it to
// completion. It skips (does not fail) unless SMILE_PARTNER_ID and
// SMILE_API_KEY are set. It targets the sandbox unless SMILE_BASE_URL points it
// somewhere else. It uses the documented "clear" identity, which
// non-production environments match on given names, last name and email.
//
// The credential values are never printed or logged.
func TestE2EEnhancedKYCSandbox(t *testing.T) {
	partnerID := os.Getenv("SMILE_PARTNER_ID")
	apiKey := os.Getenv("SMILE_API_KEY")
	if partnerID == "" || apiKey == "" {
		t.Skip("set SMILE_PARTNER_ID and SMILE_API_KEY to run the sandbox end-to-end test")
	}

	cfg := Config{
		PartnerID:   partnerID,
		APIKey:      apiKey,
		Environment: Sandbox,
	}
	if baseURL := os.Getenv("SMILE_BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Logf("base URL: %s", client.cfg.baseURL)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	accepted, err := client.EnhancedKYC.Verify(ctx, EnhancedKYCParams{
		Country:  "NG",
		IDType:   "NIN",
		IDNumber: "12345678901",
		UserDetails: UserDetails{
			GivenNames: "Amina Fatou",
			LastName:   "Clearwater",
			Email:      String("amina.clearwater@example.com"),
		},
		Consent: GrantConsent(time.Now(), "EN", "https://example.com/privacy"),
	})
	if err != nil {
		t.Fatalf("EnhancedKYC.Verify: %v", err)
	}
	if !accepted.IsAccepted() {
		t.Fatalf("job was not accepted: status=%q", accepted.Status)
	}
	if accepted.JobID == "" {
		t.Fatal("no job_id returned")
	}

	final, err := client.Verifications.WaitUntilComplete(ctx, accepted.JobID, WaitOptions{
		Interval: 3 * time.Second,
		Timeout:  60 * time.Second,
	})
	if err != nil {
		t.Fatalf("WaitUntilComplete: %v", err)
	}
	if !final.IsComplete() {
		t.Fatalf("final status = %q, want a terminal decision", final.Status)
	}
	if final.Status != "clear" {
		t.Fatalf("final status = %q, want clear for the Clearwater identity", final.Status)
	}
}
