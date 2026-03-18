package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Middleware records Prometheus metrics for each HTTP request.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip metrics for /metrics endpoint itself
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		rw := &responseWriter{ResponseWriter: w, status: 200}
		start := time.Now()

		next.ServeHTTP(rw, r)

		// Normalize path: replace ULIDs/IDs with :id
		path := normalizePath(r.URL.Path)
		status := strconv.Itoa(rw.status)

		APIRequests.WithLabelValues(r.Method, path, status).Inc()
		APILatency.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}

// normalizePath replaces dynamic path segments with :id for cardinality control.
func normalizePath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		// Replace anything that looks like a prefixed ULID (4 chars + _ + 26 chars)
		if len(p) > 20 && strings.Contains(p[:6], "_") {
			parts[i] = ":id"
		}
	}
	return strings.Join(parts, "/")
}
