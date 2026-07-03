package smileid

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/smileidentity/smileid-sdk-go/v12/generated/models"
)

// GrantConsent builds a granted Consent with the required fields set.
// grantedAt is formatted as an ISO 8601 UTC timestamp with milliseconds.
func GrantConsent(grantedAt time.Time, noticeLanguage, noticePrivacyPolicyURL string) Consent {
	return Consent{
		Granted:                true,
		GrantedAt:              grantedAt.UTC().Format(timestampLayout),
		NoticeLanguage:         noticeLanguage,
		NoticePrivacyPolicyURL: noticePrivacyPolicyURL,
	}
}

// Ptr returns a pointer to v. It is a convenience for setting the optional
// pointer fields on params structs.
func Ptr[T any](v T) *T { return &v }

// String returns a pointer to s.
func String(s string) *string { return &s }

// Bool returns a pointer to b.
func Bool(b bool) *bool { return &b }

// Float64 returns a pointer to f.
func Float64(f float64) *float64 { return &f }

func validateUserDetails(ud UserDetails) error {
	if strings.TrimSpace(ud.GivenNames) == "" {
		return validationErrorf("user_details.given_names is required")
	}
	if strings.TrimSpace(ud.LastName) == "" {
		return validationErrorf("user_details.last_name is required")
	}
	emailSet := ud.Email != nil && *ud.Email != ""
	phoneSet := ud.PhoneNumber != nil && *ud.PhoneNumber != ""
	if !emailSet && !phoneSet {
		return validationErrorf("user_details requires at least one of email or phone_number")
	}
	return nil
}

func validateAuthentication(p AuthenticationParams) error {
	if strings.TrimSpace(p.UserID) == "" {
		return validationErrorf("user_id is required for authentication")
	}
	useEnrolled := p.UseEnrolledImage != nil && *p.UseEnrolledImage
	if !useEnrolled {
		if p.SelfieImage == nil {
			return validationErrorf("selfie_image is required unless use_enrolled_image is true")
		}
		if len(p.LivenessImages) == 0 {
			return validationErrorf("liveness_images are required unless use_enrolled_image is true")
		}
	}
	return validateUserDetails(p.UserDetails)
}

func validateReportFraud(p ReportFraudParams) error {
	if strings.TrimSpace(p.ReportedBy) == "" {
		return validationErrorf("reported_by is required")
	}
	if p.IsFraud {
		if p.Reason == nil || *p.Reason == "" {
			return validationErrorf("reason is required when is_fraud is true")
		}
		if !validFraudReason(*p.Reason) {
			return validationErrorf("reason must be one of %s", strings.Join(models.FraudReasons, ", "))
		}
		if *p.Reason == models.ReasonOther && (p.Notes == nil || *p.Notes == "") {
			return validationErrorf("notes is required when reason is OTHER")
		}
	} else {
		if p.Notes == nil || *p.Notes == "" {
			return validationErrorf("notes is required when is_fraud is false")
		}
	}
	return nil
}

func validFraudReason(reason string) bool {
	for _, r := range models.FraudReasons {
		if r == reason {
			return true
		}
	}
	return false
}

// WaitOptions tunes WaitUntilComplete. Zero values fall back to the defaults:
// Interval 2s, Timeout 60s, and TreatNotFoundAsPending true (nil pointer).
type WaitOptions struct {
	// Interval is the delay between polls. Defaults to 2s.
	Interval time.Duration
	// Timeout is the maximum time to poll. Defaults to 60s.
	Timeout time.Duration
	// TreatNotFoundAsPending keeps polling on a not_found status. A nil
	// pointer means the default, true; set Bool(false) to return the
	// not_found status immediately.
	TreatNotFoundAsPending *bool
}

// WaitUntilComplete polls Verifications.Retrieve until the job completes,
// returning the terminal JobStatus. It returns a *TimeoutError if the timeout
// elapses first. If TreatNotFoundAsPending is false it returns a not_found
// status instead of continuing to poll.
func (r *VerificationsResource) WaitUntilComplete(ctx context.Context, jobID string, opts WaitOptions) (*JobStatus, error) {
	interval := opts.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	treatNotFoundAsPending := true
	if opts.TreatNotFoundAsPending != nil {
		treatNotFoundAsPending = *opts.TreatNotFoundAsPending
	}

	deadline := time.Now().Add(timeout)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	for {
		js, err := r.Retrieve(ctx, jobID)
		if err != nil {
			return nil, err
		}
		if js.Status == "complete" {
			return js, nil
		}
		if js.Status == "not_found" && !treatNotFoundAsPending {
			return js, nil
		}
		if !time.Now().Before(deadline) {
			return nil, timeoutError(jobID, timeout)
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, timeoutError(jobID, timeout)
			}
			return nil, connectionError(ctx.Err())
		case <-timer.C:
		}
	}
}

func timeoutError(jobID string, timeout time.Duration) error {
	return &TimeoutError{&smileIDError{
		Message: fmt.Sprintf("wait_until_complete timed out after %s waiting for job %s", timeout, jobID),
	}}
}
