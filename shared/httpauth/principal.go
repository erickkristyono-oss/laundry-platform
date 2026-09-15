// Package httpauth reads the trusted principal headers the Gateway sets
// after validating a bearer token (docs/09-rbac.md §3-4) and provides the
// role-check middleware every service uses for its own "fine" layer of
// authorization, on top of the Gateway's "coarse" layer.
//
// The header names are defined here (not duplicated in the Gateway) so
// producer and every consumer agree by construction — importing another
// service's internal package to share this would violate the service
// boundary rule in docs/03-system-architecture.md §4, but importing this
// shared, business-rule-free constant set does not.
package httpauth

import (
	"net/http"
	"strings"

	"laundry-platform/shared/authtoken"
	"laundry-platform/shared/respond"
)

const (
	HeaderPrincipalType = "X-Principal-Type"
	HeaderPrincipalID   = "X-Principal-ID"
	HeaderRoles         = "X-Roles"
	HeaderOutletIDs     = "X-Outlet-Ids"
)

// Principal is the authenticated caller, as forwarded by the Gateway.
type Principal struct {
	Type      string // authtoken.PrincipalStaff or authtoken.PrincipalCustomer, "" if unauthenticated
	ID        string
	Roles     []string
	OutletIDs []string // empty means GLOBAL scope (OWNER/SUPER_ADMIN), per docs/09-rbac.md §4.1
}

func (p Principal) IsAuthenticated() bool { return p.Type != "" }

func (p Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole reports whether the principal holds at least one of roles.
func (p Principal) HasAnyRole(roles ...string) bool {
	for _, want := range roles {
		if p.HasRole(want) {
			return true
		}
	}
	return false
}

// IsGlobalStaff reports whether the principal is OWNER/SUPER_ADMIN — the
// two roles with implicit global outlet scope (BR-21, docs/09-rbac.md §4.1).
func (p Principal) IsGlobalStaff() bool {
	return p.HasAnyRole("OWNER", "SUPER_ADMIN")
}

// CanAccessOutlet reports whether the principal may act on outletID:
// global staff always can; otherwise outletID must be in OutletIDs.
func (p Principal) CanAccessOutlet(outletID string) bool {
	if p.IsGlobalStaff() {
		return true
	}
	for _, id := range p.OutletIDs {
		if id == outletID {
			return true
		}
	}
	return false
}

// FromRequest reads the Principal the Gateway attached to r. Returns a
// zero-value (unauthenticated) Principal if none is present — callers
// that require authentication must check IsAuthenticated() or use
// RequireAuth/RequireRoles below.
func FromRequest(r *http.Request) Principal {
	p := Principal{
		Type: r.Header.Get(HeaderPrincipalType),
		ID:   r.Header.Get(HeaderPrincipalID),
	}
	if roles := r.Header.Get(HeaderRoles); roles != "" {
		p.Roles = strings.Split(roles, ",")
	}
	if outlets := r.Header.Get(HeaderOutletIDs); outlets != "" {
		p.OutletIDs = strings.Split(outlets, ",")
	}
	return p
}

// RequireAuth rejects any request the Gateway did not attach a validated
// principal to.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !FromRequest(r).IsAuthenticated() {
			respond.Error(w, r, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "This endpoint requires a valid access token.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireStaffRole rejects any request whose principal is not a STAFF
// principal (ADR-013: a customer token never satisfies this, regardless
// of its Roles claim) holding at least one of allowedRoles.
func RequireStaffRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := FromRequest(r)
			if p.Type != authtoken.PrincipalStaff {
				respond.Error(w, r, http.StatusForbidden, "FORBIDDEN", "This endpoint is staff-only.")
				return
			}
			if !p.HasAnyRole(allowedRoles...) {
				respond.Error(w, r, http.StatusForbidden, "FORBIDDEN", "Your role is not authorized for this action.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireCustomer rejects any request whose principal is not a CUSTOMER
// principal.
func RequireCustomer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if FromRequest(r).Type != authtoken.PrincipalCustomer {
			respond.Error(w, r, http.StatusForbidden, "FORBIDDEN", "This endpoint is for customers only.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
