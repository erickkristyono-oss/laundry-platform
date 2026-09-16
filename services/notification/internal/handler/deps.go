// Package handler renders and delivers notifications from consumed events
// (docs/04-service-boundaries.md §5). Phase 2: WhatsApp only (UQ-08 leaves
// SMS/EMAIL/PUSH for later — the notification_deliveries.channel column
// already allows for them).
package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"laundry-platform/services/notification/internal/config"
	"laundry-platform/services/notification/internal/whatsapp"
)

type Handler struct {
	Pool   *pgxpool.Pool
	Cfg    config.Config
	Logger *slog.Logger
	HTTP   *http.Client
	Sender whatsapp.Sender
}

func New(pool *pgxpool.Pool, cfg config.Config, logger *slog.Logger) *Handler {
	return &Handler{
		Pool:   pool,
		Cfg:    cfg,
		Logger: logger,
		HTTP:   &http.Client{Timeout: 5 * time.Second},
		Sender: newSender(cfg, logger),
	}
}

func newSender(cfg config.Config, logger *slog.Logger) whatsapp.Sender {
	if cfg.FonnteToken == "" {
		return whatsapp.NoopSender{Logger: logger}
	}
	return whatsapp.FonnteSender{
		Token:  cfg.FonnteToken,
		APIURL: cfg.FonnteAPIURL,
		HTTP:   &http.Client{Timeout: 10 * time.Second},
	}
}
