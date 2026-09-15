package handler

import (
	"github.com/go-chi/chi/v5"

	"laundry-platform/shared/httpauth"
)

// Mount attaches Payment's routes (docs/07-api-contract.md §11) to r.
// Refunds (§12) are a secondary feature deferred per the "core flow
// first" prioritization and are not mounted here yet.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/api/v1/payments", func(r chi.Router) {
		r.Use(httpauth.RequireAuth)
		r.Post("/", h.CreatePayment)
		r.Get("/{id}", h.GetPayment)
		r.Get("/", h.ListPayments)
	})
}
