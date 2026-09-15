package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"laundry-platform/shared/security"
)

// EnsureDefaultOwner creates one OWNER account if the users table is
// completely empty, so a fresh local/dev environment has a way to log in
// at all. The generated password is logged once, loudly, and is never
// persisted anywhere else — this is a local-development bootstrap, not a
// production provisioning mechanism (production stands up its first OWNER
// out-of-band, e.g. via a one-off `make migrate-up`-style operator task).
func EnsureDefaultOwner(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return fmt.Errorf("bootstrap: count users: %w", err)
	}
	if count > 0 {
		return nil
	}

	const defaultEmail = "owner@laundryku.local"
	const defaultPassword = "ChangeMe123!"

	hash, err := security.HashPassword(defaultPassword)
	if err != nil {
		return fmt.Errorf("bootstrap: hash password: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx, `
		INSERT INTO users (email, full_name, password_hash)
		VALUES ($1, 'Default Owner', $2)
		RETURNING id
	`, defaultEmail, hash).Scan(&userID)
	if err != nil {
		return fmt.Errorf("bootstrap: insert user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE code = 'OWNER'
	`, userID); err != nil {
		return fmt.Errorf("bootstrap: assign OWNER role: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("bootstrap: commit: %w", err)
	}

	logger.Warn("bootstrapped default OWNER account for local development — change this password immediately in any shared environment",
		slog.String("email", defaultEmail),
		slog.String("password", defaultPassword),
	)
	return nil
}
