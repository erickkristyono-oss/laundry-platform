// Package whatsapp abstracts the WhatsApp send call behind one interface,
// the same shape as internal/gateway in Payment Service: swapping providers
// (or going from Noop to a real one once FONNTE_TOKEN is set) never touches
// the caller.
package whatsapp

import "context"

type Sender interface {
	// Send delivers message to phone (Indonesian local/international format,
	// e.g. "0812..." or "62812..." — Fonnte accepts either) and returns the
	// provider's own reference on success.
	Send(ctx context.Context, phone, message string) (providerRef string, err error)
}
