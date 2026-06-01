package redis

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/llmwatch/llmwatch/internal/models"
	goredis "github.com/redis/go-redis/v9"
)

// Key constants for all Redis data structures used by LLMWatch.
const (
	// KeyCallsPerMinute is a sorted set where member=unix_minute, score=count.
	// We maintain a sliding window of the last 60 minutes.
	KeyCallsPerMinute = "llmwatch:calls:per_minute"

	// KeyLatencyFmt is a Redis list (key per provider+model) storing the last
	// 1000 latency values for percentile calculation.
	// Format: llmwatch:latency:{provider}:{model}
	KeyLatencyFmt = "llmwatch:latency:%s:%s"

	// KeyCostTotal is a hash keyed by provider, value is accumulated cost in USD.
	KeyCostTotal = "llmwatch:cost:total"

	// KeyDedupFmt is used to detect duplicate events (TTL 24h).
	// Format: llmwatch:dedup:{event_id}
	KeyDedupFmt = "llmwatch:dedup:%s"

	// DedupTTL is how long dedup keys are kept.
	DedupTTL = 24 * time.Hour

	// MaxLatencySamples is the maximum number of latency samples stored per model.
	MaxLatencySamples = 1000

	// CallsWindowMinutes is the sliding window size for calls/minute.
	CallsWindowMinutes = 60
)

// Cache is the interface for all Redis operations.
type Cache interface {
	// IsDuplicate checks if event_id has been seen before (and marks it).
	// Returns true if this is a duplicate (already seen).
	IsDuplicate(ctx context.Context, eventID string) (bool, error)

	// IncrCallsPerMinute records a call in the sliding window sorted set.
	IncrCallsPerMinute(ctx context.Context, ts time.Time) error

	// GetCallsPerMinute returns the calls per minute for the last N minutes.
	GetCallsPerMinute(ctx context.Context, minutes int) ([]*models.CallsPerMinutePoint, error)

	// AppendLatency adds a latency sample for a provider+model pair.
	AppendLatency(ctx context.Context, provider models.Provider, model string, latencyMS int64) error

	// GetLatencyPercentiles calculates p50/p95/p99 for all tracked models.
	GetLatencyPercentiles(ctx context.Context) ([]*models.LatencyPercentiles, error)

	// IncrCost adds the given cost to the provider's running total.
	IncrCost(ctx context.Context, provider models.Provider, costUSD float64) error

	// GetCostByProvider returns the total cost per provider.
	GetCostByProvider(ctx context.Context) ([]*models.CostByProvider, error)

	// HealthCheck verifies the Redis connection.
	HealthCheck(ctx context.Context) error

	// Close closes the Redis client.
	Close() error
}

// RedisCache is the concrete Redis-backed Cache implementation.
type RedisCache struct {
	client *goredis.Client
	logger *slog.Logger
}

// CacheConfig holds configuration for creating a RedisCache.
type CacheConfig struct {
	Addr     string
	Password string
	DB       int
	Logger   *slog.Logger
}

// NewRedisCache creates a new RedisCache.
func NewRedisCache(ctx context.Context, cfg CacheConfig) (*RedisCache, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
		MinIdleConns: 5,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping: %w", err)
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &RedisCache{client: client, logger: logger}, nil
}

// IsDuplicate checks and marks an event_id. Returns true if already seen.
func (c *RedisCache) IsDuplicate(ctx context.Context, eventID string) (bool, error) {
	key := fmt.Sprintf(KeyDedupFmt, eventID)
	// SetNX returns false (0) if key already exists.
	set, err := c.client.SetNX(ctx, key, "1", DedupTTL).Result()
	if err != nil {
		return false, fmt.Errorf("redis: dedup check: %w", err)
	}
	// set=true means key was newly created → NOT a duplicate.
	// set=false means key already existed → IS a duplicate.
	return !set, nil
}

