package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/llmwatch/llmwatch/internal/models"
	"github.com/llmwatch/llmwatch/internal/redis"
)

func TestMockCache_IsDuplicate(t *testing.T) {
	cache := redis.NewMockCache()
	ctx := context.Background()

	// First call — should NOT be a duplicate.
	isDup, err := cache.IsDuplicate(ctx, "event-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isDup {
		t.Error("first occurrence should not be a duplicate")
	}

	// Second call with same ID — should be a duplicate.
	isDup, err = cache.IsDuplicate(ctx, "event-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isDup {
		t.Error("second occurrence should be a duplicate")
	}

	// Different ID — should not be a duplicate.
	isDup, err = cache.IsDuplicate(ctx, "event-002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isDup {
		t.Error("different event_id should not be a duplicate")
	}
}

func TestMockCache_IncrCallsPerMinute(t *testing.T) {
	cache := redis.NewMockCache()
	ctx := context.Background()
	now := time.Now()

	for i := 0; i < 5; i++ {
		if err := cache.IncrCallsPerMinute(ctx, now); err != nil {
			t.Fatalf("incr calls: %v", err)
		}
	}

	bucket := now.Truncate(time.Minute).Unix()
	if cache.CallCounts[bucket] != 5 {
		t.Errorf("expected 5 calls, got %d", cache.CallCounts[bucket])
	}
}

func TestMockCache_AppendLatency(t *testing.T) {
	cache := redis.NewMockCache()
	ctx := context.Background()

	latencies := []int64{100, 200, 300, 400, 500}
	for _, l := range latencies {
		if err := cache.AppendLatency(ctx, models.ProviderOpenAI, "gpt-4o", l); err != nil {
			t.Fatalf("append latency: %v", err)
		}
	}

	key := "openai:gpt-4o"
	if len(cache.LatencyData[key]) != 5 {
		t.Errorf("expected 5 latency samples, got %d", len(cache.LatencyData[key]))
	}
}

func TestMockCache_IncrCostAndGet(t *testing.T) {
	cache := redis.NewMockCache()
	ctx := context.Background()

	costs := []float64{0.01, 0.02, 0.005, 0.015}
	expected := 0.0
	for _, c := range costs {
		expected += c
		if err := cache.IncrCost(ctx, models.ProviderAnthropic, c); err != nil {
			t.Fatalf("incr cost: %v", err)
		}
	}

	costData, err := cache.GetCostByProvider(ctx)
	if err != nil {
		t.Fatalf("get cost: %v", err)
	}

	var total float64
	for _, c := range costData {
		if c.Provider == models.ProviderAnthropic {
			total = c.TotalUSD
		}
	}

	if total != expected {
		t.Errorf("expected total cost %.4f, got %.4f", expected, total)
	}
}

func TestMockCache_MultipleProviders(t *testing.T) {
	cache := redis.NewMockCache()
	ctx := context.Background()

	_ = cache.IncrCost(ctx, models.ProviderOpenAI, 1.0)
	_ = cache.IncrCost(ctx, models.ProviderAnthropic, 2.0)
	_ = cache.IncrCost(ctx, models.ProviderGemini, 0.5)

	costs, err := cache.GetCostByProvider(ctx)
	if err != nil {
		t.Fatalf("get cost: %v", err)
	}

	if len(costs) != 3 {
		t.Errorf("expected 3 providers, got %d", len(costs))
	}
}

func TestMockCache_HealthCheck(t *testing.T) {
	cache := redis.NewMockCache()
	if err := cache.HealthCheck(context.Background()); err != nil {
		t.Errorf("unexpected health check error: %v", err)
	}
}

func TestMockCache_Close(t *testing.T) {
	cache := redis.NewMockCache()
	if err := cache.Close(); err != nil {
		t.Errorf("unexpected close error: %v", err)
	}
}
