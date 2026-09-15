package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"laundry-platform/shared/httpauth"
)

// Mount attaches every Identity route (docs/07-api-contract.md §1-2) to r.
func (h *Handler) Mount(r chi.Router) {
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login", h.StaffLogin)
		r.Post("/refresh", h.StaffRefresh)
		r.Post("/logout", h.StaffLogout)
		r.With(httpauth.RequireAuth).Get("/me", func(w http.ResponseWriter, r *http.Request) {
			h.StaffMe(w, r, httpauth.FromRequest(r).ID)
		})
	})

	r.Route("/api/v1/customer-auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
		r.Post("/refresh", h.CustomerRefresh)
		r.Post("/logout", h.CustomerLogout)
		r.With(httpauth.RequireAuth).Get("/me", func(w http.ResponseWriter, r *http.Request) {
			h.CustomerMe(w, r, httpauth.FromRequest(r).ID)
		})
	})
}
