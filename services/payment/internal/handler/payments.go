package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"laundry-platform/shared/bizid"
	"laundry-platform/shared/respond"
)

var validMethods = map[string]bool{"CASH": true, "QRIS": true, "TRANSFER": true, "CARD": true}

type paymentResponse struct {
	ID          string     `json:"id"`
	PaymentCode string     `json:"payment_code"`
	OrderID     string     `json:"order_id"`
	CustomerID  string     `json:"customer_id"`
	Amount      int64      `json:"amount"`
	Method      string     `json:"method"`
	Status      string     `json:"status"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type createPaymentRequest struct {
	OrderID string `json:"order_id"`
	Amount  int64  `json:"amount"`
	Method  string `json:"method"`
}

// CreatePayment implements POST /api/v1/payments (docs/07-api-contract.md §11):
// synchronously re-verifies the amount against Core rather than trusting
// the client (docs/14-architecture-decisions.md ADR-011), then — for CASH —
// marks the payment PAID immediately (QRIS/TRANSFER/CARD gateway callbacks
// are a Phase 1+ integration, out of scope here).
func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
	var req createPaymentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.OrderID == "" || req.Amount <= 0 || !validMethods[req.Method] {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "order_id, a positive amount, and a valid method are required.")
		return
	}

	ctx := r.Context()

	order, err := h.fetchCoreOrder(ctx, req.OrderID)
	if err != nil {
		h.Logger.Error("create payment: fetch core order failed", "error", err)
		respond.Error(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Could not verify the order right now.")
		return
	}
	if order.Status != "WEIGHING" || order.TotalAmount == nil {
		respond.Error(w, r, http.StatusConflict, "INVALID_ORDER_STATE", "Order must have finalized weighing before payment.")
		return
	}
	if order.PaymentStatus == "PAID" {
		respond.Error(w, r, http.StatusConflict, "ORDER_ALREADY_PAID", "This order has already been paid.")
		return
	}
	if *order.TotalAmount != req.Amount {
		respond.Error(w, r, http.StatusConflict, "AMOUNT_MISMATCH", "Payment amount does not match the order's total.")
		return
	}

	// TODO(phase-2+): verify principal authorization (CASHIER/OUTLET_ADMIN
	// for the order's outlet, or the owning customer for self-checkout) —
	// deferred alongside the customer-ownership TODO in Core's
	// canAccessOrder. Every caller must still present a valid access
	// token (RequireAuth in router.go); only the fine-grained ownership
	// check is deferred.

	id := uuid.New().String()
	paymentCode := bizid.Payment()
	status := "PENDING"
	var paidAt *time.Time
	if req.Method == "CASH" {
		now := time.Now()
		paidAt = &now
		status = "PAID"
	}

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO payments (id, payment_code, order_id, customer_id, amount, method, status, paid_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, paymentCode, req.OrderID, order.CustomerID, req.Amount, req.Method, status, paidAt)
	if err != nil {
		if isUniqueViolation(err) {
			respond.Error(w, r, http.StatusConflict, "ORDER_ALREADY_PAID", "This order has already been paid.")
			return
		}
		h.Logger.Error("create payment: insert failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	txType := "CHARGE"
	txStatus := "PENDING"
	if status == "PAID" {
		txStatus = "SUCCESS"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO payment_transactions (payment_id, transaction_type, amount, status) VALUES ($1, $2, $3, $4)
	`, id, txType, req.Amount, txStatus); err != nil {
		h.Logger.Error("create payment: transaction log failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if err := writeOutboxEvent(ctx, tx, "payment", id, "payment.created", map[string]any{
		"payment_id": id, "payment_code": paymentCode, "order_id": req.OrderID, "amount": req.Amount, "method": req.Method, "status": status,
	}); err != nil {
		h.Logger.Error("create payment: outbox write (created) failed", "error", err)
	}
	if status == "PAID" {
		if err := writeOutboxEvent(ctx, tx, "payment", id, "payment.paid", map[string]any{
			"payment_id": id, "order_id": req.OrderID, "amount": req.Amount, "paid_at": paidAt,
		}); err != nil {
			h.Logger.Error("create payment: outbox write (paid) failed", "error", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		h.Logger.Error("create payment: commit failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	respond.JSON(w, http.StatusCreated, paymentResponse{
		ID: id, PaymentCode: paymentCode, OrderID: req.OrderID, CustomerID: order.CustomerID,
		Amount: req.Amount, Method: req.Method, Status: status, PaidAt: paidAt, CreatedAt: time.Now(),
	})
}

// GetPayment implements GET /api/v1/payments/{id}.
func (h *Handler) GetPayment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	out, err := h.paymentByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Payment not found.")
		return
	}
	if err != nil {
		h.Logger.Error("get payment failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// ListPayments implements GET /api/v1/payments?order_id=.
func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	if orderID == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "order_id is required.")
		return
	}

	rows, err := h.Pool.Query(r.Context(), `
		SELECT id, payment_code, order_id, customer_id, amount, method, status, paid_at, created_at
		FROM payments WHERE order_id = $1 ORDER BY created_at DESC
	`, orderID)
	if err != nil {
		h.Logger.Error("list payments failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer rows.Close()

	out := []paymentResponse{}
	for rows.Next() {
		var p paymentResponse
		if err := rows.Scan(&p.ID, &p.PaymentCode, &p.OrderID, &p.CustomerID, &p.Amount, &p.Method, &p.Status, &p.PaidAt, &p.CreatedAt); err != nil {
			h.Logger.Error("list payments: scan failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		out = append(out, p)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) paymentByID(ctx context.Context, id string) (paymentResponse, error) {
	var p paymentResponse
	err := h.Pool.QueryRow(ctx, `
		SELECT id, payment_code, order_id, customer_id, amount, method, status, paid_at, created_at
		FROM payments WHERE id = $1
	`, id).Scan(&p.ID, &p.PaymentCode, &p.OrderID, &p.CustomerID, &p.Amount, &p.Method, &p.Status, &p.PaidAt, &p.CreatedAt)
	return p, err
}
