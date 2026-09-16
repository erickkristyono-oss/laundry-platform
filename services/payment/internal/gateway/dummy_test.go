package gateway

import (
	"context"
	"strings"
	"testing"
)

func TestDummyProvider_Name(t *testing.T) {
	if got := (DummyProvider{}).Name(); got != "DUMMY" {
		t.Errorf("Name() = %q, want %q", got, "DUMMY")
	}
}

func TestDummyProvider_Charge_QRIS(t *testing.T) {
	d := DummyProvider{WebBaseURL: "http://localhost:3000"}
	result, err := d.Charge(context.Background(), ChargeRequest{
		PaymentID:   "pay-uuid-1",
		PaymentCode: "PAY-20260101-000001",
		OrderID:     "order-uuid-1",
		Amount:      22400,
		Method:      "QRIS",
	})
	if err != nil {
		t.Fatalf("Charge() error = %v", err)
	}

	if result.Status != "PENDING" {
		t.Errorf("Status = %q, want PENDING — a charge must never start out already settled", result.Status)
	}
	if result.QRString == "" {
		t.Error("expected a non-empty QRString for method=QRIS")
	}
	if !strings.Contains(result.CheckoutURL, "pay-uuid-1") {
		t.Errorf("CheckoutURL = %q, expected it to reference the payment id so /payments/{id}/simulate routes to the right payment", result.CheckoutURL)
	}
	if !strings.HasPrefix(result.CheckoutURL, "http://localhost:3000") {
		t.Errorf("CheckoutURL = %q, expected it to be built from WebBaseURL", result.CheckoutURL)
	}
	if result.ProviderRef == "" {
		t.Error("expected a non-empty ProviderRef")
	}
}

// Non-QRIS methods (TRANSFER, CARD) don't have a QR code to scan — the
// dummy provider must not fabricate one for them.
func TestDummyProvider_Charge_NonQRISHasNoQRString(t *testing.T) {
	d := DummyProvider{WebBaseURL: "http://localhost:3000"}
	result, err := d.Charge(context.Background(), ChargeRequest{
		PaymentID:   "pay-uuid-2",
		PaymentCode: "PAY-20260101-000002",
		OrderID:     "order-uuid-1",
		Amount:      10000,
		Method:      "TRANSFER",
	})
	if err != nil {
		t.Fatalf("Charge() error = %v", err)
	}
	if result.QRString != "" {
		t.Errorf("expected no QRString for method=TRANSFER, got %q", result.QRString)
	}
	if result.CheckoutURL == "" {
		t.Error("expected a CheckoutURL regardless of method")
	}
}
