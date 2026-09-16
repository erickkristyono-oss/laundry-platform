package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"

	"laundry-platform/shared/outbox"
)

// ConsumeCoreEvents drives WhatsApp notifications for the order lifecycle
// (docs/04-service-boundaries.md §5). Deliberately narrow: a WhatsApp send
// costs real money per message, so the customer gets pinged only at
// order.created ("pesanan diterima") and order.status_changed → COMPLETED
// — not one message per intermediate journey step
// (RECEIVED/WASHING/DRYING/IRONING/PACKING/READY/PICKED_UP/DELIVERED),
// which was confirmed too expensive after testing. Journey progress in
// between is still visible in-app (the order detail page's status
// timeline), just not pushed to WhatsApp.
func (h *Handler) ConsumeCoreEvents(ctx context.Context, deliveries <-chan amqp.Delivery) {
	h.consume(ctx, deliveries, h.handleCoreEvent)
}

// ConsumePaymentEvents drives WhatsApp notifications for payment outcomes.
func (h *Handler) ConsumePaymentEvents(ctx context.Context, deliveries <-chan amqp.Delivery) {
	h.consume(ctx, deliveries, h.handlePaymentEvent)
}

func (h *Handler) consume(ctx context.Context, deliveries <-chan amqp.Delivery, handle func(context.Context, outbox.Envelope) error) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			var env outbox.Envelope
			if err := json.Unmarshal(d.Body, &env); err != nil {
				h.Logger.Error("notification: invalid envelope", slog.Any("error", err))
				_ = d.Ack(false) // poison message
				continue
			}
			if err := handle(ctx, env); err != nil {
				h.Logger.Error("notification: handling failed", slog.String("event_type", env.EventType), slog.Any("error", err))
				_ = d.Nack(false, true)
				continue
			}
			_ = d.Ack(false)
		}
	}
}

func (h *Handler) handleCoreEvent(ctx context.Context, env outbox.Envelope) error {
	switch env.EventType {
	case "order.created":
		var p struct {
			OrderID    string `json:"order_id"`
			OrderCode  string `json:"order_code"`
			CustomerID string `json:"customer_id"`
		}
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil // malformed payload — nothing sensible to notify, don't block the queue
		}
		return h.notifyCustomer(ctx, env, p.CustomerID, func(customerName string) string {
			return orderCreatedMessage(customerName, p.OrderCode)
		})

	case "order.status_changed":
		// Deliberately narrow: WhatsApp messages cost real money per send
		// (Fonnte), and a message for every intermediate journey step
		// (RECEIVED/WASHING/DRYING/IRONING/PACKING/READY/PICKED_UP/
		// DELIVERED) was confirmed too expensive in practice. Only three
		// moments are worth pinging for: order received (order.created,
		// below), payment status (handlePaymentEvent), and completion.
		// Completion is never seen here, though — Core emits a distinct
		// "order.completed" event type for that one transition instead of
		// order.status_changed (services/core/internal/handler/orders.go,
		// transitionStatusTx: `if to == "COMPLETED" { eventType =
		// "order.completed" }`) — handled in the case below. Every other
		// to_status reaching this case is one of the ones we're
		// intentionally staying silent on.
		return nil

	case "order.completed":
		var p struct {
			OrderID string `json:"order_id"`
		}
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil
		}
		order, err := h.fetchCoreOrder(ctx, p.OrderID)
		if err != nil {
			return err // transient — worth a retry
		}
		return h.notifyCustomer(ctx, env, order.CustomerID, func(customerName string) string {
			return orderStatusChangedMessage(customerName, order.OrderCode, "COMPLETED")
		})

	default:
		return nil
	}
}

func (h *Handler) handlePaymentEvent(ctx context.Context, env outbox.Envelope) error {
	var p struct {
		OrderID string `json:"order_id"`
		Amount  int64  `json:"amount"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return nil
	}

	switch env.EventType {
	case "payment.paid", "payment.failed", "payment.refunded":
		order, err := h.fetchCoreOrder(ctx, p.OrderID)
		if err != nil {
			return err
		}
		return h.notifyCustomer(ctx, env, order.CustomerID, func(customerName string) string {
			switch env.EventType {
			case "payment.paid":
				return paymentPaidMessage(customerName, order.OrderCode, p.Amount)
			case "payment.failed":
				return paymentFailedMessage(customerName, order.OrderCode)
			default:
				return paymentRefundedMessage(customerName, order.OrderCode, p.Amount)
			}
		})
	default:
		return nil
	}
}

// notifyCustomer renders (via render, once the customer's name is known),
// persists (dedup on source_event_id — docs/06-database-schema.md §4.1),
// and sends one WhatsApp message. It is a no-op (not an error) if the
// customer has no phone on file, or if this event_id was already processed.
func (h *Handler) notifyCustomer(ctx context.Context, env outbox.Envelope, customerID string, render func(customerName string) string) error {
	var alreadyProcessed bool
	if err := h.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM notifications WHERE source_event_id = $1)`, env.EventID).Scan(&alreadyProcessed); err != nil {
		return err
	}
	if alreadyProcessed {
		return nil
	}

	customer, err := h.fetchCoreCustomer(ctx, customerID)
	if err != nil {
		return err
	}
	if customer.Phone == "" {
		return nil
	}

	message := render(customer.Name)

	var notificationID string
	if err := h.Pool.QueryRow(ctx, `
		INSERT INTO notifications (type, recipient_customer_id, source_event_id, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (source_event_id) DO NOTHING
		RETURNING id
	`, env.EventType, customerID, env.EventID, map[string]any{"message": message}).Scan(&notificationID); err != nil {
		return err
	}
	if notificationID == "" {
		return nil // lost a dedup race to another delivery of the same event
	}

	providerRef, sendErr := h.Sender.Send(ctx, customer.Phone, message)
	status := "SENT"
	var errMsg *string
	if sendErr != nil {
		status = "FAILED"
		msg := sendErr.Error()
		errMsg = &msg
		h.Logger.Error("whatsapp send failed", slog.String("phone", customer.Phone), slog.Any("error", sendErr))
	}
	_ = providerRef

	if _, err := h.Pool.Exec(ctx, `
		INSERT INTO notification_deliveries (notification_id, channel, status, attempt_count, last_attempt_at, error_message)
		VALUES ($1, 'WHATSAPP', $2, 1, now(), $3)
	`, notificationID, status, errMsg); err != nil {
		return err
	}
	return nil
}
