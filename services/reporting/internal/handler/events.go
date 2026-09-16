package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5"
	amqp "github.com/rabbitmq/amqp091-go"

	"laundry-platform/shared/outbox"
)

// ConsumeCoreEvents builds rpt_order_summary and (on completion)
// rpt_daily_outlet_sales from order.* events (docs/07-api-contract.md §13).
//
// NOTE(phase-2+): rpt_service_performance is intentionally left unwired —
// it needs per-item (service_id, weight, revenue) data that no current
// event carries (docs/08-event-contract.md's order.weighed payload is
// order-level totals only). Populating it correctly requires either a new
// item-level event or a documented redesign; GET .../services/performance
// below reads whatever this table has (currently nothing) rather than
// silently fabricating numbers.
func (h *Handler) ConsumeCoreEvents(ctx context.Context, deliveries <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			h.handleCoreEvent(ctx, d)
		}
	}
}

// ConsumePaymentEvents builds rpt_refund_summary from payment.refunded.
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

func (h *Handler) alreadyProcessed(ctx context.Context, eventID string) (bool, error) {
	var exists bool
	err := h.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1)`, eventID).Scan(&exists)
	return exists, err
}

func markProcessed(ctx context.Context, tx pgx.Tx, eventID string) error {
	_, err := tx.Exec(ctx, `INSERT INTO processed_events (event_id) VALUES ($1) ON CONFLICT DO NOTHING`, eventID)
	return err
}

func (h *Handler) handleCoreEvent(ctx context.Context, d amqp.Delivery) {
	var env outbox.Envelope
	if err := json.Unmarshal(d.Body, &env); err != nil {
		h.Logger.Error("core event: invalid envelope", slog.Any("error", err))
		_ = d.Ack(false)
		return
	}

	done, err := h.alreadyProcessed(ctx, env.EventID.String())
	if err != nil {
		h.Logger.Error("core event: dedup check failed", slog.Any("error", err))
		_ = d.Nack(false, true)
		return
	}
	if done {
		_ = d.Ack(false)
		return
	}

	switch env.EventType {
	case "order.created":
		err = h.projectOrderCreated(ctx, env)
	case "order.weighed":
		err = h.projectOrderWeighed(ctx, env)
	case "order.status_changed", "order.completed", "order.cancelled":
		err = h.projectOrderStatusChanged(ctx, env)
	default:
		err = h.markOnly(ctx, env)
	}

	if err != nil {
		h.Logger.Error("core event: handling failed", slog.String("event_type", env.EventType), slog.Any("error", err))
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}

func (h *Handler) markOnly(ctx context.Context, env outbox.Envelope) error {
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := markProcessed(ctx, tx, env.EventID.String()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (h *Handler) projectOrderCreated(ctx context.Context, env outbox.Envelope) error {
	var payload struct {
		OrderID    string `json:"order_id"`
		OrderCode  string `json:"order_code"`
		CustomerID string `json:"customer_id"`
		OutletID   string `json:"outlet_id"`
		Status     string `json:"status"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO rpt_order_summary (order_id, order_code, outlet_id, customer_id, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (order_id) DO NOTHING
	`, payload.OrderID, payload.OrderCode, payload.OutletID, payload.CustomerID, payload.Status, env.OccurredAt); err != nil {
		return err
	}
	if err := markProcessed(ctx, tx, env.EventID.String()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (h *Handler) projectOrderWeighed(ctx context.Context, env outbox.Envelope) error {
	var payload struct {
		OrderID     string `json:"order_id"`
		TotalAmount int64  `json:"total_amount"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE rpt_order_summary SET total_amount = $1 WHERE order_id = $2`, payload.TotalAmount, payload.OrderID); err != nil {
		return err
	}
	if err := markProcessed(ctx, tx, env.EventID.String()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (h *Handler) projectOrderStatusChanged(ctx context.Context, env outbox.Envelope) error {
	var payload struct {
		OrderID  string `json:"order_id"`
		ToStatus string `json:"to_status"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE rpt_order_summary SET status = $1 WHERE order_id = $2`, payload.ToStatus, payload.OrderID); err != nil {
		return err
	}

	if payload.ToStatus == "COMPLETED" {
		var outletID string
		var totalAmount *int64
		if err := tx.QueryRow(ctx, `SELECT outlet_id, total_amount FROM rpt_order_summary WHERE order_id = $1`, payload.OrderID).
			Scan(&outletID, &totalAmount); err != nil && err != pgx.ErrNoRows {
			return err
		}
		if outletID != "" {
			amount := int64(0)
			if totalAmount != nil {
				amount = *totalAmount
			}
			salesDate := env.OccurredAt.Format("2006-01-02")
			if _, err := tx.Exec(ctx, `
				UPDATE rpt_order_summary SET completed_at = $1 WHERE order_id = $2
			`, env.OccurredAt, payload.OrderID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO rpt_daily_outlet_sales (sales_date, outlet_id, gross_amount, order_count, updated_at)
				VALUES ($1, $2, $3, 1, now())
				ON CONFLICT (sales_date, outlet_id) DO UPDATE SET
					gross_amount = rpt_daily_outlet_sales.gross_amount + EXCLUDED.gross_amount,
					order_count = rpt_daily_outlet_sales.order_count + 1,
					updated_at = now()
			`, salesDate, outletID, amount); err != nil {
				return err
			}
		}
	}

	if err := markProcessed(ctx, tx, env.EventID.String()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (h *Handler) handlePaymentEvent(ctx context.Context, d amqp.Delivery) {
	var env outbox.Envelope
	if err := json.Unmarshal(d.Body, &env); err != nil {
		h.Logger.Error("payment event: invalid envelope", slog.Any("error", err))
		_ = d.Ack(false)
		return
	}

	done, err := h.alreadyProcessed(ctx, env.EventID.String())
	if err != nil {
		h.Logger.Error("payment event: dedup check failed", slog.Any("error", err))
		_ = d.Nack(false, true)
		return
	}
	if done {
		_ = d.Ack(false)
		return
	}

	if env.EventType == "payment.refunded" {
		err = h.projectRefund(ctx, env)
	} else {
		err = h.markOnly(ctx, env)
	}

	if err != nil {
		h.Logger.Error("payment event: handling failed", slog.String("event_type", env.EventType), slog.Any("error", err))
		_ = d.Nack(false, true)
		return
	}
	_ = d.Ack(false)
}

func (h *Handler) projectRefund(ctx context.Context, env outbox.Envelope) error {
	var payload struct {
		OrderID string `json:"order_id"`
		Amount  int64  `json:"amount"`
	}
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		return err
	}

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var outletID string
	err = tx.QueryRow(ctx, `SELECT outlet_id FROM rpt_order_summary WHERE order_id = $1`, payload.OrderID).Scan(&outletID)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if outletID != "" {
		periodDate := env.OccurredAt.Format("2006-01-02")
		if _, err := tx.Exec(ctx, `
			INSERT INTO rpt_refund_summary (period_date, outlet_id, refund_count, refund_amount)
			VALUES ($1, $2, 1, $3)
			ON CONFLICT (period_date, outlet_id) DO UPDATE SET
				refund_count = rpt_refund_summary.refund_count + 1,
				refund_amount = rpt_refund_summary.refund_amount + EXCLUDED.refund_amount
		`, periodDate, outletID, payload.Amount); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE rpt_daily_outlet_sales SET refunded_amount = refunded_amount + $1, updated_at = now()
			WHERE sales_date = $2 AND outlet_id = $3
		`, payload.Amount, periodDate, outletID); err != nil {
			return err
		}
	}

	if err := markProcessed(ctx, tx, env.EventID.String()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
