package models

import (
	"encoding/json"
	"testing"
)

func TestAcceptedResponseIsAccepted(t *testing.T) {
	tests := map[string]bool{
		"Accepted": true,
		"accepted": true,
		"rejected": false,
		"":         false,
	}
	for status, want := range tests {
		if got := (AcceptedResponse{Status: status}).IsAccepted(); got != want {
			t.Errorf("IsAccepted() with status %q = %v, want %v", status, got, want)
		}
	}
}

func TestJobStatusIsComplete(t *testing.T) {
	tests := map[string]bool{
		"":             false,
		"processing":   false,
		" Processing ": false,
		"not_found":    false,
		"NOT_FOUND":    false,
		"clear":        true,
		"block":        true,
		"attention":    true,
		"error":        true,
	}
	for status, want := range tests {
		if got := (JobStatus{Status: status}).IsComplete(); got != want {
			t.Errorf("IsComplete() with status %q = %v, want %v", status, got, want)
		}
	}
}

func TestEnhancedKYCResultUnmarshalKeepsRawBody(t *testing.T) {
	var r EnhancedKYCResult
	if err := json.Unmarshal([]byte(`{"job_id":"j1","user_id":"u1","status":"clear","message":"ok","reason":null,"kyc_receipt":"https://r","id_info":{"dob":"1990-01-01"}}`), &r); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if r.JobID != "j1" || r.UserID != "u1" || r.Status != "clear" || r.Message != "ok" || r.Reason != nil || r.KYCReceipt == nil || *r.KYCReceipt != "https://r" {
		t.Errorf("typed fields = %+v", r)
	}
	info, ok := r.Raw["id_info"].(map[string]interface{})
	if !ok || info["dob"] != "1990-01-01" || len(r.Raw) != 7 {
		t.Errorf("raw = %v", r.Raw)
	}
}

func TestEnhancedKYCResultUnmarshalErrors(t *testing.T) {
	for name, body := range map[string]string{
		"not an object":    `[]`,
		"wrong field type": `{"status":5}`,
	} {
		t.Run(name, func(t *testing.T) {
			var r EnhancedKYCResult
			if err := json.Unmarshal([]byte(body), &r); err == nil {
				t.Errorf("Unmarshal(%s) succeeded: %+v", body, r)
			}
		})
	}
}
