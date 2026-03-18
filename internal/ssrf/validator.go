// Package ssrf provides Server-Side Request Forgery protection for HTTP targets.
//
// Blocks requests to private IPs, cloud metadata endpoints, link-local addresses,
// and internal hostnames. Supports optional URL allowlists per workspace.
package ssrf

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// blockedHostnames that should never be targeted.
var blockedHostnames = map[string]bool{
	"localhost":                true,
	"metadata.google.internal": true,
}

// blockedHostSuffixes that should never be targeted.
var blockedHostSuffixes = []string{
	".internal",
	".local",
	".localhost",
}

// ValidateFormat checks URL format, scheme, hostname, and allowlist WITHOUT DNS resolution.
// Use this for fast pre-validation. Use Validate for full check with DNS.
func ValidateFormat(rawURL string, allowedDomains []string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q: only http and https are allowed", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	lower := strings.ToLower(host)
	if blockedHostnames[lower] {
		return fmt.Errorf("blocked hostname: %s", host)
	}
	for _, suffix := range blockedHostSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return fmt.Errorf("blocked hostname suffix: %s", host)
		}
	}

	// If host is an IP literal, check it directly
	if ip := net.ParseIP(host); ip != nil {
		if err := checkIP(ip); err != nil {
			return fmt.Errorf("blocked IP %s: %w", host, err)
		}
	}

	if len(allowedDomains) > 0 {
		if !matchesAllowlist(host, allowedDomains) {
			return fmt.Errorf("host %q not in allowed domains: %v", host, allowedDomains)
		}
	}

	return nil
}

// Validate checks a URL for SSRF risks including DNS resolution.
// allowedDomains is optional — if non-empty, the URL's host must match one of them.
func Validate(rawURL string, allowedDomains []string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q: only http and https are allowed", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	// Block known hostnames
	lower := strings.ToLower(host)
	if blockedHostnames[lower] {
		return fmt.Errorf("blocked hostname: %s", host)
	}
	for _, suffix := range blockedHostSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return fmt.Errorf("blocked hostname suffix: %s", host)
		}
	}

	// Resolve and check IP
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("DNS resolution failed for %s: %w", host, err)
	}

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if err := checkIP(ip); err != nil {
			return fmt.Errorf("blocked IP %s for host %s: %w", ipStr, host, err)
		}
	}

	// Check allowlist (if specified — typically for free tier)
	if len(allowedDomains) > 0 {
		if !matchesAllowlist(host, allowedDomains) {
			return fmt.Errorf("host %q not in allowed domains: %v", host, allowedDomains)
		}
	}

	return nil
}

func checkIP(ip net.IP) error {
	// Loopback (127.0.0.0/8, ::1)
	if ip.IsLoopback() {
		return fmt.Errorf("loopback address")
	}

	// Private ranges (10.x, 172.16-31.x, 192.168.x)
	if ip.IsPrivate() {
		return fmt.Errorf("private address")
	}

	// Link-local (169.254.x.x, fe80::/10)
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("link-local address")
	}

	// Cloud metadata (169.254.169.254)
	metadata := net.ParseIP("169.254.169.254")
	if ip.Equal(metadata) {
		return fmt.Errorf("cloud metadata endpoint")
	}

	// Unspecified (0.0.0.0, ::)
	if ip.IsUnspecified() {
		return fmt.Errorf("unspecified address")
	}

	return nil
}

func matchesAllowlist(host string, allowed []string) bool {
	lower := strings.ToLower(host)
	for _, domain := range allowed {
		d := strings.ToLower(strings.TrimSpace(domain))
		if d == "" {
			continue
		}
		// Exact match or subdomain match
		if lower == d || strings.HasSuffix(lower, "."+d) {
			return true
		}
	}
	return false
}
