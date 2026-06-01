package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/llmwatch/llmwatch/internal/db"
	"github.com/llmwatch/llmwatch/internal/kafka"
	"github.com/llmwatch/llmwatch/internal/models"
	"github.com/llmwatch/llmwatch/internal/redis"
)

// Handlers holds all HTTP handler dependencies.
type Handlers struct {
	producer kafka.EventProducer
	store    db.Store
	cache    redis.Cache
	logger   *slog.Logger
	// SSE broadcaster
	sseMu      sync.RWMutex
	sseClients map[chan []byte]struct{}
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(
	producer kafka.EventProducer,
	store db.Store,
	cache redis.Cache,
	logger *slog.Logger,
) *Handlers {
	return &Handlers{
		producer:   producer,
		store:      store,
		cache:      cache,
		logger:     logger,
		sseClients: make(map[chan []byte]struct{}),
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return // headers already sent, nothing we can do
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ── Health ────────────────────────────────────────────────────────────────────

// HandleHealth returns a simple liveness probe.
func (h *Handlers) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := "ok"
	code := http.StatusOK

	checks := map[string]string{}
	if err := h.store.HealthCheck(ctx); err != nil {
		checks["postgres"] = err.Error()
		status = "degraded"
		code = http.StatusServiceUnavailable
	} else {
		checks["postgres"] = "ok"
	}

	if err := h.cache.HealthCheck(ctx); err != nil {
		checks["redis"] = err.Error()
		status = "degraded"
		code = http.StatusServiceUnavailable
	} else {
		checks["redis"] = "ok"
	}

	writeJSON(w, code, map[string]any{
		"status": status,
		"checks": checks,
		"time":   time.Now().UTC(),
	})
}

// ── Ingest ────────────────────────────────────────────────────────────────────

// HandleIngestEvent accepts a single LLM call event and publishes it to Kafka.
func (h *Handlers) HandleIngestEvent(w http.ResponseWriter, r *http.Request) {
	var event models.LLMCallEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %s", err))
		return
	}

	event.Normalize()
	if err := event.Validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	if err := h.producer.Publish(r.Context(), &event); err != nil {
		h.logger.Error("failed to publish event to kafka", "event_id", event.EventID, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to queue event")
		return
	}

	// Broadcast to SSE subscribers immediately (before consumer processes it).
	h.broadcastSSE(&event)

	writeJSON(w, http.StatusAccepted, map[string]string{"event_id": event.EventID})
}

// HandleIngestBatch accepts a batch of LLM call events.
func (h *Handlers) HandleIngestBatch(w http.ResponseWriter, r *http.Request) {
	var events []models.LLMCallEvent
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %s", err))
		return
	}
	if len(events) > 1000 {
		writeError(w, http.StatusRequestEntityTooLarge, "batch size exceeds 1000")
		return
	}

	var accepted []string
	var failed []string
	for i := range events {
		events[i].Normalize()
		if err := events[i].Validate(); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %s", events[i].EventID, err))
			continue
		}
		if err := h.producer.Publish(r.Context(), &events[i]); err != nil {
			h.logger.Error("batch: failed to publish", "event_id", events[i].EventID, "error", err)
			failed = append(failed, events[i].EventID)
			continue
		}
		h.broadcastSSE(&events[i])
		accepted = append(accepted, events[i].EventID)
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"accepted": len(accepted),
		"failed":   len(failed),
		"details":  failed,
	})
}

// ── Metrics ───────────────────────────────────────────────────────────────────

