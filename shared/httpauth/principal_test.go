package httpauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"laundry-platform/shared/authtoken"
)

func TestPrincipal_HasRole(t *testing.T) {
	p := Principal{Roles: []string{"CASHIER", "OUTLET_ADMIN"}}

	if !p.HasRole("CASHIER") {
		t.Error("expected HasRole(CASHIER) to be true")
	}
	if p.HasRole("OWNER") {
		t.Error("expected HasRole(OWNER) to be false")
	}
}

func TestPrincipal_HasAnyRole(t *testing.T) {
	p := Principal{Roles: []string{"CASHIER"}}

	if !p.HasAnyRole("OWNER", "CASHIER") {
		t.Error("expected HasAnyRole to match on CASHIER")
	}
	if p.HasAnyRole("OWNER", "SUPER_ADMIN") {
		t.Error("expected HasAnyRole to find no match")
	}
	if (Principal{}).HasAnyRole() {
		t.Error("expected HasAnyRole with no candidates to be false")
	}
}

func TestPrincipal_IsGlobalStaff(t *testing.T) {
	cases := []struct {
		name  string
		roles []string
		want  bool
	}{
		{"owner is global", []string{"OWNER"}, true},
		{"super admin is global", []string{"SUPER_ADMIN"}, true},
		{"outlet admin is not global", []string{"OUTLET_ADMIN"}, false},
		{"cashier is not global", []string{"CASHIER"}, false},
		{"no roles is not global", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := Principal{Roles: c.roles}
			if got := p.IsGlobalStaff(); got != c.want {
				t.Errorf("IsGlobalStaff() = %v, want %v", got, c.want)
			}
		})
	}
}

// CanAccessOutlet is the RBAC check every outlet-scoped read/write relies
// on (docs/09-rbac.md §4). A regression here means a non-global staff
// member could read or act on another outlet's data.
func TestPrincipal_CanAccessOutlet(t *testing.T) {
	owner := Principal{Roles: []string{"OWNER"}, OutletIDs: nil}
	if !owner.CanAccessOutlet("outlet-anything") {
		t.Error("OWNER must be able to access any outlet regardless of OutletIDs")
	}

	scoped := Principal{Roles: []string{"CASHIER"}, OutletIDs: []string{"outlet-a", "outlet-b"}}
	if !scoped.CanAccessOutlet("outlet-a") {
		t.Error("expected access to an outlet in OutletIDs")
	}
	if scoped.CanAccessOutlet("outlet-c") {
		t.Error("expected no access to an outlet not in OutletIDs — this is the exact bug class docs/09-rbac.md §4 guards against")
	}

	noOutlets := Principal{Roles: []string{"CASHIER"}, OutletIDs: nil}
	if noOutlets.CanAccessOutlet("outlet-a") {
		t.Error("a non-global staff member with zero assigned outlets must not access any outlet")
	}
}

func TestFromRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	r.Header.Set(HeaderPrincipalType, authtoken.PrincipalStaff)
	r.Header.Set(HeaderPrincipalID, "user-123")
	r.Header.Set(HeaderRoles, "CASHIER,OUTLET_ADMIN")
	r.Header.Set(HeaderOutletIDs, "outlet-a,outlet-b")

	p := FromRequest(r)

	if p.Type != authtoken.PrincipalStaff || p.ID != "user-123" {
		t.Fatalf("unexpected principal: %+v", p)
	}
	if !p.HasRole("CASHIER") || !p.HasRole("OUTLET_ADMIN") {
		t.Errorf("expected both roles parsed from header, got %v", p.Roles)
	}
	if len(p.OutletIDs) != 2 || p.OutletIDs[0] != "outlet-a" {
		t.Errorf("expected outlet ids parsed from header, got %v", p.OutletIDs)
	}
}

func TestFromRequest_Unauthenticated(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	p := FromRequest(r)
	if p.IsAuthenticated() {
		t.Error("a request with no principal headers must not be authenticated")
	}
}

func TestRequireAuth(t *testing.T) {
	handlerCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handlerCalled = true })

	t.Run("rejects unauthenticated", func(t *testing.T) {
		handlerCalled = false
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		RequireAuth(next).ServeHTTP(w, r)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
		if handlerCalled {
			t.Error("next handler must not run for an unauthenticated request")
		}
	})

	t.Run("passes authenticated", func(t *testing.T) {
		handlerCalled = false
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set(HeaderPrincipalType, authtoken.PrincipalCustomer)
		r.Header.Set(HeaderPrincipalID, "cust-1")
		w := httptest.NewRecorder()
		RequireAuth(next).ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
		if !handlerCalled {
			t.Error("next handler must run for an authenticated request")
		}
	})
}

// RequireStaffRole must reject a CUSTOMER-principal token even if it
// somehow carries a staff-shaped role claim — this is the ADR-013
// guarantee that a customer can never acquire staff permissions.
func TestRequireStaffRole_RejectsCustomerPrincipal(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := RequireStaffRole("OWNER", "SUPER_ADMIN")(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(HeaderPrincipalType, authtoken.PrincipalCustomer)
	r.Header.Set(HeaderPrincipalID, "cust-1")
	r.Header.Set(HeaderRoles, "OWNER") // a customer token should never carry this, but even if it did:
	w := httptest.NewRecorder()

	mw.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for a CUSTOMER principal on a staff-only route, got %d", w.Code)
	}
}

func TestRequireStaffRole_RejectsWrongRole(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := RequireStaffRole("OWNER", "SUPER_ADMIN")(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(HeaderPrincipalType, authtoken.PrincipalStaff)
	r.Header.Set(HeaderPrincipalID, "user-1")
	r.Header.Set(HeaderRoles, "CASHIER")
	w := httptest.NewRecorder()

	mw.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for a role not in the allow-list, got %d", w.Code)
	}
}

func TestRequireStaffRole_AllowsMatchingRole(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mw := RequireStaffRole("OWNER", "SUPER_ADMIN")(next)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(HeaderPrincipalType, authtoken.PrincipalStaff)
	r.Header.Set(HeaderPrincipalID, "user-1")
	r.Header.Set(HeaderRoles, "OWNER")
	w := httptest.NewRecorder()

	mw.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for an allow-listed role, got %d", w.Code)
	}
}

func TestRequireCustomer(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	t.Run("rejects staff principal", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set(HeaderPrincipalType, authtoken.PrincipalStaff)
		w := httptest.NewRecorder()
		RequireCustomer(next).ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for a staff principal, got %d", w.Code)
		}
	})

	t.Run("allows customer principal", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set(HeaderPrincipalType, authtoken.PrincipalCustomer)
		w := httptest.NewRecorder()
		RequireCustomer(next).ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for a customer principal, got %d", w.Code)
		}
	})
}
