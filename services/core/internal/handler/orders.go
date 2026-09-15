package handler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"laundry-platform/shared/bizid"
	"laundry-platform/shared/httpauth"
	"laundry-platform/shared/respond"
)

type orderItemResponse struct {
	ID                  string  `json:"id"`
	ServiceID           string  `json:"service_id"`
	ServiceNameSnapshot string  `json:"service_name_snapshot"`
	Unit                string  `json:"unit"`
	EstimatedWeightKg   *string `json:"estimated_weight_kg,omitempty"`
	ActualWeightKg      *string `json:"actual_weight_kg,omitempty"`
	UnitPriceSnapshot   int64   `json:"unit_price_snapshot"`
	Subtotal            *int64  `json:"subtotal,omitempty"`
	IsLocked            bool    `json:"is_locked"`
}

type orderResponse struct {
	ID                   string              `json:"id"`
	OrderCode            string              `json:"order_code"`
	CustomerID           string              `json:"customer_id"`
	CurrentOutletID      string              `json:"current_outlet_id"`
	Source               string              `json:"source"`
	FulfillmentType      string              `json:"fulfillment_type"`
	Status               string              `json:"status"`
	PaymentStatus        string              `json:"payment_status"`
	EstimatedTotalAmount *int64              `json:"estimated_total_amount,omitempty"`
	SubtotalAmount       *int64              `json:"subtotal_amount,omitempty"`
	DiscountAmount       int64               `json:"discount_amount"`
	TaxAmount            int64               `json:"tax_amount"`
	TaxRateSnapshot      string              `json:"tax_rate_snapshot"`
	PickupFee            int64               `json:"pickup_fee"`
	DeliveryFee          int64               `json:"delivery_fee"`
	TotalAmount          *int64              `json:"total_amount,omitempty"`
	Notes                *string             `json:"notes,omitempty"`
	CreatedAt            time.Time           `json:"created_at"`
	Items                []orderItemResponse `json:"items"`
}

// --- POST /api/v1/orders ---

