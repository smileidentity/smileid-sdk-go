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

func validateCommon(country string, consent Consent, ud UserDetails, callbackURL *string) error {
	if strings.TrimSpace(country) == "" {
		return validationErrorf("country is required")
	}
	if err := validateConsent(consent); err != nil {
		return err
	}
	if err := validateUserDetails(ud); err != nil {
		return err
	}
	return validateOptionalCallback(callbackURL)
}

func validateConsent(c Consent) error {
	if !c.Granted {
		return validationErrorf("consent.granted must be true")
	}
	if strings.TrimSpace(c.GrantedAt) == "" {
		return validationErrorf("consent.granted_at is required")
	}
	if strings.TrimSpace(c.NoticeLanguage) == "" {
		return validationErrorf("consent.notice_language is required")
	}
	if strings.TrimSpace(c.NoticePrivacyPolicyURL) == "" {
		return validationErrorf("consent.notice_privacy_policy_url is required")
	}
	return nil
}

func validateOptionalCallback(callbackURL *string) error {
	if callbackURL == nil || strings.TrimSpace(*callbackURL) == "" {
		return nil
	}
	return validateCallbackURL(*callbackURL)
}

func validateLivenessImages(images []*BinaryInput) error {
	if len(images) < 6 || len(images) > 8 {
		return validationErrorf("liveness_images must contain 6 to 8 images")
	}
	for i, img := range images {
		if img == nil {
			return validationErrorf("liveness_images[%d] is required", i)
		}
	}
	return nil
}

func validateBinary(name string, b *BinaryInput) error {
	if b == nil {
		return validationErrorf("%s is required", name)
	}
	return nil
}

func validateEnhancedKYC(p EnhancedKYCParams) error {
	if strings.TrimSpace(p.IDType) == "" {
		return validationErrorf("id_type is required")
	}
	if strings.TrimSpace(p.IDNumber) == "" {
		return validationErrorf("id_number is required")
	}
	return validateCommon(p.Country, p.Consent, p.UserDetails, p.CallbackURL)
}

func validateDocumentVerification(p DocumentVerificationParams) error {
	if err := validateCommon(p.Country, p.Consent, p.UserDetails, p.CallbackURL); err != nil {
		return err
	}
	if err := validateBinary("selfie_image", p.SelfieImage); err != nil {
		return err
	}
	if err := validateLivenessImages(p.LivenessImages); err != nil {
		return err
	}
	return validateBinary("document", p.Document)
}

func validateBiometricKYC(p BiometricKYCParams) error {
	if strings.TrimSpace(p.IDType) == "" {
		return validationErrorf("id_type is required")
	}
	if strings.TrimSpace(p.IDNumber) == "" {
		return validationErrorf("id_number is required")
	}
	if err := validateCommon(p.Country, p.Consent, p.UserDetails, p.CallbackURL); err != nil {
		return err
	}
	if err := validateBinary("selfie_image", p.SelfieImage); err != nil {
		return err
	}
	return validateLivenessImages(p.LivenessImages)
}

func validateRegistration(p RegistrationParams) error {
	if err := validateConsent(p.Consent); err != nil {
		return err
	}
	if err := validateUserDetails(p.UserDetails); err != nil {
		return err
	}
	if err := validateOptionalCallback(p.CallbackURL); err != nil {
		return err
	}
	if err := validateBinary("selfie_image", p.SelfieImage); err != nil {
		return err
	}
	return validateLivenessImages(p.LivenessImages)
}

func validateAuthentication(p AuthenticationParams) error {
	if strings.TrimSpace(p.UserID) == "" {
		return validationErrorf("user_id is required for authentication")
	}
	if err := validateConsent(p.Consent); err != nil {
		return err
	}
	if err := validateOptionalCallback(p.CallbackURL); err != nil {
		return err
	}
	useEnrolled := p.UseEnrolledImage != nil && *p.UseEnrolledImage
	if !useEnrolled {
		if p.SelfieImage == nil {
			return validationErrorf("selfie_image is required unless use_enrolled_image is true")
		}
		if err := validateLivenessImages(p.LivenessImages); err != nil {
			return err
		}
	}
	return validateUserDetails(p.UserDetails)
}

func validateCompare(p CompareParams) error {
	if err := validateConsent(p.Consent); err != nil {
		return err
	}
	if err := validateUserDetails(p.UserDetails); err != nil {
		return err
	}
	if err := validateOptionalCallback(p.CallbackURL); err != nil {
		return err
	}
	if err := validateBinary("selfie_image", p.SelfieImage); err != nil {
		return err
	}
	if err := validateBinary("comparison_image", p.ComparisonImage); err != nil {
		return err
	}
	if strings.TrimSpace(p.ComparisonImageType) == "" {
		return validationErrorf("comparison_image_type is required")
	}
	if p.ComparisonImageType != ComparisonImageTypeDocument &&
		p.ComparisonImageType != ComparisonImageTypeIDPhoto &&
		p.ComparisonImageType != ComparisonImageTypePortrait {
		return validationErrorf("comparison_image_type must be one of %s, %s, %s", ComparisonImageTypeDocument, ComparisonImageTypeIDPhoto, ComparisonImageTypePortrait)
	}
	if len(p.LivenessImages) > 0 {
		return validateLivenessImages(p.LivenessImages)
	}
	return nil
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

	parent := ctx
	deadline := time.Now().Add(timeout)
	pollCtx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()

	for {
		js, err := r.Retrieve(pollCtx, jobID)
		if err != nil {
			// The poll deadline can expire while a Retrieve is in flight; the
			// transport surfaces that as a ConnectionError wrapping
			// context.DeadlineExceeded. Convert it to a TimeoutError, but only
			// when it is attributable to the poll deadline: if the caller's
			// own context is done, their cancellation or deadline wins and the
			// original error is returned unchanged.
			if parent.Err() == nil && pollCtx.Err() != nil && errors.Is(err, context.DeadlineExceeded) {
				return nil, timeoutError(jobID, timeout)
			}
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
		case <-pollCtx.Done():
			timer.Stop()
			if parent.Err() != nil {
				return nil, connectionError(parent.Err())
			}
			return nil, timeoutError(jobID, timeout)
		case <-timer.C:
		}
	}
}

func timeoutError(jobID string, timeout time.Duration) error {
	return &TimeoutError{&smileIDError{
		Message: fmt.Sprintf("wait_until_complete timed out after %s waiting for job %s", timeout, jobID),
	}}
}
