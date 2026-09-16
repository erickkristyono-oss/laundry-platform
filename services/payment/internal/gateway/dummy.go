package gateway

import (
	"context"
	"fmt"
)

// DummyProvider stands in for a real payment gateway during local
// development and demos, before a real account (Midtrans, Xendit, ...) is
// registered. It never calls a real network endpoint: CheckoutURL points at
// the frontend's own /payments/{id}/simulate page, where a human (or a test)
// manually marks the charge PAID/FAILED to simulate the provider's webhook.
type DummyProvider struct {
	// WebBaseURL is the frontend's public origin, e.g. "http://localhost:3000".
	WebBaseURL string
}

func (DummyProvider) Name() string { return "DUMMY" }

func (d DummyProvider) Charge(_ context.Context, req ChargeRequest) (ChargeResult, error) {
	result := ChargeResult{
		ProviderRef: fmt.Sprintf("DUMMY-%s", req.PaymentCode),
		CheckoutURL: fmt.Sprintf("%s/payments/%s/simulate", d.WebBaseURL, req.PaymentID),
		Status:      "PENDING",
	}
	if req.Method == "QRIS" {
		// Shaped like a QRIS payload but not a real one — clearly a dummy
		// value, never meant to be scanned by a real wallet app.
		result.QRString = fmt.Sprintf("DUMMY-QRIS|%s|%d", req.PaymentCode, req.Amount)
	}
	return result, nil
}
