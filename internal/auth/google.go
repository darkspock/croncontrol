package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// GoogleOAuthConfig holds Google OAuth 2.0 settings.
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // e.g., "http://localhost:8080/api/v1/auth/google/callback"
}

// GoogleAuthenticator handles Google OAuth 2.0 flow.
type GoogleAuthenticator struct {
	config *oauth2.Config
}

// NewGoogleAuth creates a new Google OAuth authenticator.
// Returns nil if client ID or secret is empty (OAuth disabled).
func NewGoogleAuth(cfg GoogleOAuthConfig) *GoogleAuthenticator {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil
	}
	return &GoogleAuthenticator{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// GoogleUserInfo represents the user info returned by Google's userinfo endpoint.
type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// LoginURL returns the Google OAuth authorization URL with a random state token.
func (g *GoogleAuthenticator) LoginURL() (url string, state string) {
	state = generateState()
	url = g.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return
}

// Exchange exchanges an authorization code for user info.
func (g *GoogleAuthenticator) Exchange(ctx context.Context, code string) (*GoogleUserInfo, error) {
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	client := g.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("get userinfo: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read userinfo: %w", err)
	}

	var info GoogleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse userinfo: %w", err)
	}

	if info.Email == "" {
		return nil, fmt.Errorf("no email in Google response")
	}

	return &info, nil
}

// SessionToken represents a signed session for cookie-based auth.
type SessionToken struct {
	UserID      string
	WorkspaceID string
	Role        string
	ExpiresAt   time.Time
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
