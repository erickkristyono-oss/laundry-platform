// Command server runs the Core Service (docs/04-service-boundaries.md §3):
// customers, outlets, catalog/pricing, tax rate (UQ-09), and the order
// lifecycle.
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

	"laundry-platform/services/core/internal/config"
	"laundry-platform/services/core/internal/handler"
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

	if err := handler.EnsureSeedCatalog(ctx, pool, logger); err != nil {
		logger.Error("failed to bootstrap seed catalog", slog.Any("error", err))
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

	h := handler.New(pool, cfg, logger)

	// Consume payment.* to drive orders.payment_status and the
	// system-triggered WEIGHING -> WASHING transition (docs/04-service-boundaries.md §3).
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
	go h.ConsumePaymentEvents(ctx, deliveries)

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

	h.Mount(r)
	// TODO(phase-2+): mount /api/v1/orders/{id}/transfer, /pickups,
	// /deliveries, and the UQ-05 override endpoint — secondary features
	// deferred per the "core flow first" prioritization.

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
