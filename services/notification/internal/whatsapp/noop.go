package whatsapp

import (
	"context"
	"log/slog"
)

// NoopSender stands in for FonnteSender when FONNTE_TOKEN is not configured
// (e.g. local dev before a real Fonnte device is registered): it logs the
// message instead of sending it, so the rest of the pipeline (rendering,
// persistence, dedup) can be built and tested without a real account.
type NoopSender struct {
	Logger *slog.Logger
}

func (n NoopSender) Send(_ context.Context, phone, message string) (string, error) {
	n.Logger.Info("whatsapp send (noop — FONNTE_TOKEN not set)", slog.String("phone", phone), slog.String("message", message))
	return "NOOP", nil
}
