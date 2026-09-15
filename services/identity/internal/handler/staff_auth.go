package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"laundry-platform/shared/authtoken"
	"laundry-platform/shared/respond"
	"laundry-platform/shared/security"
)

// --- Request/response shapes (docs/07-api-contract.md §1) ---

type staffLoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type staffAuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int          `json:"expires_in"`
	User         staffSummary `json:"user"`
}

type staffSummary struct {
	ID       string   `json:"id"`
	FullName string   `json:"full_name"`
	Roles    []string `json:"roles"`
}

// StaffLogin implements POST /api/v1/auth/login.
func (h *Handler) StaffLogin(w http.ResponseWriter, r *http.Request) {
	var req staffLoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Identifier = strings.TrimSpace(req.Identifier)
	if req.Identifier == "" || req.Password == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Identifier and password are required.")
		return
	}

	ctx := r.Context()
	var (
		userID       uuid.UUID
		fullName     string
		passwordHash string
		isActive     bool
	)
	err := h.Pool.QueryRow(ctx, `
		SELECT id, full_name, password_hash, is_active
		FROM users
		WHERE email = $1 OR phone = $1
	`, req.Identifier).Scan(&userID, &fullName, &passwordHash, &isActive)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Identifier or password is incorrect.")
		return
	}
	if err != nil {
		h.Logger.Error("staff login: query user failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}
	if !isActive {
		respond.Error(w, r, http.StatusLocked, "ACCOUNT_INACTIVE", "This account has been deactivated.")
		return
	}

	ok, err := security.VerifyPassword(req.Password, passwordHash)
	if err != nil || !ok {
		respond.Error(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Identifier or password is incorrect.")
		return
	}

	roles, err := h.rolesForUser(ctx, userID)
	if err != nil {
		h.Logger.Error("staff login: fetch roles failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}
	outletIDs, err := h.outletsForUser(ctx, userID)
	if err != nil {
		h.Logger.Error("staff login: fetch outlets failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	if _, err := h.Pool.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, userID); err != nil {
		h.Logger.Warn("staff login: update last_login_at failed", "error", err)
	}

	h.issueStaffSession(w, r, userID, fullName, roles, outletIDs, http.StatusOK)
}

// StaffMe implements GET /api/v1/auth/me.
func (h *Handler) StaffMe(w http.ResponseWriter, r *http.Request, principalID string) {
	ctx := r.Context()
	userID, err := uuid.Parse(principalID)
	if err != nil {
		respond.Error(w, r, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid principal.")
		return
	}

	var fullName string
	err = h.Pool.QueryRow(ctx, `SELECT full_name FROM users WHERE id = $1`, userID).Scan(&fullName)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "User not found.")
		return
	}
	if err != nil {
		h.Logger.Error("staff me: query failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	roles, _ := h.rolesForUser(ctx, userID)
	outletIDs, _ := h.outletsForUser(ctx, userID)

	respond.JSON(w, http.StatusOK, map[string]any{
		"id":         userID.String(),
		"full_name":  fullName,
		"roles":      roles,
		"outlet_ids": outletIDs,
	})
}

func (h *Handler) issueStaffSession(w http.ResponseWriter, r *http.Request, userID uuid.UUID, fullName string, roles, outletIDs []string, status int) {
	accessToken, _, err := authtoken.Issue(h.Cfg.JWTSigningKey, userID.String(), authtoken.PrincipalStaff, roles, outletIDs, h.Cfg.AccessTokenTTL)
	if err != nil {
		h.Logger.Error("issue access token failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	refreshToken := uuid.NewString() + uuid.NewString()
	hash := hashToken(refreshToken)

	_, err = h.Pool.Exec(r.Context(), `
		INSERT INTO sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, hash, r.UserAgent(), clientIP(r), time.Now().Add(h.Cfg.RefreshTTL))
	if err != nil {
		h.Logger.Error("issue session: insert session failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong. Please try again.")
		return
	}

	respond.JSON(w, status, staffAuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(h.Cfg.AccessTokenTTL.Seconds()),
		User:         staffSummary{ID: userID.String(), FullName: fullName, Roles: roles},
	})
}

func (h *Handler) rolesForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := h.Pool.Query(ctx, `
		SELECT r.code FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = $1
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		roles = append(roles, code)
	}
	return roles, rows.Err()
}

func (h *Handler) outletsForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := h.Pool.Query(ctx, `SELECT outlet_id FROM user_outlets WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id.String())
	}
	return ids, rows.Err()
}
