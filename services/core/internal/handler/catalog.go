package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"laundry-platform/shared/respond"
)

type serviceResponse struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Unit         string `json:"unit"`
	IsActive     bool   `json:"is_active"`
	PricePerUnit *int64 `json:"price_per_unit,omitempty"`
}

// ListServices implements GET /api/v1/services — active catalog with
// current effective (global) price (docs/07-api-contract.md §6).
func (h *Handler) ListServices(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Pool.Query(r.Context(), `
		SELECT s.id, s.code, s.name, s.unit, s.is_active, sp.price_per_unit
		FROM services s
		LEFT JOIN service_prices sp ON sp.service_id = s.id AND sp.outlet_id IS NULL AND sp.effective_to IS NULL
		WHERE s.is_active
		ORDER BY s.name
	`)
	if err != nil {
		h.Logger.Error("list services failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer rows.Close()

	var out []serviceResponse
	for rows.Next() {
		var s serviceResponse
		if err := rows.Scan(&s.ID, &s.Code, &s.Name, &s.Unit, &s.IsActive, &s.PricePerUnit); err != nil {
			h.Logger.Error("scan service failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		out = append(out, s)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"data": out})
}

type createServiceRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Unit string `json:"unit"`
}

// CreateService implements POST /api/v1/services (SUPER_ADMIN/OWNER).
func (h *Handler) CreateService(w http.ResponseWriter, r *http.Request) {
	var req createServiceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	if req.Unit == "" {
		req.Unit = "kg"
	}
	if req.Code == "" || req.Name == "" || (req.Unit != "kg" && req.Unit != "pcs") {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "code, name are required and unit must be 'kg' or 'pcs'.")
		return
	}

	id := uuid.New().String()
	_, err := h.Pool.Exec(r.Context(), `INSERT INTO services (id, code, name, unit) VALUES ($1, $2, $3, $4)`, id, req.Code, req.Name, req.Unit)
	if err != nil {
		h.Logger.Error("create service failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	respond.JSON(w, http.StatusCreated, serviceResponse{ID: id, Code: req.Code, Name: req.Name, Unit: req.Unit, IsActive: true})
}

type createPricingRequest struct {
	ServiceID    string `json:"service_id"`
	OutletID     string `json:"outlet_id"`
	PricePerUnit int64  `json:"price_per_unit"`
}

// CreatePricing implements POST /api/v1/pricing (SUPER_ADMIN/OWNER) —
// inserts a new versioned price row, closing the previous active one for
// the same (service_id, outlet_id) so history is preserved (BR-08).
func (h *Handler) CreatePricing(w http.ResponseWriter, r *http.Request) {
	var req createPricingRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ServiceID == "" || req.PricePerUnit <= 0 {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "service_id is required and price_per_unit must be > 0.")
		return
	}

	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	var outletID *string
	if req.OutletID != "" {
		outletID = &req.OutletID
	}

	if outletID == nil {
		if _, err := tx.Exec(ctx, `UPDATE service_prices SET effective_to = now() WHERE service_id = $1 AND outlet_id IS NULL AND effective_to IS NULL`, req.ServiceID); err != nil {
			h.Logger.Error("close previous price failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE service_prices SET effective_to = now() WHERE service_id = $1 AND outlet_id = $2 AND effective_to IS NULL`, req.ServiceID, outletID); err != nil {
			h.Logger.Error("close previous price failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
	}

	id := uuid.New().String()
	if _, err := tx.Exec(ctx, `
		INSERT INTO service_prices (id, service_id, outlet_id, price_per_unit) VALUES ($1, $2, $3, $4)
	`, id, req.ServiceID, outletID, req.PricePerUnit); err != nil {
		h.Logger.Error("insert price failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{"id": id, "service_id": req.ServiceID, "outlet_id": outletID, "price_per_unit": req.PricePerUnit})
}

// GetActivePrice implements GET /api/v1/pricing?service_id=&outlet_id=
// resolved outlet-specific-first-then-global (BR-08/BR-09).
func (h *Handler) GetActivePrice(w http.ResponseWriter, r *http.Request) {
	serviceID := r.URL.Query().Get("service_id")
	outletID := r.URL.Query().Get("outlet_id")
	if serviceID == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "service_id is required.")
		return
	}

	price, err := h.activePrice(r.Context(), serviceID, outletID)
	if err != nil {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "No active price found for this service.")
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"service_id": serviceID, "price_per_unit": price})
}

// activePrice resolves outlet-specific first, else global — the read
// pattern documented for service_prices (docs/06-database-schema.md §2.6).
func (h *Handler) activePrice(ctx context.Context, serviceID, outletID string) (int64, error) {
	var price int64
	if outletID != "" {
		err := h.Pool.QueryRow(ctx, `
			SELECT price_per_unit FROM service_prices
			WHERE service_id = $1 AND outlet_id = $2 AND effective_to IS NULL
		`, serviceID, outletID).Scan(&price)
		if err == nil {
			return price, nil
		}
	}
	err := h.Pool.QueryRow(ctx, `
		SELECT price_per_unit FROM service_prices
		WHERE service_id = $1 AND outlet_id IS NULL AND effective_to IS NULL
	`, serviceID).Scan(&price)
	return price, err
}
