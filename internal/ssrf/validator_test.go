package ssrf

import (
	"strings"
	"testing"
)

func TestValidatePublicURLs(t *testing.T) {
	// Use ValidateFormat (no DNS) for unit tests
	urls := []string{
		"https://api.example.com/webhook",
		"https://httpbin.org/post",
		"http://example.com/cron",
	}
	for _, u := range urls {
		if err := ValidateFormat(u, nil); err != nil {
			t.Errorf("expected %q to be valid: %v", u, err)
		}
	}
}

func TestBlockPrivateIPs(t *testing.T) {
	urls := []string{
		"http://127.0.0.1/secret",
		"http://10.0.0.1/internal",
		"http://192.168.1.1/admin",
		"http://172.16.0.1/mgmt",
	}
	for _, u := range urls {
		err := Validate(u, nil)
		if err == nil {
			t.Errorf("expected %q to be blocked", u)
		}
	}
}

func TestBlockMetadata(t *testing.T) {
	err := Validate("http://169.254.169.254/latest/meta-data/", nil)
	if err == nil {
		t.Error("expected metadata endpoint to be blocked")
	}
}

func TestBlockHostnames(t *testing.T) {
	blocked := []string{
		"http://localhost/test",
		"http://something.internal/api",
		"http://service.local/hook",
	}
	for _, u := range blocked {
		err := Validate(u, nil)
		if err == nil {
			t.Errorf("expected %q to be blocked", u)
		}
	}
}

func TestBlockBadSchemes(t *testing.T) {
	err := Validate("ftp://example.com/file", nil)
	if err == nil {
		t.Error("expected ftp scheme to be blocked")
	}
	err = Validate("file:///etc/passwd", nil)
	if err == nil {
		t.Error("expected file scheme to be blocked")
	}
}

func TestAllowlist(t *testing.T) {
	allowed := []string{"example.com", "api.acme.io"}

	// Should pass (format-only, no DNS)
	if err := ValidateFormat("https://example.com/hook", allowed); err != nil {
		t.Errorf("expected example.com to pass allowlist: %v", err)
	}
	if err := ValidateFormat("https://sub.example.com/hook", allowed); err != nil {
		t.Errorf("expected sub.example.com to pass allowlist: %v", err)
	}

	// Should fail
	err := ValidateFormat("https://evil.com/hook", allowed)
	if err == nil {
		t.Error("expected evil.com to fail allowlist")
	}
	if !strings.Contains(err.Error(), "not in allowed domains") {
		t.Errorf("expected allowlist error, got: %v", err)
	}
}
