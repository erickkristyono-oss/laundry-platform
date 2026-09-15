// Package logging provides the structured JSON logger baseline required by
// docs/13-observability.md §1: one JSON object per line, with service name,
// request_id and correlation_id attached from context where available.
//
// This is pure plumbing (allowed in shared/ per docs/03-system-architecture.md §4)
// — it encodes no business rule.
package logging

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey string

const (
	ctxRequestID     ctxKey = "request_id"
	ctxCorrelationID ctxKey = "correlation_id"
)

// New returns a JSON slog.Logger with the given service name attached to
// every line, per docs/13-observability.md §1's minimum-fields requirement.
func New(serviceName string, level slog.Level) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	return slog.New(handler).With(slog.String("service", serviceName))
}

// WithRequestID returns a context carrying the given request ID, so a later
// FromContext call can attach it to log lines automatically.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxRequestID, requestID)
}

// WithCorrelationID returns a context carrying the given correlation ID
// (docs/13-observability.md §3 — a business operation may span many events).
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, ctxCorrelationID, correlationID)
}

// RequestIDFromContext returns the request ID stored on ctx, or "".
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxRequestID).(string)
	return v
}

// CorrelationIDFromContext returns the correlation ID stored on ctx, or "".
func CorrelationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxCorrelationID).(string)
	return v
}

// FromContext returns a logger enriched with request_id/correlation_id
// pulled from ctx, if present, so call sites don't need to thread them
// through manually on every log call.
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	l := base
	if rid := RequestIDFromContext(ctx); rid != "" {
		l = l.With(slog.String("request_id", rid))
	}
	if cid := CorrelationIDFromContext(ctx); cid != "" {
		l = l.With(slog.String("correlation_id", cid))
	}
	return l
}
