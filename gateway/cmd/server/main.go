// Command server runs the API Gateway (docs/03-system-architecture.md §1):
// the only entry point for clients — routing, request-ID injection, coarse
// authentication, rate limiting, and CORS. No business logic lives here.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"laundry-platform/gateway/internal/authn"
	"laundry-platform/gateway/internal/config"
	"laundry-platform/gateway/internal/docsui"
	"laundry-platform/gateway/internal/proxy"
	"laundry-platform/gateway/internal/ratelimit"
	"laundry-platform/shared/health"
	"laundry-platform/shared/httpmiddleware"
	"laundry-platform/shared/logging"
	"laundry-platform/shared/metrics"
	"laundry-platform/shared/redisutil"
)

const serviceName = "gateway"

func main() {
	cfg := config.Load()
	logger := logging.New(serviceName, parseLevel(cfg.LogLevel))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	redisClient, err := redisutil.NewClient(ctx, cfg.RedisAddr)
	if err != nil {
		logger.Warn("redis unavailable at startup; rate limiting fails open", slog.Any("error", err))
	}
	if redisClient != nil {
		defer redisClient.Close()
	}

	routes := proxy.Routes(cfg.IdentityUpstream, cfg.CoreUpstream, cfg.PaymentUpstream, cfg.ReportingUpstream)
	proxyHandler, err := proxy.NewHandler(routes)
	if err != nil {
		logger.Error("failed to build proxy routes", slog.Any("error", err))
		os.Exit(1)
	}

	m := metrics.New(serviceName)

	var handler http.Handler = proxyHandler
	handler = ratelimit.PerIP(redisClient, "gateway:", "default", 120, time.Minute)(handler)
	handler = authn.Authenticate(cfg.JWTSigningKey)(handler)
	handler = corsMiddleware(cfg.AllowedOrigin)(handler)
	handler = m.Middleware(handler)
	handler = httpmiddleware.AccessLog(logger)(handler)
	handler = httpmiddleware.Recover(logger)(handler)
	handler = httpmiddleware.RequestID(handler)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health.Livez)
	mux.HandleFunc("/readyz", health.Readyz(map[string]health.Check{
		"redis": func(ctx context.Context) error {
			if redisClient == nil {
				return nil // degrades rate limiting only, not readiness (see ratelimit package doc)
			}
			return redisClient.Ping(ctx).Err()
		},
	}))
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/docs", docsui.SwaggerUI)
	mux.HandleFunc("/docs/openapi.yaml", docsui.OpenAPISpec(cfg.OpenAPISpecPath))
	mux.Handle("/", handler)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("gateway listening", slog.String("port", cfg.Port))
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

// corsMiddleware allows only the configured origin (docs/12-security-baseline.md
// — no wildcard "*" in production).
func corsMiddleware(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key, X-Request-ID")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
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
