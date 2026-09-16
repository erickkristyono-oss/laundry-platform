// Command server runs the Notification Service (docs/04-service-boundaries.md §5):
// consumes order.*/payment.* events and sends WhatsApp updates via Fonnte
// (internal/whatsapp) for the customer-facing subset of the order journey;
// publishes nothing consumed downstream in MVP (UQ-08). pickup.*/delivery.*
// are subscribed to but not yet rendered — no customer-facing copy for
// those exists today.
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

	"laundry-platform/services/notification/internal/config"
	"laundry-platform/services/notification/internal/handler"
	"laundry-platform/services/notification/migrations"
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

const serviceName = "notification"

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

	// Present for platform consistency (UQ-08: expected to stay empty in MVP).
	if err := conn.DeclareTopicExchange("notification.events"); err != nil {
		logger.Error("failed to declare notification.events exchange", slog.Any("error", err))
		os.Exit(1)
	}
	publisher := broker.NewExchangePublisher(conn, "notification.events")
	relay := outbox.NewRelay(pool, publisher, "notification-service", logger)
	go relay.Run(ctx)

	if err := conn.DeclareTopicExchange("core.events"); err != nil {
		logger.Error("failed to declare core.events exchange", slog.Any("error", err))
		os.Exit(1)
	}
	if err := conn.DeclareTopicExchange("payment.events"); err != nil {
		logger.Error("failed to declare payment.events exchange", slog.Any("error", err))
		os.Exit(1)
	}

	h := handler.New(pool, cfg, logger)

	coreDeliveries, err := conn.DeclareConsumerQueue("notification.core-events", "core.events", []string{
		"order.#", "pickup.#", "delivery.#",
	})
	if err != nil {
		logger.Error("failed to declare notification.core-events queue", slog.Any("error", err))
		os.Exit(1)
	}
	go h.ConsumeCoreEvents(ctx, coreDeliveries)

	paymentDeliveries, err := conn.DeclareConsumerQueue("notification.payment-events", "payment.events", []string{
		"payment.#",
	})
	if err != nil {
		logger.Error("failed to declare notification.payment-events queue", slog.Any("error", err))
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

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("notification service listening", slog.String("port", cfg.Port))
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
