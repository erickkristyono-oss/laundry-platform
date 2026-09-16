package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"laundry-platform/shared/respond"
)

type simulateGatewayRequest struct {
	Outcome string `json:"outcome"` // "PAID" or "FAILED"
}

// SimulateGatewayCallback implements POST /api/v1/payments/{id}/simulate.
// It stands in for a real gateway's webhook while PaymentProvider is
// "dummy" (see internal/gateway/dummy.go) — a human confirms or rejects
// the charge from the frontend's /payments/{id}/simulate page instead of a
// real provider posting a signed callback. Once a real provider is
// registered (PAYMENT_PROVIDER=midtrans/xendit/...), this endpoint refuses
// every request: the real webhook handler takes over that job.
func (h *Handler) SimulateGatewayCallback(w http.ResponseWriter, r *http.Request) {
	if h.Provider.Name() != "DUMMY" {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "No dummy payment provider is active.")
		return
	}

	id := chi.URLParam(r, "id")
	var req simulateGatewayRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Outcome != "PAID" && req.Outcome != "FAILED" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "outcome must be PAID or FAILED.")
		return
	}

	ctx := r.Context()
	payment, err := h.paymentByID(ctx, id)
	if err != nil {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Payment not found.")
		return
	}
	if payment.Provider != "DUMMY" {
		respond.Error(w, r, http.StatusConflict, "NOT_A_GATEWAY_PAYMENT", "This payment was not started through the gateway.")
		return
	}
	if payment.Status != "PENDING" {
		respond.Error(w, r, http.StatusConflict, "INVALID_STATE", "This payment is no longer pending.")
		return
	}

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	var paidAt *time.Time
	if req.Outcome == "PAID" {
		now := time.Now()
		paidAt = &now
	}
	if _, err := tx.Exec(ctx, `UPDATE payments SET status = $1, paid_at = $2 WHERE id = $3`, req.Outcome, paidAt, id); err != nil {
		if isUniqueViolation(err) {
			respond.Error(w, r, http.StatusConflict, "ORDER_ALREADY_PAID", "This order has already been paid.")
			return
		}
		h.Logger.Error("simulate gateway: update failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	txStatus := "SUCCESS"
	if req.Outcome == "FAILED" {
		txStatus = "FAILED"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO payment_transactions (payment_id, transaction_type, amount, status, gateway_response)
		VALUES ($1, 'CHARGE', $2, $3, $4)
	`, id, payment.Amount, txStatus, `{"simulated": true}`); err != nil {
		h.Logger.Error("simulate gateway: transaction log failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	eventType := "payment.paid"
	eventPayload := map[string]any{"payment_id": id, "order_id": payment.OrderID, "amount": payment.Amount, "paid_at": paidAt}
	if req.Outcome == "FAILED" {
		eventType = "payment.failed"
		eventPayload = map[string]any{"payment_id": id, "order_id": payment.OrderID, "amount": payment.Amount}
	}
	if err := writeOutboxEvent(ctx, tx, "payment", id, eventType, eventPayload); err != nil {
		h.Logger.Error("simulate gateway: outbox write failed", "error", err)
	}

	if err := tx.Commit(ctx); err != nil {
		h.Logger.Error("simulate gateway: commit failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	payment.Status = req.Outcome
	payment.PaidAt = paidAt
	respond.JSON(w, http.StatusOK, payment)
}
