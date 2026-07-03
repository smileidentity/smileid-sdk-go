package smileid

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// smileIDError is the base error type shared by every typed SDK error. It is
// exported as [Error]; the typed wrappers below embed it so callers can read
// the common fields (StatusCode, Message, and so on) directly through
// promotion and match a specific type with errors.As.
type smileIDError struct {
	// StatusCode is the HTTP status code, or 0 for connection and local errors.
	StatusCode int
	// Status is the HTTP status text from the response body, when present.
	Status string
	// Message is the human-readable error message.
	Message string
	// Code is the numeric code present only on the services {error, code} shape.
	Code string
	// RequestID is populated from a response header when one is present.
	RequestID string
	// RawBody is the unparsed response body.
	RawBody string
}

func (e *smileIDError) Error() string {
	switch {
	case e.StatusCode != 0 && e.Message != "":
		return fmt.Sprintf("smileid: HTTP %d: %s", e.StatusCode, e.Message)
	case e.StatusCode != 0:
		return fmt.Sprintf("smileid: HTTP %d", e.StatusCode)
	case e.Message != "":
		return "smileid: " + e.Message
	default:
		return "smileid: unknown error"
	}
}

// Error is the base type carrying the fields common to every SDK error.
type Error = smileIDError

// InvalidRequestError is raised for HTTP 400 and 415 responses.
type InvalidRequestError struct{ *smileIDError }

// AuthenticationError is raised for HTTP 401 responses.
type AuthenticationError struct{ *smileIDError }

// PaymentRequiredError is raised for HTTP 402 responses.
type PaymentRequiredError struct{ *smileIDError }

// PermissionError is raised for HTTP 403 responses, including the
// unauthenticated services {error, code} shape.
type PermissionError struct{ *smileIDError }

// NotFoundError is raised for HTTP 404 responses. It is never raised by
// Verifications.Retrieve, which returns a not_found JobStatus instead.
type NotFoundError struct{ *smileIDError }

// ConflictError is raised for HTTP 409 responses (for example a replay of a
// verification that is still processing). It is never auto-retried.
type ConflictError struct{ *smileIDError }

// PayloadTooLargeError is raised for HTTP 413 responses.
type PayloadTooLargeError struct{ *smileIDError }

// RateLimitError is raised for HTTP 429 responses.
type RateLimitError struct{ *smileIDError }

// APIError is raised for HTTP 5xx responses. An unmapped non-5xx status
// returns the bare base [Error] instead.
type APIError struct{ *smileIDError }

// ConnectionError is raised when no HTTP response was received (network
// failure, timeout, or context cancellation). It wraps the underlying error.
type ConnectionError struct {
	*smileIDError
	Err error
}

// Unwrap returns the underlying transport error.
func (e *ConnectionError) Unwrap() error { return e.Err }

// ValidationError is raised for client-side validation failures, before any
// request is sent.
type ValidationError struct{ *smileIDError }

// TimeoutError is raised by Verifications.WaitUntilComplete when polling
// exceeds the configured timeout.
type TimeoutError struct{ *smileIDError }

// validationErrorf builds a ValidationError with a formatted message.
func validationErrorf(format string, args ...interface{}) error {
	return &ValidationError{&smileIDError{Message: fmt.Sprintf(format, args...)}}
}

// parseError turns a non-2xx HTTP response into the correct typed error. The
// class is selected by HTTP status; the message is read from the body's
// "message" then "error" field, and "code"/"status" are read when present.
func parseError(status int, body []byte, requestID string) error {
	base := &smileIDError{StatusCode: status, RequestID: requestID, RawBody: string(body)}

	if len(body) > 0 {
		var parsed struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Error   string `json:"error"`
			Code    string `json:"code"`
		}
		if json.Unmarshal(body, &parsed) == nil {
			base.Status = parsed.Status
			base.Code = parsed.Code
			switch {
			case parsed.Message != "":
				base.Message = parsed.Message
			case parsed.Error != "":
				base.Message = parsed.Error
			}
		}
	}
	if base.Message == "" {
		base.Message = http.StatusText(status)
	}

	switch status {
	case http.StatusBadRequest, http.StatusUnsupportedMediaType:
		return &InvalidRequestError{base}
	case http.StatusUnauthorized:
		return &AuthenticationError{base}
	case http.StatusPaymentRequired:
		return &PaymentRequiredError{base}
	case http.StatusForbidden:
		return &PermissionError{base}
	case http.StatusNotFound:
		return &NotFoundError{base}
	case http.StatusConflict:
		return &ConflictError{base}
	case http.StatusRequestEntityTooLarge:
		return &PayloadTooLargeError{base}
	case http.StatusTooManyRequests:
		return &RateLimitError{base}
	default:
		if status >= 500 {
			return &APIError{base}
		}
		return base
	}
}
