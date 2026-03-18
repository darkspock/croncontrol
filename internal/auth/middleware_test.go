package auth

import "testing"

func TestHasMinRole(t *testing.T) {
	tests := []struct {
		actual, required string
		expected         bool
	}{
		{"admin", "admin", true},
		{"admin", "operator", true},
		{"admin", "viewer", true},
		{"operator", "admin", false},
		{"operator", "operator", true},
		{"operator", "viewer", true},
		{"viewer", "admin", false},
		{"viewer", "operator", false},
		{"viewer", "viewer", true},
	}
	for _, tt := range tests {
		t.Run(tt.actual+"_needs_"+tt.required, func(t *testing.T) {
			if got := hasMinRole(tt.actual, tt.required); got != tt.expected {
				t.Errorf("hasMinRole(%q, %q) = %v, want %v", tt.actual, tt.required, got, tt.expected)
			}
		})
	}
}
