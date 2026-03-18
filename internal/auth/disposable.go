package auth

import "strings"

// disposableDomains is a curated list of common disposable email providers.
// This is intentionally small and focused on the most common ones.
// For production, consider using a service like kickbox.io or mailcheck.ai.
var disposableDomains = map[string]bool{
	"mailinator.com": true, "guerrillamail.com": true, "tempmail.com": true,
	"throwaway.email": true, "yopmail.com": true, "10minutemail.com": true,
	"trashmail.com": true, "mailnesia.com": true, "temp-mail.org": true,
	"fakeinbox.com": true, "sharklasers.com": true, "guerrillamailblock.com": true,
	"grr.la": true, "guerrillamail.info": true, "guerrillamail.net": true,
	"guerrillamail.de": true, "getnada.com": true, "dispostable.com": true,
	"maildrop.cc": true, "mailsac.com": true, "harakirimail.com": true,
	"jetable.org": true, "discard.email": true, "tmpmail.net": true,
	"tmpmail.org": true, "boun.cr": true, "mt2015.com": true,
	"tmail.ws": true, "mohmal.com": true, "tempail.com": true,
	"burnermail.io": true, "mailcatch.com": true, "emailondeck.com": true,
}

// IsDisposableEmail checks if an email address uses a known disposable domain.
func IsDisposableEmail(email string) bool {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(strings.TrimSpace(parts[1]))
	return disposableDomains[domain]
}
