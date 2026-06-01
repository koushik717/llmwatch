package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for the API server.
var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llmwatch_api_requests_total",
		Help: "Total HTTP requests by method, path, and status code.",
	}, []string{"method", "path", "status"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llmwatch_api_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	eventsIngestedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llmwatch_events_ingested_total",
		Help: "Total LLM call events ingested by provider.",
	}, []string{"provider", "status"})
)

// RequestLogger is a structured logging middleware using slog.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				duration := time.Since(start)
				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
				}

				logger.Info("http request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", status,
					"duration_ms", duration.Milliseconds(),
					"bytes", ww.BytesWritten(),
					"request_id", middleware.GetReqID(r.Context()),
					"remote_addr", r.RemoteAddr,
				)

				httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, http.StatusText(status)).Inc()
				httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration.Seconds())
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

// CORSMiddleware adds permissive CORS headers for the dashboard.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Request-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RecordEventIngested records a Prometheus metric for each ingested event.
func RecordEventIngested(provider, status string) {
	eventsIngestedTotal.WithLabelValues(provider, status).Inc()
}
