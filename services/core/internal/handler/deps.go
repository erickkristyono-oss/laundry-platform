// Package handler implements Core Service's HTTP handlers
// (docs/07-api-contract.md §4-10).
package handler

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"laundry-platform/services/core/internal/config"
)

type Handler struct {
	Pool   *pgxpool.Pool
	Cfg    config.Config
	Logger *slog.Logger
}

func New(pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) *Handler {
	return &Handler{Pool: pool, Cfg: cfg, Logger: logger}
}
