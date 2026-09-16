package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"laundry-platform/shared/httpauth"
	"laundry-platform/shared/respond"
)

type fulfillmentRequestResponse struct {
	ID          string     `json:"id"`
	OrderID     string     `json:"order_id"`
	AddressID   *string    `json:"address_id,omitempty"`
	Status      string     `json:"status"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Notes       *string    `json:"notes,omitempty"`
}

type requestPickupOrDeliveryRequest struct {
	AddressID       string `json:"address_id"`
	PreferredWindow string `json:"preferred_window"`
}

// RequestPickup implements POST /api/v1/orders/{id}/pickup
// (docs/07-api-contract.md §9) — covers a post-creation pickup request;
// order creation with pickup_requested=true (orders.go) already covers
// the at-creation-time path.
func (h *Handler) RequestPickup(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	var req requestPickupOrDeliveryRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	var addressID *string
	if req.AddressID != "" {
		addressID = &req.AddressID
	}
	var notes *string
	if req.PreferredWindow != "" {
		notes = &req.PreferredWindow
	}

	id := uuid.New().String()
	_, err := h.Pool.Exec(r.Context(), `
		INSERT INTO pickup_requests (id, order_id, address_id, notes) VALUES ($1, $2, $3, $4)
	`, id, orderID, addressID, notes)
	if err != nil {
		if isUniqueViolation(err) {
			respond.Error(w, r, http.StatusConflict, "PICKUP_ALREADY_REQUESTED", "A pickup has already been requested for this order.")
			return
		}
		h.Logger.Error("request pickup failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	respond.JSON(w, http.StatusCreated, fulfillmentRequestResponse{ID: id, OrderID: orderID, AddressID: addressID, Status: "REQUESTED", Notes: notes})
}

// RequestDelivery implements POST /api/v1/orders/{id}/delivery
// (docs/07-api-contract.md §10) — precondition: order status = READY.
func (h *Handler) RequestDelivery(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "id")
	var req requestPickupOrDeliveryRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	ctx := r.Context()
	var status string
	if err := h.Pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Order not found.")
			return
		}
		h.Logger.Error("request delivery: lookup order failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	if status != "READY" {
		respond.Error(w, r, http.StatusConflict, "INVALID_STATE", "Order must be READY before requesting delivery.")
		return
	}

	var addressID *string
	if req.AddressID != "" {
		addressID = &req.AddressID
	}
	var notes *string
	if req.PreferredWindow != "" {
		notes = &req.PreferredWindow
	}

	id := uuid.New().String()
	_, err := h.Pool.Exec(ctx, `
		INSERT INTO delivery_requests (id, order_id, address_id, notes) VALUES ($1, $2, $3, $4)
	`, id, orderID, addressID, notes)
	if err != nil {
		if isUniqueViolation(err) {
			respond.Error(w, r, http.StatusConflict, "DELIVERY_ALREADY_REQUESTED", "A delivery has already been requested for this order.")
			return
		}
		h.Logger.Error("request delivery failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	respond.JSON(w, http.StatusCreated, fulfillmentRequestResponse{ID: id, OrderID: orderID, AddressID: addressID, Status: "REQUESTED", Notes: notes})
}

type updateFulfillmentRequest struct {
	Status      string     `json:"status"`
	ScheduledAt *time.Time `json:"scheduled_at"`
}

var validFulfillmentStatus = map[string]bool{"SCHEDULED": true, "COMPLETED": true, "CANCELLED": true}

// UpdatePickup implements PATCH /api/v1/pickups/{id} (docs/07-api-contract.md §9):
// COMPLETED triggers order status CREATED -> RECEIVED, if the order hasn't
// already moved past CREATED through some other path.
func (h *Handler) UpdatePickup(w http.ResponseWriter, r *http.Request) {
	h.updateFulfillment(w, r, "pickup_requests", "CREATED", "RECEIVED")
}

// UpdateDelivery implements PATCH /api/v1/deliveries/{id} (docs/07-api-contract.md §10):
// COMPLETED triggers order status READY -> DELIVERED.
func (h *Handler) UpdateDelivery(w http.ResponseWriter, r *http.Request) {
	h.updateFulfillment(w, r, "delivery_requests", "READY", "DELIVERED")
}

// updateFulfillment is shared by UpdatePickup/UpdateDelivery: on COMPLETED,
// transitions the order fromStatus -> toStatus only if it is still at
// fromStatus (i.e. hasn't already moved past it through the normal
// weighing/washing flow) — a no-op transition attempt is silently skipped,
// not an error, since the fulfillment side effect (marking the pickup/
// delivery itself completed) still succeeded.
func (h *Handler) updateFulfillment(w http.ResponseWriter, r *http.Request, table, fromStatus, toStatus string) {
	id := chi.URLParam(r, "id")
	var req updateFulfillmentRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validFulfillmentStatus[req.Status] {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "status must be SCHEDULED, COMPLETED, or CANCELLED.")
		return
	}

	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	defer tx.Rollback(ctx)

	var orderID string
	if err := tx.QueryRow(ctx, `SELECT order_id FROM `+table+` WHERE id = $1`, id).Scan(&orderID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Not found.")
			return
		}
		h.Logger.Error("update fulfillment: lookup failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	var completedAt *time.Time
	if req.Status == "COMPLETED" {
		now := time.Now()
		completedAt = &now
	}
	if _, err := tx.Exec(ctx, `UPDATE `+table+` SET status = $1, scheduled_at = COALESCE($2, scheduled_at), completed_at = $3 WHERE id = $4`,
		req.Status, req.ScheduledAt, completedAt, id); err != nil {
		h.Logger.Error("update fulfillment: update failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	if req.Status == "COMPLETED" {
		var currentStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&currentStatus); err != nil {
			h.Logger.Error("update fulfillment: lookup order status failed", "error", err)
			respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
			return
		}
		if currentStatus == fromStatus {
			principal := httpauth.FromRequest(r)
			if err := h.transitionStatusTx(ctx, tx, orderID, fromStatus, toStatus, principal.ID, nil, false); err != nil {
				h.Logger.Error("update fulfillment: order transition failed", "error", err)
				respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
				return
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	respond.JSON(w, http.StatusOK, map[string]any{"id": id, "order_id": orderID, "status": req.Status, "completed_at": completedAt})
}
