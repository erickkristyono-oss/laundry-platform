// Package handler implements Payment Service's HTTP handlers
// (docs/07-api-contract.md §11).
package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"laundry-platform/services/payment/internal/config"
	"laundry-platform/services/payment/internal/gateway"
)

type Handler struct {
	Pool     *pgxpool.Pool
	Cfg      config.Config
	Logger   *slog.Logger
	HTTP     *http.Client
	Provider gateway.Provider
}

func New(pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		Pool:     pool,
		Cfg:      cfg,
		Logger:   logger,
		HTTP:     &http.Client{Timeout: 5 * time.Second},
		Provider: newProvider(cfg),
	}
}

// newProvider selects the online-gateway implementation from config.
// "dummy" is the only one wired up today (see internal/gateway/dummy.go);
// a real provider (Midtrans, Xendit, ...) plugs in here behind the same
// gateway.Provider interface once its account/API keys exist.
func newProvider(cfg config.Config) gateway.Provider {
	switch cfg.PaymentProvider {
	default:
		return gateway.DummyProvider{WebBaseURL: cfg.WebBaseURL}
	}
}
