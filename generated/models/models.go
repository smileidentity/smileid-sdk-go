// Package models holds the wire request and response types for the Smile ID
// V3 API. These types mirror the OpenAPI specification field-for-field and are
// owned by the code generator; do not hand-edit them. The root smileid package
// re-exports them as type aliases so callers import a single package.
package models

import (
	"encoding/json"
	"strings"
)

// Consent captures the user's consent. Serialized as a JSON multipart part
// named "consent".
type Consent struct {
	Granted                bool   `json:"granted"`
	GrantedAt              string `json:"granted_at"`
	NoticeLanguage         string `json:"notice_language"`
	NoticePrivacyPolicyURL string `json:"notice_privacy_policy_url"`
}

// UserDetails identifies the subject of a verification. Serialized as a JSON
// multipart part named "user_details". At least one of Email or PhoneNumber
// must be set; the SDK enforces this before sending.
type UserDetails struct {
	GivenNames  string  `json:"given_names"`
	LastName    string  `json:"last_name"`
	Email       *string `json:"email,omitempty"`
	PhoneNumber *string `json:"phone_number,omitempty"`
}

// MetadataEntry is a single entry in the optional metadata array.
type MetadataEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// AcceptedResponse is returned by the entry endpoints on HTTP 202.
type AcceptedResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	JobID     string `json:"job_id"`
	UserID    string `json:"user_id"`
	CreatedAt string `json:"created_at,omitempty"`
}

// IsAccepted reports whether the job was accepted, normalizing the mixed
// "Accepted"/"accepted" casing returned across endpoints.
func (r AcceptedResponse) IsAccepted() bool {
	return strings.EqualFold(r.Status, "accepted")
}

// JobStatus is returned by GET /v3/status/{jobId}. Status is "processing"
// while the job runs, "not_found" for an unknown job, and otherwise the
// terminal decision itself: "clear", "block", "attention" or "error". Message
// is human-readable text ("Job completed" on a finished job), not the decision.
type JobStatus struct {
	Status  string `json:"status"`
	JobID   string `json:"job_id"`
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

// IsComplete reports whether the job reached a terminal decision, that is any
// status other than "processing" and "not_found".
func (s JobStatus) IsComplete() bool {
	switch strings.ToLower(strings.TrimSpace(s.Status)) {
	case "", "processing", "not_found":
		return false
	default:
		return true
	}
}

// BankCode is one entry in a BankCodesResponse.
type BankCode struct {
	Code    string `json:"code"`
	Country string `json:"country"`
	Name    string `json:"name"`
}

// BankCodesResponse is returned by GET /v3/services/bank_codes.
type BankCodesResponse struct {
	BankCodes []BankCode `json:"bank_codes"`
}

// IDType is one entry in a SupportedIDTypesResponse.
type IDType struct {
	BankCode       string   `json:"bank_code,omitempty"`
	Country        string   `json:"country"`
	Label          string   `json:"label"`
	Regex          string   `json:"regex"`
	RequiredFields []string `json:"required_fields"`
	Type           string   `json:"type"`
}

// SupportedIDTypesResponse is returned by GET /v3/services/supported_id_types.
type SupportedIDTypesResponse struct {
	IDTypes []IDType `json:"id_types"`
}

// DocumentCountry describes the country of a supported document.
type DocumentCountry struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Continent string `json:"continent"`
}

// DocumentIDType describes a supported document type.
type DocumentIDType struct {
	Code    string   `json:"code"`
	Name    string   `json:"name"`
	Example []string `json:"example"`
	HasBack bool     `json:"has_back"`
}

// ValidDocument is one entry in a SupportedDocumentsResponse.
type ValidDocument struct {
	Country DocumentCountry  `json:"country"`
	IDTypes []DocumentIDType `json:"id_types"`
}

// SupportedDocumentsResponse is returned by GET /v3/services/supported_documents.
type SupportedDocumentsResponse struct {
	ValidDocuments []ValidDocument `json:"valid_documents"`
}

// IDStatusResponse is returned by GET /v3/services/id_status.
type IDStatusResponse struct {
	LastChecked          string `json:"last_checked"`
	LastCheckStatus      string `json:"last_check_status"`
	LastHourSuccessRate  string `json:"last_hour_success_rate"`
	LastKnownStatus      string `json:"last_known_status"`
	LastCheckSuccessRate string `json:"last_check_success_rate"`
}

