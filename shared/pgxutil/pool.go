// Package pgxutil provides a thin, business-rule-free Postgres connection
// helper (allowed in shared/ per docs/03-system-architecture.md §4). It only
// opens a pool and checks liveness — no queries, no models.
package pgxutil

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pgx connection pool against dsn and verifies connectivity
// with a bounded ping, so misconfiguration fails fast at service startup
// rather than on the first request.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxutil: parse dsn: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgxutil: open pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgxutil: ping: %w", err)
	}

	return pool, nil
}
