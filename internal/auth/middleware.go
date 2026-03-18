package auth

import (
	"encoding/json"
	"net/http"
)

// Middleware authenticates requests via API key or session cookie.
// Skips auth for paths in skipPaths.
func Middleware(apiKeyAuth *APIKeyAuthenticator, skipPaths map[string]bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for specified paths
			if skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// Try API key first
			actor, err := apiKeyAuth.Authenticate(r.Context(), r)
			if err != nil {
				// TODO: try session cookie auth (Google OAuth)
				writeAuthError(w, err.Error())
				return
			}

			// Store actor in context
			ctx := WithActor(r.Context(), *actor)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole returns middleware that enforces minimum role level.
func RequireRole(minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := Role(r.Context())
			if role == "" {
				writeAuthError(w, "not authenticated")
				return
			}

			if !hasMinRole(role, minRole) {
				writeForbiddenError(w, "insufficient permissions: requires "+minRole)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// hasMinRole checks if actualRole meets the minimum required role.
// admin > operator > viewer
func hasMinRole(actual, required string) bool {
	levels := map[string]int{
		"viewer":   1,
		"operator": 2,
		"admin":    3,
	}
	return levels[actual] >= levels[required]
}

// RequirePlatformAdmin returns middleware that enforces platform admin access.
func RequirePlatformAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsPlatformAdmin(r.Context()) {
				writeForbiddenError(w, "platform admin access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "UNAUTHORIZED",
			"message": message,
			"hint":    "Provide a valid API key in the X-API-Key header",
		},
	})
}

func writeForbiddenError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    "FORBIDDEN",
			"message": message,
		},
	})
}
