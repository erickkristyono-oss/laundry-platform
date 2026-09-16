// Package handler implements Reporting Service's event consumers and
// read-only HTTP handlers (docs/07-api-contract.md §13).
package handler

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
}

func New(pool *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{Pool: pool, Logger: logger}
}