// ReplayCallbackResponse is returned by POST /v3/replay/{job_id}.
type ReplayCallbackResponse struct {
	Status  string `json:"status"`
	JobID   string `json:"job_id"`
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

// ReportUserFraudResponse is returned by POST /v3/users/{user_id}/report_fraud.
type ReportUserFraudResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	UserID  string `json:"user_id"`
}

// AMLNewsSummary is the per-category news exposure summary of an AMLPersonMatch.
type AMLNewsSummary struct {
	NewsCategory      string `json:"news_category"`
	NewsCategoryLabel string `json:"news_category_label"`
	NumberOfArticles  int    `json:"number_of_articles"`
}

// AMLPEP describes the politically exposed person details of an AMLPersonMatch.
type AMLPEP struct {
	PEPLevel           *int                     `json:"pep_level"`
	PoliticalPositions []string                 `json:"political_positions"`
	Sources            []map[string]interface{} `json:"sources"`
}

// AMLPersonMatch is a single sanctions, PEP or adverse-media hit in a One-Time
// AML result. The nested record lists are free-form objects.
type AMLPersonMatch struct {
	Name               string                   `json:"name"`
	Addresses          []string                 `json:"addresses"`
	NewsSummary        []AMLNewsSummary         `json:"news_summary"`
	Aliases            []string                 `json:"aliases"`
	Associations       []map[string]interface{} `json:"associations"`
	DatesOfBirth       []string                 `json:"dates_of_birth"`
	EnforcementActions []map[string]interface{} `json:"enforcement_actions"`
	Nationalities      []string                 `json:"nationalities"`
	Sanctions          []map[string]interface{} `json:"sanctions"`
	PEP                *AMLPEP                  `json:"pep,omitempty"`
}

// AMLUserProvidedInfo echoes the user details submitted with a One-Time AML job.
type AMLUserProvidedInfo struct {
	GivenNames  *string `json:"given_names"`
	LastName    *string `json:"last_name"`
	Email       *string `json:"email"`
	PhoneNumber *string `json:"phone_number"`
}

// OneTimeAMLResult is the terminal One-Time AML result. It is the 200 body of
// POST /v3/aml/sync and the payload the callback delivers for POST /v3/aml.
// Status is "clear", "block", "attention" or "error". Reason is nil for a
// clear outcome.
type OneTimeAMLResult struct {
	Status           string                 `json:"status"`
	Message          string                 `json:"message"`
	Reason           *string                `json:"reason"`
	CreatedAt        string                 `json:"created_at"`
	CompletedAt      string                 `json:"completed_at,omitempty"`
	Product          string                 `json:"product"`
	PartnerParams    map[string]string      `json:"partner_params"`
	KYCReceipt       *string                `json:"kyc_receipt"`
	StrictMatch      bool                   `json:"strict_match"`
	NoOfPersonsFound int                    `json:"no_of_persons_found"`
	People           []AMLPersonMatch       `json:"people"`
	UserProvidedInfo AMLUserProvidedInfo    `json:"user_provided_info"`
	Consent          map[string]interface{} `json:"consent"`
	Antifraud        map[string]interface{} `json:"antifraud"`
	DeviceSignals    map[string]interface{} `json:"device_signals"`
	// JobID is the Smile job ID from the SmileID-Job-ID response header of
	// POST /v3/aml/sync. It is not part of the body, so it is empty on a
	// callback payload.
	JobID string `json:"-"`
	// UserID, ResponseSignature and ResponseTimestamp are the User-ID,
	// Response-Signature and Response-Timestamp headers of
	// POST /v3/aml/sync, for checking with your own webhook verification
	// code. They are not part of the body, so they are empty on a callback
	// payload, and empty when the response has no such header.
	UserID            string `json:"-"`
	ResponseSignature string `json:"-"`
	ResponseTimestamp string `json:"-"`
}

