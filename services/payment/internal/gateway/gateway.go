// Package gateway abstracts the online-payment provider behind one interface
// so a real provider (Midtrans, Xendit, ...) can be dropped in later without
// touching payments.go — only Charge() and a Provider constructor change.
// Today the only implementation is DummyProvider (dummy.go), which simulates
// a charge without calling any real network endpoint.
package gateway

import "context"

// ChargeRequest is everything a provider needs to start an online charge.
type ChargeRequest struct {
	PaymentID     string
	PaymentCode   string
	OrderID       string
	Amount        int64
	Method        string // QRIS, TRANSFER, CARD — never CASH (CASH never reaches a Provider)
	CustomerName  string
	CustomerPhone string
}

// ChargeResult is what the caller stores on the payment row and returns to
// the client. Status is always PENDING for a freshly created charge — a
// gateway confirms PAID/FAILED asynchronously via webhook (or, for
// DummyProvider, via the /simulate endpoint).
type ChargeResult struct {
	ProviderRef string
	CheckoutURL string
	QRString    string
	Status      string
}

type Provider interface {
	// Name identifies the provider for the payments.provider column and logs.
	Name() string
	Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
}
