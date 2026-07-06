package usesmileid

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestE2EEnhancedKYCSandbox submits a real Enhanced KYC job to the sandbox and
// polls it to completion. It skips (does not fail) unless SMILE_PARTNER_ID and
// SMILE_API_KEY are set. It uses the documented sandbox "clear" identity,
// which the sandbox matches on given names, last name and email.
//
// The credential values are never printed or logged.
func TestE2EEnhancedKYCSandbox(t *testing.T) {
	partnerID := os.Getenv("SMILE_PARTNER_ID")
	apiKey := os.Getenv("SMILE_API_KEY")
	if partnerID == "" || apiKey == "" {
		t.Skip("set SMILE_PARTNER_ID and SMILE_API_KEY to run the sandbox end-to-end test")
	}

	client, err := NewClient(Config{
		PartnerID:   partnerID,
		APIKey:      apiKey,
		Environment: Sandbox,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	accepted, err := client.EnhancedKYC.Verify(ctx, EnhancedKYCParams{
		Country:  "NG",
		IDType:   "NIN",
		IDNumber: "00000000000",
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
	if final.Status != "complete" {
		t.Fatalf("final status = %q, want complete", final.Status)
	}
}
