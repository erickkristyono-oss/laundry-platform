package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"laundry-platform/shared/httpauth"
	"laundry-platform/shared/respond"
)

type taxRateResponse struct {
	RatePercentage string `json:"rate_percentage"`
	EffectiveFrom  string `json:"effective_from"`
}

// GetTaxRate implements GET /api/v1/settings/tax-rate (PUBLIC, UQ-09).
func (h *Handler) GetTaxRate(w http.ResponseWriter, r *http.Request) {
	rate, effectiveFrom, err := h.activeTaxRate(r.Context())
	if err != nil {
		h.Logger.Error("get tax rate failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, taxRateResponse{RatePercentage: rate, EffectiveFrom: effectiveFrom})
}

type setTaxRateRequest struct {
	RatePercentage float64 `json:"rate_percentage"`
	Reason         string  `json:"reason"`
}

// SetTaxRate implements PUT /api/v1/settings/tax-rate
// (OWNER/SUPER_ADMIN only — docs/07-api-contract.md §7, UQ-09).
func (h *Handler) SetTaxRate(w http.ResponseWriter, r *http.Request) {
	var req setTaxRateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.RatePercentage < 0 || req.RatePercentage > 100 || req.Reason == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "rate_percentage must be between 0 and 100 and reason is required.")
		return
	}

	principal := httpauth.FromRequest(r)
	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE tax_rates SET effective_to = now() WHERE effective_to IS NULL`); err != nil {
		h.Logger.Error("close previous tax rate failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	id := uuid.New().String()
	if _, err := tx.Exec(ctx, `
		INSERT INTO tax_rates (id, rate_percentage, reason, created_by) VALUES ($1, $2, $3, $4)
	`, id, req.RatePercentage, req.Reason, principal.ID); err != nil {
		h.Logger.Error("insert tax rate failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{"id": id, "rate_percentage": req.RatePercentage, "reason": req.Reason})
}

// activeTaxRateValue returns the active rate as a float for computation
// (finalize-weighing, orders.go) — kept separate from activeTaxRate below,
// which returns the string-formatted API response shape.
func (h *Handler) activeTaxRateValue(ctx context.Context) (float64, error) {
	var rate float64
	err := h.Pool.QueryRow(ctx, `SELECT rate_percentage FROM tax_rates WHERE effective_to IS NULL`).Scan(&rate)
	return rate, err
}

func (h *Handler) activeTaxRate(ctx context.Context) (ratePercentage string, effectiveFrom string, err error) {
	var effFrom time.Time
	err = h.Pool.QueryRow(ctx, `
		SELECT rate_percentage::text, effective_from FROM tax_rates WHERE effective_to IS NULL
	`).Scan(&ratePercentage, &effFrom)
	if err != nil {
		return "", "", err
	}
	return ratePercentage, effFrom.Format(time.RFC3339), nil
}
