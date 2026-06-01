package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/llmwatch/llmwatch/internal/db"
	"github.com/llmwatch/llmwatch/internal/models"
	"github.com/llmwatch/llmwatch/internal/redis"
)

// Processor handles the core business logic for processing an LLM call event:
//  1. Deduplication (via Redis)
//  2. Persistent storage (PostgreSQL)
//  3. Real-time counters (Redis)
//
// It is the single unit that the Kafka consumer calls for each message.
type Processor struct {
	store  db.Store
	cache  redis.Cache
	logger *slog.Logger
}

// ProcessResult describes the outcome of processing a single event.
type ProcessResult struct {
	EventID   string
	Duplicate bool
	Inserted  bool
}

// NewProcessor creates a new Processor.
func NewProcessor(store db.Store, cache redis.Cache, logger *slog.Logger) *Processor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Processor{
		store:  store,
		cache:  cache,
		logger: logger,
	}
}

// Process handles a single LLM call event end-to-end.
// It is safe to call concurrently from multiple goroutines.
func (p *Processor) Process(ctx context.Context, event *models.LLMCallEvent) (*ProcessResult, error) {
	result := &ProcessResult{EventID: event.EventID}

	// 1. Validate and normalise.
	event.Normalize()
	if err := event.Validate(); err != nil {
		return nil, fmt.Errorf("processor: invalid event %s: %w", event.EventID, err)
	}

	// 2. Deduplication check via Redis.
	isDup, err := p.cache.IsDuplicate(ctx, event.EventID)
	if err != nil {
		// On Redis failure, log and continue (dedup is best-effort).
		p.logger.Warn("processor: dedup check failed, continuing",
			"event_id", event.EventID,
			"error", err,
		)
	} else if isDup {
		p.logger.Debug("processor: duplicate event skipped", "event_id", event.EventID)
		result.Duplicate = true
		return result, nil
	}

	// 3. Write to PostgreSQL.
	eventTime := time.UnixMilli(event.Timestamp)

	// Ensure the day partition exists before inserting.
	if err := p.store.EnsurePartition(ctx, eventTime); err != nil {
		// Log but don't fail — the default partition will catch it.
		p.logger.Warn("processor: ensure partition failed",
			"date", eventTime.Format("2006-01-02"),
			"error", err,
		)
	}

	inserted, err := p.store.InsertEvent(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("processor: insert event %s: %w", event.EventID, err)
	}
	result.Inserted = inserted

	// 4. Update Redis real-time counters (best-effort — don't fail on Redis errors).
	if redisErr := p.updateRealTimeCounters(ctx, event, eventTime); redisErr != nil {
		p.logger.Warn("processor: redis update failed",
			"event_id", event.EventID,
			"error", redisErr,
		)
	}

	p.logger.Debug("processor: event processed",
		"event_id", event.EventID,
		"provider", event.Provider,
		"model", event.Model,
		"inserted", inserted,
	)

	return result, nil
}

// updateRealTimeCounters updates all Redis counters for live dashboard data.
func (p *Processor) updateRealTimeCounters(ctx context.Context, event *models.LLMCallEvent, ts time.Time) error {
	// Calls per minute sliding window.
	if err := p.cache.IncrCallsPerMinute(ctx, ts); err != nil {
		return fmt.Errorf("incr calls per minute: %w", err)
	}

	// Latency sample for percentile calculation.
	if err := p.cache.AppendLatency(ctx, event.Provider, event.Model, event.LatencyMS); err != nil {
		return fmt.Errorf("append latency: %w", err)
	}

	// Cost accumulation by provider.
	if err := p.cache.IncrCost(ctx, event.Provider, event.CostUSD); err != nil {
		return fmt.Errorf("incr cost: %w", err)
	}

	return nil
}

// BuildHourlyRollup creates a HourlyMetrics record from a slice of events
// for a specific (hour, provider, model) combination.
// This is called by the consumer's hourly rollup job.
func BuildHourlyRollup(hourBucket time.Time, provider models.Provider, model string, events []*models.LLMCallEvent) *db.HourlyMetrics {
	hm := &db.HourlyMetrics{
		HourBucket:   hourBucket,
		Provider:     provider,
		Model:        model,
		MinLatencyMS: int64(^uint64(0) >> 1), // MaxInt64
	}

	for _, e := range events {
		hm.TotalCalls++
		if e.Status == models.StatusSuccess {
			hm.SuccessCalls++
		} else {
			hm.ErrorCalls++
		}
		hm.TotalCostUSD += e.CostUSD
		hm.TotalLatencyMS += e.LatencyMS
		hm.TotalInputTokens += int64(e.InputTokens)
		hm.TotalOutputTokens += int64(e.OutputTokens)

		if e.LatencyMS < hm.MinLatencyMS {
			hm.MinLatencyMS = e.LatencyMS
		}
		if e.LatencyMS > hm.MaxLatencyMS {
			hm.MaxLatencyMS = e.LatencyMS
		}
	}

	if hm.TotalCalls == 0 {
		hm.MinLatencyMS = 0
	}
	return hm
}
