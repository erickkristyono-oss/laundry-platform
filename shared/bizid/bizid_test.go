package bizid

import (
	"regexp"
	"testing"
	"time"
)

func TestCustomer_Format(t *testing.T) {
	re := regexp.MustCompile(`^CUS-\d{6}$`)
	id := Customer()
	if !re.MatchString(id) {
		t.Errorf("Customer() = %q, want format CUS-xxxxxx", id)
	}
}

func TestOrder_Format(t *testing.T) {
	today := time.Now().Format("20060102")
	re := regexp.MustCompile(`^ORD-\d{8}-\d{6}$`)
	id := Order()
	if !re.MatchString(id) {
		t.Errorf("Order() = %q, want format ORD-YYYYMMDD-xxxxxx", id)
	}
	if id[4:12] != today {
		t.Errorf("Order() date segment = %q, want today (%q)", id[4:12], today)
	}
}

func TestPayment_Format(t *testing.T) {
	re := regexp.MustCompile(`^PAY-\d{8}-\d{6}$`)
	id := Payment()
	if !re.MatchString(id) {
		t.Errorf("Payment() = %q, want format PAY-YYYYMMDD-xxxxxx", id)
	}
}

func TestRefund_Format(t *testing.T) {
	re := regexp.MustCompile(`^RFD-\d{8}-\d{6}$`)
	id := Refund()
	if !re.MatchString(id) {
		t.Errorf("Refund() = %q, want format RFD-YYYYMMDD-xxxxxx", id)
	}
}

// Sanity-checks that back-to-back calls actually vary (the RNG is wired up,
// not returning a fixed value). Kept deliberately small: the suffix is only
// 6 random digits (1,000,000 possibilities), so by the birthday paradox a
// collision becomes *likely*, not exceptional, well under 5,000 draws in
// the same day — a real limitation worth knowing about (order_code is a
// UNIQUE column, so a collision would surface as a real insert failure),
// not something this test should assert against at a size that would make
// it flaky.
func TestOrder_ConsecutiveCallsVary(t *testing.T) {
	seen := make(map[string]bool)
	const n = 30
	for i := 0; i < n; i++ {
		id := Order()
		if seen[id] {
			t.Fatalf("Order() produced a duplicate id within %d calls: %q", n, id)
		}
		seen[id] = true
	}
}
