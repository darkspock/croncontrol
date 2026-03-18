package id

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{"workspace", PrefixWorkspace},
		{"user", PrefixUser},
		{"process", PrefixProcess},
		{"run", PrefixRun},
		{"queue", PrefixQueue},
		{"job", PrefixJob},
		{"worker", PrefixWorker},
		{"api_key", PrefixAPIKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := New(tt.prefix)
			if !strings.HasPrefix(id, tt.prefix) {
				t.Errorf("expected prefix %q, got %q", tt.prefix, id)
			}
			if len(id) != len(tt.prefix)+26 { // ULID is 26 chars
				t.Errorf("unexpected length %d for id %q", len(id), id)
			}
		})
	}
}

func TestNewUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := NewProcess()
		if seen[id] {
			t.Fatalf("duplicate id generated: %s", id)
		}
		seen[id] = true
	}
}

func TestParse(t *testing.T) {
	id := NewWorkspace()
	prefix, raw, err := Parse(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prefix != PrefixWorkspace {
		t.Errorf("expected prefix %q, got %q", PrefixWorkspace, prefix)
	}
	if raw == "" {
		t.Error("expected non-empty raw ULID")
	}
}

func TestParseInvalid(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"no separator", "abc123"},
		{"empty after prefix", "wsp_"},
		{"unknown prefix", "xxx_01HYX"},
		{"invalid ulid", "wsp_notaulid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Parse(tt.id)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	valid := NewRun()
	if !IsValid(valid) {
		t.Errorf("expected %q to be valid", valid)
	}
	if IsValid("garbage") {
		t.Error("expected 'garbage' to be invalid")
	}
}

func TestHasPrefix(t *testing.T) {
	id := NewJob()
	if !HasPrefix(id, PrefixJob) {
		t.Errorf("expected %q to have prefix %q", id, PrefixJob)
	}
	if HasPrefix(id, PrefixRun) {
		t.Errorf("expected %q NOT to have prefix %q", id, PrefixRun)
	}
}

func TestConvenienceGenerators(t *testing.T) {
	generators := []struct {
		name   string
		fn     func() string
		prefix string
	}{
		{"workspace", NewWorkspace, PrefixWorkspace},
		{"user", NewUser, PrefixUser},
		{"membership", NewMembership, PrefixMembership},
		{"worker", NewWorker, PrefixWorker},
		{"process", NewProcess, PrefixProcess},
		{"run", NewRun, PrefixRun},
		{"run_attempt", NewRunAttempt, PrefixRunAttempt},
		{"queue", NewQueue, PrefixQueue},
		{"job", NewJob, PrefixJob},
		{"job_attempt", NewJobAttempt, PrefixJobAttempt},
		{"webhook_sub", NewWebhookSub, PrefixWebhookSub},
		{"api_key", NewAPIKey, PrefixAPIKey},
		{"ssh_cred", NewSSHCred, PrefixSSHCred},
		{"ssm_profile", NewSSMProfile, PrefixSSMProfile},
		{"k8s_cluster", NewK8sCluster, PrefixK8sCluster},
	}

	for _, g := range generators {
		t.Run(g.name, func(t *testing.T) {
			id := g.fn()
			if !HasPrefix(id, g.prefix) {
				t.Errorf("expected prefix %q in %q", g.prefix, id)
			}
			if !IsValid(id) {
				t.Errorf("expected %q to be valid", id)
			}
		})
	}
}
