package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"

	"laundry-platform/shared/outbox"
)

// ConsumePaymentEvents drives orders.payment_status (docs/06-database-schema.md
// §2.7 "Why payment_status lives on orders") and, on payment.paid, the
// system-triggered WEIGHING -> WASHING transition (docs/10-state-machines.md §1.1)
// — the one transition never reachable through the generic status endpoint.
func (h *Handler) ConsumePaymentEvents(ctx context.Context, deliveries <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			h.handlePaymentEvent(ctx, d)
		}
	}
}

func (h *Handler) handlePaymentEvent(ctx context.Context, d amqp.Delivery) {
	var env outbox.Envelope
	if err := json.Unmarshal(d.Body, &env); err != nil {
		h.Logger.Error("payment event: invalid envelope", slog.Any("error", err))
		_ = d.Ack(false) // poison message — acking avoids an infinite redelivery loop
		return
	}

	// Idempotency (docs/06-database-schema.md §6): skip if already processed.
	var alreadyProcessed bool
	if err := h.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1)`, env.EventID).Scan(&alreadyProcessed); err != nil {
		h.Logger.Error("payment event: dedup check failed", slog.Any("error", err))
		_ = d.Nack(false, true) // requeue — likely a transient DB issue
		return
	}
	if alreadyProcessed {
		_ = d.Ack(false)
		return
	}

	var err error
	switch env.EventType {
	case "payment.created":
		err = h.projectPaymentStatus(ctx, env, "PENDING")
	case "payment.paid":
		err = h.handlePaymentPaid(ctx, env)
	case "payment.failed":
		err = h.projectPaymentStatus(ctx, env, "FAILED")
	case "payment.refunded":
		err = h.projectPaymentStatus(ctx, env, "REFUNDED")
	}

	if err != nil {
		h.Logger.Error("payment event: handling failed", slog.String("event_type", env.EventType), slog.Any("error", err))
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}

func (h *Handler) projectPaymentStatus(ctx context.Context, env outbox.Envelope, status string) error {
	orderID, ok := extractOrderID(env)
	if !ok {
		return nil
	}

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE orders SET payment_status = $1, updated_at = now() WHERE id = $2`, status, orderID); err != nil {
		return err
	}
	if err := markEventProcessed(ctx, tx, env.EventID.String()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// handlePaymentPaid additionally performs the WEIGHING -> WASHING
// transition, since Core's own domain logic (not Payment's) owns the
// order state machine.
func (h *Handler) handlePaymentPaid(ctx context.Context, env outbox.Envelope) error {
	orderID, ok := extractOrderID(env)
	if !ok {
		return nil
	}

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE orders SET payment_status = 'PAID', updated_at = now() WHERE id = $1`, orderID); err != nil {
		return err
	}

	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&status); err != nil {
		return err
	}
	if status == "WEIGHING" {
		if err := h.transitionStatusTx(ctx, tx, orderID, "WEIGHING", "WASHING", "", nil, false); err != nil {
			return err
		}
	}

	if err := markEventProcessed(ctx, tx, env.EventID.String()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func extractOrderID(env outbox.Envelope) (string, bool) {
	var payload struct {
		OrderID string `json:"order_id"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil || payload.OrderID == "" {
		return "", false
	}
	return payload.OrderID, true
}
