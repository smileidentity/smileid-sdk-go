package models

import "testing"

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
