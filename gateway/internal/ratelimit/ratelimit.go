// Package ratelimit implements the fixed-window Redis rate limiter required
// at the Gateway by docs/12-security-baseline.md (stricter limits on
// POST /auth/login, /customer-auth/login, /orders, /payments).
//
// Redis is never a system of record (ADR-005) — if Redis is unavailable,
// requests are allowed through rather than blocked, so a cache outage
// degrades security posture but never business availability.
package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"laundry-platform/shared/respond"
)

// PerIP rate-limits by client IP + route prefix: at most `limit` requests
// per `window`. `keyPrefix` should be "gateway:" per the key-namespacing
// rule in docs/14-architecture-decisions.md ADR-005.
func PerIP(client *redis.Client, keyPrefix, routeLabel string, limit int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if client == nil {
				// Redis unavailable — fail open (see package doc).
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r)
			key := fmt.Sprintf("%sratelimit:%s:%s", keyPrefix, routeLabel, ip)

			ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
			defer cancel()

			count, err := client.Incr(ctx, key).Result()
			if err != nil {
				next.ServeHTTP(w, r) // fail open
				return
			}
			if count == 1 {
				client.Expire(ctx, key, window)
			}
			if count > int64(limit) {
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				respond.Error(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests. Please try again later.")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}
