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

// AML Monitoring news categories accepted by
// GET /v3/aml/monitoring/{job_id}/news/{reference}.
var AMLNewsCategories = []string{
	"arms_trafficking",
	"credit_update",
	"environmental_crime",
	"financial_crime",
	"human_rights_violation",
	"leadership_change",
	"legal_risk",
	"location_change",
	"narcotics",
	"other",
	"other_crime",
	"ownership_change",
	"product_change",
	"regulatory",
	"sexual_crime",
	"technology_event",
	"terrorism",
	"violent_crime",
}