// IncrCallsPerMinute increments the call counter for the current minute bucket.
// Uses a sorted set with score=unix_second and member=unix_minute_bucket.
func (c *RedisCache) IncrCallsPerMinute(ctx context.Context, ts time.Time) error {
	// Bucket to the minute.
	bucket := ts.Truncate(time.Minute).Unix()
	member := strconv.FormatInt(bucket, 10)

	pipe := c.client.Pipeline()
	pipe.ZIncrBy(ctx, KeyCallsPerMinute, 1, member)
	// Prune entries older than the window.
	cutoff := ts.Add(-time.Duration(CallsWindowMinutes) * time.Minute).Unix()
	pipe.ZRemRangeByScore(ctx, KeyCallsPerMinute, "-inf", strconv.FormatInt(cutoff, 10))
	// Expire the key itself after the window + buffer.
	pipe.Expire(ctx, KeyCallsPerMinute, time.Duration(CallsWindowMinutes+5)*time.Minute)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: incr calls per minute: %w", err)
	}
	return nil
}

// GetCallsPerMinute returns calls per minute for the last `minutes` minutes.
// Fills in zero for buckets with no calls.
func (c *RedisCache) GetCallsPerMinute(ctx context.Context, minutes int) ([]*models.CallsPerMinutePoint, error) {
	now := time.Now().Truncate(time.Minute)
	from := now.Add(-time.Duration(minutes) * time.Minute)

	// Fetch all members in the window.
	members, err := c.client.ZRangeWithScores(ctx, KeyCallsPerMinute,
		0, -1, // all
	).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: get calls per minute: %w", err)
	}

	// Build a map of bucket -> count.
	counts := make(map[int64]int64, len(members))
	for _, m := range members {
		bucket, _ := strconv.ParseInt(m.Member.(string), 10, 64)
		counts[bucket] = int64(m.Score)
	}

	// Build the time series with zeros for missing minutes.
	points := make([]*models.CallsPerMinutePoint, 0, minutes)
	for i := 0; i < minutes; i++ {
		t := from.Add(time.Duration(i) * time.Minute)
		bucket := t.Unix()
		points = append(points, &models.CallsPerMinutePoint{
			Minute:    t,
			CallCount: counts[bucket],
		})
	}
	return points, nil
}

// AppendLatency adds a latency sample for a provider+model pair.
// Keeps only the last MaxLatencySamples values.
func (c *RedisCache) AppendLatency(ctx context.Context, provider models.Provider, model string, latencyMS int64) error {
	key := fmt.Sprintf(KeyLatencyFmt, provider, sanitizeModel(model))
	val := strconv.FormatInt(latencyMS, 10)

	pipe := c.client.Pipeline()
	pipe.RPush(ctx, key, val)
	pipe.LTrim(ctx, key, -MaxLatencySamples, -1) // keep last N
	pipe.Expire(ctx, key, 25*time.Hour)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis: append latency: %w", err)
	}
	return nil
}

// GetLatencyPercentiles fetches p50/p95/p99 for all tracked provider+model pairs.
func (c *RedisCache) GetLatencyPercentiles(ctx context.Context) ([]*models.LatencyPercentiles, error) {
	// Scan for all latency keys.
	pattern := fmt.Sprintf(KeyLatencyFmt, "*", "*")
	keys, err := c.scanKeys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("redis: scan latency keys: %w", err)
	}

	var results []*models.LatencyPercentiles
	for _, key := range keys {
		// Parse provider and model from key: llmwatch:latency:{provider}:{model}
		provider, model, ok := parseLatencyKey(key)
		if !ok {
			continue
		}

		vals, err := c.client.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			c.logger.Warn("redis: failed to get latency list", "key", key, "error", err)
			continue
		}
		if len(vals) == 0 {
			continue
		}

		samples := make([]float64, 0, len(vals))
		for _, v := range vals {
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				samples = append(samples, f)
			}
		}
		sort.Float64s(samples)

		results = append(results, &models.LatencyPercentiles{
			Provider: models.Provider(provider),
			Model:    model,
			P50:      percentile(samples, 50),
			P95:      percentile(samples, 95),
			P99:      percentile(samples, 99),
		})
	}
	return results, nil
}

// IncrCost adds to the provider's running cost total.
func (c *RedisCache) IncrCost(ctx context.Context, provider models.Provider, costUSD float64) error {
	err := c.client.HIncrByFloat(ctx, KeyCostTotal, string(provider), costUSD).Err()
	if err != nil {
		return fmt.Errorf("redis: incr cost: %w", err)
	}
	return nil
}