// BasicKYCMatchResults holds the verdict per submitted field. Each verdict is
// one of "exact_match", "partial_match", "transposed", "no_match" or
// "not_provided".
type BasicKYCMatchResults struct {
	Overall           string `json:"overall"`
	Names             string `json:"names"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	OtherNames        string `json:"other_names"`
	DateOfBirth       string `json:"date_of_birth"`
	Gender            string `json:"gender"`
	PhoneNumber       string `json:"phone_number"`
	SecondaryIDNumber string `json:"secondary_id_number"`
}

// BasicKYCUserProvidedInfo echoes the identity details submitted with a Basic
// KYC request.
type BasicKYCUserProvidedInfo struct {
	GivenNames  *string `json:"given_names"`
	LastName    *string `json:"last_name"`
	Email       *string `json:"email"`
	PhoneNumber *string `json:"phone_number"`
	IDNumber    *string `json:"id_number"`
}

// BasicKYCResult is the terminal Basic KYC result. It is the 200 body of
// POST /v3/basic_kyc/sync and the payload the callback delivers for
// POST /v3/basic_kyc. Status is "clear", "attention", "block" or "error".
// Reason is nil for a clear outcome. MatchResults is nil when the check never
// produced a verdict. The result carries no identity fields.
type BasicKYCResult struct {
	Status           string                   `json:"status"`
	Message          string                   `json:"message"`
	Reason           *string                  `json:"reason"`
	CreatedAt        string                   `json:"created_at"`
	CompletedAt      string                   `json:"completed_at,omitempty"`
	Product          string                   `json:"product"`
	PartnerParams    map[string]interface{}   `json:"partner_params"`
	MatchResults     *BasicKYCMatchResults    `json:"match_results"`
	UserProvidedInfo BasicKYCUserProvidedInfo `json:"user_provided_info"`
	Consent          map[string]interface{}   `json:"consent"`
	Antifraud        map[string]interface{}   `json:"antifraud"`
	DeviceSignals    map[string]interface{}   `json:"device_signals"`
	KYCReceipt       *string                  `json:"kyc_receipt"`
	// JobID is the Smile job ID from the SmileID-Job-ID response header of
	// POST /v3/basic_kyc/sync. It is not part of the body, so it is empty on
	// a callback payload.
	JobID string `json:"-"`
	// UserID, ResponseSignature and ResponseTimestamp are the User-ID,
	// Response-Signature and Response-Timestamp headers of
	// POST /v3/basic_kyc/sync, for checking with your own webhook
	// verification code. They are not part of the body, so they are empty on
	// a callback payload, and empty when the response has no such header.
	UserID            string `json:"-"`
	ResponseSignature string `json:"-"`
	ResponseTimestamp string `json:"-"`
}

// EnhancedKYCResult is the terminal Enhanced KYC result: the 200 body of
// POST /v3/enhanced_kyc/sync, which is the payload the callback delivers for
// POST /v3/enhanced_kyc. Status is "clear", "attention", "block" or "error".
// Reason is nil for a clear outcome. The spec leaves the body open, so Raw
// keeps every body field, including the ID authority's data that has no
// typed field here.
type EnhancedKYCResult struct {
	// JobID is the SmileID-Job-ID response header of
	// POST /v3/enhanced_kyc/sync, or the body's job_id when the header is
	// absent or blank.
	JobID string `json:"job_id"`
	// UserID is the User-ID response header of POST /v3/enhanced_kyc/sync,
	// or the body's user_id when the header is absent or blank.
	UserID     string  `json:"user_id"`
	Status     string  `json:"status"`
	Message    string  `json:"message"`
	Reason     *string `json:"reason"`
	KYCReceipt *string `json:"kyc_receipt"`
	// ResponseSignature and ResponseTimestamp are the Response-Signature and
	// Response-Timestamp headers of POST /v3/enhanced_kyc/sync, for checking
	// with your own webhook verification code. They are not part of the
	// body, so they are empty on a callback payload, and empty when the
	// response has no such header.
	ResponseSignature string `json:"-"`
	ResponseTimestamp string `json:"-"`
	// Raw is the whole decoded body, keyed by field name. It holds the
	// fields above too, as they appear in the body.
	Raw map[string]interface{} `json:"-"`
}

// UnmarshalJSON decodes the typed fields and keeps the whole body in Raw.
func (r *EnhancedKYCResult) UnmarshalJSON(b []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	type plain EnhancedKYCResult
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	p.Raw = raw
	*r = EnhancedKYCResult(p)
	return nil
}

// AMLMonitoringNewsMeta is the paging block of an AMLMonitoringNewsResponse.
type AMLMonitoringNewsMeta struct {
	CurrentPage int `json:"current_page"`
	Limit       int `json:"limit"`
	TotalCount  int `json:"total_count"`
}

// AMLMonitoringNewsItem is one adverse-media article.
type AMLMonitoringNewsItem struct {
	Body                 string `json:"body"`
	PublishedDate        string `json:"published_date"`
	Publisher            string `json:"publisher"`
	PublisherCountryCode string `json:"publisher_country_code"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
}

