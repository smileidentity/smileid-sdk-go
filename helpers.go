package usesmileid

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/smileidentity/smileid-sdk-go/v13/generated/models"
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
	if p.SelfieImage == nil {
		return validationErrorf("selfie_image is required for authentication")
	}
	if len(p.LivenessImages) == 0 {
		return validationErrorf("liveness_images are required for authentication")
	}
	return validateUserDetails(p.UserDetails)
}

func validateBiometricKYC(p BiometricKYCParams, useEnrolledImage bool) error {
	if useEnrolledImage {
		if p.UserID == nil || strings.TrimSpace(*p.UserID) == "" {
			return validationErrorf("user_id is required when use_enrolled_image is true")
		}
	} else {
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

// WaitUntilComplete polls Verifications.Retrieve until the job reaches a
// terminal decision — any status other than "processing" and "not_found", so
// "clear", "block", "attention" or "error" — and returns that JobStatus. It
// returns a *TimeoutError if the timeout elapses first. If
// TreatNotFoundAsPending is false it returns a not_found status instead of
// continuing to poll.
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
		if js.IsComplete() {
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

var (
	birthYearPattern      = regexp.MustCompile(`^\d{4}$`)
	idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)
)

func validateAMLScreening(countries []string, aliases []string, birthYear *string, searchExistingUser *bool, userID *string) error {
	if len(countries) == 0 {
		return validationErrorf("countries is required")
	}
	for _, c := range countries {
		if len(c) != 2 {
			return validationErrorf("countries must be two-letter ISO 3166-1 alpha-2 codes")
		}
	}
	if len(aliases) > 20 {
		return validationErrorf("aliases must contain at most 20 entries")
	}
	if birthYear != nil && !birthYearPattern.MatchString(*birthYear) {
		return validationErrorf("birth_year must be a four digit year")
	}
	if searchExistingUser != nil && *searchExistingUser && (userID == nil || strings.TrimSpace(*userID) == "") {
		return validationErrorf("user_id is required when search_existing_user is true")
	}
	return nil
}

func validateOneTimeAML(p OneTimeAMLParams) error {
	if strings.TrimSpace(p.FullName) == "" {
		return validationErrorf("full_name is required for AML screening")
	}
	if err := validateAMLScreening(p.Countries, p.Aliases, p.BirthYear, p.SearchExistingUser, p.UserID); err != nil {
		return err
	}
	return validateUserDetails(p.UserDetails)
}

func validateAMLMonitoring(p AMLMonitoringParams) error {
	searching := p.SearchExistingUser != nil && *p.SearchExistingUser
	if !searching && (p.FullName == nil || strings.TrimSpace(*p.FullName) == "") {
		return validationErrorf("full_name is required unless search_existing_user is true")
	}
	if err := validateAMLScreening(p.Countries, p.Aliases, p.BirthYear, p.SearchExistingUser, p.UserID); err != nil {
		return err
	}
	return validateUserDetails(p.UserDetails)
}

func validateAMLMonitoringNews(jobID, reference string, p AMLMonitoringNewsParams) error {
	if strings.TrimSpace(jobID) == "" {
		return validationErrorf("job_id is required")
	}
	if strings.TrimSpace(reference) == "" {
		return validationErrorf("reference is required")
	}
	if !slices.Contains(models.AMLNewsCategories, p.NewsCategory) {
		return validationErrorf("news_category must be one of %s", strings.Join(models.AMLNewsCategories, ", "))
	}
	if p.Page != nil && (*p.Page < 1 || *p.Page > 100) {
		return validationErrorf("page must be between 1 and 100")
	}
	if p.Limit != nil && (*p.Limit < 1 || *p.Limit > 100) {
		return validationErrorf("limit must be between 1 and 100")
	}
	return nil
}

var (
	e164Pattern        = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)
	countryCodePattern = regexp.MustCompile(`^[A-Z]{2}$`)
	phoneOperators     = []string{"MTN", "VODAFONE", "AIRTELTIGO"}
	utilityTypes       = []string{"PrePaid", "PostPaid"}
	addressCountries   = []string{"NG", "ZA"}
)

func validatePhoneVerification(p PhoneVerificationParams) error {
	if !countryCodePattern.MatchString(p.Country) {
		return validationErrorf("country must be a two-letter uppercase ISO 3166-1 alpha-2 code")
	}
	if !e164Pattern.MatchString(p.PhoneNumber) {
		return validationErrorf("phone_number must be in E.164 format, for example +233201234567")
	}
	if p.Operator != nil && !slices.Contains(phoneOperators, *p.Operator) {
		return validationErrorf("operator must be one of %s", strings.Join(phoneOperators, ", "))
	}
	if p.Country == "GH" && p.Operator == nil {
		return validationErrorf("operator is required for Ghana")
	}
	if p.IDNumber != nil && strings.TrimSpace(*p.IDNumber) == "" {
		return validationErrorf("id_number must not be empty")
	}
	if p.Country == "KE" && p.IDNumber == nil {
		return validationErrorf("id_number is required for Kenya")
	}
	return validateUserDetails(p.UserDetails)
}

func validateAddressVerification(p AddressVerificationParams) error {
	if strings.TrimSpace(p.Address) == "" || len(p.Address) > 512 {
		return validationErrorf("address is required and must be at most 512 characters")
	}
	if !slices.Contains(addressCountries, p.Country) {
		return validationErrorf("country must be one of %s", strings.Join(addressCountries, ", "))
	}
	idSet := p.IDNumber != nil && strings.TrimSpace(*p.IDNumber) != ""
	utilitySet := p.UtilityNumber != nil && strings.TrimSpace(*p.UtilityNumber) != ""
	if !idSet && !utilitySet {
		return validationErrorf("id_number or utility_number is required")
	}
	if p.UtilityType != nil && !slices.Contains(utilityTypes, *p.UtilityType) {
		return validationErrorf("utility_type must be one of %s", strings.Join(utilityTypes, ", "))
	}
	if p.Country == "ZA" && !idSet {
		return validationErrorf("id_number is required for South Africa")
	}
	if p.Country == "NG" && (!utilitySet || p.UtilityProvider == nil || strings.TrimSpace(*p.UtilityProvider) == "" || p.UtilityType == nil) {
		return validationErrorf("utility_number, utility_provider and utility_type are required for Nigeria")
	}
	return validateUserDetails(p.UserDetails)
}

func validateSmileLinkIDTypes(idTypes []SmileLinkIDType) error {
	if len(idTypes) == 0 {
		return validationErrorf("id_types must contain at least one entry")
	}
	for _, t := range idTypes {
		if strings.TrimSpace(t.Country) == "" || strings.TrimSpace(t.IDType) == "" || strings.TrimSpace(t.VerificationMethod) == "" {
			return validationErrorf("each id_types entry needs country, id_type and verification_method")
		}
	}
	return nil
}

func validateCreateSmileLink(p CreateSmileLinkParams) error {
	if err := validateSmileLinkIDTypes(p.IDTypes); err != nil {
		return err
	}
	if strings.TrimSpace(p.Name) == "" {
		return validationErrorf("name is required")
	}
	if err := validateHTTPSURL("notice_privacy_policy_url", p.NoticePrivacyPolicyURL); err != nil {
		return err
	}
	if p.CompanyName != nil && len(*p.CompanyName) > 255 {
		return validationErrorf("company_name must be at most 255 characters")
	}
	if p.ThemeColor != nil && len(*p.ThemeColor) > 32 {
		return validationErrorf("theme_color must be at most 32 characters")
	}
	if p.ExpiresAt != nil {
		if _, err := time.Parse(time.RFC3339, *p.ExpiresAt); err != nil {
			return validationErrorf("expires_at must be an RFC 3339 date-time")
		}
	}
	for name, v := range map[string]*string{"logo_url": p.LogoURL, "redirect_url": p.RedirectURL} {
		if v != nil {
			if err := validateHTTPSURL(name, *v); err != nil {
				return err
			}
		}
	}
	if p.UserID != nil && (*p.UserID == "" || *p.UserID != strings.TrimSpace(*p.UserID)) {
		return validationErrorf("user_id must not be empty or padded with whitespace")
	}
	return nil
}

func validateListSmileLinks(p ListSmileLinksParams) error {
	if p.Limit != nil && (*p.Limit < 1 || *p.Limit > 100) {
		return validationErrorf("limit must be between 1 and 100")
	}
	if p.Page != nil && *p.Page < 1 {
		return validationErrorf("page must be at least 1")
	}
	return nil
}

func validateUpdateSmileLink(refID string, p UpdateSmileLinkParams) error {
	if strings.TrimSpace(refID) == "" {
		return validationErrorf("ref_id is required")
	}
	if p.IDTypes == nil && p.IsDisabled == nil {
		return validationErrorf("id_types or is_disabled is required")
	}
	if p.IDTypes == nil {
		return nil
	}
	if len(p.IDTypes) > 50 {
		return validationErrorf("id_types must contain at most 50 entries")
	}
	return validateSmileLinkIDTypes(p.IDTypes)
}
