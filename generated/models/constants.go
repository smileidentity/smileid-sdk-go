package models

// Fraud report reasons accepted by POST /v3/users/{user_id}/report_fraud.
const (
	ReasonFirstPartyFraud   = "FIRST_PARTY_FRAUD"
	ReasonSecondPartyFraud  = "SECOND_PARTY_FRAUD"
	ReasonThirdPartyFraud   = "THIRD_PARTY_FRAUD"
	ReasonSyntheticIdentity = "SYNTHETIC_IDENTITY"
	ReasonAccountTakeover   = "ACCOUNT_TAKEOVER"
	ReasonDocumentForgery   = "DOCUMENT_FORGERY"
	ReasonIdentityFarming   = "IDENTITY_FARMING"
	ReasonMuleAccount       = "MULE_ACCOUNT"
	ReasonOther             = "OTHER"
)

// FraudReasons lists every valid fraud report reason.
var FraudReasons = []string{
	ReasonFirstPartyFraud,
	ReasonSecondPartyFraud,
	ReasonThirdPartyFraud,
	ReasonSyntheticIdentity,
	ReasonAccountTakeover,
	ReasonDocumentForgery,
	ReasonIdentityFarming,
	ReasonMuleAccount,
	ReasonOther,
}

// Comparison image types accepted by POST /v3/compare.
const (
	ComparisonImageTypeDocument = "DOCUMENT"
	ComparisonImageTypeIDPhoto  = "ID_PHOTO"
	ComparisonImageTypePortrait = "PORTRAIT"
)
