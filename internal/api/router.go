package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter creates and returns the fully configured chi router.
func NewRouter(h *Handlers) http.Handler {
	r := chi.NewRouter()

	// Global middleware stack.
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(RequestLogger(h.logger))
	r.Use(middleware.Recoverer)
	r.Use(CORSMiddleware)
	r.Use(middleware.Compress(5))

	// Health and metrics.
	r.Get("/health", h.HandleHealth)
	r.Handle("/metrics", promhttp.Handler())

	// SSE stream.
	r.Get("/events/stream", h.HandleSSEStream)

	// API v1.
	r.Route("/api/v1", func(r chi.Router) {
		// Event ingestion.
		r.Post("/events", h.HandleIngestEvent)
		r.Post("/events/batch", h.HandleIngestBatch)

		// Call history.
		r.Get("/calls", h.HandleGetCalls)

		// Metrics endpoints.
		r.Route("/metrics", func(r chi.Router) {
			r.Get("/summary", h.HandleGetSummary)
			r.Get("/calls-per-minute", h.HandleGetCallsPerMinute)
			r.Get("/latency-percentiles", h.HandleGetLatencyPercentiles)
			r.Get("/cost-by-provider", h.HandleGetCostByProvider)
			r.Get("/error-rates", h.HandleGetErrorRates)
			r.Get("/provider-comparison", h.HandleGetProviderComparison)
		})
	})

	return r
}
