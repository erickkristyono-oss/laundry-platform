package handler

import (
	"github.com/go-chi/chi/v5"

	"laundry-platform/shared/httpauth"
)

var reportViewers = []string{"SUPER_ADMIN", "OWNER", "OUTLET_ADMIN"}
var performanceViewers = []string{"SUPER_ADMIN", "OWNER"}

// Mount attaches Reporting's read-only routes (docs/07-api-contract.md §13) to r.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/api/v1/reports", func(r chi.Router) {
		r.Use(httpauth.RequireAuth)
		r.With(httpauth.RequireStaffRole(reportViewers...)).Get("/sales", h.GetSalesReport)
		r.With(httpauth.RequireStaffRole(reportViewers...)).Get("/orders/summary", h.GetOrderSummaryReport)
		r.With(httpauth.RequireStaffRole(performanceViewers...)).Get("/services/performance", h.GetServicePerformanceReport)
		r.With(httpauth.RequireStaffRole(reportViewers...)).Get("/refunds", h.GetRefundsReport)
	})
}
