package smileid

import "github.com/smileidentity/smileid-sdk-go/v12/generated/models"

// Wire request and response models, re-exported from the generated package so
// callers import only the root smileid package.
type (
	Consent       = models.Consent
	UserDetails   = models.UserDetails
	MetadataEntry = models.MetadataEntry
	BinaryInput   = models.BinaryInput

	AcceptedResponse           = models.AcceptedResponse
	JobStatus                  = models.JobStatus
	BankCode                   = models.BankCode
	BankCodesResponse          = models.BankCodesResponse
	IDType                     = models.IDType
	SupportedIDTypesResponse   = models.SupportedIDTypesResponse
	DocumentCountry            = models.DocumentCountry
	DocumentIDType             = models.DocumentIDType
	ValidDocument              = models.ValidDocument
	SupportedDocumentsResponse = models.SupportedDocumentsResponse
	IDStatusResponse           = models.IDStatusResponse
	ReplayCallbackResponse     = models.ReplayCallbackResponse
	ReportUserFraudResponse    = models.ReportUserFraudResponse

	EnhancedKYCParams          = models.EnhancedKYCParams
	DocumentVerificationParams = models.DocumentVerificationParams
	BiometricKYCParams         = models.BiometricKYCParams
	RegistrationParams         = models.RegistrationParams
	AuthenticationParams       = models.AuthenticationParams
	CompareParams              = models.CompareParams
	ReplayParams               = models.ReplayParams
	ReportFraudParams          = models.ReportFraudParams
	FlagFraudParams            = models.FlagFraudParams
	ClearFraudParams           = models.ClearFraudParams
	IDStatusParams             = models.IDStatusParams
	BankCodesParams            = models.BankCodesParams
	SupportedIDTypesParams     = models.SupportedIDTypesParams
	SupportedDocumentsParams   = models.SupportedDocumentsParams
)

// Binary input constructors, re-exported from the generated package.
var (
	FromFile   = models.FromFile
	FromBytes  = models.FromBytes
	FromReader = models.FromReader
)

// Fraud report reasons.
const (
	ReasonFirstPartyFraud   = models.ReasonFirstPartyFraud
	ReasonSecondPartyFraud  = models.ReasonSecondPartyFraud
	ReasonThirdPartyFraud   = models.ReasonThirdPartyFraud
	ReasonSyntheticIdentity = models.ReasonSyntheticIdentity
	ReasonAccountTakeover   = models.ReasonAccountTakeover
	ReasonDocumentForgery   = models.ReasonDocumentForgery
	ReasonIdentityFarming   = models.ReasonIdentityFarming
	ReasonMuleAccount       = models.ReasonMuleAccount
	ReasonOther             = models.ReasonOther
)

// Comparison image types.
const (
	ComparisonImageTypeDocument = models.ComparisonImageTypeDocument
	ComparisonImageTypeIDPhoto  = models.ComparisonImageTypeIDPhoto
	ComparisonImageTypePortrait = models.ComparisonImageTypePortrait
)
