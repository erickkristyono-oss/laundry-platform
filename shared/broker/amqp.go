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
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"laundry-platform/shared/outbox"
)

// Conn wraps a RabbitMQ connection + channel, and keeps them alive: if the
// connection drops for any reason (RabbitMQ restarting, a network blip),
// it transparently redials and re-establishes the channel in the
// background, rather than leaving every future publish/consume failing
// forever. Found the hard way — a docker-compose rebuild that recreated
// only the RabbitMQ container left every dependent service's connection
// dead, silently dropping every outbox event until the service itself was
// restarted. Callers (ExchangePublisher, DeclareConsumerQueue) always read
// the *current* channel through Conn rather than holding one of their own.
type Conn struct {
	url string
	// ctx is Conn's own background lifecycle — deliberately NOT the ctx
	// passed into Connect, which by convention (every service's main.go)
	// is wrapped in a short context.WithTimeout bounding only the startup
	// dial attempt and cancelled right after Connect returns. Reusing that
	// ctx here was the actual bug in an earlier version of this fix: the
	// reconnect watcher read it as already-cancelled and exited within
	// moments of starting, silently disabling reconnection for the rest of
	// the process's life. cancel() is called from Close().
	ctx    context.Context
	cancel context.CancelFunc

	mu          sync.RWMutex
	conn        *amqp.Connection
	ch          *amqp.Channel
	reconnected chan struct{} // closed and replaced on every successful (re)connect
}

// Connect dials url and opens one channel, retrying with backoff until ctx
// is done. RabbitMQ's AMQP listener can accept connections slightly later
// than its own health check reports ready (docker-compose healthcheck races
// this window), so a single dial attempt at service startup is not reliable
// — every caller needs this retry, hence it lives here rather than being
// duplicated per service. ctx only bounds this initial attempt (callers
// commonly pass a short-lived context.WithTimeout here so a misconfigured
// broker fails startup fast rather than hanging forever) — the returned
// Conn keeps itself alive independently after that, redialing
// automatically for as long as the process runs, until Close() is called.
func Connect(ctx context.Context, url string) (*Conn, error) {
	connCtx, cancel := context.WithCancel(context.Background())
	c := &Conn{url: url, ctx: connCtx, cancel: cancel, reconnected: make(chan struct{})}
	if err := c.dial(ctx); err != nil {
		cancel()
		return nil, err
	}
	go c.watch()
	return c, nil
}

func (c *Conn) dial(ctx context.Context) error {
	backoff := 500 * time.Millisecond
	const maxBackoff = 5 * time.Second

	var lastErr error
	for {
		conn, err := amqp.Dial(c.url)
		if err == nil {
			ch, chErr := conn.Channel()
			if chErr == nil {
				c.mu.Lock()
				c.conn, c.ch = conn, ch
				old := c.reconnected
				c.reconnected = make(chan struct{})
				c.mu.Unlock()
				close(old)
				return nil
			}
			_ = conn.Close()
			lastErr = chErr
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("broker: connect: %w (last error: %v)", ctx.Err(), lastErr)
		case <-time.After(backoff):
		}
		if backoff < maxBackoff {
			backoff *= 2
		}
	}
}

// watch waits for the current connection to close — for any reason — and
// redials, forever, until c.ctx is done.
func (c *Conn) watch() {
	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		closeErr := make(chan *amqp.Error, 1)
		conn.NotifyClose(closeErr)

		select {
		case <-c.ctx.Done():
			return
		case <-closeErr:
		}

		if err := c.dial(c.ctx); err != nil {
			return // c.ctx was cancelled during redial
		}
	}
}

// channel returns the current, live AMQP channel. Never cache this value
// across a potential reconnect — always call channel() again.
func (c *Conn) channel() *amqp.Channel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ch
}

// waitReconnect returns a channel that closes the next time Conn
// successfully (re)connects.
func (c *Conn) waitReconnect() <-chan struct{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reconnected
}

func (c *Conn) Close() {
	c.cancel() // stops watch() and every DeclareConsumerQueue forwarding goroutine
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

// DeclareTopicExchange declares (idempotently) a durable topic exchange,
// e.g. "core.events", per docs/08-event-contract.md §1. Durable exchanges
// are stored by the RabbitMQ server itself, so this only needs to run once
// at startup — it survives a broker restart without needing to be
// re-declared on reconnect.
func (c *Conn) DeclareTopicExchange(name string) error {
	return c.channel().ExchangeDeclare(name, "topic", true, false, false, false, nil)
}

// ExchangePublisher implements outbox.Publisher by publishing to a named
// topic exchange with routing key = event_type. It always publishes
// through Conn's current channel, so it keeps working across a reconnect
// without being rebuilt.
type ExchangePublisher struct {
	conn     *Conn
	exchange string
}

func NewExchangePublisher(c *Conn, exchange string) *ExchangePublisher {
	return &ExchangePublisher{conn: c, exchange: exchange}
}

func (p *ExchangePublisher) Publish(ctx context.Context, env outbox.Envelope) error {
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("broker: marshal envelope: %w", err)
	}
	return p.conn.channel().PublishWithContext(ctx, p.exchange, env.EventType, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		MessageId:    env.EventID.String(),
		DeliveryMode: amqp.Persistent,
	})
}

// DeclareConsumerQueue declares a durable queue bound to exchange for each
// of routingKeys, and returns a delivery channel that stays valid for the
// lifetime of Conn: if the underlying AMQP channel dies, a background
// goroutine re-declares the (durable, so already exists server-side) queue
// and its bindings on the fresh channel once Conn reconnects, and resumes
// forwarding deliveries into the same channel callers already hold —
// callers never need to know a reconnect happened. Callers are responsible
// for acking/nacking each delivery (at-least-once consumption, per
// docs/06-database-schema.md §6's idempotency requirement on the consumer
// side).
func (c *Conn) DeclareConsumerQueue(queueName, exchange string, routingKeys []string) (<-chan amqp.Delivery, error) {
	subscribe := func() (<-chan amqp.Delivery, error) {
		ch := c.channel()
		if _, err := ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
			return nil, fmt.Errorf("broker: declare queue %s: %w", queueName, err)
		}
		for _, key := range routingKeys {
			if err := ch.QueueBind(queueName, key, exchange, false, nil); err != nil {
				return nil, fmt.Errorf("broker: bind %s to %s/%s: %w", queueName, exchange, key, err)
			}
		}
		return ch.Consume(queueName, "", false, false, false, false, nil)
	}

	deliveries, err := subscribe()
	if err != nil {
		return nil, err
	}

	out := make(chan amqp.Delivery)
	go func() {
		defer close(out)
		for {
			for d := range deliveries {
				select {
				case out <- d:
				case <-c.ctx.Done():
					return
				}
			}
			if c.ctx.Err() != nil {
				return
			}
			// deliveries closed — the channel/connection died. Try again
			// right away (a reconnect may have already completed by now);
			// only block on waitReconnect() if that immediate attempt
			// still fails.
			var subErr error
			deliveries, subErr = subscribe()
			for subErr != nil {
				select {
				case <-c.ctx.Done():
					return
				case <-c.waitReconnect():
				}
				deliveries, subErr = subscribe()
			}
		}
	}()

	return out, nil
}
