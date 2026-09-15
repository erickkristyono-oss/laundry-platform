// Package handler implements Identity Service's HTTP handlers
// (docs/07-api-contract.md §1-3).
package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"laundry-platform/services/identity/internal/config"
)

// Handler holds every dependency the auth/user handlers need. A single
// struct (rather than free functions closing over globals) keeps
// construction explicit and testable.
type Handler struct {
	Pool   *pgxpool.Pool
	Cfg    config.Config
	Logger *slog.Logger
	HTTP   *http.Client
}

func New(pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		Pool:   pool,
		Cfg:    cfg,
		Logger: logger,
		HTTP:   &http.Client{Timeout: 5 * time.Second},
	}
}
