package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"laundry-platform/shared/httpauth"
	"laundry-platform/shared/respond"
	"laundry-platform/shared/security"
)

var globalRoles = map[string]bool{"OWNER": true, "SUPER_ADMIN": true}

type userResponse struct {
	ID        string   `json:"id"`
	FullName  string   `json:"full_name"`
	Email     *string  `json:"email,omitempty"`
	Phone     *string  `json:"phone,omitempty"`
	IsActive  bool     `json:"is_active"`
	Roles     []string `json:"roles"`
	OutletIDs []string `json:"outlet_ids"`
}

type createUserRequest struct {
	FullName  string   `json:"full_name"`
	Email     string   `json:"email"`
	Phone     string   `json:"phone"`
	Password  string   `json:"password"`
	RoleCodes []string `json:"role_codes"`
	OutletIDs []string `json:"outlet_ids"`
}

// CreateUser implements POST /api/v1/users (docs/07-api-contract.md §3).
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.FullName = strings.TrimSpace(req.FullName)
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)

	var details []respond.ErrorDetail
	if req.FullName == "" {
		details = append(details, respond.ErrorDetail{Field: "full_name", Issue: "required"})
	}
	if req.Email == "" && req.Phone == "" {
		details = append(details, respond.ErrorDetail{Field: "email", Issue: "at least one of email/phone is required"})
	}
	if len(req.Password) < 8 {
		details = append(details, respond.ErrorDetail{Field: "password", Issue: "minimum 8 characters"})
	}
	if len(req.RoleCodes) == 0 {
		details = append(details, respond.ErrorDetail{Field: "role_codes", Issue: "required, non-empty"})
	}
	hasGlobalRole := false
	for _, code := range req.RoleCodes {
		if globalRoles[code] {
			hasGlobalRole = true
		}
	}
	if !hasGlobalRole && len(req.OutletIDs) == 0 {
		details = append(details, respond.ErrorDetail{Field: "outlet_ids", Issue: "required unless role is OWNER/SUPER_ADMIN"})
	}
	if len(details) > 0 {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "One or more fields are invalid.", details...)
		return
	}

	ctx := r.Context()

	roleIDs, unknown, err := h.roleIDsForCodes(ctx, req.RoleCodes)
	if err != nil {
		h.Logger.Error("create user: lookup roles failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	if len(unknown) > 0 {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Unknown role code(s): "+strings.Join(unknown, ", "))
		return
	}

	hash, err := security.HashPassword(req.Password)
	if err != nil {
		h.Logger.Error("create user: hash password failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	var email, phone *string
	if req.Email != "" {
		email = &req.Email
	}
	if req.Phone != "" {
		phone = &req.Phone
	}

	userID := uuid.New().String()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, email, phone, full_name, password_hash) VALUES ($1, $2, $3, $4, $5)
	`, userID, email, phone, req.FullName, hash); err != nil {
		if isUniqueViolation(err) {
			respond.Error(w, r, http.StatusConflict, "EMAIL_ALREADY_EXISTS", "A user with this email or phone already exists.")
			return
		}
		h.Logger.Error("create user: insert failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	for _, roleID := range roleIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, userID, roleID); err != nil {
			h.Logger.Error("create user: assign role failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
	}
	if !hasGlobalRole {
		for _, outletID := range req.OutletIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO user_outlets (user_id, outlet_id) VALUES ($1, $2)`, userID, outletID); err != nil {
				h.Logger.Error("create user: assign outlet failed", "error", err)
				respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
				return
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	out, err := h.fetchUser(ctx, userID)
	if err != nil {
		h.Logger.Error("create user: fetch after create failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "User created but could not be loaded.")
		return
	}
	respond.JSON(w, http.StatusCreated, out)
}

// ListUsers implements GET /api/v1/users — OUTLET_ADMIN is scoped to their
// own outlet's staff (docs/07-api-contract.md §3).
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	principal := httpauth.FromRequest(r)
	ctx := r.Context()

	var rows pgx.Rows
	var err error
	if principal.IsGlobalStaff() {
		rows, err = h.Pool.Query(ctx, `SELECT id FROM users ORDER BY created_at DESC`)
	} else {
		if len(principal.OutletIDs) == 0 {
			respond.JSON(w, http.StatusOK, map[string]any{"data": []userResponse{}})
			return
		}
		rows, err = h.Pool.Query(ctx, `
			SELECT DISTINCT u.id FROM users u
			JOIN user_outlets uo ON uo.user_id = u.id
			WHERE uo.outlet_id = ANY($1)
			ORDER BY u.created_at DESC
		`, principal.OutletIDs)
	}
	if err != nil {
		h.Logger.Error("list users failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			h.Logger.Error("list users: scan failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		ids = append(ids, id)
	}
	rows.Close()

	out := make([]userResponse, 0, len(ids))
	for _, id := range ids {
		u, err := h.fetchUser(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, u)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// GetUser implements GET /api/v1/users/{id}.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	principal := httpauth.FromRequest(r)

	out, err := h.fetchUser(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "User not found.")
		return
	}
	if err != nil {
		h.Logger.Error("get user failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if !principal.IsGlobalStaff() {
		inScope := false
		for _, outletID := range out.OutletIDs {
			if principal.CanAccessOutlet(outletID) {
				inScope = true
				break
			}
		}
		if !inScope {
			respond.Error(w, r, http.StatusForbidden, "FORBIDDEN", "You are not authorized to view this user.")
			return
		}
	}

	respond.JSON(w, http.StatusOK, out)
}

type updateUserRequest struct {
	IsActive  *bool    `json:"is_active"`
	RoleCodes []string `json:"role_codes"`
	OutletIDs []string `json:"outlet_ids"`
}

// UpdateUser implements PATCH /api/v1/users/{id} (SUPER_ADMIN/OWNER only).
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, id).Scan(&exists); err != nil {
		h.Logger.Error("update user: exists check failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	if !exists {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "User not found.")
		return
	}

	if req.IsActive != nil {
		if _, err := tx.Exec(ctx, `UPDATE users SET is_active = $1, updated_at = now() WHERE id = $2`, *req.IsActive, id); err != nil {
			h.Logger.Error("update user: set is_active failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
	}

	if req.RoleCodes != nil {
		roleIDs, unknown, err := h.roleIDsForCodesTx(ctx, tx, req.RoleCodes)
		if err != nil {
			h.Logger.Error("update user: lookup roles failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		if len(unknown) > 0 {
			respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Unknown role code(s): "+strings.Join(unknown, ", "))
			return
		}
		if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1`, id); err != nil {
			h.Logger.Error("update user: clear roles failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		for _, roleID := range roleIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, id, roleID); err != nil {
				h.Logger.Error("update user: assign role failed", "error", err)
				respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
				return
			}
		}
	}

	if req.OutletIDs != nil {
		if _, err := tx.Exec(ctx, `DELETE FROM user_outlets WHERE user_id = $1`, id); err != nil {
			h.Logger.Error("update user: clear outlets failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		for _, outletID := range req.OutletIDs {
			if _, err := tx.Exec(ctx, `INSERT INTO user_outlets (user_id, outlet_id) VALUES ($1, $2)`, id, outletID); err != nil {
				h.Logger.Error("update user: assign outlet failed", "error", err)
				respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
				return
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	out, err := h.fetchUser(ctx, id)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// --- helpers ---

func (h *Handler) fetchUser(ctx context.Context, id string) (userResponse, error) {
	var out userResponse
	err := h.Pool.QueryRow(ctx, `SELECT id, full_name, email, phone, is_active FROM users WHERE id = $1`, id).
		Scan(&out.ID, &out.FullName, &out.Email, &out.Phone, &out.IsActive)
	if err != nil {
		return userResponse{}, err
	}

	roles, err := h.rolesForUser(ctx, uuid.MustParse(id))
	if err != nil {
		return userResponse{}, err
	}
	out.Roles = roles

	outlets, err := h.outletsForUser(ctx, uuid.MustParse(id))
	if err != nil {
		return userResponse{}, err
	}
	out.OutletIDs = outlets

	return out, nil
}

func (h *Handler) roleIDsForCodes(ctx context.Context, codes []string) (ids []string, unknown []string, err error) {
	return h.roleIDsForCodesTx(ctx, h.Pool, codes)
}

// dbQuerier abstracts pgxpool.Pool/pgx.Tx so role lookup works both inside
// and outside an explicit transaction.
type dbQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (h *Handler) roleIDsForCodesTx(ctx context.Context, q dbQuerier, codes []string) (ids []string, unknown []string, err error) {
	found := map[string]string{}
	rows, err := q.Query(ctx, `SELECT id, code FROM roles WHERE code = ANY($1)`, codes)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, code string
		if err := rows.Scan(&id, &code); err != nil {
			return nil, nil, err
		}
		found[code] = id
	}
	for _, code := range codes {
		if id, ok := found[code]; ok {
			ids = append(ids, id)
		} else {
			unknown = append(unknown, code)
		}
	}
	return ids, unknown, nil
}
