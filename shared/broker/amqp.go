// Package broker wraps RabbitMQ connection, topic-exchange publishing, and
// queue consumption. Pure transport plumbing — no event-type knowledge, no
// business logic — per the "RabbitMQ outbox publisher/consumer helper"
// allow-list entry in docs/03-system-architecture.md §4.
//
// Topology follows docs/08-event-contract.md §1: one topic exchange per
// producing service ("identity.events", "core.events", "payment.events"),
// routing key = event_type.
package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"laundry-platform/shared/outbox"
)

// Conn wraps a single AMQP connection + channel for a service.
type Conn struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// Connect dials url and opens one channel, retrying with backoff until ctx
// is done. RabbitMQ's AMQP listener can accept connections slightly later
// than its own health check reports ready (docker-compose healthcheck races
// this window), so a single dial attempt at service startup is not reliable
// — every caller needs this retry, hence it lives here rather than being
// duplicated per service.
func Connect(ctx context.Context, url string) (*Conn, error) {
	backoff := 500 * time.Millisecond
	const maxBackoff = 5 * time.Second

	var lastErr error
	for {
		conn, err := amqp.Dial(url)
		if err == nil {
			ch, chErr := conn.Channel()
			if chErr == nil {
				return &Conn{conn: conn, ch: ch}, nil
			}
			_ = conn.Close()
			lastErr = chErr
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("broker: connect: %w (last error: %v)", ctx.Err(), lastErr)
		case <-time.After(backoff):
		}
		if backoff < maxBackoff {
			backoff *= 2
		}
	}
}

func (c *Conn) Close() {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

// DeclareTopicExchange declares (idempotently) a durable topic exchange,
// e.g. "core.events", per docs/08-event-contract.md §1.
func (c *Conn) DeclareTopicExchange(name string) error {
	return c.ch.ExchangeDeclare(name, "topic", true, false, false, false, nil)
}

// ExchangePublisher implements outbox.Publisher by publishing to a named
// topic exchange with routing key = event_type.
type ExchangePublisher struct {
	ch       *amqp.Channel
	exchange string
}

func NewExchangePublisher(c *Conn, exchange string) *ExchangePublisher {
	return &ExchangePublisher{ch: c.ch, exchange: exchange}
}

func (p *ExchangePublisher) Publish(ctx context.Context, env outbox.Envelope) error {
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("broker: marshal envelope: %w", err)
	}
	return p.ch.PublishWithContext(ctx, p.exchange, env.EventType, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		MessageId:    env.EventID.String(),
		DeliveryMode: amqp.Persistent,
	})
}

// DeclareConsumerQueue declares a durable queue bound to exchange for each
// of routingKeys, and returns a delivery channel. Callers are responsible
// for acking/nacking each delivery (at-least-once consumption, per
// docs/06-database-schema.md §6's idempotency requirement on the consumer side).
func (c *Conn) DeclareConsumerQueue(queueName, exchange string, routingKeys []string) (<-chan amqp.Delivery, error) {
	if _, err := c.ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("broker: declare queue %s: %w", queueName, err)
	}
	for _, key := range routingKeys {
		if err := c.ch.QueueBind(queueName, key, exchange, false, nil); err != nil {
			return nil, fmt.Errorf("broker: bind %s to %s/%s: %w", queueName, exchange, key, err)
		}
	}
	return c.ch.Consume(queueName, "", false, false, false, false, nil)
}
