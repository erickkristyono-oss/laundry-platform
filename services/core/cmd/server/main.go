// Command server runs the Core Service (docs/04-service-boundaries.md §3):
// customers, outlets, catalog/pricing, tax rate (UQ-09), and the full order
// lifecycle including the UQ-05 administrative payment-gate override.
//
// Phase 1 technical-foundation scaffolding only — see the note in
// services/identity/cmd/server/main.go. Business handlers
// (docs/07-api-contract.md §4-9) are a Phase 2 concern.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	amqp "github.com/rabbitmq/amqp091-go"

	"laundry-platform/services/core/internal/config"
	"laundry-platform/services/core/migrations"
	"laundry-platform/shared/broker"
	"laundry-platform/shared/health"
	"laundry-platform/shared/httpmiddleware"
	"laundry-platform/shared/logging"
	"laundry-platform/shared/metrics"
	"laundry-platform/shared/migrator"
	"laundry-platform/shared/outbox"
	"laundry-platform/shared/pgxutil"
	"laundry-platform/shared/redisutil"
)

const serviceName = "core"

func main() {
	cfg := config.Load()
	logger := logging.New(serviceName, parseLevel(cfg.LogLevel))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxutil.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}
	defer pool.Close()

	if err := migrator.Up(ctx, pool, migrations.FS); err != nil {
		logger.Error("failed to apply migrations", slog.Any("error", err))
		os.Exit(1)
	}

	redisClient, err := redisutil.NewClient(ctx, cfg.RedisAddr)
	if err != nil {
		logger.Warn("redis unavailable at startup; pricing/outlet cache degraded", slog.Any("error", err))
	}
	if redisClient != nil {
		defer redisClient.Close()
	}

	brokerCtx, brokerCancel := context.WithTimeout(ctx, 30*time.Second)
	conn, err := broker.Connect(brokerCtx, cfg.RabbitMQURL)
	brokerCancel()
	if err != nil {
		logger.Error("failed to connect to RabbitMQ", slog.Any("error", err))
		os.Exit(1)
	}
	defer conn.Close()

	if err := conn.DeclareTopicExchange("core.events"); err != nil {
		logger.Error("failed to declare core.events exchange", slog.Any("error", err))
		os.Exit(1)
	}
	publisher := broker.NewExchangePublisher(conn, "core.events")
	relay := outbox.NewRelay(pool, publisher, "core-service", logger)
	go relay.Run(ctx)

	// Consume payment.* to drive orders.payment_status projection
	// (docs/04-service-boundaries.md §3, docs/06-database-schema.md §2.7
	// "Why payment_status lives on orders"). Projection logic itself is a
	// Phase 2 concern; this scaffold only wires the subscription.
	if err := conn.DeclareTopicExchange("payment.events"); err != nil {
		logger.Error("failed to declare payment.events exchange", slog.Any("error", err))
		os.Exit(1)
	}
	deliveries, err := conn.DeclareConsumerQueue("core.payment-events", "payment.events", []string{
		"payment.created", "payment.paid", "payment.failed", "payment.refunded",
	})
	if err != nil {
		logger.Error("failed to declare core.payment-events queue", slog.Any("error", err))
		os.Exit(1)
	}
	go consumePaymentEvents(ctx, deliveries, logger)

	m := metrics.New(serviceName)

	r := chi.NewRouter()
	r.Use(httpmiddleware.RequestID)
	r.Use(httpmiddleware.Recover(logger))
	r.Use(httpmiddleware.AccessLog(logger))
	r.Use(m.Middleware)

	r.Get("/healthz", health.Livez)
	r.Get("/readyz", health.Readyz(map[string]health.Check{
		"database": func(ctx context.Context) error { return pool.Ping(ctx) },
	}))
	r.Handle("/metrics", metrics.Handler())

	// TODO(phase-2): mount /api/v1/customers, /outlets, /services, /pricing,
	// /settings/tax-rate, /orders, /pickups, /deliveries per
	// docs/07-api-contract.md §4-9.

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("core service listening", slog.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func consumePaymentEvents(ctx context.Context, deliveries <-chan amqp.Delivery, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case d, ok := <-deliveries:
			if !ok {
				return
			}
			logger.Info("received payment event", slog.String("routing_key", d.RoutingKey))
			// TODO(phase-2): idempotent projection update per
			// docs/06-database-schema.md §6 consumer-idempotency contract
			// (dedup on event_id before writing orders.payment_status).
			_ = d.Ack(false)
		}
	}
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
