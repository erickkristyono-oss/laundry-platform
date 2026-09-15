// Package respond implements the standard error/success envelope shape
// defined in docs/11-error-handling.md §1. This is a shape-only helper
// (allowed in shared/ per docs/03-system-architecture.md §4) — it does not
// decide *which* error code applies to a given business situation; callers
// pass that in.
package respond

import (
	"encoding/json"
	"net/http"

	"laundry-platform/shared/logging"
)

// ErrorDetail is one field-level validation issue (docs/11-error-handling.md §1).
type ErrorDetail struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

// ErrorBody is the "error" object of the standard envelope.
type ErrorBody struct {
	Code          string        `json:"code"`
	Message       string        `json:"message"`
	Details       []ErrorDetail `json:"details,omitempty"`
	RequestID     string        `json:"request_id"`
	CorrelationID string        `json:"correlation_id,omitempty"`
}

type errorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// JSON writes v as the response body with the given status code.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes the standard error envelope (docs/11-error-handling.md §1),
// pulling request_id/correlation_id from ctx via the logging package so
// every handler doesn't have to thread them through by hand.
func Error(w http.ResponseWriter, r *http.Request, status int, code, message string, details ...ErrorDetail) {
	body := errorEnvelope{Error: ErrorBody{
		Code:          code,
		Message:       message,
		Details:       details,
		RequestID:     logging.RequestIDFromContext(r.Context()),
		CorrelationID: logging.CorrelationIDFromContext(r.Context()),
	}}
	JSON(w, status, body)
}
