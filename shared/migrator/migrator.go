// Package migrator is a minimal, dependency-free SQL migration runner.
// Each service embeds its own migrations/*.sql files (via go:embed) and
// calls Up/Down here — this package itself contains no business logic and
// no per-service knowledge, so it is allowed in shared/ per
// docs/03-system-architecture.md §4.
//
// Migration files must be named "NNNN_description.up.sql" /
// "NNNN_description.down.sql" and are applied in ascending NNNN order.
// Applied "up" migrations are recorded in a per-database schema_migrations
// table so re-running Up is a no-op for already-applied versions.
package migrator

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type migration struct {
	version int
	name    string
	upSQL   string
	downSQL string
}

func load(migrations fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return nil, fmt.Errorf("migrator: read dir: %w", err)
	}

	byVersion := map[int]*migration{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		parts := strings.SplitN(name, "_", 2)
		if len(parts) != 2 {
			continue
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		content, err := fs.ReadFile(migrations, name)
		if err != nil {
			return nil, fmt.Errorf("migrator: read %s: %w", name, err)
		}
		m := byVersion[version]
		if m == nil {
			m = &migration{version: version, name: parts[1]}
			byVersion[version] = m
		}
		switch {
		case strings.HasSuffix(name, ".up.sql"):
			m.upSQL = string(content)
		case strings.HasSuffix(name, ".down.sql"):
			m.downSQL = string(content)
		}
	}

	out := make([]migration, 0, len(byVersion))
	for _, m := range byVersion {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func ensureSchemaMigrationsTable(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     INTEGER PRIMARY KEY,
			name        TEXT NOT NULL,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		)`)
	return err
}

// Up applies every migration in migrations that has not yet been recorded
// as applied, in ascending version order, each inside its own transaction.
func Up(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS) error {
	if err := ensureSchemaMigrationsTable(ctx, pool); err != nil {
		return fmt.Errorf("migrator: ensure schema_migrations: %w", err)
	}

	all, err := load(migrations)
	if err != nil {
		return err
	}

	for _, m := range all {
		var alreadyApplied bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, m.version).Scan(&alreadyApplied)
		if err != nil {
			return fmt.Errorf("migrator: check version %d: %w", m.version, err)
		}
		if alreadyApplied {
			continue
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("migrator: begin tx for %d: %w", m.version, err)
		}
		if _, err := tx.Exec(ctx, m.upSQL); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrator: apply %04d_%s: %w", m.version, m.name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.version, m.name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrator: record %d: %w", m.version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("migrator: commit %d: %w", m.version, err)
		}
	}

	return nil
}

// Down rolls back the single most-recently-applied migration.
func Down(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS) error {
	if err := ensureSchemaMigrationsTable(ctx, pool); err != nil {
		return fmt.Errorf("migrator: ensure schema_migrations: %w", err)
	}

	var version int
	err := pool.QueryRow(ctx, `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version)
	if err != nil {
		return fmt.Errorf("migrator: no applied migrations to roll back: %w", err)
	}

	all, err := load(migrations)
	if err != nil {
		return err
	}

	var target *migration
	for i := range all {
		if all[i].version == version {
			target = &all[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("migrator: no migration file found for applied version %d", version)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migrator: begin tx: %w", err)
	}
	if _, err := tx.Exec(ctx, target.downSQL); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("migrator: rollback %04d_%s: %w", target.version, target.name, err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, target.version); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("migrator: unrecord %d: %w", target.version, err)
	}
	return tx.Commit(ctx)
}
