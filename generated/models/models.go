// Package models holds the wire request and response types for the Smile ID
// V3 API. These types mirror the OpenAPI specification field-for-field and are
// owned by the code generator; do not hand-edit them. The root smileid package
// re-exports them as type aliases so callers import a single package.
package models

import "strings"

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
