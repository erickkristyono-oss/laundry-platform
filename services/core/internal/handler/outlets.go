package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"laundry-platform/shared/respond"
)

type outletResponse struct {
	ID       string  `json:"id"`
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Address  string  `json:"address"`
	Phone    *string `json:"phone,omitempty"`
	IsActive bool    `json:"is_active"`
}

// ListOutlets implements GET /api/v1/outlets.
func (h *Handler) ListOutlets(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(), `SELECT id, code, name, address, phone, is_active FROM outlets WHERE is_active ORDER BY name`)
	if err != nil {
		h.Logger.Error("list outlets failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer rows.Close()

	var out []outletResponse
	for rows.Next() {
		var o outletResponse
		if err := rows.Scan(&o.ID, &o.Code, &o.Name, &o.Address, &o.Phone, &o.IsActive); err != nil {
			h.Logger.Error("scan outlet failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		out = append(out, o)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"data": out})
}

type createOutletRequest struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Phone   string `json:"phone"`
}

// CreateOutlet implements POST /api/v1/outlets (SUPER_ADMIN/OWNER only —
// enforced by the caller's route registration, see router.go).
func (h *Handler) CreateOutlet(w http.ResponseWriter, r *http.Request) {
	var req createOutletRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	req.Address = strings.TrimSpace(req.Address)
	if req.Code == "" || req.Name == "" || req.Address == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "code, name, and address are required.")
		return
	}

	var phone *string
	if req.Phone != "" {
		phone = &req.Phone
	}

	id := uuid.New().String()
	_, err := h.Pool.Exec(r.Context(), `
		INSERT INTO outlets (id, code, name, address, phone) VALUES ($1, $2, $3, $4, $5)
	`, id, req.Code, req.Name, req.Address, phone)
	if err != nil {
		h.Logger.Error("create outlet failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	respond.JSON(w, http.StatusCreated, outletResponse{ID: id, Code: req.Code, Name: req.Name, Address: req.Address, Phone: phone, IsActive: true})
}

// RecommendOutlet implements GET /api/v1/outlets/recommend?postal_code=&area_label=
// (BR-10/BR-13): a simple coverage-area lookup, falling back to the first
// active outlet if no coverage row matches — no geospatial infra per
// Master Prompt §10's "do not introduce AI or complex geospatial
// infrastructure unless justified".
func (h *Handler) RecommendOutlet(w http.ResponseWriter, r *http.Request) {
	postalCode := r.URL.Query().Get("postal_code")
	areaLabel := r.URL.Query().Get("area_label")

	ctx := r.Context()
	var outlet outletResponse
	err := h.Pool.QueryRow(ctx, `
		SELECT o.id, o.code, o.name, o.address, o.phone, o.is_active
		FROM outlet_coverage_areas ca
		JOIN outlets o ON o.id = ca.outlet_id AND o.is_active
		WHERE ($1 <> '' AND ca.postal_code = $1) OR ($2 <> '' AND ca.area_label = $2)
		ORDER BY ca.priority ASC
		LIMIT 1
	`, postalCode, areaLabel).Scan(&outlet.ID, &outlet.Code, &outlet.Name, &outlet.Address, &outlet.Phone, &outlet.IsActive)

	if errors.Is(err, pgx.ErrNoRows) {
		err = h.Pool.QueryRow(ctx, `SELECT id, code, name, address, phone, is_active FROM outlets WHERE is_active ORDER BY created_at LIMIT 1`).
			Scan(&outlet.ID, &outlet.Code, &outlet.Name, &outlet.Address, &outlet.Phone, &outlet.IsActive)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusUnprocessableEntity, "NO_OUTLET_AVAILABLE", "No outlet is currently available to serve this order.")
		return
	}
	if err != nil {
		h.Logger.Error("recommend outlet failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	respond.JSON(w, http.StatusOK, outlet)
}
