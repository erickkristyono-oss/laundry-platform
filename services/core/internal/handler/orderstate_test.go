package handler

import "testing"

// isValidTransition is the entire enforcement of docs/10-state-machines.md
// §1.1 — every status change in the system passes through it. These cases
// cover the full lifecycle plus the transitions that must be impossible.
func TestIsValidTransition(t *testing.T) {
	cases := []struct {
		from, to string
		want     bool
	}{
		{"CREATED", "RECEIVED", true},
		{"CREATED", "CANCELLED", true},
		{"CREATED", "ON_HOLD", true},
		{"RECEIVED", "ON_HOLD", true},
		{"RECEIVED", "CANCELLED", true},
		{"WEIGHING", "ON_HOLD", true},
		{"WASHING", "DRYING", true},
		{"DRYING", "IRONING", true},
		{"IRONING", "PACKING", true},
		{"PACKING", "READY", true},
		{"READY", "PICKED_UP", true},
		{"READY", "DELIVERED", true},
		{"PICKED_UP", "COMPLETED", true},
		{"DELIVERED", "COMPLETED", true},

		// The one transition that must NEVER be reachable through this
		// generic endpoint (docs/07-api-contract.md §8, the note under
		// POST /orders/{id}/status): it's system-triggered by payment.paid,
		// or via the dedicated UQ-05 override endpoint, never here.
		{"WEIGHING", "WASHING", false},

		// Can't skip stages.
		{"CREATED", "WASHING", false},
		{"RECEIVED", "READY", false},
		{"WASHING", "READY", false},

		// Can't go backwards.
		{"DRYING", "WASHING", false},

		// Terminal states have no outbound transitions at all.
		{"COMPLETED", "READY", false},
		{"CANCELLED", "RECEIVED", false},

		// Unknown "from" status.
		{"NOT_A_REAL_STATUS", "RECEIVED", false},
	}

	for _, c := range cases {
		t.Run(c.from+"->"+c.to, func(t *testing.T) {
			if got := isValidTransition(c.from, c.to); got != c.want {
				t.Errorf("isValidTransition(%q, %q) = %v, want %v", c.from, c.to, got, c.want)
			}
		})
	}
}

func TestRequiresReason(t *testing.T) {
	cases := map[string]bool{
		"ON_HOLD":   true,
		"CANCELLED": true,
		"RECEIVED":  false,
		"COMPLETED": false,
	}
	for to, want := range cases {
		if got := requiresReason(to); got != want {
			t.Errorf("requiresReason(%q) = %v, want %v", to, got, want)
		}
	}
}

// requiresPaidBeforeCompleting is BR-02's money-safety net: it's the only
// thing stopping an order from reaching COMPLETED while still UNPAID once
// the UQ-05 administrative override has been used to skip the normal
// payment gate.
func TestRequiresPaidBeforeCompleting(t *testing.T) {
	if !requiresPaidBeforeCompleting("COMPLETED") {
		t.Error("expected COMPLETED to require payment, got false")
	}
	if requiresPaidBeforeCompleting("READY") {
		t.Error("expected READY to not require payment, got true")
	}
}

func TestStaffRolesFor(t *testing.T) {
	t.Run("ON_HOLD and CANCELLED are admin-only", func(t *testing.T) {
		for _, to := range []string{"ON_HOLD", "CANCELLED"} {
			roles := staffRolesFor("WASHING", to)
			for _, forbidden := range []string{"CASHIER", "LAUNDRY_STAFF"} {
				for _, r := range roles {
					if r == forbidden {
						t.Errorf("staffRolesFor(_, %q) unexpectedly allows %q — ON_HOLD/CANCELLED must be admin-only per docs/09-rbac.md §2", to, forbidden)
					}
				}
			}
		}
	})

	t.Run("normal progression allows line staff", func(t *testing.T) {
		roles := staffRolesFor("WASHING", "DRYING")
		found := false
		for _, r := range roles {
			if r == "LAUNDRY_STAFF" {
				found = true
			}
		}
		if !found {
			t.Error("expected LAUNDRY_STAFF to be allowed to progress a normal wash-cycle transition")
		}
	})
}
