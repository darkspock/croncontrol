// Package frontend serves the embedded React SPA.
package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist/*
var distFS embed.FS

// Handler returns an http.Handler that serves the embedded frontend.
// For non-API routes, it serves index.html (SPA catch-all).
func Handler() http.Handler {
	dist, _ := fs.Sub(distFS, "dist")
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// API, health, and auth routes are handled by the Go server
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/health") || strings.HasPrefix(path, "/auth/") {
			http.NotFound(w, r)
			return
		}

		// Try to serve the file directly
		if path != "/" {
			// Check if file exists
			f, err := dist.Open(strings.TrimPrefix(path, "/"))
			if err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// SPA catch-all: serve index.html for all other routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
