package models

// EnhancedKYCParams holds the request fields for enhanced KYC verification.
type EnhancedKYCParams struct {
	Country       string
	IDType        string
	IDNumber      string
	UserDetails   UserDetails
	Consent       Consent
	CallbackURL   *string
	BankCode      *string
	Operator      *string
	PartnerParams map[string]string
	Metadata      []MetadataEntry
	UserID        *string // routed to the User-ID header
}

// DocumentVerificationParams holds the request fields for document
// verification and enhanced document verification. IDType is optional for
// documents.Verify and required for documents.VerifyEnhanced.
type DocumentVerificationParams struct {
	SelfieImage    *BinaryInput
	LivenessImages []*BinaryInput
	Document       *BinaryInput
	DocumentBack   *BinaryInput
	Consent        Consent
	Country        string
	IDType         *string
	UserDetails    UserDetails
	CallbackURL    *string
	PartnerParams  map[string]string
	Metadata       []MetadataEntry
	UserID         *string // routed to the User-ID header
}

// ResidencyDocumentVerificationParams holds the request fields for residency
// document verification (POST /v3/residency_document_verification). Visa is
// required. IDType defaults to "PASSPORT" when empty; any other value is rejected.
type ResidencyDocumentVerificationParams struct {
	SelfieImage    *BinaryInput
	LivenessImages []*BinaryInput
	Document       *BinaryInput
	DocumentBack   *BinaryInput
	Visa           *BinaryInput
	Consent        Consent
	Country        string
	IDType         string
	UserDetails    UserDetails
	CallbackURL    *string
	PartnerParams  map[string]string
	Metadata       []MetadataEntry
	UserID         *string // routed to the User-ID header
}

// BiometricKYCParams holds the request fields for biometric KYC verification.
// SelfieImage and LivenessImages are required unless the call passes
// usesmileid.WithEnrolledImage.
type BiometricKYCParams struct {
	SelfieImage    *BinaryInput
	LivenessImages []*BinaryInput
	Consent        Consent
	Country        string
	IDType         string
	IDNumber       string
	UserDetails    UserDetails
	CallbackURL    *string
	SandboxResult  *float64
	PartnerParams  map[string]string
	Metadata       []MetadataEntry
	UserID         *string // routed to the User-ID header
}

// RegistrationParams holds the request fields for biometric enrollment.
type RegistrationParams struct {
	SelfieImage    *BinaryInput
	LivenessImages []*BinaryInput
	Consent        Consent
	UserDetails    UserDetails
	AllowNewEnroll *bool
	CallbackURL    *string
	SandboxResult  *float64
	PartnerParams  map[string]string
	Metadata       []MetadataEntry
	UserID         *string // routed to the User-ID header
}

// AuthenticationParams holds the request fields for biometric authentication.
// UserID is required and routed to the request body (not the User-ID header).
type AuthenticationParams struct {
	UserID         string
	SelfieImage    *BinaryInput
	LivenessImages []*BinaryInput
	Consent        Consent
	UserDetails    UserDetails
	CallbackURL    *string
	SandboxResult  *float64
	PartnerParams  map[string]string
	Metadata       []MetadataEntry
}

// CompareParams holds the request fields for the smart selfie compare
// operation. UserID is optional and routed to the request body.
type CompareParams struct {
	SelfieImage         *BinaryInput
	ComparisonImage     *BinaryInput
	ComparisonImageType string
	Consent             Consent
	UserDetails         UserDetails
	LivenessImages      []*BinaryInput
	AllowNewEnroll      *bool
	UserID              *string
	CallbackURL         *string
	SandboxResult       *float64
	PartnerParams       map[string]string
	Metadata            []MetadataEntry
}

// ReplayParams holds the optional fields for a callback replay.
type ReplayParams struct {
	CallbackURL *string
}

// ReportFraudParams holds the request fields for reporting user fraud.
type ReportFraudParams struct {
	IsFraud    bool
	ReportedBy string
	Reason     *string
	Notes      *string
}

// FlagFraudParams holds the fields for the FlagFraud convenience wrapper.
type FlagFraudParams struct {
	Reason     string
	Notes      *string
	ReportedBy string
}

// ClearFraudParams holds the fields for the ClearFraud convenience wrapper.
type ClearFraudParams struct {
	Notes      string
	ReportedBy string
}

// IDStatusParams holds the query fields for GET /v3/services/id_status.
type IDStatusParams struct {
	Country string
	IDType  string
}

// BankCodesParams holds the optional query for GET /v3/services/bank_codes.
type BankCodesParams struct {
	Country *string
}

// SupportedIDTypesParams holds the optional query for GET
// /v3/services/supported_id_types.
type SupportedIDTypesParams struct {
	Country *string
}

// SupportedDocumentsParams holds the optional query for GET
// /v3/services/supported_documents.
type SupportedDocumentsParams struct {
	Continent   *string
	CountryCode *string
	Locale      *string
}

// OneTimeAMLParams holds the request fields for One-Time AML screening
// (POST /v3/aml and POST /v3/aml/sync). Countries and FullName are required.
// Countries and Aliases are sent as JSON-encoded array strings. UserID is a
// body field and is required when SearchExistingUser is true.
type OneTimeAMLParams struct {
	Countries          []string
	FullName           string
	Consent            Consent
	UserDetails        UserDetails
	BirthYear          *string
	StrictMatch        *bool
	Aliases            []string
	SearchExistingUser *bool
	UserID             *string
	CallbackURL        *string
	PartnerParams      map[string]string
	Metadata           []MetadataEntry
}

// OneTimeAMLSyncParams holds the request fields for the synchronous One-Time
// AML screening. CallbackURL is optional and never triggers a webhook.
// IdempotencyKey, when set, is sent as the SmileID-Idempotency-Key header so a
// repeated submission returns the original result instead of a second,
// separately billed screening.
type OneTimeAMLSyncParams struct {
	OneTimeAMLParams
	IdempotencyKey *string
}

// AMLMonitoringParams holds the request fields for AML Monitoring screening
// (POST /v3/aml/monitoring). Countries is required. FullName is required
// unless SearchExistingUser is true, and UserID is then required.
type AMLMonitoringParams struct {
	Countries          []string
	FullName           *string
	Consent            Consent
	UserDetails        UserDetails
	BirthYear          *string
	StrictMatch        *bool
	Aliases            []string
	SearchExistingUser *bool
	UserID             *string
	CallbackURL        *string
	PartnerParams      map[string]string
	Metadata           []MetadataEntry
}

// AMLMonitoringNewsParams holds the query fields for
// GET /v3/aml/monitoring/{job_id}/news/{reference}. NewsCategory is required;
// Page and Limit default to 1 and 25 on the server.
type AMLMonitoringNewsParams struct {
	NewsCategory string
	Page         *int
	Limit        *int
}