type createOrderRequest struct {
	Customer struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"customer"`
	OutletID        string `json:"outlet_id"`
	Source          string `json:"source"`
	FulfillmentType string `json:"fulfillment_type"`
	Items           []struct {
		ServiceID         string   `json:"service_id"`
		EstimatedWeightKg *float64 `json:"estimated_weight_kg"`
	} `json:"items"`
	PickupRequested   bool    `json:"pickup_requested"`
	DeliveryRequested bool    `json:"delivery_requested"`
	DiscountAmount    *int64  `json:"discount_amount"`
	Notes             *string `json:"notes"`
}

var validSources = map[string]bool{"WEBSITE": true, "POS": true, "WHATSAPP": true}
var validFulfillment = map[string]bool{"WALK_IN": true, "PICKUP": true, "DELIVERY": true, "PICKUP_AND_DELIVERY": true}

// CreateOrder implements POST /api/v1/orders (docs/07-api-contract.md §8).
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.FulfillmentType == "" {
		req.FulfillmentType = "WALK_IN"
	}
	if len(req.Items) == 0 {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "items must not be empty.")
		return
	}
	if !validSources[req.Source] {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "source must be WEBSITE, POS, or WHATSAPP.")
		return
	}
	if !validFulfillment[req.FulfillmentType] {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid fulfillment_type.")
		return
	}

	ctx := r.Context()

	// Resolve customer: reuse an existing id, or upsert-by-phone for a new guest.
	customerID := req.Customer.ID
	if customerID == "" {
		if strings.TrimSpace(req.Customer.Name) == "" || strings.TrimSpace(req.Customer.Phone) == "" {
			respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "customer.name and customer.phone are required for a new guest.")
			return
		}
		existing, err := h.customerByPhone(ctx, req.Customer.Phone)
		if err == nil {
			customerID = existing.ID
		} else if errors.Is(err, pgx.ErrNoRows) {
			customerID = uuid.New().String()
			_, err := h.Pool.Exec(ctx, `
				INSERT INTO customers (id, customer_code, name, phone, is_guest) VALUES ($1, $2, $3, $4, true)
			`, customerID, bizid.Customer(), strings.TrimSpace(req.Customer.Name), strings.TrimSpace(req.Customer.Phone))
			if err != nil {
				h.Logger.Error("create order: create guest customer failed", "error", err)
				respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
				return
			}
		} else {
			h.Logger.Error("create order: lookup customer failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
	}

	// Resolve outlet: explicit id, or auto-recommend (BR-10).
	outletID := req.OutletID
	if outletID == "" {
		var err error
		outletID, err = h.recommendOutletID(ctx)
		if err != nil {
			respond.Error(w, r, http.StatusUnprocessableEntity, "NO_OUTLET_AVAILABLE", "No outlet is currently available to serve this order.")
			return
		}
	}

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	orderID := uuid.New().String()
	orderCode := bizid.Order()
	discount := int64(0)
	if req.DiscountAmount != nil {
		discount = *req.DiscountAmount
	}

	var estimatedTotal *int64
	var estSum int64
	haveEstimate := true

	type preparedItem struct {
		id, serviceID, name, unit string
		estimatedWeightKg         *float64
		unitPrice                 int64
	}
	prepared := make([]preparedItem, 0, len(req.Items))

	for _, item := range req.Items {
		var name, unit string
		var isActive bool
		err := tx.QueryRow(ctx, `SELECT name, unit, is_active FROM services WHERE id = $1`, item.ServiceID).Scan(&name, &unit, &isActive)
		if errors.Is(err, pgx.ErrNoRows) || !isActive {
			respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", fmt.Sprintf("service %s is not available.", item.ServiceID))
			return
		}
		if err != nil {
			h.Logger.Error("create order: lookup service failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}

		price, err := h.activePriceTx(ctx, tx, item.ServiceID, outletID)
		if err != nil {
			respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", fmt.Sprintf("no active price for service %s.", item.ServiceID))
			return
		}

		if item.EstimatedWeightKg != nil {
			estSum += int64(math.Round(*item.EstimatedWeightKg * float64(price)))
		} else {
			haveEstimate = false
		}

		prepared = append(prepared, preparedItem{
			id: uuid.New().String(), serviceID: item.ServiceID, name: name, unit: unit,
			estimatedWeightKg: item.EstimatedWeightKg, unitPrice: price,
		})
	}
	if haveEstimate {
		estimatedTotal = &estSum
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO orders (id, order_code, customer_id, current_outlet_id, source, fulfillment_type, discount_amount, estimated_total_amount, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, orderID, orderCode, customerID, outletID, req.Source, req.FulfillmentType, discount, estimatedTotal, req.Notes)
	if err != nil {
		h.Logger.Error("create order: insert order failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	for _, item := range prepared {
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (id, order_id, service_id, service_name_snapshot, unit, estimated_weight_kg, unit_price_snapshot)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, item.id, orderID, item.serviceID, item.name, item.unit, item.estimatedWeightKg, item.unitPrice)
		if err != nil {
			h.Logger.Error("create order: insert item failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
	}

	if _, err := tx.Exec(ctx, `INSERT INTO order_status_history (order_id, from_status, to_status) VALUES ($1, NULL, 'CREATED')`, orderID); err != nil {
		h.Logger.Error("create order: history insert failed", "error", err)
	}

	if req.PickupRequested {
		if _, err := tx.Exec(ctx, `INSERT INTO pickup_requests (order_id) VALUES ($1)`, orderID); err != nil {
			h.Logger.Error("create order: pickup request insert failed", "error", err)
		}
	}
	if req.DeliveryRequested {
		if _, err := tx.Exec(ctx, `INSERT INTO delivery_requests (order_id) VALUES ($1)`, orderID); err != nil {
			h.Logger.Error("create order: delivery request insert failed", "error", err)
		}
	}

	if err := writeOutboxEvent(ctx, tx, "order", orderID, "order.created", map[string]any{
		"order_id": orderID, "order_code": orderCode, "customer_id": customerID, "outlet_id": outletID, "status": "CREATED",
	}); err != nil {
		h.Logger.Error("create order: outbox write failed", "error", err)
	}

	if err := tx.Commit(ctx); err != nil {
		h.Logger.Error("create order: commit failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	out, err := h.fetchOrder(ctx, orderID)
	if err != nil {
		h.Logger.Error("create order: fetch after create failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Order created but could not be loaded.")
		return
	}
	respond.JSON(w, http.StatusCreated, out)
}

// GetOrderInternal implements GET /internal/orders/{id} — the
// service-to-service read Payment uses to independently verify an
// order's total before charging it (docs/14-architecture-decisions.md
// ADR-011). Unauthenticated by design: reachable only inside the Docker
// network, since the Gateway's routing table has no /internal prefix.
func (h *Handler) GetOrderInternal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	out, err := h.fetchOrder(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Order not found.")
		return
	}
	if err != nil {
		h.Logger.Error("get order (internal) failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// GetOrder implements GET /api/v1/orders/{id}.
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	out, err := h.fetchOrder(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Order not found.")
		return
	}
	if err != nil {
		h.Logger.Error("get order failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if !h.canAccessOrder(r, out) {
		respond.Error(w, r, http.StatusForbidden, "FORBIDDEN", "You are not authorized to view this order.")
		return
	}

	respond.JSON(w, http.StatusOK, out)
}

// ListOrders implements GET /api/v1/orders?customer_id=&outlet_id=&status=&payment_status=
func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	principal := httpauth.FromRequest(r)

	var (
		conditions []string
		args       []any
	)
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	// A CUSTOMER principal is always scoped to their own orders
	// (docs/07-api-contract.md §8: "customer may fetch own orders only")
	// — any client-supplied customer_id is ignored in favor of the one
	// resolved from the authenticated principal, so a customer can never
	// list another customer's orders by guessing an id.
	if principal.Type == "CUSTOMER" {
		customerID, err := h.customerIDForIdentityAccount(r.Context(), principal.ID)
		if err != nil {
			respond.JSON(w, http.StatusOK, map[string]any{"data": []orderResponse{}})
			return
		}
		conditions = append(conditions, "customer_id = "+arg(customerID))
	} else if v := q.Get("customer_id"); v != "" {
		conditions = append(conditions, "customer_id = "+arg(v))
	}
	if v := q.Get("outlet_id"); v != "" {
		conditions = append(conditions, "current_outlet_id = "+arg(v))
	}
	if v := q.Get("status"); v != "" {
		conditions = append(conditions, "status = "+arg(v))
	}
	if v := q.Get("payment_status"); v != "" {
		conditions = append(conditions, "payment_status = "+arg(v))
	}

	// Non-global staff are scoped to their assigned outlets (docs/09-rbac.md §4).
	if principal.Type == "STAFF" && !principal.IsGlobalStaff() {
		if len(principal.OutletIDs) == 0 {
			respond.JSON(w, http.StatusOK, map[string]any{"data": []orderResponse{}})
			return
		}
		conditions = append(conditions, "current_outlet_id = ANY("+arg(principal.OutletIDs)+")")
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := h.Pool.Query(r.Context(), `SELECT id FROM orders `+where+` ORDER BY created_at DESC LIMIT 100`, args...)
	if err != nil {
		h.Logger.Error("list orders failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			h.Logger.Error("list orders: scan failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		ids = append(ids, id)
	}
	rows.Close()

	out := make([]orderResponse, 0, len(ids))
	for _, id := range ids {
		o, err := h.fetchOrder(r.Context(), id)
		if err != nil {
			continue
		}
		out = append(out, o)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"data": out})
}

// --- PUT /api/v1/orders/{id}/items/{itemId}/weigh ---

type weighRequest struct {
	ActualWeightKg float64 `json:"actual_weight_kg"`
}

func (h *Handler) WeighItem(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemId")

	var req weighRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ActualWeightKg < 0 {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "actual_weight_kg must be >= 0.")
		return
	}

	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	var status string
	var isLocked bool
	var unitPrice int64
	err = tx.QueryRow(ctx, `
		SELECT o.status, oi.is_locked, oi.unit_price_snapshot
		FROM order_items oi JOIN orders o ON o.id = oi.order_id
		WHERE oi.id = $1 AND oi.order_id = $2
	`, itemID, orderID).Scan(&status, &isLocked, &unitPrice)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Order item not found.")
		return
	}
	if err != nil {
		h.Logger.Error("weigh: lookup failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	if status != "RECEIVED" && status != "WEIGHING" {
		respond.Error(w, r, http.StatusConflict, "INVALID_STATE", "Order must be RECEIVED or WEIGHING to record weight.")
		return
	}
	if isLocked {
		respond.Error(w, r, http.StatusConflict, "ITEM_LOCKED", "This item can no longer be modified.")
		return
	}

	subtotal := int64(math.Round(req.ActualWeightKg * float64(unitPrice)))
	if _, err := tx.Exec(ctx, `UPDATE order_items SET actual_weight_kg = $1, subtotal = $2 WHERE id = $3`, req.ActualWeightKg, subtotal, itemID); err != nil {
		h.Logger.Error("weigh: update item failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if status == "RECEIVED" {
		if err := h.transitionStatusTx(ctx, tx, orderID, "RECEIVED", "WEIGHING", httpauth.FromRequest(r).ID, nil, false); err != nil {
			h.Logger.Error("weigh: auto-transition to WEIGHING failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	out, err := h.fetchOrder(ctx, orderID)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// --- POST /api/v1/orders/{id}/finalize-weighing ---

type finalizeWeighingRequest struct {
	PickupFee   *int64 `json:"pickup_fee"`
	DeliveryFee *int64 `json:"delivery_fee"`
}

func (h *Handler) FinalizeWeighing(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	var req finalizeWeighingRequest
	_ = decodeJSONOptional(r, &req)

	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	var incomplete bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM order_items WHERE order_id = $1 AND actual_weight_kg IS NULL)
	`, orderID).Scan(&incomplete); err != nil {
		h.Logger.Error("finalize: check completeness failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	if incomplete {
		respond.Error(w, r, http.StatusConflict, "INCOMPLETE_WEIGHING", "All items must be weighed before finalizing.")
		return
	}

	var subtotal int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(subtotal), 0) FROM order_items WHERE order_id = $1`, orderID).Scan(&subtotal); err != nil {
		h.Logger.Error("finalize: sum subtotal failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	taxRate, err := h.activeTaxRateValue(ctx)
	if err != nil {
		h.Logger.Error("finalize: read tax rate failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	taxAmount := int64(math.Round(taxRate / 100 * float64(subtotal)))

	var discount int64
	var pickupFee, deliveryFee int64
	if err := tx.QueryRow(ctx, `SELECT discount_amount, pickup_fee, delivery_fee FROM orders WHERE id = $1`, orderID).Scan(&discount, &pickupFee, &deliveryFee); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Order not found.")
			return
		}
		h.Logger.Error("finalize: read order failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	if req.PickupFee != nil {
		pickupFee = *req.PickupFee
	}
	if req.DeliveryFee != nil {
		deliveryFee = *req.DeliveryFee
	}

	total := subtotal - discount + taxAmount + pickupFee + deliveryFee

	_, err = tx.Exec(ctx, `
		UPDATE orders SET subtotal_amount = $1, tax_rate_snapshot = $2, tax_amount = $3,
			pickup_fee = $4, delivery_fee = $5, total_amount = $6, updated_at = now()
		WHERE id = $7
	`, subtotal, taxRate, taxAmount, pickupFee, deliveryFee, total, orderID)
	if err != nil {
		h.Logger.Error("finalize: update order failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if err := writeOutboxEvent(ctx, tx, "order", orderID, "order.weighed", map[string]any{
		"order_id": orderID, "subtotal_amount": subtotal, "tax_amount": taxAmount, "total_amount": total,
	}); err != nil {
		h.Logger.Error("finalize: outbox write failed", "error", err)
	}

	if err := tx.Commit(ctx); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	out, err := h.fetchOrder(ctx, orderID)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// --- POST /api/v1/orders/{id}/status ---

type changeStatusRequest struct {
	ToStatus string `json:"to_status"`
	Reason   string `json:"reason"`
}

func (h *Handler) ChangeOrderStatus(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	var req changeStatusRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ctx := r.Context()
	var currentStatus, paymentStatus string
	if err := h.Pool.QueryRow(ctx, `SELECT status, payment_status FROM orders WHERE id = $1`, orderID).Scan(&currentStatus, &paymentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Order not found.")
			return
		}
		h.Logger.Error("change status: lookup failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if req.ToStatus == "WASHING" && currentStatus == "WEIGHING" {
		respond.Error(w, r, http.StatusConflict, "INVALID_TRANSITION", "WEIGHING to WASHING happens automatically once payment is confirmed, not via this endpoint.")
		return
	}
	if !isValidTransition(currentStatus, req.ToStatus) {
		respond.Error(w, r, http.StatusConflict, "INVALID_TRANSITION", fmt.Sprintf("%s -> %s is not a valid transition.", currentStatus, req.ToStatus))
		return
	}
	if requiresReason(req.ToStatus) && strings.TrimSpace(req.Reason) == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "reason is required for this transition.")
		return
	}
	if requiresPaidBeforeCompleting(req.ToStatus) && paymentStatus != "PAID" {
		respond.Error(w, r, http.StatusConflict, "PAYMENT_NOT_COLLECTED", "Order cannot be completed until payment_status is PAID.")
		return
	}

	principal := httpauth.FromRequest(r)
	if principal.Type != "STAFF" || !principal.HasAnyRole(staffRolesFor(currentStatus, req.ToStatus)...) {
		respond.Error(w, r, http.StatusForbidden, "FORBIDDEN_TRANSITION", "Your role cannot perform this transition.")
		return
	}

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	var reason *string
	if req.Reason != "" {
		reason = &req.Reason
	}
	if err := h.transitionStatusTx(ctx, tx, orderID, currentStatus, req.ToStatus, principal.ID, reason, false); err != nil {
		h.Logger.Error("change status: transition failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	out, err := h.fetchOrder(ctx, orderID)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// transitionStatusTx updates orders.status and writes the audit trail +
// order.status_changed event, all inside the caller's transaction.
func (h *Handler) transitionStatusTx(ctx context.Context, tx pgx.Tx, orderID, from, to, performedBy string, reason *string, isOverride bool) error {
	if _, err := tx.Exec(ctx, `UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`, to, orderID); err != nil {
		return err
	}

	var performedByPtr *string
	if performedBy != "" {
		performedByPtr = &performedBy
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO order_status_history (order_id, from_status, to_status, reason, performed_by, is_administrative_override)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, orderID, from, to, reason, performedByPtr, isOverride); err != nil {
		return err
	}

	eventType := "order.status_changed"
	if to == "COMPLETED" {
		eventType = "order.completed"
	} else if to == "CANCELLED" {
		eventType = "order.cancelled"
	}
	return writeOutboxEvent(ctx, tx, "order", orderID, eventType, map[string]any{
		"order_id": orderID, "from_status": from, "to_status": to, "performed_by": performedByPtr, "reason": reason,
	})
}

// --- shared helpers ---

func (h *Handler) recommendOutletID(ctx context.Context) (string, error) {
	var id string
	err := h.Pool.QueryRow(ctx, `SELECT id FROM outlets WHERE is_active ORDER BY created_at LIMIT 1`).Scan(&id)
	return id, err
}

func (h *Handler) activePriceTx(ctx context.Context, tx pgx.Tx, serviceID, outletID string) (int64, error) {
	var price int64
	if outletID != "" {
		err := tx.QueryRow(ctx, `
			SELECT price_per_unit FROM service_prices WHERE service_id = $1 AND outlet_id = $2 AND effective_to IS NULL
		`, serviceID, outletID).Scan(&price)
		if err == nil {
			return price, nil
		}
	}
	err := tx.QueryRow(ctx, `
		SELECT price_per_unit FROM service_prices WHERE service_id = $1 AND outlet_id IS NULL AND effective_to IS NULL
	`, serviceID).Scan(&price)
	return price, err
}

// canAccessOrder implements the ownership check from
// docs/07-api-contract.md §8 ("customer may fetch own orders only").
// A CUSTOMER principal's JWT subject is its `customer_accounts.id`
// (docs/06-database-schema.md §1.9); Core only knows the inverse link
// (`customers.identity_account_id`), so it resolves that customer's
// `customers.id` and compares it against the order's owner.
func (h *Handler) canAccessOrder(r *http.Request, order orderResponse) bool {
	principal := httpauth.FromRequest(r)
	switch principal.Type {
	case "STAFF":
		return principal.CanAccessOutlet(order.CurrentOutletID)
	case "CUSTOMER":
		customerID, err := h.customerIDForIdentityAccount(r.Context(), principal.ID)
		if err != nil {
			return false
		}
		return customerID == order.CustomerID
	default:
		return false
	}
}

func (h *Handler) customerIDForIdentityAccount(ctx context.Context, identityAccountID string) (string, error) {
	var id string
	err := h.Pool.QueryRow(ctx, `SELECT id FROM customers WHERE identity_account_id = $1`, identityAccountID).Scan(&id)
	return id, err
}

func (h *Handler) fetchOrder(ctx context.Context, id string) (orderResponse, error) {
	var o orderResponse
	var estimatedTotal, subtotalAmount, totalAmount *int64
	err := h.Pool.QueryRow(ctx, `
		SELECT id, order_code, customer_id, current_outlet_id, source, fulfillment_type, status, payment_status,
			estimated_total_amount, subtotal_amount, discount_amount, tax_amount, tax_rate_snapshot::text,
			pickup_fee, delivery_fee, total_amount, notes, created_at
		FROM orders WHERE id = $1
	`, id).Scan(&o.ID, &o.OrderCode, &o.CustomerID, &o.CurrentOutletID, &o.Source, &o.FulfillmentType, &o.Status, &o.PaymentStatus,
		&estimatedTotal, &subtotalAmount, &o.DiscountAmount, &o.TaxAmount, &o.TaxRateSnapshot,
		&o.PickupFee, &o.DeliveryFee, &totalAmount, &o.Notes, &o.CreatedAt)
	if err != nil {
		return orderResponse{}, err
	}
	o.EstimatedTotalAmount = estimatedTotal
	o.SubtotalAmount = subtotalAmount
	o.TotalAmount = totalAmount

	rows, err := h.Pool.Query(ctx, `
		SELECT id, service_id, service_name_snapshot, unit, estimated_weight_kg::text, actual_weight_kg::text, unit_price_snapshot, subtotal, is_locked
		FROM order_items WHERE order_id = $1 ORDER BY created_at
	`, id)
	if err != nil {
		return orderResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var it orderItemResponse
		var estWeight, actWeight *string
		if err := rows.Scan(&it.ID, &it.ServiceID, &it.ServiceNameSnapshot, &it.Unit, &estWeight, &actWeight, &it.UnitPriceSnapshot, &it.Subtotal, &it.IsLocked); err != nil {
			return orderResponse{}, err
		}
		it.EstimatedWeightKg = estWeight
		it.ActualWeightKg = actWeight
		o.Items = append(o.Items, it)
	}
	return o, rows.Err()
}
