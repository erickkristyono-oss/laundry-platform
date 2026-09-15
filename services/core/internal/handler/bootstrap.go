package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureSeedCatalog seeds one outlet and one service+price if the catalog
// is completely empty, so a fresh local/dev environment can create an
// order immediately. Not a production data-loading mechanism — a real
// deployment's outlets/catalog come from the admin API.
func EnsureSeedCatalog(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	var outletCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM outlets`).Scan(&outletCount); err != nil {
		return fmt.Errorf("bootstrap: count outlets: %w", err)
	}
	if outletCount == 0 {
		outletID := uuid.New().String()
		if _, err := pool.Exec(ctx, `
			INSERT INTO outlets (id, code, name, address) VALUES ($1, 'OUT-001', 'Outlet Pusat', 'Jl. Contoh No. 1')
		`, outletID); err != nil {
			return fmt.Errorf("bootstrap: insert outlet: %w", err)
		}
		logger.Warn("bootstrapped default outlet for local development", slog.String("outlet_id", outletID))
	}

	var serviceCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM services`).Scan(&serviceCount); err != nil {
		return fmt.Errorf("bootstrap: count services: %w", err)
	}
	if serviceCount == 0 {
		serviceID := uuid.New().String()
		if _, err := pool.Exec(ctx, `
			INSERT INTO services (id, code, name, unit) VALUES ($1, 'CUCI-KILOAN', 'Cuci Kiloan', 'kg')
		`, serviceID); err != nil {
			return fmt.Errorf("bootstrap: insert service: %w", err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO service_prices (service_id, price_per_unit) VALUES ($1, 7000)
		`, serviceID); err != nil {
			return fmt.Errorf("bootstrap: insert price: %w", err)
		}
		logger.Warn("bootstrapped default service+price for local development", slog.String("service_id", serviceID))
	}

	return nil
}
