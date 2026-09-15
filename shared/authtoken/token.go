// Package authtoken is the single source of truth for the access-token
// shape and signing, shared by every issuer (Identity Service, for both
// staff and customer principals) and the one verifier (the Gateway,
// docs/09-rbac.md §3-4). Keeping Issue/Parse in one place — rather than
// duplicating a Claims struct in Identity and in the Gateway — guarantees
// the two can never drift out of sync on field names or signing method.
package authtoken

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	PrincipalStaff    = "STAFF"
	PrincipalCustomer = "CUSTOMER"
)

// Claims is the access-token payload. Both principal types use this same
// shape; PrincipalType distinguishes them (ADR-013,
// docs/14-architecture-decisions.md) — a customer token never carries
// staff Roles, and every staff-only route must reject
// PrincipalType != PrincipalStaff regardless of what Roles says.
type Claims struct {
	jwt.RegisteredClaims
	PrincipalType string   `json:"principal_type"`
	Roles         []string `json:"roles,omitempty"`
	OutletIDs     []string `json:"outlet_ids,omitempty"`
}

// Issue signs a new access token for subjectID (a users.id or
// customer_accounts.id), valid for ttl.
func Issue(signingKey, subjectID, principalType string, roles, outletIDs []string, ttl time.Duration) (string, time.Time, error) {
	expiresAt := time.Now().Add(ttl)
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subjectID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		PrincipalType: principalType,
		Roles:         roles,
		OutletIDs:     outletIDs,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(signingKey))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("authtoken: sign: %w", err)
	}
	return signed, expiresAt, nil
}

// Parse validates a token's signature/expiry and returns its claims.
func Parse(signingKey, tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		return []byte(signingKey), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("authtoken: invalid token: %w", err)
	}
	return claims, nil
}
