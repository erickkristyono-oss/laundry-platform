package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"laundry-platform/shared/respond"
)

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// StaffRefresh implements POST /api/v1/auth/refresh. Refresh tokens are
// single-use with rotation (docs/12-security-baseline.md §4): the
// presented token is revoked and a new one issued in the same call.
func (h *Handler) StaffRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	hash := hashToken(req.RefreshToken)

	ctx := r.Context()
	var (
		sessionID uuid.UUID
		userID    uuid.UUID
		expiresAt time.Time
		revokedAt *time.Time
	)
	err := h.Pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked_at FROM sessions WHERE refresh_token_hash = $1
	`, hash).Scan(&sessionID, &userID, &expiresAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) || revokedAt != nil || time.Now().After(expiresAt) {
		respond.Error(w, r, http.StatusUnauthorized, "TOKEN_REVOKED", "Refresh token is invalid, expired, or already used.")
		return
	}
	if err != nil {
		h.Logger.Error("staff refresh: query failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if _, err := h.Pool.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1`, sessionID); err != nil {
		h.Logger.Error("staff refresh: revoke old session failed", "error", err)
	}

	var fullName string
	_ = h.Pool.QueryRow(ctx, `SELECT full_name FROM users WHERE id = $1`, userID).Scan(&fullName)
	roles, _ := h.rolesForUser(ctx, userID)
	outletIDs, _ := h.outletsForUser(ctx, userID)

	h.issueStaffSession(w, r, userID, fullName, roles, outletIDs, http.StatusOK)
}

// StaffLogout implements POST /api/v1/auth/logout.
func (h *Handler) StaffLogout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	hash := hashToken(req.RefreshToken)
	if _, err := h.Pool.Exec(r.Context(), `UPDATE sessions SET revoked_at = now() WHERE refresh_token_hash = $1 AND revoked_at IS NULL`, hash); err != nil {
		h.Logger.Error("staff logout: revoke failed", "error", err)
	}
	w.WriteHeader(http.StatusNoContent)
}

// CustomerRefresh implements POST /api/v1/customer-auth/refresh.
func (h *Handler) CustomerRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	hash := hashToken(req.RefreshToken)

	ctx := r.Context()
	var (
		sessionID  uuid.UUID
		accountID  uuid.UUID
		customerID uuid.UUID
		expiresAt  time.Time
		revokedAt  *time.Time
	)
	err := h.Pool.QueryRow(ctx, `
		SELECT cs.id, cs.customer_account_id, ca.customer_id, cs.expires_at, cs.revoked_at
		FROM customer_sessions cs
		JOIN customer_accounts ca ON ca.id = cs.customer_account_id
		WHERE cs.refresh_token_hash = $1
	`, hash).Scan(&sessionID, &accountID, &customerID, &expiresAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) || revokedAt != nil || time.Now().After(expiresAt) {
		respond.Error(w, r, http.StatusUnauthorized, "TOKEN_REVOKED", "Refresh token is invalid, expired, or already used.")
		return
	}
	if err != nil {
		h.Logger.Error("customer refresh: query failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if _, err := h.Pool.Exec(ctx, `UPDATE customer_sessions SET revoked_at = now() WHERE id = $1`, sessionID); err != nil {
		h.Logger.Error("customer refresh: revoke old session failed", "error", err)
	}

	customer, err := h.fetchCoreCustomer(ctx, customerID.String())
	if err != nil {
		respond.Error(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Could not reach the customer profile service.")
		return
	}

	h.issueCustomerSession(w, r, accountID.String(), customer, http.StatusOK)
}

// CustomerLogout implements POST /api/v1/customer-auth/logout.
func (h *Handler) CustomerLogout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	hash := hashToken(req.RefreshToken)
	if _, err := h.Pool.Exec(r.Context(), `UPDATE customer_sessions SET revoked_at = now() WHERE refresh_token_hash = $1 AND revoked_at IS NULL`, hash); err != nil {
		h.Logger.Error("customer logout: revoke failed", "error", err)
	}
	w.WriteHeader(http.StatusNoContent)
}
