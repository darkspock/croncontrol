package logging

import (
	"regexp"
	"strings"
)

// Known secret patterns to mask in log output.
var secretPatterns = []*regexp.Regexp{
	// CronControl API keys: cc_live_<hex>
	regexp.MustCompile(`cc_live_[0-9a-fA-F]{16,}`),
	// Worker credentials: wrk_cred_<hex>
	regexp.MustCompile(`wrk_cred_[0-9a-fA-F]{16,}`),
	// Enrollment tokens: enroll_cc_live_<hex>
	regexp.MustCompile(`enroll_cc_live_[0-9a-fA-F]{16,}`),
	// Bearer tokens in headers
	regexp.MustCompile(`Bearer\s+[A-Za-z0-9_\-\.]{20,}`),
	// Generic password-like patterns in key=value
	regexp.MustCompile(`(?i)(password|passwd|secret|token|api_key|apikey|authorization)\s*[=:]\s*\S+`),
	// AWS access keys
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	// AWS secret keys (40 char base64)
	regexp.MustCompile(`(?i)aws_secret_access_key\s*[=:]\s*\S+`),
}

// MaskSecrets replaces known secret patterns in text with redacted placeholders.
// Used to sanitize log output before storage.
func MaskSecrets(text string) string {
	result := text
	for _, pattern := range secretPatterns {
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			if len(match) <= 8 {
				return "****"
			}
			// Keep first 4 chars for identification, mask the rest
			prefix := match[:4]
			if strings.Contains(match, "=") || strings.Contains(match, ":") {
				// For key=value patterns, keep the key visible
				parts := regexp.MustCompile(`[=:]\s*`).Split(match, 2)
				if len(parts) == 2 {
					sep := "="
					if strings.Contains(match, ":") {
						sep = ":"
					}
					return parts[0] + sep + "****"
				}
			}
			return prefix + "****"
		})
	}
	return result
}
