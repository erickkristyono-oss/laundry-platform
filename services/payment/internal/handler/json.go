package handler

import (
	"encoding/json"
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
