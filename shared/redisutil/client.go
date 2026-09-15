// Package redisutil provides a thin Redis client wrapper. Redis is never a
// system of record (docs/14-architecture-decisions.md ADR-005) — this
// package only connects and pings; every caller is responsible for
// tolerating Redis being unavailable without corrupting business data.
package redisutil

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewClient opens a Redis client against addr and verifies connectivity.
// keyPrefix should be the owning service's namespace (e.g. "identity:",
// "core:", "gateway:") per docs/14-architecture-decisions.md ADR-005's
// key-namespacing rule — callers are responsible for prefixing their own keys.
func NewClient(ctx context.Context, addr string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{Addr: addr})

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redisutil: ping: %w", err)
	}

	return client, nil
}
