package notifier

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestMatchesFilter(t *testing.T) {
	tests := []struct {
		eventType string
		filters   []string
		match     bool
	}{
		{"run.completed", []string{"run.completed"}, true},
		{"run.completed", []string{"run.*"}, true},
		{"run.failed", []string{"run.*"}, true},
		{"job.completed", []string{"run.*"}, false},
		{"run.completed", []string{"job.*", "run.completed"}, true},
		{"run.completed", nil, true},  // no filter = match all
		{"run.completed", []string{}, true},
		{"usage.warning", []string{"usage.warning"}, true},
		{"worker.offline", []string{"worker.*"}, true},
	}

	for _, tt := range tests {
		got := matchesFilter(tt.eventType, tt.filters)
		if got != tt.match {
			t.Errorf("matchesFilter(%q, %v) = %v, want %v", tt.eventType, tt.filters, got, tt.match)
		}
	}
}

func TestSign(t *testing.T) {
	payload := []byte(`{"type":"run.completed"}`)
	secret := "test-secret"

	sig := sign(payload, secret)

	// Verify independently
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if sig != expected {
		t.Errorf("signature mismatch: got %s, want %s", sig, expected)
	}
}
