package smileid

import (
	"context"

	"github.com/smileidentity/smileid-sdk-go/v12/generated/operations"
)

// EnhancedKYCResource covers enhanced KYC verification.
type EnhancedKYCResource struct{ c *Client }

// Verify submits an enhanced KYC verification (POST /v3/enhanced_kyc).
func (r *EnhancedKYCResource) Verify(ctx context.Context, params EnhancedKYCParams, opts ...RequestOption) (*AcceptedResponse, error) {
	ro := resolveOptions(opts)
	if err := validateUserDetails(params.UserDetails); err != nil {
		return nil, err
	}
	cb, err := r.c.resolveCallback(params.CallbackURL, ro)
	if err != nil {
		return nil, err
	}
	params.CallbackURL = cb

	ctx, cancel := r.c.withTimeout(ctx, ro)
	defer cancel()

	var out AcceptedResponse
	if err := operations.EnhancedKYC(ctx, r.c.transport, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DocumentsResource covers document verification and enhanced document verification.
type DocumentsResource struct{ c *Client }

// Verify submits a document verification (POST /v3/document_verification).
func (r *DocumentsResource) Verify(ctx context.Context, params DocumentVerificationParams, opts ...RequestOption) (*AcceptedResponse, error) {
	ro := resolveOptions(opts)
	if err := validateUserDetails(params.UserDetails); err != nil {
		return nil, err
	}
	cb, err := r.c.resolveCallback(params.CallbackURL, ro)
	if err != nil {
		return nil, err
	}
	params.CallbackURL = cb

	ctx, cancel := r.c.withTimeout(ctx, ro)
	defer cancel()

	var out AcceptedResponse
	if err := operations.DocumentVerification(ctx, r.c.transport, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// VerifyEnhanced submits an enhanced document verification
// (POST /v3/enhanced_document_verification). id_type is required.
func (r *DocumentsResource) VerifyEnhanced(ctx context.Context, params DocumentVerificationParams, opts ...RequestOption) (*AcceptedResponse, error) {
	ro := resolveOptions(opts)
	if params.IDType == nil || *params.IDType == "" {
		return nil, validationErrorf("id_type is required for enhanced document verification")
	}
	if err := validateUserDetails(params.UserDetails); err != nil {
		return nil, err
	}
	cb, err := r.c.resolveCallback(params.CallbackURL, ro)
	if err != nil {
		return nil, err
	}
	params.CallbackURL = cb

	ctx, cancel := r.c.withTimeout(ctx, ro)
	defer cancel()

	var out AcceptedResponse
	if err := operations.EnhancedDocumentVerification(ctx, r.c.transport, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BiometricKYCResource covers biometric KYC verification.
type BiometricKYCResource struct{ c *Client }

// Verify submits a biometric KYC verification (POST /v3/biometric_kyc).
func (r *BiometricKYCResource) Verify(ctx context.Context, params BiometricKYCParams, opts ...RequestOption) (*AcceptedResponse, error) {
	ro := resolveOptions(opts)
	if err := validateUserDetails(params.UserDetails); err != nil {
		return nil, err
	}
	cb, err := r.c.resolveCallback(params.CallbackURL, ro)
	if err != nil {
		return nil, err
	}
	params.CallbackURL = cb

	ctx, cancel := r.c.withTimeout(ctx, ro)
	defer cancel()

	var out AcceptedResponse
	if err := operations.BiometricKYC(ctx, r.c.transport, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// BiometricResource covers enrollment, authentication and compare.
type BiometricResource struct{ c *Client }

// Enroll submits a biometric enrollment (POST /v3/registration).
func (r *BiometricResource) Enroll(ctx context.Context, params RegistrationParams, opts ...RequestOption) (*AcceptedResponse, error) {
	ro := resolveOptions(opts)
	if err := validateUserDetails(params.UserDetails); err != nil {
		return nil, err
	}
	cb, err := r.c.resolveCallback(params.CallbackURL, ro)
	if err != nil {
		return nil, err
	}
	params.CallbackURL = cb

	ctx, cancel := r.c.withTimeout(ctx, ro)
	defer cancel()

	var out AcceptedResponse
	if err := operations.Registration(ctx, r.c.transport, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Authenticate submits a biometric authentication (POST /v3/authentication).
func (r *BiometricResource) Authenticate(ctx context.Context, params AuthenticationParams, opts ...RequestOption) (*AcceptedResponse, error) {
	ro := resolveOptions(opts)
	if err := validateAuthentication(params); err != nil {
		return nil, err
	}
	cb, err := r.c.resolveCallback(params.CallbackURL, ro)
	if err != nil {
		return nil, err
	}
	params.CallbackURL = cb

	ctx, cancel := r.c.withTimeout(ctx, ro)
	defer cancel()

	var out AcceptedResponse
	if err := operations.Authentication(ctx, r.c.transport, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Compare submits a smart selfie compare (POST /v3/compare).
func (r *BiometricResource) Compare(ctx context.Context, params CompareParams, opts ...RequestOption) (*AcceptedResponse, error) {
	ro := resolveOptions(opts)
	if err := validateUserDetails(params.UserDetails); err != nil {
		return nil, err
	}
	cb, err := r.c.resolveCallback(params.CallbackURL, ro)
	if err != nil {
		return nil, err
	}
	params.CallbackURL = cb

	ctx, cancel := r.c.withTimeout(ctx, ro)
	defer cancel()

	var out AcceptedResponse
	if err := operations.Compare(ctx, r.c.transport, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// VerificationsResource covers status retrieval, polling and replay.
type VerificationsResource struct{ c *Client }

// Retrieve fetches a job's status (GET /v3/status/{jobId}). A not-found job is
// returned as a JobStatus with Status "not_found", not as an error.
func (r *VerificationsResource) Retrieve(ctx context.Context, jobID string, opts ...RequestOption) (*JobStatus, error) {
	ro := resolveOptions(opts)
	ctx, cancel := r.c.withTimeout(ctx, ro)
	defer cancel()

	var out JobStatus
	if err := operations.RetrieveStatus(ctx, r.c.transport, jobID, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Replay re-sends the callback for a completed verification
// (POST /v3/replay/{job_id}).
func (r *VerificationsResource) Replay(ctx context.Context, jobID string, params ReplayParams, opts ...RequestOption) (*ReplayCallbackResponse, error) {
	ro := resolveOptions(opts)
	if ro.callbackURL != nil {
		params.CallbackURL = ro.callbackURL
	}
	if params.CallbackURL != nil {
		if err := validateHTTPSURL("callback_url", *params.CallbackURL); err != nil {
			return nil, err
		}
	}

	ctx, cancel := r.c.withTimeout(ctx, ro)
	defer cancel()

	var out ReplayCallbackResponse
	if err := operations.Replay(ctx, r.c.transport, jobID, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UsersResource covers fraud reporting.
type UsersResource struct{ c *Client }

// ReportFraud reports (or clears) fraud for a user
// (POST /v3/users/{user_id}/report_fraud).
func (r *UsersResource) ReportFraud(ctx context.Context, userID string, params ReportFraudParams, opts ...RequestOption) (*ReportUserFraudResponse, error) {
	ro := resolveOptions(opts)
	if err := validateReportFraud(params); err != nil {
		return nil, err
	}

	ctx, cancel := r.c.withTimeout(ctx, ro)
	defer cancel()

	var out ReportUserFraudResponse
	if err := operations.ReportFraud(ctx, r.c.transport, userID, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FlagFraud flags a user as fraudulent (is_fraud=true).
func (r *UsersResource) FlagFraud(ctx context.Context, userID string, params FlagFraudParams, opts ...RequestOption) (*ReportUserFraudResponse, error) {
	reason := params.Reason
	return r.ReportFraud(ctx, userID, ReportFraudParams{
		IsFraud:    true,
		Reason:     &reason,
		Notes:      params.Notes,
		ReportedBy: params.ReportedBy,
	}, opts...)
}

// ClearFraud clears a previous fraud flag (is_fraud=false).
func (r *UsersResource) ClearFraud(ctx context.Context, userID string, params ClearFraudParams, opts ...RequestOption) (*ReportUserFraudResponse, error) {
	notes := params.Notes
	return r.ReportFraud(ctx, userID, ReportFraudParams{
		IsFraud:    false,
		Notes:      &notes,
		ReportedBy: params.ReportedBy,
	}, opts...)
}

// ServicesResource covers the four services endpoints.
type ServicesResource struct{ c *Client }

// BankCodes lists supported bank codes (GET /v3/services/bank_codes,
// unauthenticated).
func (r *ServicesResource) BankCodes(ctx context.Context, params BankCodesParams, opts ...RequestOption) (*BankCodesResponse, error) {
	ctx, cancel := r.c.withTimeout(ctx, resolveOptions(opts))
	defer cancel()

	var out BankCodesResponse
	if err := operations.BankCodes(ctx, r.c.transport, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SupportedIDTypes lists supported ID types (GET /v3/services/supported_id_types,
// unauthenticated).
func (r *ServicesResource) SupportedIDTypes(ctx context.Context, params SupportedIDTypesParams, opts ...RequestOption) (*SupportedIDTypesResponse, error) {
	ctx, cancel := r.c.withTimeout(ctx, resolveOptions(opts))
	defer cancel()

	var out SupportedIDTypesResponse
	if err := operations.SupportedIDTypes(ctx, r.c.transport, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SupportedDocuments lists supported documents
// (GET /v3/services/supported_documents, unauthenticated).
func (r *ServicesResource) SupportedDocuments(ctx context.Context, params SupportedDocumentsParams, opts ...RequestOption) (*SupportedDocumentsResponse, error) {
	ctx, cancel := r.c.withTimeout(ctx, resolveOptions(opts))
	defer cancel()

	var out SupportedDocumentsResponse
	if err := operations.SupportedDocuments(ctx, r.c.transport, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// IDStatus reports the status of an ID type (GET /v3/services/id_status,
// authenticated).
func (r *ServicesResource) IDStatus(ctx context.Context, params IDStatusParams, opts ...RequestOption) (*IDStatusResponse, error) {
	ctx, cancel := r.c.withTimeout(ctx, resolveOptions(opts))
	defer cancel()

	var out IDStatusResponse
	if err := operations.IDStatus(ctx, r.c.transport, params, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
