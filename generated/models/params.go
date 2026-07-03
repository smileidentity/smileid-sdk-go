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

// BiometricKYCParams holds the request fields for biometric KYC verification.
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
// SelfieImage and LivenessImages are required unless UseEnrolledImage is true.
type AuthenticationParams struct {
	UserID           string
	SelfieImage      *BinaryInput
	LivenessImages   []*BinaryInput
	Consent          Consent
	UserDetails      UserDetails
	UseEnrolledImage *bool
	CallbackURL      *string
	SandboxResult    *float64
	PartnerParams    map[string]string
	Metadata         []MetadataEntry
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
