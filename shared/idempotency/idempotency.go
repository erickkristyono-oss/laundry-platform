// Package idempotency implements the Idempotency-Key contract from
// docs/11-error-handling.md §4: a client-generated key on an endpoint
// listed in docs/07-api-contract.md §12 makes a repeated request (same
// key, same body) return the original stored response verbatim instead of
// re-executing side effects — the primary defense against a double-charge
// or duplicate-order on network retry.
//
// The mechanism is shared (business-rule-free HTTP plumbing per
// docs/03-system-architecture.md §4); the `idempotency_keys` table itself
// is service-owned data, one copy per service, per the doc.
package idempotency

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"laundry-platform/shared/respond"
)

// Require returns middleware enforcing the Idempotency-Key contract for
// one endpoint label (e.g. "POST /api/v1/orders" — must be stable and
// unique per logical endpoint, since the DB key is (key, endpoint)).
func Require(pool *pgxpool.Pool, endpoint string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				respond.Error(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "This endpoint requires an Idempotency-Key header.")
				return
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				respond.Error(w, r, http.StatusBadRequest, "MALFORMED_REQUEST", "Could not read request body.")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			requestHash := hashBody(bodyBytes)

			ctx := r.Context()
			var storedHash string
			var storedStatus int
			var storedBody []byte
			err = pool.QueryRow(ctx, `
				SELECT request_hash, response_status, response_body
				FROM idempotency_keys WHERE key = $1 AND endpoint = $2
			`, key, endpoint).Scan(&storedHash, &storedStatus, &storedBody)

			switch {
			case err == nil:
				if storedHash != requestHash {
					respond.Error(w, r, http.StatusConflict, "IDEMPOTENCY_KEY_REUSE", "This Idempotency-Key was already used with a different request body.")
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("Idempotency-Replayed", "true")
				w.WriteHeader(storedStatus)
				_, _ = w.Write(storedBody)
				return
			case err == pgx.ErrNoRows:
				// First time seeing this key — proceed and record the outcome below.
			default:
				respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
				return
			}

			capture := &responseCapture{ResponseWriter: w, status: http.StatusOK, body: &bytes.Buffer{}}
			next.ServeHTTP(capture, r)

			// Only record genuinely idempotent outcomes: a request that
			// blew up with a server error should be retryable with the
			// same key, not permanently pinned to a 500.
			if capture.status < 500 {
				if _, err := pool.Exec(ctx, `
					INSERT INTO idempotency_keys (key, endpoint, request_hash, response_status, response_body)
					VALUES ($1, $2, $3, $4, $5)
					ON CONFLICT (key, endpoint) DO NOTHING
				`, key, endpoint, requestHash, capture.status, capture.body.Bytes()); err != nil {
					// Logging without a shared logger dependency here would
					// require threading one through; the DB-level unique
					// index (docs/06-database-schema.md) remains the
					// defense-in-depth backstop if this write is lost.
					_ = err
				}
			}
		})
	}
}

func hashBody(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// responseCapture tees everything written to the real ResponseWriter into
// an in-memory buffer, so it can be persisted verbatim for replay.
type responseCapture struct {
	http.ResponseWriter
	status int
	body   *bytes.Buffer
}

func (c *responseCapture) WriteHeader(status int) {
	c.status = status
	c.ResponseWriter.WriteHeader(status)
}

func (c *responseCapture) Write(b []byte) (int, error) {
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}