// GetCostByProvider returns the accumulated cost per provider.
func (c *RedisCache) GetCostByProvider(ctx context.Context) ([]*models.CostByProvider, error) {
	vals, err := c.client.HGetAll(ctx, KeyCostTotal).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: get cost by provider: %w", err)
	}

	result := make([]*models.CostByProvider, 0, len(vals))
	for provider, costStr := range vals {
		cost, err := strconv.ParseFloat(costStr, 64)
		if err != nil {
			continue
		}
		result = append(result, &models.CostByProvider{
			Provider: models.Provider(provider),
			TotalUSD: cost,
		})
	}
	return result, nil
}

// HealthCheck pings Redis.
func (c *RedisCache) HealthCheck(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the Redis client.
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (c *RedisCache) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	return keys, iter.Err()
}

// percentile calculates the p-th percentile of a sorted slice using linear interpolation.
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}

	rank := (p / 100) * float64(n-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper {
		return sorted[lower]
	}
	return sorted[lower] + (rank-float64(lower))*(sorted[upper]-sorted[lower])
}

// sanitizeModel replaces characters that are invalid in Redis key segments.
func sanitizeModel(model string) string {
	result := make([]byte, len(model))
	for i := 0; i < len(model); i++ {
		c := model[i]
		if c == ':' || c == ' ' {
			result[i] = '_'
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// parseLatencyKey extracts provider and model from a latency key.
// Key format: llmwatch:latency:{provider}:{model}
func parseLatencyKey(key string) (provider, model string, ok bool) {
	// prefix is "llmwatch:latency:"
	const prefix = "llmwatch:latency:"
	if len(key) <= len(prefix) {
		return "", "", false
	}
	rest := key[len(prefix):]
	// Find the first colon separating provider from model.
	for i, c := range rest {
		if c == ':' {
			return rest[:i], rest[i+1:], true
		}
	}
	return "", "", false
}

// MockCache is an in-memory Cache implementation for unit tests.
type MockCache struct {
	Duplicates  map[string]bool
	LatencyData map[string][]float64
	CostData    map[string]float64
	CallCounts  map[int64]int64
}

// NewMockCache creates an empty MockCache.
func NewMockCache() *MockCache {
	return &MockCache{
		Duplicates:  make(map[string]bool),
		LatencyData: make(map[string][]float64),
		CostData:    make(map[string]float64),
		CallCounts:  make(map[int64]int64),
	}
}

func (m *MockCache) IsDuplicate(_ context.Context, eventID string) (bool, error) {
	if m.Duplicates[eventID] {
		return true, nil
	}
	m.Duplicates[eventID] = true
	return false, nil
}

func (m *MockCache) IncrCallsPerMinute(_ context.Context, ts time.Time) error {
	bucket := ts.Truncate(time.Minute).Unix()
	m.CallCounts[bucket]++
	return nil
}

func (m *MockCache) GetCallsPerMinute(_ context.Context, _ int) ([]*models.CallsPerMinutePoint, error) {
	return nil, nil
}

func (m *MockCache) AppendLatency(_ context.Context, provider models.Provider, model string, latencyMS int64) error {
	key := fmt.Sprintf("%s:%s", provider, model)
	m.LatencyData[key] = append(m.LatencyData[key], float64(latencyMS))
	return nil
}

func (m *MockCache) GetLatencyPercentiles(_ context.Context) ([]*models.LatencyPercentiles, error) {
	return nil, nil
}

func (m *MockCache) IncrCost(_ context.Context, provider models.Provider, costUSD float64) error {
	m.CostData[string(provider)] += costUSD
	return nil
}

func (m *MockCache) GetCostByProvider(_ context.Context) ([]*models.CostByProvider, error) {
	var result []*models.CostByProvider
	for p, cost := range m.CostData {
		result = append(result, &models.CostByProvider{
			Provider: models.Provider(p),
			TotalUSD: cost,
		})
	}
	return result, nil
}

func (m *MockCache) HealthCheck(_ context.Context) error { return nil }
func (m *MockCache) Close() error                        { return nil }
