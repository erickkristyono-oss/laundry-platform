// Package metrics exposes the Prometheus HTTP metrics baseline required by
// docs/13-observability.md §5 (http_requests_total, http_request_duration_seconds),
// with a per-service constructor so metric labels stay consistent across
// services without duplicating registration code.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HTTPMetrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

// New registers the standard HTTP metrics for one service.
func New(serviceName string) *HTTPMetrics {
	return &HTTPMetrics{
		requestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "Total HTTP requests processed.",
			ConstLabels: prometheus.Labels{"service": serviceName},
		}, []string{"route", "method", "status"}),
		requestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "HTTP request duration in seconds.",
			ConstLabels: prometheus.Labels{"service": serviceName},
			Buckets:     prometheus.DefBuckets,
		}, []string{"route"}),
	}
}

// Middleware records requestsTotal/requestDuration for every request.
func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		route := r.URL.Path
		m.requestsTotal.WithLabelValues(route, r.Method, strconv.Itoa(sw.status)).Inc()
		m.requestDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
	})
}

// Handler returns the Prometheus scrape endpoint handler, mounted at
// /metrics per docs/13-observability.md §6.
func Handler() http.Handler {
	return promhttp.Handler()
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
