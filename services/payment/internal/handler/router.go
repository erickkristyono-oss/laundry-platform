package handler

import (
	"github.com/go-chi/chi/v5"

	"laundry-platform/shared/httpauth"
	"laundry-platform/shared/idempotency"
)

var refundStaff = []string{"SUPER_ADMIN", "OWNER", "OUTLET_ADMIN", "CASHIER"}
var refundApprovers = []string{"SUPER_ADMIN", "OWNER", "OUTLET_ADMIN"} // never the customer — BR-18

// Mount attaches Payment's routes (docs/07-api-contract.md §11-12) to r.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/api/v1/payments", func(r chi.Router) {
		r.Use(httpauth.RequireAuth)
		r.With(idempotency.Require(h.Pool, "POST /api/v1/payments")).Post("/", h.CreatePayment)
		r.Get("/{id}", h.GetPayment)
		r.Get("/", h.ListPayments)
	})

	r.Route("/api/v1/refunds", func(r chi.Router) {
		r.Use(httpauth.RequireAuth)
		// TODO(phase-2+): docs also allow the owning customer to self-request
		// (see the note on CreateRefund) — staff-only until that's wired.
		r.With(httpauth.RequireStaffRole(refundStaff...), idempotency.Require(h.Pool, "POST /api/v1/refunds")).Post("/", h.CreateRefund)
		r.With(httpauth.RequireStaffRole(refundApprovers...)).Post("/{id}/approve", h.ApproveRefund)
		r.With(httpauth.RequireStaffRole(refundApprovers...)).Post("/{id}/reject", h.RejectRefund)
		r.Get("/{id}", h.GetRefund)
		r.Get("/", h.ListRefunds)
	})
}
