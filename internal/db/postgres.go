package db

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/llmwatch/llmwatch/internal/models"
)

// Store is the interface for all database operations.
// Using an interface enables mock implementations in tests.
type Store interface {
	// InsertEvent inserts a single LLM call event. Returns false if the event
	// already exists (idempotent — event_id is deduplicated at DB level too).
	InsertEvent(ctx context.Context, event *models.LLMCallEvent) (bool, error)

	// GetRecentCalls returns the most recent N calls ordered by timestamp DESC.
	GetRecentCalls(ctx context.Context, limit int) ([]*models.LLMCallEvent, error)

	// GetSummary returns aggregated summary metrics for the given duration.
	GetSummary(ctx context.Context, since time.Duration) (*models.SummaryMetrics, error)

	// GetErrorRates returns error rates per (provider, model) since the given time.
	GetErrorRates(ctx context.Context, since time.Duration) ([]*models.ErrorRateByModel, error)

	// GetProviderComparison returns comparative stats per provider.
	GetProviderComparison(ctx context.Context, since time.Duration) ([]*models.ProviderComparison, error)

	// UpsertHourlyMetrics upserts the hourly rollup for a given hour bucket.
	UpsertHourlyMetrics(ctx context.Context, metrics *HourlyMetrics) error

	// EnsurePartition creates a daily partition for the given date if it doesn't exist.
	EnsurePartition(ctx context.Context, date time.Time) error

	// HealthCheck verifies the database connection is live.
	HealthCheck(ctx context.Context) error

	// Close releases the connection pool.
	Close()
}

// HourlyMetrics represents a pre-aggregated hourly rollup record.
type HourlyMetrics struct {
	HourBucket        time.Time
	Provider          models.Provider
	Model             string
	TotalCalls        int64
	SuccessCalls      int64
	ErrorCalls        int64
	TotalCostUSD      float64
	TotalLatencyMS    int64
	MinLatencyMS      int64
	MaxLatencyMS      int64
	TotalInputTokens  int64
	TotalOutputTokens int64
}

// PostgresStore is the concrete PostgreSQL-backed Store implementation.
type PostgresStore struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
	partitions sync.Map
}

// StoreConfig holds configuration for creating a PostgresStore.
type StoreConfig struct {
	DatabaseURL string
	MaxConns    int32
	MinConns    int32
	MaxConnLife time.Duration
	Logger      *slog.Logger
}

// NewPostgresStore creates a new PostgresStore with a pgxpool connection pool.
func NewPostgresStore(ctx context.Context, cfg StoreConfig) (*PostgresStore, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	if cfg.MaxConnLife > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLife
	}
	poolCfg.ConnConfig.ConnectTimeout = 10 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &PostgresStore{pool: pool, logger: logger}, nil
}

// InsertEvent inserts an LLM call event into the llm_calls table.
// It returns (true, nil) on success, (false, nil) if the event_id already exists.
func (s *PostgresStore) InsertEvent(ctx context.Context, event *models.LLMCallEvent) (bool, error) {
	const q = `
		INSERT INTO llm_calls (
			event_id, provider, model, latency_ms, input_tokens, output_tokens,
			cost_usd, status, error_message, event_ts, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10
		)
		ON CONFLICT (event_id, created_at) DO NOTHING
		RETURNING id
	`

	eventTime := time.UnixMilli(event.Timestamp).UTC()
	var id int64
	err := s.pool.QueryRow(ctx, q,
		event.EventID,
		string(event.Provider),
		event.Model,
		event.LatencyMS,
		event.InputTokens,
		event.OutputTokens,
		event.CostUSD,
		string(event.Status),
		nullableString(event.ErrorMessage),
		eventTime,
	).Scan(&id)

	if err == pgx.ErrNoRows {
		// ON CONFLICT DO NOTHING triggered — duplicate event.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("db: insert event: %w", err)
	}
	return true, nil
}

