// Command migrate applies or rolls back Identity Service migrations
// standalone (without starting the HTTP server) — used by CI's
// migration-check step and by operators for manual rollback.
//
// Usage: migrate <up|down>
package main

import (
	"context"
	"fmt"
	"os"

	"laundry-platform/services/identity/internal/config"
	"laundry-platform/services/identity/migrations"
	"laundry-platform/shared/migrator"
	"laundry-platform/shared/pgxutil"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "down") {
		fmt.Fprintln(os.Stderr, "usage: migrate <up|down>")
		os.Exit(2)
	}

	cfg := config.Load()
	ctx := context.Background()

	pool, err := pgxutil.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	switch os.Args[1] {
	case "up":
		err = migrator.Up(ctx, pool, migrations.FS)
	case "down":
		err = migrator.Down(ctx, pool, migrations.FS)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", os.Args[1], err)
		os.Exit(1)
	}
	fmt.Printf("migrate %s: ok\n", os.Args[1])
}
