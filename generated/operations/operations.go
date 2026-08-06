// Package operations holds one thin function per Smile ID V3 operation. Each
// function maps typed parameters onto a neutral [Request] and hands it to a
// [Doer] (the hand-written transport in the root package). This package is
// owned by the code generator; the transport, auth, error and helper logic
// lives outside it so a future generator run can own this tree without
// clobbering customization.
package operations

import (
	"context"
	"net/url"
	"strconv"

	"github.com/smileidentity/smileid-sdk-go/v12/generated/models"
)

// BodyKind describes how a request body is encoded.
type BodyKind int

const (
	// BodyNone is a request with no body (GETs).
	BodyNone BodyKind = iota
	// BodyMultipart is a multipart/form-data body (entry endpoints, report_fraud).
	BodyMultipart
	// BodyJSON is an application/json body (replay only).
	BodyJSON
)

// PartKind describes how a single multipart part is encoded.
type PartKind int

const (
	// PartScalar is a plain text part.
	PartScalar PartKind = iota
	// PartJSON is a JSON part with Content-Type: application/json.
	PartJSON
	// PartBinary is a single binary part with a filename.
	PartBinary
	// PartBinaryArray is a repeated binary part (one part per element).
	PartBinaryArray
)

// Part is a single multipart part.
type Part struct {
	Name   string
	Kind   PartKind
	Scalar string
	JSON   interface{}
	Binary *models.BinaryInput
	Array  []*models.BinaryInput
}

// Request is the neutral description of an HTTP operation that the transport
// executes.
type Request struct {
	Method               string
	Path                 string
	Query                url.Values
	Headers              map[string]string // operation-specific header overrides
	UserIDHeader         string            // sent as the User-ID header when non-empty
	Authenticated        bool              // attach a bearer token
	Idempotent           bool              // eligible for automatic retries
	NeedsPartnerIDHeader bool              // send SmileID-Partner-ID
	NotFoundReturnsBody  bool              // decode a 404 body instead of raising
	BodyKind             BodyKind
	Parts                []Part
	JSONBody             interface{}
}

// Doer executes a Request and decodes a successful response into out.
type Doer interface {
	Do(ctx context.Context, req *Request, out interface{}) error
}

func (r *Request) scalar(name, value string) {
	r.Parts = append(r.Parts, Part{Name: name, Kind: PartScalar, Scalar: value})
}

func (r *Request) jsonPart(name string, v interface{}) {
	r.Parts = append(r.Parts, Part{Name: name, Kind: PartJSON, JSON: v})
}

func (r *Request) binary(name string, b *models.BinaryInput) {
	r.Parts = append(r.Parts, Part{Name: name, Kind: PartBinary, Binary: b})
}

func (r *Request) binaryArray(name string, arr []*models.BinaryInput) {
	r.Parts = append(r.Parts, Part{Name: name, Kind: PartBinaryArray, Array: arr})
}

func boolStr(b bool) string { return strconv.FormatBool(b) }

