// Command server runs the Identity Service (docs/04-service-boundaries.md §2):
// staff authN/authZ primitives and, per the UQ-03 resolution, customer
// self-service authentication.
//
// This is Phase 1 "technical foundation" scaffolding: process wiring,
// health/readiness/metrics, DB/broker/cache connections, and the outbox
// relay. Business handlers (docs/07-api-contract.md §1-3) are a Phase 2
// concern and are not implemented here, per the Master Prompt's
// architecture-first sequencing.
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

	"laundry-platform/services/identity/internal/config"
	"laundry-platform/services/identity/migrations"
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

const serviceName = "identity"

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
		logger.Warn("redis unavailable at startup; session cache degraded", slog.Any("error", err))
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
	if err := conn.DeclareTopicExchange("identity.events"); err != nil {
		logger.Error("failed to declare identity.events exchange", slog.Any("error", err))
		os.Exit(1)
	}

	publisher := broker.NewExchangePublisher(conn, "identity.events")
	relay := outbox.NewRelay(pool, publisher, "identity-service", logger)
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

	// TODO(phase-2): mount /api/v1/auth/* and /api/v1/customer-auth/* and
	// /api/v1/users/* handlers here per docs/07-api-contract.md §1-3.

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("identity service listening", slog.String("port", cfg.Port))
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
