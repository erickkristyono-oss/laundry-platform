// Command server runs the Reporting Service (docs/04-service-boundaries.md §6):
// read-only projections consumed from order.*/payment.* events, rebuildable
// from the event log. Publishes nothing — no outbox_events table, no relay
// (docs/06-database-schema.md §5).
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

	"laundry-platform/services/reporting/internal/config"
	"laundry-platform/services/reporting/internal/handler"
	"laundry-platform/services/reporting/migrations"
	"laundry-platform/shared/broker"
	"laundry-platform/shared/health"
	"laundry-platform/shared/httpmiddleware"
	"laundry-platform/shared/logging"
	"laundry-platform/shared/metrics"
	"laundry-platform/shared/migrator"
	"laundry-platform/shared/pgxutil"
)

const serviceName = "reporting"

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
	if err := conn.DeclareTopicExchange("payment.events"); err != nil {
		logger.Error("failed to declare payment.events exchange", slog.Any("error", err))
		os.Exit(1)
	}

	h := handler.New(pool, logger)

	coreDeliveries, err := conn.DeclareConsumerQueue("reporting.core-events", "core.events", []string{"order.#"})
	if err != nil {
		logger.Error("failed to declare reporting.core-events queue", slog.Any("error", err))
		os.Exit(1)
	}
	go h.ConsumeCoreEvents(ctx, coreDeliveries)

	paymentDeliveries, err := conn.DeclareConsumerQueue("reporting.payment-events", "payment.events", []string{"payment.#"})
	if err != nil {
		logger.Error("failed to declare reporting.payment-events queue", slog.Any("error", err))
		os.Exit(1)
	}
	go h.ConsumePaymentEvents(ctx, paymentDeliveries)

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

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("reporting service listening", slog.String("port", cfg.Port))
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
