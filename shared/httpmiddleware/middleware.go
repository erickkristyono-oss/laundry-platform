// Package httpmiddleware provides cross-cutting HTTP middleware with no
// business meaning (request-ID injection, panic recovery, access logging),
// per the allow-list in docs/03-system-architecture.md §4.
package httpmiddleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"laundry-platform/shared/logging"
)

const RequestIDHeader = "X-Request-ID"

// RequestID assigns a request ID (docs/13-observability.md §2): reuses the
// client-supplied X-Request-ID if present, otherwise generates a UUID.
// Propagated unchanged downstream and echoed back in the response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(RequestIDHeader)
		if reqID == "" {
			reqID = uuid.NewString()
		}
		w.Header().Set(RequestIDHeader, reqID)
		ctx := logging.WithRequestID(r.Context(), reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Recover converts a panic into a structured 500 log line instead of
// crashing the process, per the "never expose internals" rule in
// docs/11-error-handling.md.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logging.FromContext(r.Context(), logger).Error("panic recovered",
						slog.Any("panic", rec),
						slog.String("path", r.URL.Path),
					)
					w.WriteHeader(http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// AccessLog logs one structured line per request: method, path, status,
// duration — the HTTP metrics/logging baseline in docs/13-observability.md §1/§5.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			logging.FromContext(r.Context(), logger).Info("http_request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.status),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