// HandleGetSummary returns the 24h aggregated summary.
func (h *Handlers) HandleGetSummary(w http.ResponseWriter, r *http.Request) {
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if n, err := strconv.Atoi(hoursStr); err == nil && n > 0 && n <= 168 {
			hours = n
		}
	}

	summary, err := h.store.GetSummary(r.Context(), time.Duration(hours)*time.Hour)
	if err != nil {
		h.logger.Error("get summary failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch summary")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// HandleGetCallsPerMinute returns the calls/minute time series from Redis.
func (h *Handlers) HandleGetCallsPerMinute(w http.ResponseWriter, r *http.Request) {
	minutesStr := r.URL.Query().Get("minutes")
	minutes := 60
	if minutesStr != "" {
		if n, err := strconv.Atoi(minutesStr); err == nil && n > 0 && n <= 1440 {
			minutes = n
		}
	}

	points, err := h.cache.GetCallsPerMinute(r.Context(), minutes)
	if err != nil {
		h.logger.Error("get calls per minute failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch calls per minute")
		return
	}
	writeJSON(w, http.StatusOK, points)
}

// HandleGetLatencyPercentiles returns p50/p95/p99 per model from Redis.
func (h *Handlers) HandleGetLatencyPercentiles(w http.ResponseWriter, r *http.Request) {
	percentiles, err := h.cache.GetLatencyPercentiles(r.Context())
	if err != nil {
		h.logger.Error("get latency percentiles failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch latency percentiles")
		return
	}
	if percentiles == nil {
		percentiles = []*models.LatencyPercentiles{}
	}
	writeJSON(w, http.StatusOK, percentiles)
}

// HandleGetCostByProvider returns accumulated cost per provider from Redis.
func (h *Handlers) HandleGetCostByProvider(w http.ResponseWriter, r *http.Request) {
	costs, err := h.cache.GetCostByProvider(r.Context())
	if err != nil {
		h.logger.Error("get cost by provider failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch cost data")
		return
	}
	if costs == nil {
		costs = []*models.CostByProvider{}
	}
	writeJSON(w, http.StatusOK, costs)
}

// HandleGetErrorRates returns error rates per model from PostgreSQL.
func (h *Handlers) HandleGetErrorRates(w http.ResponseWriter, r *http.Request) {
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if n, err := strconv.Atoi(hoursStr); err == nil && n > 0 {
			hours = n
		}
	}

	rates, err := h.store.GetErrorRates(r.Context(), time.Duration(hours)*time.Hour)
	if err != nil {
		h.logger.Error("get error rates failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch error rates")
		return
	}
	if rates == nil {
		rates = []*models.ErrorRateByModel{}
	}
	writeJSON(w, http.StatusOK, rates)
}

// HandleGetCalls returns the N most recent calls.
func (h *Handlers) HandleGetCalls(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	calls, err := h.store.GetRecentCalls(r.Context(), limit)
	if err != nil {
		h.logger.Error("get calls failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch calls")
		return
	}
	if calls == nil {
		calls = []*models.LLMCallEvent{}
	}
	writeJSON(w, http.StatusOK, calls)
}

// HandleGetProviderComparison returns comparative stats per provider.
func (h *Handlers) HandleGetProviderComparison(w http.ResponseWriter, r *http.Request) {
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if n, err := strconv.Atoi(hoursStr); err == nil && n > 0 {
			hours = n
		}
	}

	comparisons, err := h.store.GetProviderComparison(r.Context(), time.Duration(hours)*time.Hour)
	if err != nil {
		h.logger.Error("get provider comparison failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch provider comparison")
		return
	}
	if comparisons == nil {
		comparisons = []*models.ProviderComparison{}
	}
	writeJSON(w, http.StatusOK, comparisons)
}

// ── SSE ───────────────────────────────────────────────────────────────────────

// HandleSSEStream sends a stream of LLMCallEvent updates to the client.
func (h *Handlers) HandleSSEStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "SSE not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Nginx: disable buffering

	ch := make(chan []byte, 64)
	h.registerSSEClient(ch)
	defer h.unregisterSSEClient(ch)

	// Send initial ping so the client knows the connection is live.
	fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
	flusher.Flush()

	// Send a keepalive every 15 seconds.
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: llm-call\ndata: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}

func (h *Handlers) registerSSEClient(ch chan []byte) {
	h.sseMu.Lock()
	h.sseClients[ch] = struct{}{}
	h.sseMu.Unlock()
}

func (h *Handlers) unregisterSSEClient(ch chan []byte) {
	h.sseMu.Lock()
	delete(h.sseClients, ch)
	close(ch)
	h.sseMu.Unlock()
}

func (h *Handlers) broadcastSSE(event *models.LLMCallEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.sseMu.RLock()
	defer h.sseMu.RUnlock()

	for ch := range h.sseClients {
		select {
		case ch <- data:
		default:
			// Slow subscriber — drop message rather than block.
		}
	}
}

// BroadcastEvent is exported so the consumer can also push events to SSE.
func (h *Handlers) BroadcastEvent(ctx context.Context, event *models.LLMCallEvent) {
	h.broadcastSSE(event)
}
