package usesmileid

import "testing"

func TestAcceptedResponseNormalization(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{"Accepted", true},
		{"accepted", true},
		{"ACCEPTED", true},
		{"processing", false},
		{"", false},
	}
	for _, tt := range tests {
		got := AcceptedResponse{Status: tt.status}.IsAccepted()
		if got != tt.want {
			t.Errorf("IsAccepted(%q) = %v, want %v", tt.status, got, tt.want)
		}
	}
}
