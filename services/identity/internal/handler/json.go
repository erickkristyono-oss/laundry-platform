package handler

import (
	"encoding/json"
	"net/http"

	"laundry-platform/shared/respond"
)

// decodeJSON reads and decodes r's body into v, writing the standard
// MALFORMED_REQUEST error (docs/11-error-handling.md §2) on failure.
// Returns false if it already wrote a response — callers should return.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		respond.Error(w, r, http.StatusBadRequest, "MALFORMED_REQUEST", "Request body is not valid JSON.")
		return false
	}
	return true
}
