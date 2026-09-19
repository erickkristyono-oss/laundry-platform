package handler

import (
	"strings"
	"testing"
)

func TestFormatIDR(t *testing.T) {
	cases := map[int64]string{
		0:       "Rp 0",
		7000:    "Rp 7.000",
		22400:   "Rp 22.400",
		1000000: "Rp 1.000.000",
		1234567: "Rp 1.234.567",
	}
	for amount, want := range cases {
		if got := formatIDR(amount); got != want {
			t.Errorf("formatIDR(%d) = %q, want %q", amount, got, want)
		}
	}
}

func TestOrderCreatedMessage_ContainsKeyFacts(t *testing.T) {
	msg := orderCreatedMessage("Budi", "ORD-20260916-000001")
	mustContain(t, msg, "Budi")
	mustContain(t, msg, "ORD-20260916-000001")
}

func TestOrderStatusChangedMessage_UsesKnownLabel(t *testing.T) {
	msg := orderStatusChangedMessage("Siti", "ORD-20260916-000002", "WASHING")
	mustContain(t, msg, "Siti")
	mustContain(t, msg, "ORD-20260916-000002")
	mustContain(t, msg, "Sedang dicuci")
}

// A status value that isn't in statusLabel (e.g. a future status added to
// Core without updating this map) must still produce a message — falling
// back to the raw status code — rather than sending blank/broken text to a
// customer over WhatsApp.
func TestOrderStatusChangedMessage_FallsBackForUnknownStatus(t *testing.T) {
	msg := orderStatusChangedMessage("Budi", "ORD-1", "SOME_FUTURE_STATUS")
	mustContain(t, msg, "SOME_FUTURE_STATUS")
}

func TestOrderWeighedMessage_ContainsTotalAndBothPaymentOptions(t *testing.T) {
	msg := orderWeighedMessage("Budi", "ORD-1", 22400)
	mustContain(t, msg, "ORD-1")
	mustContain(t, msg, "Rp 22.400")
	mustContain(t, msg, "bayar sekarang")
	mustContain(t, msg, "pengambilan")
}

func TestOrderReadyMessage_BranchesOnFulfillmentType(t *testing.T) {
	pickup := orderReadyMessage("Budi", "ORD-1", "WALK_IN")
	mustContain(t, pickup, "diambil di outlet")

	delivery := orderReadyMessage("Budi", "ORD-1", "DELIVERY")
	mustContain(t, delivery, "antar")

	both := orderReadyMessage("Budi", "ORD-1", "PICKUP_AND_DELIVERY")
	mustContain(t, both, "antar")
}

func TestPaymentPaidMessage_ContainsAmount(t *testing.T) {
	msg := paymentPaidMessage("Budi", "ORD-1", 22400)
	mustContain(t, msg, "Rp 22.400")
	mustContain(t, msg, "ORD-1")
}

func TestPaymentFailedMessage_ContainsOrderCode(t *testing.T) {
	msg := paymentFailedMessage("Budi", "ORD-1")
	mustContain(t, msg, "ORD-1")
	mustContain(t, msg, "gagal")
}

func TestPaymentRefundedMessage_ContainsAmount(t *testing.T) {
	msg := paymentRefundedMessage("Budi", "ORD-1", 15000)
	mustContain(t, msg, "Rp 15.000")
}

func mustContain(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected message to contain %q, got: %q", needle, haystack)
	}
}
