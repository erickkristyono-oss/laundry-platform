package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"laundry-platform/shared/bizid"
	"laundry-platform/shared/respond"
)

type customerResponse struct {
	ID                string  `json:"id"`
	CustomerCode      string  `json:"customer_code"`
	Name              string  `json:"name"`
	Phone             string  `json:"phone"`
	Email             *string `json:"email,omitempty"`
	IsGuest           bool    `json:"is_guest"`
	IdentityAccountID *string `json:"identity_account_id,omitempty"`
}

type createCustomerRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// CreateCustomer implements POST /api/v1/customers (docs/07-api-contract.md §4):
// upsert-by-phone so the guest-checkout flow never errors on a repeat phone number.
func (h *Handler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req createCustomerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Email = strings.TrimSpace(req.Email)

	if req.Name == "" || req.Phone == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "name and phone are required.")
		return
	}

	ctx := r.Context()
	existing, err := h.customerByPhone(ctx, req.Phone)
	if err == nil {
		respond.JSON(w, http.StatusOK, existing)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		h.Logger.Error("create customer: lookup failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	var email *string
	if req.Email != "" {
		email = &req.Email
	}

	out := customerResponse{ID: uuid.New().String(), CustomerCode: bizid.Customer(), Name: req.Name, Phone: req.Phone, Email: email, IsGuest: true}
	_, err = h.Pool.Exec(ctx, `
		INSERT INTO customers (id, customer_code, name, phone, email, is_guest)
		VALUES ($1, $2, $3, $4, $5, true)
	`, out.ID, out.CustomerCode, out.Name, out.Phone, email)
	if err != nil {
		h.Logger.Error("create customer: insert failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	respond.JSON(w, http.StatusCreated, out)
}

// GetCustomerInternal implements GET /internal/customers/{id} — the
// service-to-service read Identity uses after login/registration to
// return the customer's profile (docs/06-database-schema.md §1.9). Unlike
// the public GET /api/v1/customers/{id}, this is not proxied by the
// Gateway, so it needs no auth check of its own.
func (h *Handler) GetCustomerInternal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	out, err := h.customerByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Customer not found.")
		return
	}
	if err != nil {
		h.Logger.Error("get customer (internal) failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

// GetCustomer implements GET /api/v1/customers/{id}.
func (h *Handler) GetCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	out, err := h.customerByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Error(w, r, http.StatusNotFound, "NOT_FOUND", "Customer not found.")
		return
	}
	if err != nil {
		h.Logger.Error("get customer failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	respond.JSON(w, http.StatusOK, out)
}

type addAddressRequest struct {
	Label       string `json:"label"`
	AddressLine string `json:"address_line"`
	PostalCode  string `json:"postal_code"`
	IsDefault   bool   `json:"is_default"`
}

// AddCustomerAddress implements POST /api/v1/customers/{id}/addresses.
func (h *Handler) AddCustomerAddress(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	var req addAddressRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Label) == "" || strings.TrimSpace(req.AddressLine) == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "label and address_line are required.")
		return
	}

	var postalCode *string
	if req.PostalCode != "" {
		postalCode = &req.PostalCode
	}

	id := uuid.New().String()
	_, err := h.Pool.Exec(r.Context(), `
		INSERT INTO customer_addresses (id, customer_id, label, address_line, postal_code, is_default)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, id, customerID, req.Label, req.AddressLine, postalCode, req.IsDefault)
	if err != nil {
		h.Logger.Error("add address failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}

	respond.JSON(w, http.StatusCreated, map[string]any{
		"id": id, "customer_id": customerID, "label": req.Label,
		"address_line": req.AddressLine, "postal_code": postalCode, "is_default": req.IsDefault,
	})
}

type linkIdentityRequest struct {
	IdentityAccountID string `json:"identity_account_id"`
}

// LinkIdentity implements POST /internal/customers/{id}/link-identity —
// service-to-service only, called by Identity right after it creates a
// customer_accounts row (UQ-03 resolution, docs/06-database-schema.md §1.9).
// Not proxied by the Gateway (gateway/internal/proxy's routing table has no
// /internal prefix), so it is only reachable inside the Docker network.
func (h *Handler) LinkIdentity(w http.ResponseWriter, r *http.Request) {
	customerID := chi.URLParam(r, "id")
	var req linkIdentityRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.IdentityAccountID == "" {
		respond.Error(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "identity_account_id is required.")
		return
	}

	tag, err := h.Pool.Exec(r.Context(), `
		UPDATE customers SET is_guest = false, identity_account_id = $1
		WHERE id = $2 AND (identity_account_id IS NULL OR identity_account_id = $1)
	`, req.IdentityAccountID, customerID)
	if err != nil {
		h.Logger.Error("link identity failed", "error", err)
		respond.Error(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Something went wrong.")
		return
	}
	if tag.RowsAffected() == 0 {
		respond.Error(w, r, http.StatusConflict, "ALREADY_LINKED", "This customer is already linked to a different account.")
		return
	}
	w.WriteHeader(http.StatusOK)
}

// --- shared lookups ---

const customerColumns = `id, customer_code, name, phone, email, is_guest, identity_account_id`

func (h *Handler) customerByPhone(ctx context.Context, phone string) (customerResponse, error) {
	row := h.Pool.QueryRow(ctx, `SELECT `+customerColumns+` FROM customers WHERE phone = $1`, phone)
	return scanCustomer(row)
}

func (h *Handler) customerByID(ctx context.Context, id string) (customerResponse, error) {
	row := h.Pool.QueryRow(ctx, `SELECT `+customerColumns+` FROM customers WHERE id = $1`, id)
	return scanCustomer(row)
}

func scanCustomer(row pgx.Row) (customerResponse, error) {
	var out customerResponse
	var identityAccountID *string
	err := row.Scan(&out.ID, &out.CustomerCode, &out.Name, &out.Phone, &out.Email, &out.IsGuest, &identityAccountID)
	out.IdentityAccountID = identityAccountID
	return out, err
}
