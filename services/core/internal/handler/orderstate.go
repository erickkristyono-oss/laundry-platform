package handler

// transitions is the staff-driven subset of the order state machine
// (docs/10-state-machines.md §1.1) reachable through the generic
// POST /orders/{id}/status endpoint. WEIGHING -> WASHING is deliberately
// absent: it is system-triggered by payment.paid (see payment_events.go)
// or, for the UQ-05 administrative override, a dedicated endpoint not yet
// implemented (secondary feature, deferred per the "core flow first" scope).
var transitions = map[string][]string{
	"CREATED":   {"RECEIVED", "ON_HOLD", "CANCELLED"},
	"RECEIVED":  {"ON_HOLD", "CANCELLED"},
	"WEIGHING":  {"ON_HOLD", "CANCELLED"},
	"WASHING":   {"DRYING", "ON_HOLD"},
	"DRYING":    {"IRONING", "ON_HOLD"},
	"IRONING":   {"PACKING", "ON_HOLD"},
	"PACKING":   {"READY", "ON_HOLD"},
	"READY":     {"PICKED_UP", "DELIVERED", "ON_HOLD"},
	"PICKED_UP": {"COMPLETED"},
	"DELIVERED": {"COMPLETED"},
}

// staffRolesFor returns which roles may perform fromStatus -> toStatus,
// per the RBAC matrix (docs/09-rbac.md §2, order.status.transition row).
func staffRolesFor(from, to string) []string {
	if to == "ON_HOLD" || to == "CANCELLED" {
		return []string{"OUTLET_ADMIN", "OWNER", "SUPER_ADMIN"}
	}
	return []string{"CASHIER", "LAUNDRY_STAFF", "OUTLET_ADMIN", "OWNER", "SUPER_ADMIN"}
}

func isValidTransition(from, to string) bool {
	for _, allowed := range transitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

func requiresReason(to string) bool {
	return to == "ON_HOLD" || to == "CANCELLED"
}

// requiresPaidBeforeCompleting is the BR-02 money-safety net
// (docs/10-state-machines.md §1.2): even though every normal path to
// COMPLETED was already paid before WASHING, this check is what actually
// closes the gap opened by the (not-yet-implemented) UQ-05 override.
func requiresPaidBeforeCompleting(to string) bool {
	return to == "COMPLETED"
}
