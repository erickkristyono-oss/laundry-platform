package authtoken

import (
	"testing"
	"time"
)

func TestIssueAndParse_RoundTrip(t *testing.T) {
	signed, expiresAt, err := Issue("test-signing-key", "user-123", PrincipalStaff, []string{"CASHIER"}, []string{"outlet-a"}, 15*time.Minute)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if signed == "" {
		t.Fatal("Issue() returned an empty token")
	}
	if time.Until(expiresAt) <= 0 || time.Until(expiresAt) > 16*time.Minute {
		t.Errorf("expiresAt = %v, expected roughly 15 minutes from now", expiresAt)
	}

	claims, err := Parse("test-signing-key", signed)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "user-123")
	}
	if claims.PrincipalType != PrincipalStaff {
		t.Errorf("PrincipalType = %q, want %q", claims.PrincipalType, PrincipalStaff)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "CASHIER" {
		t.Errorf("Roles = %v, want [CASHIER]", claims.Roles)
	}
	if len(claims.OutletIDs) != 1 || claims.OutletIDs[0] != "outlet-a" {
		t.Errorf("OutletIDs = %v, want [outlet-a]", claims.OutletIDs)
	}
}

// A token signed with one key must never validate against a different key
// — this is the entire security guarantee of the scheme.
func TestParse_RejectsWrongSigningKey(t *testing.T) {
	signed, _, err := Issue("key-one", "user-123", PrincipalStaff, nil, nil, time.Hour)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if _, err := Parse("key-two", signed); err == nil {
		t.Error("expected Parse() to reject a token signed with a different key, got no error")
	}
}

func TestParse_RejectsExpiredToken(t *testing.T) {
	signed, _, err := Issue("test-signing-key", "user-123", PrincipalCustomer, nil, nil, -1*time.Minute)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	if _, err := Parse("test-signing-key", signed); err == nil {
		t.Error("expected Parse() to reject an already-expired token, got no error")
	}
}

func TestParse_RejectsGarbageToken(t *testing.T) {
	if _, err := Parse("test-signing-key", "not-a-jwt-at-all"); err == nil {
		t.Error("expected Parse() to reject a malformed token string, got no error")
	}
}

// A customer token must never carry a Roles claim a staff-only check would
// accept — Issue itself doesn't prevent that (the caller decides what to
// put in it), but PrincipalType round-tripping correctly is what lets
// httpauth.RequireStaffRole tell the two principal types apart at all
// (ADR-013).
func TestIssue_CustomerTokenCarriesNoImplicitRoles(t *testing.T) {
	signed, _, err := Issue("test-signing-key", "cust-1", PrincipalCustomer, nil, nil, time.Hour)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	claims, err := Parse("test-signing-key", signed)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if claims.PrincipalType != PrincipalCustomer {
		t.Errorf("PrincipalType = %q, want %q", claims.PrincipalType, PrincipalCustomer)
	}
	if len(claims.Roles) != 0 {
		t.Errorf("expected no roles on a customer token, got %v", claims.Roles)
	}
}