// GetRecentCalls returns the N most recent calls.
func (s *PostgresStore) GetRecentCalls(ctx context.Context, limit int) ([]*models.LLMCallEvent, error) {
	const q = `
		SELECT event_id, provider, model, latency_ms, input_tokens, output_tokens,
		       cost_usd, status, COALESCE(error_message, ''), event_ts
		FROM llm_calls
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := s.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("db: get recent calls: %w", err)
	}
	defer rows.Close()

	var events []*models.LLMCallEvent
	for rows.Next() {
		var e models.LLMCallEvent
		var ts time.Time
		var provider, status string
		if err := rows.Scan(
			&e.EventID, &provider, &e.Model, &e.LatencyMS,
			&e.InputTokens, &e.OutputTokens, &e.CostUSD,
			&status, &e.ErrorMessage, &ts,
		); err != nil {
			return nil, fmt.Errorf("db: scan call row: %w", err)
		}
		e.Provider = models.Provider(provider)
		e.Status = models.Status(status)
		e.Timestamp = ts.UnixMilli()
		events = append(events, &e)
	}
	return events, rows.Err()
}

// GetSummary returns aggregated 24h metrics.
func (s *PostgresStore) GetSummary(ctx context.Context, since time.Duration) (*models.SummaryMetrics, error) {
	const q = `
		SELECT
			COUNT(*) as total_calls,
			COALESCE(SUM(cost_usd), 0) as total_cost,
			COALESCE(AVG(latency_ms), 0) as avg_latency,
			COALESCE(SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0), 0) as error_rate
		FROM llm_calls
		WHERE created_at >= NOW() - $1::interval
	`

	interval := fmt.Sprintf("%d seconds", int(since.Seconds()))
	var m models.SummaryMetrics
	err := s.pool.QueryRow(ctx, q, interval).Scan(
		&m.TotalCalls, &m.TotalCostUSD, &m.AvgLatencyMS, &m.ErrorRate,
	)
	if err != nil {
		return nil, fmt.Errorf("db: get summary: %w", err)
	}
	m.Period = since.String()
	return &m, nil
}

// GetErrorRates returns error rates per (provider, model).
func (s *PostgresStore) GetErrorRates(ctx context.Context, since time.Duration) ([]*models.ErrorRateByModel, error) {
	const q = `
		SELECT
			provider, model,
			COUNT(*) as total,
			SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as errors,
			COALESCE(SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0), 0) as error_rate
		FROM llm_calls
		WHERE created_at >= NOW() - $1::interval
		GROUP BY provider, model
		ORDER BY error_rate DESC
	`

	interval := fmt.Sprintf("%d seconds", int(since.Seconds()))
	rows, err := s.pool.Query(ctx, q, interval)
	if err != nil {
		return nil, fmt.Errorf("db: get error rates: %w", err)
	}
	defer rows.Close()

	var result []*models.ErrorRateByModel
	for rows.Next() {
		var r models.ErrorRateByModel
		var provider string
		if err := rows.Scan(&provider, &r.Model, &r.Total, &r.Errors, &r.ErrorRate); err != nil {
			return nil, fmt.Errorf("db: scan error rate row: %w", err)
		}
		r.Provider = models.Provider(provider)
		result = append(result, &r)
	}
	return result, rows.Err()
}

// GetProviderComparison returns comparative stats per provider.
func (s *PostgresStore) GetProviderComparison(ctx context.Context, since time.Duration) ([]*models.ProviderComparison, error) {
	const q = `
		SELECT
			provider,
			COUNT(*) as total_calls,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms,
			COALESCE(SUM(cost_usd), 0) as total_cost_usd,
			COALESCE(SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0), 0) as error_rate,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms), 0) as p95_latency_ms
		FROM llm_calls
		WHERE created_at >= NOW() - $1::interval
		GROUP BY provider
		ORDER BY total_calls DESC
	`

	interval := fmt.Sprintf("%d seconds", int(since.Seconds()))
	rows, err := s.pool.Query(ctx, q, interval)
	if err != nil {
		return nil, fmt.Errorf("db: get provider comparison: %w", err)
	}
	defer rows.Close()

	var result []*models.ProviderComparison
	for rows.Next() {
		var r models.ProviderComparison
		var provider string
		if err := rows.Scan(&provider, &r.TotalCalls, &r.AvgLatencyMS, &r.TotalCostUSD, &r.ErrorRate, &r.P95LatencyMS); err != nil {
			return nil, fmt.Errorf("db: scan provider comparison row: %w", err)
		}
		r.Provider = models.Provider(provider)
		result = append(result, &r)
	}
	return result, rows.Err()
}

// UpsertHourlyMetrics inserts or updates a pre-aggregated hourly row.
func (s *PostgresStore) UpsertHourlyMetrics(ctx context.Context, hm *HourlyMetrics) error {
	const q = `
		INSERT INTO llm_metrics_hourly (
			hour_bucket, provider, model, total_calls, success_calls, error_calls,
			total_cost_usd, total_latency_ms, min_latency_ms, max_latency_ms,
			total_input_tokens, total_output_tokens
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (hour_bucket, provider, model) DO UPDATE SET
			total_calls         = llm_metrics_hourly.total_calls + EXCLUDED.total_calls,
			success_calls       = llm_metrics_hourly.success_calls + EXCLUDED.success_calls,
			error_calls         = llm_metrics_hourly.error_calls + EXCLUDED.error_calls,
			total_cost_usd      = llm_metrics_hourly.total_cost_usd + EXCLUDED.total_cost_usd,
			total_latency_ms    = llm_metrics_hourly.total_latency_ms + EXCLUDED.total_latency_ms,
			min_latency_ms      = LEAST(llm_metrics_hourly.min_latency_ms, EXCLUDED.min_latency_ms),
			max_latency_ms      = GREATEST(llm_metrics_hourly.max_latency_ms, EXCLUDED.max_latency_ms),
			total_input_tokens  = llm_metrics_hourly.total_input_tokens + EXCLUDED.total_input_tokens,
			total_output_tokens = llm_metrics_hourly.total_output_tokens + EXCLUDED.total_output_tokens
	`

	_, err := s.pool.Exec(ctx, q,
		hm.HourBucket, string(hm.Provider), hm.Model,
		hm.TotalCalls, hm.SuccessCalls, hm.ErrorCalls,
		hm.TotalCostUSD, hm.TotalLatencyMS, hm.MinLatencyMS, hm.MaxLatencyMS,
		hm.TotalInputTokens, hm.TotalOutputTokens,
	)
	if err != nil {
		return fmt.Errorf("db: upsert hourly metrics: %w", err)
	}
	return nil
}

// EnsurePartition creates a day partition for the given date if it doesn't exist.
func (s *PostgresStore) EnsurePartition(ctx context.Context, date time.Time) error {
	// Use UTC dates for partition names.
	d := date.UTC().Truncate(24 * time.Hour)
	partitionName := fmt.Sprintf("llm_calls_%s", d.Format("2006_01_02"))
	
	if _, ok := s.partitions.Load(partitionName); ok {
		return nil // Already ensured in memory
	}

	from := d.Format("2006-01-02")
	to := d.Add(24 * time.Hour).Format("2006-01-02")

	q := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s
		PARTITION OF llm_calls
		FOR VALUES FROM ('%s') TO ('%s')
	`, partitionName, from, to)

	_, err := s.pool.Exec(ctx, q)
	if err != nil {
		return fmt.Errorf("db: ensure partition %s: %w", partitionName, err)
	}
	s.partitions.Store(partitionName, true)
	s.logger.Debug("partition ensured", "partition", partitionName, "from", from, "to", to)
	return nil
}

// HealthCheck pings the database.
func (s *PostgresStore) HealthCheck(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Close releases all connections in the pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// nullableString converts an empty string to nil for nullable DB columns.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
