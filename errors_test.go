package smileid

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestParseErrorClassByStatus(t *testing.T) {
	tests := []struct {
		status int
		body   string
		as     func(error) bool
	}{
		{400, `{"status":"Bad Request","message":"bad"}`, func(e error) bool { var x *InvalidRequestError; return errors.As(e, &x) }},
		{415, `{"status":"Unsupported Media Type","message":"nope"}`, func(e error) bool { var x *InvalidRequestError; return errors.As(e, &x) }},
		{401, `{"status":"Unauthorized","message":"no"}`, func(e error) bool { var x *AuthenticationError; return errors.As(e, &x) }},
		{402, `{"status":"Payment Required","message":"balance"}`, func(e error) bool { var x *PaymentRequiredError; return errors.As(e, &x) }},
		{403, `{"error":"You are not authorized to do that.","code":"2413"}`, func(e error) bool { var x *PermissionError; return errors.As(e, &x) }},
		{404, `{"status":"Not Found","message":"gone"}`, func(e error) bool { var x *NotFoundError; return errors.As(e, &x) }},
		{409, `{"status":"Conflict","message":"processing"}`, func(e error) bool { var x *ConflictError; return errors.As(e, &x) }},
		{413, `{"status":"Content Too Large","message":"big"}`, func(e error) bool { var x *PayloadTooLargeError; return errors.As(e, &x) }},
		{429, `{"status":"Too Many Requests","message":"slow"}`, func(e error) bool { var x *RateLimitError; return errors.As(e, &x) }},
		{500, `{"status":"Internal Server Error","message":"boom"}`, func(e error) bool { var x *APIError; return errors.As(e, &x) }},
		{501, `{"status":"Not Implemented","message":"nope"}`, func(e error) bool { var x *APIError; return errors.As(e, &x) }},
		{599, `{"status":"Network Timeout","message":"edge"}`, func(e error) bool { var x *APIError; return errors.As(e, &x) }},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.status), func(t *testing.T) {
			err := parseError(tt.status, []byte(tt.body), "")
			if !tt.as(err) {
				t.Errorf("status %d produced %T", tt.status, err)
			}
		})
	}
}

func TestParseErrorUnmappedNonServerStatusReturnsBaseError(t *testing.T) {
	err := parseError(418, []byte(`{"status":"I'm a teapot","message":"short and stout"}`), "")

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Fatalf("unmapped non-5xx status produced *APIError: %v", err)
	}
	var base *Error
	if !errors.As(err, &base) {
		t.Fatalf("err = %T %v, want the bare base *Error", err, err)
	}
	if base.StatusCode != 418 || base.Message != "short and stout" {
		t.Errorf("base fields = %d %q", base.StatusCode, base.Message)
	}
}

func TestParseErrorStatusMessageShape(t *testing.T) {
	err := parseError(400, []byte(`{"status":"Bad Request","message":"Either email or phone_number is required."}`), "req-1")
	var ire *InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("err = %T", err)
	}
	if ire.StatusCode != 400 {
		t.Errorf("StatusCode = %d", ire.StatusCode)
	}
	if ire.Status != "Bad Request" {
		t.Errorf("Status = %q", ire.Status)
	}
	if ire.Message != "Either email or phone_number is required." {
		t.Errorf("Message = %q", ire.Message)
	}
	if ire.RequestID != "req-1" {
		t.Errorf("RequestID = %q", ire.RequestID)
	}
	if ire.Code != "" {
		t.Errorf("Code = %q, want empty", ire.Code)
	}
}

func TestParseErrorErrorCodeShape(t *testing.T) {
	err := parseError(403, []byte(`{"error":"You are not authorized to do that.","code":"2413"}`), "")
	var perm *PermissionError
	if !errors.As(err, &perm) {
		t.Fatalf("err = %T", err)
	}
	if perm.Code != "2413" {
		t.Errorf("Code = %q", perm.Code)
	}
	if perm.Message != "You are not authorized to do that." {
		t.Errorf("Message = %q", perm.Message)
	}
	if perm.Status != "" {
		t.Errorf("Status = %q, want empty for {error,code} shape", perm.Status)
	}
}

func TestParseErrorIDStatusShape(t *testing.T) {
	// id_status reorders to {message, status} — same key set.
	err := parseError(400, []byte(`{"message":"\"country\" is required","status":"Bad Request"}`), "")
	var ire *InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("err = %T", err)
	}
	if ire.Message != `"country" is required` || ire.Status != "Bad Request" {
		t.Errorf("Message=%q Status=%q", ire.Message, ire.Status)
	}
}

func TestParseErrorNonJSONBody(t *testing.T) {
	err := parseError(500, []byte("<html>gateway</html>"), "")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T", err)
	}
	if apiErr.Message != http.StatusText(500) {
		t.Errorf("Message = %q, want %q", apiErr.Message, http.StatusText(500))
	}
	if apiErr.RawBody != "<html>gateway</html>" {
		t.Errorf("RawBody = %q", apiErr.RawBody)
	}
}

func TestRetrieveNotFoundReturnsJobStatus(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"status":"not_found","job_id":"job_x","user_id":"unknown","message":"Verification not found"}`)
	})

	js, err := c.Verifications.Retrieve(context.Background(), "job_x")
	if err != nil {
		t.Fatalf("Retrieve should not raise on 404, got %v", err)
	}
	if js.Status != "not_found" {
		t.Errorf("status = %q, want not_found", js.Status)
	}
}

func TestErrorRequestIDFromHeader(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v3/token" {
			serveToken(w)
			return
		}
		w.Header().Set("X-Request-Id", "abc-123")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"status":"Bad Request","message":"bad"}`)
	})

	_, err := c.EnhancedKYC.Verify(context.Background(), EnhancedKYCParams{
		Country: "NG", IDType: "NIN", IDNumber: "1", UserDetails: validUserDetails(), Consent: validConsent(),
	})
	var ire *InvalidRequestError
	if !errors.As(err, &ire) {
		t.Fatalf("err = %T", err)
	}
	if ire.RequestID != "abc-123" {
		t.Errorf("RequestID = %q", ire.RequestID)
	}
}