func floatStr(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func addOptionalCommon(r *Request, partnerParams map[string]string, metadata []models.MetadataEntry) {
	if partnerParams != nil {
		r.jsonPart("partner_params", partnerParams)
	}
	if len(metadata) > 0 {
		r.jsonPart("metadata", metadata)
	}
}

// EnhancedKYC builds and sends POST /v3/enhanced_kyc.
func EnhancedKYC(ctx context.Context, d Doer, p models.EnhancedKYCParams, out *models.AcceptedResponse) error {
	r := &Request{Method: "POST", Path: "/v3/enhanced_kyc", Authenticated: true, BodyKind: BodyMultipart}
	r.scalar("country", p.Country)
	r.scalar("id_type", p.IDType)
	r.scalar("id_number", p.IDNumber)
	r.jsonPart("user_details", p.UserDetails)
	r.jsonPart("consent", p.Consent)
	if p.CallbackURL != nil {
		r.scalar("callback_url", *p.CallbackURL)
	}
	if p.BankCode != nil {
		r.scalar("bank_code", *p.BankCode)
	}
	if p.Operator != nil {
		r.scalar("operator", *p.Operator)
	}
	addOptionalCommon(r, p.PartnerParams, p.Metadata)
	if p.UserID != nil {
		r.UserIDHeader = *p.UserID
	}
	return d.Do(ctx, r, out)
}

// DocumentVerification builds and sends POST /v3/document_verification.
func DocumentVerification(ctx context.Context, d Doer, p models.DocumentVerificationParams, out *models.AcceptedResponse) error {
	r := &Request{Method: "POST", Path: "/v3/document_verification", Authenticated: true, NeedsPartnerIDHeader: true, BodyKind: BodyMultipart}
	buildDocParts(r, p)
	return d.Do(ctx, r, out)
}

// EnhancedDocumentVerification builds and sends POST /v3/enhanced_document_verification.
func EnhancedDocumentVerification(ctx context.Context, d Doer, p models.DocumentVerificationParams, out *models.AcceptedResponse) error {
	r := &Request{Method: "POST", Path: "/v3/enhanced_document_verification", Authenticated: true, NeedsPartnerIDHeader: true, BodyKind: BodyMultipart}
	buildDocParts(r, p)
	return d.Do(ctx, r, out)
}

func buildDocParts(r *Request, p models.DocumentVerificationParams) {
	r.scalar("country", p.Country)
	if p.IDType != nil {
		r.scalar("id_type", *p.IDType)
	}
	r.binary("selfie_image", p.SelfieImage)
	r.binaryArray("liveness_images", p.LivenessImages)
	r.binary("document", p.Document)
	if p.DocumentBack != nil {
		r.binary("document_back", p.DocumentBack)
	}
	r.jsonPart("user_details", p.UserDetails)
	r.jsonPart("consent", p.Consent)
	if p.CallbackURL != nil {
		r.scalar("callback_url", *p.CallbackURL)
	}
	addOptionalCommon(r, p.PartnerParams, p.Metadata)
	if p.UserID != nil {
		r.UserIDHeader = *p.UserID
	}
}

// BiometricKYC builds and sends POST /v3/biometric_kyc.
func BiometricKYC(ctx context.Context, d Doer, p models.BiometricKYCParams, out *models.AcceptedResponse) error {
	r := &Request{Method: "POST", Path: "/v3/biometric_kyc", Authenticated: true, NeedsPartnerIDHeader: true, BodyKind: BodyMultipart}
	r.scalar("country", p.Country)
	r.scalar("id_type", p.IDType)
	r.scalar("id_number", p.IDNumber)
	r.binary("selfie_image", p.SelfieImage)
	r.binaryArray("liveness_images", p.LivenessImages)
	r.jsonPart("user_details", p.UserDetails)
	r.jsonPart("consent", p.Consent)
	if p.CallbackURL != nil {
		r.scalar("callback_url", *p.CallbackURL)
	}
	if p.SandboxResult != nil {
		r.scalar("sandbox_result", floatStr(*p.SandboxResult))
	}
	addOptionalCommon(r, p.PartnerParams, p.Metadata)
	if p.UserID != nil {
		r.UserIDHeader = *p.UserID
	}
	return d.Do(ctx, r, out)
}

// Registration builds and sends POST /v3/registration.
func Registration(ctx context.Context, d Doer, p models.RegistrationParams, out *models.AcceptedResponse) error {
	r := &Request{Method: "POST", Path: "/v3/registration", Authenticated: true, BodyKind: BodyMultipart}
	r.binary("selfie_image", p.SelfieImage)
	r.binaryArray("liveness_images", p.LivenessImages)
	r.jsonPart("user_details", p.UserDetails)
	r.jsonPart("consent", p.Consent)
	if p.AllowNewEnroll != nil {
		r.scalar("allow_new_enroll", boolStr(*p.AllowNewEnroll))
	}
	if p.CallbackURL != nil {
		r.scalar("callback_url", *p.CallbackURL)
	}
	if p.SandboxResult != nil {
		r.scalar("sandbox_result", floatStr(*p.SandboxResult))
	}
	addOptionalCommon(r, p.PartnerParams, p.Metadata)
	if p.UserID != nil {
		r.UserIDHeader = *p.UserID
	}
	return d.Do(ctx, r, out)
}

// Authentication builds and sends POST /v3/authentication. The user_id is a
// body field for this operation, not the User-ID header.
func Authentication(ctx context.Context, d Doer, p models.AuthenticationParams, out *models.AcceptedResponse) error {
	r := &Request{Method: "POST", Path: "/v3/authentication", Authenticated: true, BodyKind: BodyMultipart}
	r.scalar("user_id", p.UserID)
	if p.SelfieImage != nil {
		r.binary("selfie_image", p.SelfieImage)
	}
	if len(p.LivenessImages) > 0 {
		r.binaryArray("liveness_images", p.LivenessImages)
	}
	r.jsonPart("user_details", p.UserDetails)
	r.jsonPart("consent", p.Consent)
	if p.UseEnrolledImage != nil {
		r.scalar("use_enrolled_image", boolStr(*p.UseEnrolledImage))
	}
	if p.CallbackURL != nil {
		r.scalar("callback_url", *p.CallbackURL)
	}
	if p.SandboxResult != nil {
		r.scalar("sandbox_result", floatStr(*p.SandboxResult))
	}
	addOptionalCommon(r, p.PartnerParams, p.Metadata)
	return d.Do(ctx, r, out)
}

// Compare builds and sends POST /v3/compare. The optional user_id is a body field.
func Compare(ctx context.Context, d Doer, p models.CompareParams, out *models.AcceptedResponse) error {
	r := &Request{Method: "POST", Path: "/v3/compare", Authenticated: true, BodyKind: BodyMultipart}
	r.binary("selfie_image", p.SelfieImage)
	r.binary("comparison_image", p.ComparisonImage)
	r.scalar("comparison_image_type", p.ComparisonImageType)
	r.jsonPart("user_details", p.UserDetails)
	r.jsonPart("consent", p.Consent)
	if len(p.LivenessImages) > 0 {
		r.binaryArray("liveness_images", p.LivenessImages)
	}
	if p.AllowNewEnroll != nil {
		r.scalar("allow_new_enroll", boolStr(*p.AllowNewEnroll))
	}
	if p.UserID != nil {
		r.scalar("user_id", *p.UserID)
	}
	if p.CallbackURL != nil {
		r.scalar("callback_url", *p.CallbackURL)
	}
	if p.SandboxResult != nil {
		r.scalar("sandbox_result", floatStr(*p.SandboxResult))
	}
	addOptionalCommon(r, p.PartnerParams, p.Metadata)
	return d.Do(ctx, r, out)
}

// RetrieveStatus builds and sends GET /v3/status/{jobId}. A 404 is decoded
// into the JobStatus body (status "not_found") rather than raising.
func RetrieveStatus(ctx context.Context, d Doer, jobID string, out *models.JobStatus) error {
	r := &Request{
		Method:              "GET",
		Path:                "/v3/status/" + url.PathEscape(jobID),
		Authenticated:       true,
		Idempotent:          true,
		NotFoundReturnsBody: true,
	}
	return d.Do(ctx, r, out)
}

// Replay builds and sends POST /v3/replay/{job_id}. A callback override is
// sent as a multipart body with one callback_url text part; with no override
// no body is sent at all.
func Replay(ctx context.Context, d Doer, jobID string, p models.ReplayParams, out *models.ReplayCallbackResponse) error {
	r := &Request{
		Method:        "POST",
		Path:          "/v3/replay/" + url.PathEscape(jobID),
		Authenticated: true,
	}
	if p.CallbackURL != nil {
		r.BodyKind = BodyMultipart
		r.scalar("callback_url", *p.CallbackURL)
	}
	return d.Do(ctx, r, out)
}

// ReportFraud builds and sends POST /v3/users/{user_id}/report_fraud.
func ReportFraud(ctx context.Context, d Doer, userID string, p models.ReportFraudParams, out *models.ReportUserFraudResponse) error {
	r := &Request{
		Method:        "POST",
		Path:          "/v3/users/" + url.PathEscape(userID) + "/report_fraud",
		Authenticated: true,
		BodyKind:      BodyMultipart,
	}
	r.scalar("is_fraud", boolStr(p.IsFraud))
	r.scalar("reported_by", p.ReportedBy)
	if p.Reason != nil {
		r.scalar("reason", *p.Reason)
	}
	if p.Notes != nil {
		r.scalar("notes", *p.Notes)
	}
	return d.Do(ctx, r, out)
}

// BankCodes builds and sends GET /v3/services/bank_codes (unauthenticated).
func BankCodes(ctx context.Context, d Doer, p models.BankCodesParams, out *models.BankCodesResponse) error {
	r := &Request{Method: "GET", Path: "/v3/services/bank_codes", Idempotent: true, Query: url.Values{}}
	if p.Country != nil {
		r.Query.Set("country", *p.Country)
	}
	return d.Do(ctx, r, out)
}

// SupportedIDTypes builds and sends GET /v3/services/supported_id_types (unauthenticated).
func SupportedIDTypes(ctx context.Context, d Doer, p models.SupportedIDTypesParams, out *models.SupportedIDTypesResponse) error {
	r := &Request{Method: "GET", Path: "/v3/services/supported_id_types", Idempotent: true, Query: url.Values{}}
	if p.Country != nil {
		r.Query.Set("country", *p.Country)
	}
	return d.Do(ctx, r, out)
}

// SupportedDocuments builds and sends GET /v3/services/supported_documents (unauthenticated).
func SupportedDocuments(ctx context.Context, d Doer, p models.SupportedDocumentsParams, out *models.SupportedDocumentsResponse) error {
	r := &Request{Method: "GET", Path: "/v3/services/supported_documents", Idempotent: true, Query: url.Values{}}
	if p.Continent != nil {
		r.Query.Set("continent", *p.Continent)
	}
	if p.CountryCode != nil {
		r.Query.Set("country_code", *p.CountryCode)
	}
	if p.Locale != nil {
		r.Query.Set("locale", *p.Locale)
	}
	return d.Do(ctx, r, out)
}

// IDStatus builds and sends GET /v3/services/id_status (authenticated).
func IDStatus(ctx context.Context, d Doer, p models.IDStatusParams, out *models.IDStatusResponse) error {
	r := &Request{Method: "GET", Path: "/v3/services/id_status", Authenticated: true, Idempotent: true, Query: url.Values{}}
	r.Query.Set("country", p.Country)
	r.Query.Set("id_type", p.IDType)
	return d.Do(ctx, r, out)
}
