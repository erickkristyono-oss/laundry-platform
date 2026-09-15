// Package proxy implements the Gateway's routing table (docs/07-api-contract.md):
// each /api/v1/* prefix is reverse-proxied to the service that owns it,
// per docs/04-service-boundaries.md's "Exposes (sync API)" list. The
// Gateway is the only entry point for clients (docs/03-system-architecture.md §1)
// — no service is directly reachable from the internet.
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Route is one path-prefix -> upstream mapping.
type Route struct {
	Prefix   string
	Upstream string
}

// Routes is the full Gateway routing table, in the order routes must be
// matched (longest/most-specific prefix first, since two services never
// share a prefix per docs/04-service-boundaries.md's exclusive ownership rule
// — order only matters here for readability, not correctness).
func Routes(identity, core, payment, reporting string) []Route {
	return []Route{
		{Prefix: "/api/v1/auth", Upstream: identity},
		{Prefix: "/api/v1/customer-auth", Upstream: identity},
		{Prefix: "/api/v1/users", Upstream: identity},

		{Prefix: "/api/v1/customers", Upstream: core},
		{Prefix: "/api/v1/outlets", Upstream: core},
		{Prefix: "/api/v1/services", Upstream: core},
		{Prefix: "/api/v1/pricing", Upstream: core},
		{Prefix: "/api/v1/settings/tax-rate", Upstream: core},
		{Prefix: "/api/v1/orders", Upstream: core},
		{Prefix: "/api/v1/pickups", Upstream: core},
		{Prefix: "/api/v1/deliveries", Upstream: core},

		{Prefix: "/api/v1/payments", Upstream: payment},
		{Prefix: "/api/v1/refunds", Upstream: payment},

		{Prefix: "/api/v1/reports", Upstream: reporting},
	}
}

// NewHandler builds one http.Handler that dispatches by longest matching
// prefix to a ReverseProxy for that route's upstream.
func NewHandler(routes []Route) (http.Handler, error) {
	type compiled struct {
		prefix string
		proxy  *httputil.ReverseProxy
	}

	compiledRoutes := make([]compiled, 0, len(routes))
	for _, rt := range routes {
		target, err := url.Parse(rt.Upstream)
		if err != nil {
			return nil, err
		}
		compiledRoutes = append(compiledRoutes, compiled{
			prefix: rt.Prefix,
			proxy:  httputil.NewSingleHostReverseProxy(target),
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var best *compiled
		for i := range compiledRoutes {
			rt := &compiledRoutes[i]
			if len(rt.prefix) > 0 && hasPrefix(r.URL.Path, rt.prefix) {
				if best == nil || len(rt.prefix) > len(best.prefix) {
					best = rt
				}
			}
		}
		if best == nil {
			http.NotFound(w, r)
			return
		}
		best.proxy.ServeHTTP(w, r)
	}), nil
}

func hasPrefix(path, prefix string) bool {
	return len(path) >= len(prefix) && path[:len(prefix)] == prefix
}
