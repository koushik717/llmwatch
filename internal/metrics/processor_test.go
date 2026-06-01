package metrics_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/llmwatch/llmwatch/internal/db"
	"github.com/llmwatch/llmwatch/internal/metrics"
	"github.com/llmwatch/llmwatch/internal/models"
	redismock "github.com/llmwatch/llmwatch/internal/redis"
)

// ── mockStore implements db.Store for testing ─────────────────────────────

type mockStore struct {
	insertedEvents []*models.LLMCallEvent
	insertErr      error
	partitions     []time.Time
}

func (m *mockStore) InsertEvent(_ context.Context, event *models.LLMCallEvent) (bool, error) {
	if m.insertErr != nil {
		return false, m.insertErr
	}
	m.insertedEvents = append(m.insertedEvents, event)
	return true, nil
}
func (m *mockStore) GetRecentCalls(_ context.Context, _ int) ([]*models.LLMCallEvent, error) {
	return m.insertedEvents, nil
}
func (m *mockStore) GetSummary(_ context.Context, _ time.Duration) (*models.SummaryMetrics, error) {
	return &models.SummaryMetrics{}, nil
}
func (m *mockStore) GetErrorRates(_ context.Context, _ time.Duration) ([]*models.ErrorRateByModel, error) {
	return nil, nil
}
func (m *mockStore) GetProviderComparison(_ context.Context, _ time.Duration) ([]*models.ProviderComparison, error) {
	return nil, nil
}
func (m *mockStore) UpsertHourlyMetrics(_ context.Context, _ *db.HourlyMetrics) error {
	return nil
}
func (m *mockStore) EnsurePartition(_ context.Context, date time.Time) error {
	m.partitions = append(m.partitions, date)
	return nil
}
func (m *mockStore) HealthCheck(_ context.Context) error { return nil }
func (m *mockStore) Close()                              {}

// ── Tests ─────────────────────────────────────────────────────────────────

