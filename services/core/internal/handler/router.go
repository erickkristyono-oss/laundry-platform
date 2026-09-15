package handler

import (
	"github.com/go-chi/chi/v5"

	"laundry-platform/shared/httpauth"
)

var staffAny = []string{"SUPER_ADMIN", "OWNER", "OUTLET_ADMIN", "CASHIER", "LAUNDRY_STAFF"}
var staffAdmin = []string{"SUPER_ADMIN", "OWNER"}

// Mount attaches every Core route (docs/07-api-contract.md §4-8) to r.
// Notes on scope: pickup/delivery (§9-10) and order transfer are secondary
// features deferred per the "core flow first" prioritization and are not
// mounted here yet.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/api/v1/customers", func(r chi.Router) {
		r.Post("/", h.CreateCustomer) // PUBLIC: guest checkout creates its own customer row
		r.With(httpauth.RequireAuth).Get("/{id}", h.GetCustomer)
		r.With(httpauth.RequireAuth).Post("/{id}/addresses", h.AddCustomerAddress)
	})

	r.Route("/api/v1/outlets", func(r chi.Router) {
		r.Get("/", h.ListOutlets) // PUBLIC
		r.Get("/recommend", h.RecommendOutlet)
		r.With(httpauth.RequireAuth, httpauth.RequireStaffRole(staffAdmin...)).Post("/", h.CreateOutlet)
	})

	r.Route("/api/v1/services", func(r chi.Router) {
		r.Get("/", h.ListServices) // PUBLIC
		r.With(httpauth.RequireAuth, httpauth.RequireStaffRole(staffAdmin...)).Post("/", h.CreateService)
	})

	r.Route("/api/v1/pricing", func(r chi.Router) {
		r.Get("/", h.GetActivePrice) // PUBLIC
		r.With(httpauth.RequireAuth, httpauth.RequireStaffRole(staffAdmin...)).Post("/", h.CreatePricing)
	})

	r.Route("/api/v1/settings/tax-rate", func(r chi.Router) {
		r.Get("/", h.GetTaxRate) // PUBLIC
		r.With(httpauth.RequireAuth, httpauth.RequireStaffRole(staffAdmin...)).Put("/", h.SetTaxRate)
	})

	r.Route("/api/v1/orders", func(r chi.Router) {
		r.Post("/", h.CreateOrder) // PUBLIC (guest) or authenticated
		r.With(httpauth.RequireAuth).Get("/", h.ListOrders)
		r.With(httpauth.RequireAuth).Get("/{id}", h.GetOrder)
		r.With(httpauth.RequireAuth, httpauth.RequireStaffRole(staffAny...)).Put("/{id}/items/{itemId}/weigh", h.WeighItem)
		r.With(httpauth.RequireAuth, httpauth.RequireStaffRole(staffAny...)).Post("/{id}/finalize-weighing", h.FinalizeWeighing)
		r.With(httpauth.RequireAuth).Post("/{id}/status", h.ChangeOrderStatus) // role checked per-transition inside the handler
	})

	// Internal, service-to-service only (not proxied by the Gateway's
	// routing table — gateway/internal/proxy has no /internal prefix).
	r.Route("/internal", func(r chi.Router) {
		r.Post("/customers/{id}/link-identity", h.LinkIdentity)
		r.Get("/customers/{id}", h.GetCustomerInternal)
		r.Get("/orders/{id}", h.GetOrderInternal)
	})
}
