// Command server runs the Payment Service (docs/04-service-boundaries.md §4):
// payments, payment_transactions, refunds — isolated for compliance/integrity
// (docs/14-architecture-decisions.md ADR-011), verifying order totals
// synchronously against Core rather than trusting client input.
//
// Phase 1 technical-foundation scaffolding only — see the note in
// services/identity/cmd/server/main.go. Business handlers
// (docs/07-api-contract.md §11-12) are a Phase 2 concern.
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

	"laundry-platform/services/payment/internal/config"
	"laundry-platform/services/payment/migrations"
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

const serviceName = "payment"

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
		logger.Warn("redis unavailable at startup", slog.Any("error", err))
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
	if err := conn.DeclareTopicExchange("payment.events"); err != nil {
		logger.Error("failed to declare payment.events exchange", slog.Any("error", err))
		os.Exit(1)
	}

	publisher := broker.NewExchangePublisher(conn, "payment.events")
	relay := outbox.NewRelay(pool, publisher, "payment-service", logger)
	go relay.Run(ctx)

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

	// TODO(phase-2): mount /api/v1/payments, /api/v1/refunds per
	// docs/07-api-contract.md §11-12, including the synchronous
	// order-total verification call to cfg.CoreServiceURL.

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("payment service listening", slog.String("port", cfg.Port))
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
