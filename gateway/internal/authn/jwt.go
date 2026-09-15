// Package authn implements the Gateway's coarse authentication check
// (docs/09-rbac.md §3): validate the bearer token's signature/expiry and
// forward the principal as trusted internal headers, never trusting any
// client-supplied version of those same headers (docs/09-rbac.md §4.1).
//
// Fine-grained authorization (permission checks, outlet-scope checks) is
// deliberately NOT done here — it happens again at the service layer, per
// the defense-in-depth design in docs/09-rbac.md §3. This middleware only
// answers "is this token valid", not "is this principal allowed to do X".
package authn

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"laundry-platform/shared/respond"
)

const (
	HeaderPrincipalType = "X-Principal-Type" // "STAFF" or "CUSTOMER" (docs/06-database-schema.md §1.9, ADR-013)
	HeaderPrincipalID   = "X-Principal-ID"
	HeaderRoles         = "X-Roles"      // comma-joined staff role codes; empty for customers
	HeaderOutletIDs     = "X-Outlet-Ids" // comma-joined; empty means GLOBAL scope (OWNER/SUPER_ADMIN)
)

// Claims is the access-token payload shape. Both staff and customer tokens
// use this same shape; PrincipalType distinguishes them (ADR-013 — a
// customer token never carries staff Roles).
type Claims struct {
	jwt.RegisteredClaims
	PrincipalType string   `json:"principal_type"`
	Roles         []string `json:"roles,omitempty"`
	OutletIDs     []string `json:"outlet_ids,omitempty"`
}

// Authenticate validates a bearer token if one is present. A request with
// no Authorization header is passed through unauthenticated — whether a
// given route requires auth at all is a service-layer decision
// (docs/07-api-contract.md marks PUBLIC routes explicitly), not this
// middleware's to make, since it has no per-route knowledge.
//
// A request that DOES present a bearer token must have it validate
// successfully, or the request is rejected here — a stale/forged token is
// never silently downgraded to "unauthenticated".
func Authenticate(signingKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Strip any client-supplied trusted headers before evaluating —
			// these must only ever be set by this middleware.
			r.Header.Del(HeaderPrincipalType)
			r.Header.Del(HeaderPrincipalID)
			r.Header.Del(HeaderRoles)
			r.Header.Del(HeaderOutletIDs)

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			tokenString, ok := strings.CutPrefix(authHeader, "Bearer ")
			if !ok {
				respond.Error(w, r, http.StatusUnauthorized, "INVALID_TOKEN", "Authorization header must be a Bearer token.")
				return
			}

			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
				return []byte(signingKey), nil
			}, jwt.WithValidMethods([]string{"HS256"}))
			if err != nil || !token.Valid {
				respond.Error(w, r, http.StatusUnauthorized, "INVALID_TOKEN", "The access token is invalid or expired.")
				return
			}

			r.Header.Set(HeaderPrincipalType, claims.PrincipalType)
			r.Header.Set(HeaderPrincipalID, claims.Subject)
			r.Header.Set(HeaderRoles, strings.Join(claims.Roles, ","))
			r.Header.Set(HeaderOutletIDs, strings.Join(claims.OutletIDs, ","))

			next.ServeHTTP(w, r)
		})
	}
}