func TestProcessor_Process_NewEvent(t *testing.T) {
	store := &mockStore{}
	cache := redismock.NewMockCache()
	p := metrics.NewProcessor(store, cache, nil)

	event := &models.LLMCallEvent{
		EventID:      "test-event-001",
		Provider:     models.ProviderOpenAI,
		Model:        "gpt-4o",
		LatencyMS:    500,
		InputTokens:  1000,
		OutputTokens: 500,
		Status:       models.StatusSuccess,
		Timestamp:    time.Now().UnixMilli(),
	}

	result, err := p.Process(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Duplicate {
		t.Error("expected event not to be a duplicate")
	}
	if !result.Inserted {
		t.Error("expected event to be inserted")
	}
	if len(store.insertedEvents) != 1 {
		t.Errorf("expected 1 inserted event, got %d", len(store.insertedEvents))
	}
}

func TestProcessor_Process_Duplicate(t *testing.T) {
	store := &mockStore{}
	cache := redismock.NewMockCache()
	p := metrics.NewProcessor(store, cache, nil)

	event := &models.LLMCallEvent{
		EventID:   "dup-event-001",
		Provider:  models.ProviderAnthropic,
		Model:     "claude-3-5-sonnet-20241022",
		LatencyMS: 800,
		Status:    models.StatusSuccess,
		Timestamp: time.Now().UnixMilli(),
	}

	// First call — should succeed.
	result1, err := p.Process(context.Background(), event)
	if err != nil {
		t.Fatalf("first process: %v", err)
	}
	if result1.Duplicate {
		t.Error("first call should not be duplicate")
	}

	// Second call with same event_id — should be a duplicate.
	result2, err := p.Process(context.Background(), event)
	if err != nil {
		t.Fatalf("second process: %v", err)
	}
	if !result2.Duplicate {
		t.Error("second call should be duplicate")
	}

	// Only one insertion should have happened.
	if len(store.insertedEvents) != 1 {
		t.Errorf("expected 1 inserted event, got %d", len(store.insertedEvents))
	}
}

func TestProcessor_Process_RedisCountersUpdated(t *testing.T) {
	store := &mockStore{}
	cache := redismock.NewMockCache()
	p := metrics.NewProcessor(store, cache, nil)

	event := &models.LLMCallEvent{
		EventID:      "redis-test-001",
		Provider:     models.ProviderGemini,
		Model:        "gemini-2.0-flash",
		LatencyMS:    250,
		InputTokens:  700,
		OutputTokens: 400,
		CostUSD:      0.000172,
		Status:       models.StatusSuccess,
		Timestamp:    time.Now().UnixMilli(),
	}

	if _, err := p.Process(context.Background(), event); err != nil {
		t.Fatalf("process: %v", err)
	}

	// Latency should be recorded.
	key := "gemini:gemini-2.0-flash"
	if len(cache.LatencyData[key]) == 0 {
		t.Error("expected latency data to be stored in Redis")
	}
	if cache.LatencyData[key][0] != 250 {
		t.Errorf("expected latency 250, got %v", cache.LatencyData[key][0])
	}

	// Cost should be accumulated.
	if cache.CostData["gemini"] == 0 {
		t.Error("expected cost to be accumulated in Redis")
	}
}

func TestProcessor_Process_InvalidEvent(t *testing.T) {
	store := &mockStore{}
	cache := redismock.NewMockCache()
	p := metrics.NewProcessor(store, cache, nil)

	// Missing required fields.
	event := &models.LLMCallEvent{
		EventID: "invalid-001",
		// Provider and Model are missing
		Status:    models.StatusSuccess,
		Timestamp: time.Now().UnixMilli(),
	}

	_, err := p.Process(context.Background(), event)
	if err == nil {
		t.Error("expected error for invalid event, got nil")
	}
}

func TestBuildHourlyRollup(t *testing.T) {
	now := time.Now().Truncate(time.Hour)
	events := []*models.LLMCallEvent{
		{Provider: models.ProviderOpenAI, Model: "gpt-4o", LatencyMS: 500, CostUSD: 0.01, Status: models.StatusSuccess, InputTokens: 1000, OutputTokens: 500},
		{Provider: models.ProviderOpenAI, Model: "gpt-4o", LatencyMS: 800, CostUSD: 0.02, Status: models.StatusError, InputTokens: 1200, OutputTokens: 0},
		{Provider: models.ProviderOpenAI, Model: "gpt-4o", LatencyMS: 300, CostUSD: 0.005, Status: models.StatusSuccess, InputTokens: 800, OutputTokens: 400},
	}

	rollup := metrics.BuildHourlyRollup(now, models.ProviderOpenAI, "gpt-4o", events)

	if rollup.TotalCalls != 3 {
		t.Errorf("expected 3 total calls, got %d", rollup.TotalCalls)
	}
	if rollup.SuccessCalls != 2 {
		t.Errorf("expected 2 success calls, got %d", rollup.SuccessCalls)
	}
	if rollup.ErrorCalls != 1 {
		t.Errorf("expected 1 error call, got %d", rollup.ErrorCalls)
	}
	expectedCost := 0.01 + 0.02 + 0.005
	if math.Abs(rollup.TotalCostUSD-expectedCost) > 1e-9 {
		t.Errorf("expected total cost %.3f, got %.3f", expectedCost, rollup.TotalCostUSD)
	}
	if rollup.MinLatencyMS != 300 {
		t.Errorf("expected min latency 300, got %d", rollup.MinLatencyMS)
	}
	if rollup.MaxLatencyMS != 800 {
		t.Errorf("expected max latency 800, got %d", rollup.MaxLatencyMS)
	}
}

// ── Cost calculation tests ─────────────────────────────────────────────────

func TestCalculateCost_OpenAI_GPT4o(t *testing.T) {
	// gpt-4o: $5/M input, $15/M output
	cost := models.CalculateCost(models.ProviderOpenAI, "gpt-4o", 1_000_000, 1_000_000)
	expected := 5.0 + 15.0 // $20
	if cost != expected {
		t.Errorf("expected $%.2f, got $%.2f", expected, cost)
	}
}

func TestCalculateCost_Anthropic_ClaudeHaiku(t *testing.T) {
	// claude-3-haiku: $0.25/M input, $1.25/M output
	cost := models.CalculateCost(models.ProviderAnthropic, "claude-3-haiku-20240307", 1_000_000, 1_000_000)
	expected := 0.25 + 1.25
	if cost != expected {
		t.Errorf("expected $%.2f, got $%.2f", expected, cost)
	}
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	cost := models.CalculateCost(models.ProviderOpenAI, "gpt-4o", 0, 0)
	if cost != 0 {
		t.Errorf("expected $0 for zero tokens, got $%.8f", cost)
	}
}