// AMLMonitoringNewsResponse is returned by
// GET /v3/aml/monitoring/{job_id}/news/{reference}.
type AMLMonitoringNewsResponse struct {
	JobID   string                  `json:"job_id"`
	Product string                  `json:"product"`
	UserID  string                  `json:"user_id"`
	Meta    AMLMonitoringNewsMeta   `json:"meta"`
	News    []AMLMonitoringNewsItem `json:"news"`
}

// SmileLinkIDType is one verification configuration offered by a Smile Link.
// VerificationMethod is "biometric_kyc", "document_verification",
// "enhanced_document_verification" or "enhanced_kyc". A link read back stores
// document verification as "doc_verification".
type SmileLinkIDType struct {
	Country            string `json:"country"`
	IDType             string `json:"id_type"`
	VerificationMethod string `json:"verification_method"`
}

// SmileLinkCreated is the 201 body of POST /v3/smile_links. Read the full
// link with GetSmileLink. UserID is nil when the link has none.
type SmileLinkCreated struct {
	Link    string  `json:"link"`
	Message string  `json:"message"`
	RefID   string  `json:"ref_id"`
	UserID  *string `json:"user_id"`
}

// SmileLinkConsent is the consent held in a link's bound token values.
type SmileLinkConsent struct {
	Granted                bool   `json:"granted"`
	GrantedAt              string `json:"granted_at"`
	NoticeLanguage         string `json:"notice_language"`
	NoticePrivacyPolicyURL string `json:"notice_privacy_policy_url"`
}

// SmileLinkTokenValues holds the values, held in the clear, of a link's
// TokenProvidedFields. Fields the token did not bind are empty or nil.
type SmileLinkTokenValues struct {
	Country string            `json:"country,omitempty"`
	IDType  string            `json:"id_type,omitempty"`
	Consent *SmileLinkConsent `json:"consent,omitempty"`
}

// SmileLink is a Smile Link resource: the 200 body of GET and PUT
// /v3/smile_links/{refId}, and the link part of each list item. Optional
// fields the API leaves out are empty or nil. RedirectURL, ExpiresAt,
// DisabledAt, PartnerParams and UserID are nil when the API returns null.
type SmileLink struct {
	RefID                  string                 `json:"ref_id"`
	Link                   string                 `json:"link"`
	Name                   string                 `json:"name"`
	CompanyName            string                 `json:"company_name"`
	Description            string                 `json:"description"`
	IDTypes                []SmileLinkIDType      `json:"id_types"`
	IsSingleUse            bool                   `json:"is_single_use"`
	CallbackURL            string                 `json:"callback_url"`
	NoticePrivacyPolicyURL string                 `json:"notice_privacy_policy_url"`
	LogoURL                string                 `json:"logo_url"`
	RedirectURL            *string                `json:"redirect_url"`
	ThemeColor             string                 `json:"theme_color"`
	ExpiresAt              *string                `json:"expires_at"`
	DisabledAt             *string                `json:"disabled_at"`
	PartnerParams          map[string]interface{} `json:"partner_params"`
	UserID                 *string                `json:"user_id"`
	TokenProvidedFields    []string               `json:"token_provided_fields"`
	TokenProvidedValues    SmileLinkTokenValues   `json:"token_provided_values"`
	CreatedAt              string                 `json:"created_at"`
	UpdatedAt              string                 `json:"updated_at"`
}

// SmileLinkJobStats counts the outcomes of the jobs run through a link.
type SmileLinkJobStats struct {
	Clear          int `json:"clear"`
	Block          int `json:"block"`
	Attention      int `json:"attention"`
	Error          int `json:"error"`
	SuccessfulJobs int `json:"successful_jobs"`
	FailedJobs     int `json:"failed_jobs"`
	TotalJobs      int `json:"total_jobs"`
}

// SmileLinkListItem is one entry of a Smile Link listing: the link and its
// job outcome stats.
type SmileLinkListItem struct {
	SmileLink
	SmileLinkJobStats
}

// SmileLinkListMeta is the paging block of a SmileLinkListResponse.
type SmileLinkListMeta struct {
	CurrentPage int `json:"current_page"`
	Limit       int `json:"limit"`
	TotalCount  int `json:"total_count"`
	TotalPages  int `json:"total_pages"`
}

// SmileLinkListResponse is the 200 body of GET /v3/smile_links.
type SmileLinkListResponse struct {
	Links []SmileLinkListItem `json:"links"`
	Meta  SmileLinkListMeta   `json:"meta"`
}
