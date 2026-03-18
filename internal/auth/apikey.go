package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	db "github.com/croncontrol/croncontrol/internal/db"
)

// APIKeyAuthenticator validates API keys from the X-API-Key header.
type APIKeyAuthenticator struct {
	queries *db.Queries
}

// NewAPIKeyAuth creates a new API key authenticator.
func NewAPIKeyAuth(queries *db.Queries) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{queries: queries}
}

// Authenticate extracts and validates the API key from the request.
// Platform admins can pass X-Workspace-ID header to act on any workspace.
func (a *APIKeyAuthenticator) Authenticate(ctx context.Context, r *http.Request) (*Actor, error) {
	key := r.Header.Get("X-API-Key")
	if key == "" {
		return nil, fmt.Errorf("missing X-API-Key header")
	}

	hash := HashAPIKey(key)

	row, err := a.queries.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Verify prefix matches (defense in depth)
	prefix := key[:min(len(key), 8)]
	if subtle.ConstantTimeCompare([]byte(prefix), []byte(row.KeyPrefix)) != 1 {
		return nil, fmt.Errorf("invalid API key")
	}

	// Check if the key owner is a platform admin
	isPlatformAdmin := false
	if row.CreatedBy != nil {
		user, err := a.queries.GetUserByID(ctx, *row.CreatedBy)
		if err == nil {
			isPlatformAdmin = user.IsPlatformAdmin
		}
	}

	// Determine workspace — platform admins can override via header
	wsID := row.WorkspaceID
	if isPlatformAdmin {
		if overrideWS := r.Header.Get("X-Workspace-ID"); overrideWS != "" {
			wsID = overrideWS
		}
	}

	// Check workspace state (skip for platform admin accessing other workspaces)
	if !isPlatformAdmin {
		if row.WorkspaceState == "suspended" || row.WorkspaceState == "archived" {
			return nil, fmt.Errorf("workspace is %s", row.WorkspaceState)
		}
	}

	// Update last used (fire-and-forget)
	go func() {
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = strings.Split(fwd, ",")[0]
		}
		ua := r.UserAgent()
		a.queries.UpdateAPIKeyUsage(context.Background(), db.UpdateAPIKeyUsageParams{
			ID:            row.ID,
			LastIp:        &ip,
			LastUserAgent: &ua,
		})
	}()

	role := row.Role
	if isPlatformAdmin {
		role = "admin" // platform admins always have admin role
	}

	slog.Debug("api key authenticated",
		"key_prefix", row.KeyPrefix,
		"workspace_id", wsID,
		"platform_admin", isPlatformAdmin,
	)

	return &Actor{
		Type:            "api_key",
		ID:              row.ID,
		WorkspaceID:     wsID,
		Role:            role,
		IsPlatformAdmin: isPlatformAdmin,
	}, nil
}

// HashAPIKey creates a SHA-256 hash of a raw API key.
func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// GenerateAPIKeyPrefix returns the first 8 chars of a key for identification.
func GenerateAPIKeyPrefix(key string) string {
	if len(key) < 8 {
		return key
	}
	return key[:8]
}
