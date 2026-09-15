package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"laundry-platform/shared/respond"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		respond.Error(w, r, http.StatusBadRequest, "MALFORMED_REQUEST", "Request body is not valid JSON.")
		return false
	}
	return true
}

// decodeJSONOptional decodes r's body into v if present, silently leaving
// v at its zero value for an empty body (used by endpoints whose request
// body is entirely optional, e.g. finalize-weighing).
func decodeJSONOptional(r *http.Request, v any) error {
	err := json.NewDecoder(r.Body).Decode(v)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
