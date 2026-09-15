// Package handler implements Payment Service's HTTP handlers
// (docs/07-api-contract.md §11).
package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"laundry-platform/services/payment/internal/config"
)

type Handler struct {
	Pool   *pgxpool.Pool
	Cfg    config.Config
	Logger *slog.Logger
	HTTP   *http.Client
}

func New(pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) *Handler {
	return &Handler{Pool: pool, Cfg: cfg, Logger: logger, HTTP: &http.Client{Timeout: 5 * time.Second}}
}
