// Package health implements the /healthz (liveness) and /readyz (readiness)
// contract from docs/13-observability.md §6: liveness never checks external
// dependencies (a slow DB must not get a healthy process killed); readiness
// runs the given checks (DB, broker, etc.) with a bounded timeout.
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// Check is one readiness dependency probe (e.g. pool.Ping).
type Check func(ctx context.Context) error

// Livez always returns 200 while the process is running.
func Livez(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"alive"}`))
}

// Readyz runs every check with a bounded timeout and returns 200 only if
// all pass, per docs/13-observability.md §6.
func Readyz(checks map[string]Check) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		results := map[string]string{}
		allOK := true
		for name, check := range checks {
			if err := check(ctx); err != nil {
				results[name] = err.Error()
				allOK = false
				continue
			}
			results[name] = "ok"
		}

		status := http.StatusOK
		if !allOK {
			status = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": map[bool]string{true: "ready", false: "not_ready"}[allOK],
			"checks": results,
		})
	}
}
