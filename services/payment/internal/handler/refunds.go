package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"laundry-platform/shared/bizid"
	"laundry-platform/shared/httpauth"
	"laundry-platform/shared/respond"
)

// refundEligibilityWindow is UQ-04's recommended default (docs/PHASE-0-REVIEW.md §3):
// 7 days post-completion, configurable later.
const refundEligibilityWindow = 7 * 24 * time.Hour

type refundResponse struct {
	ID          string     `json:"id"`
	RefundCode  string     `json:"refund_code"`
	PaymentID   string     `json:"payment_id"`
	OrderID     string     `json:"order_id"`
	Amount      int64      `json:"amount"`
	Status      string     `json:"status"`
	Reason      string     `json:"reason"`
	RequestedBy string     `json:"requested_by"`
	ApprovedBy  *string    `json:"approved_by,omitempty"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type createRefundRequest struct {
	PaymentID string `json:"payment_id"`
	Reason    string `json:"reason"`
}

// CreateRefund implements POST /api/v1/refunds (docs/07-api-contract.md §12).
//
// TODO(phase-2+): docs also allow the *owning customer* to self-request a
// refund; that needs the same identity_account_id -> customer_id
// resolution already implemented in Core's canAccessOrder, which Payment
// does not have a way to perform against Core's data without a new
// internal endpoint. Restricted to staff (on-behalf-of-customer, per the
// same docs line) until that's wired — enforced at the router.
func (h *Handler) CreateRefund(w http.ResponseWriter, r *http.Request) {
	var req createRefundRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.PaymentID == "" || req.Reason == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "payment_id and reason are required.")
		return
	}

	ctx := r.Context()
	var (
		orderID       string
		amount        int64
		paymentStatus string
	)
	err := h.Pool.QueryRow(ctx, `SELECT order_id, amount, status FROM payments WHERE id = $1`, req.PaymentID).
		Scan(&orderID, &amount, &paymentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "PAYMENT_NOT_FOUND", "Payment not found.")
		return
	}
	if err != nil {
		h.Logger.Error("create refund: lookup payment failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	if paymentStatus != "PAID" {
		respond.Error(w, r, http.StatusConflict, "REFUND_NOT_ELIGIBLE", "Only a PAID payment can be refunded.")
		return
	}

	order, err := h.fetchCoreOrder(ctx, orderID)
	if err != nil {
		h.Logger.Error("create refund: fetch core order failed", "error", err)
		respond.Error(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Could not verify the order right now.")
		return
	}
	if order.Status == "CANCELLED" {
		respond.Error(w, r, http.StatusConflict, "REFUND_NOT_ELIGIBLE", "This order is already cancelled.")
		return
	}
	// NOTE: the 7-day post-COMPLETED eligibility window (UQ-04) cannot be
	// enforced precisely yet — Core's order response does not expose a
	// completed-at timestamp over the internal API. TODO(phase-2+): add
	// one and check `time.Since(completedAt) <= refundEligibilityWindow`.
	_ = refundEligibilityWindow

	var alreadyRequested bool
	if err := h.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM refunds WHERE payment_id = $1 AND status NOT IN ('REJECTED','FAILED'))
	`, req.PaymentID).Scan(&alreadyRequested); err != nil {
		h.Logger.Error("create refund: dedup check failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	if alreadyRequested {
		respond.Error(w, r, http.StatusConflict, "REFUND_NOT_ELIGIBLE", "A refund has already been requested for this payment.")
		return
	}

	principal := httpauth.FromRequest(r)
	id := uuid.New().String()
	refundCode := bizid.Refund()
	_, err = h.Pool.Exec(ctx, `
		INSERT INTO refunds (id, refund_code, payment_id, order_id, amount, reason, requested_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, id, refundCode, req.PaymentID, orderID, amount, req.Reason, principal.ID)
	if err != nil {
		h.Logger.Error("create refund: insert failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	out, err := h.fetchRefund(ctx, id)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Refund created but could not be loaded.")
		return
	}
	respond.JSON(w, http.StatusCreated, out)
}

type approveRefundRequest struct {
	Amount *int64 `json:"amount"`
}

// ApproveRefund implements POST /api/v1/refunds/{id}/approve
// (docs/07-api-contract.md §12, BR-18: never the customer — enforced at
// the router). MVP simplification: REQUESTED -> APPROVED -> PROCESSING ->
// COMPLETED happen synchronously in this one call, mirroring how a CASH
// payment auto-completes (docs/10-state-machines.md §3's "system"
// transitions have no real async gateway to wait on for a cash refund).
func (h *Handler) ApproveRefund(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req approveRefundRequest
	_ = decodeJSONOptional(r, &req)

	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	var (
		status        string
		paymentID     string
		orderID       string
		defaultAmount int64
	)
	if err := tx.QueryRow(ctx, `SELECT status, payment_id, order_id, amount FROM refunds WHERE id = $1`, id).
		Scan(&status, &paymentID, &orderID, &defaultAmount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Refund not found.")
			return
		}
		h.Logger.Error("approve refund: lookup failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	if status != "REQUESTED" {
		respond.Error(w, r, http.StatusConflict, "INVALID_STATE", "Only a REQUESTED refund can be approved.")
		return
	}

	amount := defaultAmount
	if req.Amount != nil {
		if *req.Amount <= 0 || *req.Amount > defaultAmount {
			respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "amount must be > 0 and <= the original payment amount.")
			return
		}
		amount = *req.Amount
	}

	principal := httpauth.FromRequest(r)
	now := time.Now()
	if _, err := tx.Exec(ctx, `
		UPDATE refunds SET status = 'COMPLETED', amount = $1, approved_by = $2, processed_at = $3 WHERE id = $4
	`, amount, principal.ID, now, id); err != nil {
		h.Logger.Error("approve refund: update failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if _, err := tx.Exec(ctx, `UPDATE payments SET status = 'REFUNDED' WHERE id = $1`, paymentID); err != nil {
		h.Logger.Error("approve refund: update payment failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO payment_transactions (payment_id, transaction_type, amount, status) VALUES ($1, 'REFUND', $2, 'SUCCESS')
	`, paymentID, amount); err != nil {
		h.Logger.Error("approve refund: transaction log failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if err := writeOutboxEvent(ctx, tx, "payment", paymentID, "payment.refunded", map[string]any{
		"payment_id": paymentID, "order_id": orderID, "refund_id": id, "amount": amount, "processed_at": now,
	}); err != nil {
		h.Logger.Error("approve refund: outbox write failed", "error", err)
	}

	if err := tx.Commit(ctx); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	out, err := h.fetchRefund(ctx, id)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

type rejectRefundRequest struct {
	Reason string `json:"reason"`
}

// RejectRefund implements POST /api/v1/refunds/{id}/reject.
func (h *Handler) RejectRefund(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req rejectRefundRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "reason is required.")
		return
	}

	ctx := r.Context()
	tag, err := h.Pool.Exec(ctx, `
		UPDATE refunds SET status = 'REJECTED', reason = reason || ' | Rejected: ' || $1 WHERE id = $2 AND status = 'REQUESTED'
	`, req.Reason, id)
	if err != nil {
		h.Logger.Error("reject refund failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	if tag.RowsAffected() == 0 {
		respond.Error(w, r, http.StatusConflict, "INVALID_STATE", "Only a REQUESTED refund can be rejected.")
		return
	}

	out, err := h.fetchRefund(ctx, id)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// GetRefund implements GET /api/v1/refunds/{id}.
func (h *Handler) GetRefund(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	out, err := h.fetchRefund(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Refund not found.")
		return
	}
	if err != nil {
		h.Logger.Error("get refund failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// ListRefunds implements GET /api/v1/refunds?order_id=.
func (h *Handler) ListRefunds(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	if orderID == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "order_id is required.")
		return
	}

	rows, err := h.Pool.Query(r.Context(), `
		SELECT id, refund_code, payment_id, order_id, amount, status, reason, requested_by, approved_by, processed_at, created_at
		FROM refunds WHERE order_id = $1 ORDER BY created_at DESC
	`, orderID)
	if err != nil {
		h.Logger.Error("list refunds failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer rows.Close()

	out := []refundResponse{}
	for rows.Next() {
		var ref refundResponse
		if err := rows.Scan(&ref.ID, &ref.RefundCode, &ref.PaymentID, &ref.OrderID, &ref.Amount, &ref.Status, &ref.Reason, &ref.RequestedBy, &ref.ApprovedBy, &ref.ProcessedAt, &ref.CreatedAt); err != nil {
			h.Logger.Error("list refunds: scan failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		out = append(out, ref)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) fetchRefund(ctx context.Context, id string) (refundResponse, error) {
	var ref refundResponse
	err := h.Pool.QueryRow(ctx, `
		SELECT id, refund_code, payment_id, order_id, amount, status, reason, requested_by, approved_by, processed_at, created_at
		FROM refunds WHERE id = $1
	`, id).Scan(&ref.ID, &ref.RefundCode, &ref.PaymentID, &ref.OrderID, &ref.Amount, &ref.Status, &ref.Reason, &ref.RequestedBy, &ref.ApprovedBy, &ref.ProcessedAt, &ref.CreatedAt)
	return ref, err
}
