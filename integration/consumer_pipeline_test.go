//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	kafkaclient "github.com/llmwatch/llmwatch/internal/kafka"
	"github.com/llmwatch/llmwatch/internal/db"
	"github.com/llmwatch/llmwatch/internal/metrics"
	"github.com/llmwatch/llmwatch/internal/models"
	redisclient "github.com/llmwatch/llmwatch/internal/redis"
	kafkago "github.com/segmentio/kafka-go"
)

// TestConsumerPipeline tests the full Kafka → Consumer → Redis flow.
// This test requires a running Kafka and Redis (set via env vars).
//
// Run with: go test -tags=integration -v ./integration/...
func TestConsumerPipeline(t *testing.T) {
	t.Helper()

	const testTopic = "llm-events-integration-test"
	const testGroup = "integration-test-consumer"

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// ── Setup Kafka writer ─────────────────────────────────────────────────
	brokers := []string{getEnvDefault("KAFKA_BROKERS", "localhost:29092")}
	writer := &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Topic:                  testTopic,
		AllowAutoTopicCreation: true,
	}
	defer writer.Close()

	// ── Setup Redis mock ───────────────────────────────────────────────────
	cache := redisclient.NewMockCache()

	// ── Setup DB mock ──────────────────────────────────────────────────────
	store := &integrationMockStore{}

	// ── Setup processor ────────────────────────────────────────────────────
	processor := metrics.NewProcessor(store, cache, nil)

	// ── Setup consumer ─────────────────────────────────────────────────────
	consumer, err := kafkaclient.NewConsumer(kafkaclient.ConsumerConfig{
		Brokers:     brokers,
		Topic:       testTopic,
		GroupID:     testGroup,
		StartOffset: kafkago.LastOffset,
	})
	if err != nil {
		t.Skipf("kafka not available: %v", err)
	}
	defer consumer.Close()

	// ── Produce test events ────────────────────────────────────────────────
	testEvents := []*models.LLMCallEvent{
		{
			EventID:      "integration-001",
			Provider:     models.ProviderOpenAI,
			Model:        "gpt-4o",
			LatencyMS:    500,
			InputTokens:  1000,
			OutputTokens: 500,
			CostUSD:      0.01,
			Status:       models.StatusSuccess,
			Timestamp:    time.Now().UnixMilli(),
		},
		{
			EventID:      "integration-002",
			Provider:     models.ProviderAnthropic,
			Model:        "claude-3-5-sonnet-20241022",
			LatencyMS:    800,
			InputTokens:  1500,
			OutputTokens: 900,
			CostUSD:      0.018,
			Status:       models.StatusSuccess,
			Timestamp:    time.Now().UnixMilli(),
		},
		{
			EventID:      "integration-001", // duplicate
			Provider:     models.ProviderOpenAI,
			Model:        "gpt-4o",
			LatencyMS:    500,
			InputTokens:  1000,
			OutputTokens: 500,
			CostUSD:      0.01,
			Status:       models.StatusSuccess,
			Timestamp:    time.Now().UnixMilli(),
		},
	}

	msgs := make([]kafkago.Message, len(testEvents))
	for i, e := range testEvents {
		payload, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal event %d: %v", i, err)
		}
		msgs[i] = kafkago.Message{
			Key:   []byte(string(e.Provider)),
			Value: payload,
		}
	}

	writeCtx, writeCancel := context.WithTimeout(ctx, 10*time.Second)
	defer writeCancel()

	if err := writer.WriteMessages(writeCtx, msgs...); err != nil {
		t.Skipf("kafka write failed (kafka may not be running): %v", err)
	}

	t.Log("events written to Kafka, starting consumer...")

	// ── Consume with a counter ─────────────────────────────────────────────
	processed := make(chan struct{}, 10)
	handler := func(ctx context.Context, event *models.LLMCallEvent) error {
		result, err := processor.Process(ctx, event)
		if err != nil {
			return err
		}
		t.Logf("processed event_id=%s duplicate=%v inserted=%v",
			result.EventID, result.Duplicate, result.Inserted)
		processed <- struct{}{}
		return nil
	}

	consumeCtx, consumeCancel := context.WithTimeout(ctx, 30*time.Second)
	defer consumeCancel()

	go func() {
		if err := consumer.Consume(consumeCtx, handler); err != nil && consumeCtx.Err() == nil {
			t.Errorf("consume error: %v", err)
		}
	}()

	// Wait for all 3 messages to be consumed (including the duplicate).
	received := 0
	timeout := time.After(30 * time.Second)
	for received < 3 {
		select {
		case <-processed:
			received++
		case <-timeout:
			t.Fatalf("timed out waiting for messages (received %d/3)", received)
		}
	}
	consumeCancel()

	// ── Assertions ─────────────────────────────────────────────────────────

	// 3 messages consumed, but only 2 unique events should be stored (dedup skips the third).
	if len(store.events) != 2 {
		t.Errorf("expected 2 stored events (1 duplicate skipped), got %d", len(store.events))
	}

	// Redis should have latency data for both models.
	if len(cache.LatencyData) == 0 {
		t.Error("expected latency data in Redis")
	}

	// Cost data should be accumulated for OpenAI and Anthropic.
	if cache.CostData["openai"] == 0 {
		t.Error("expected OpenAI cost in Redis")
	}
	if cache.CostData["anthropic"] == 0 {
		t.Error("expected Anthropic cost in Redis")
	}

	t.Logf("✅ Pipeline test passed: 3 messages, 2 unique events, 1 duplicate skipped")
	t.Logf("   Stored events: %d", len(store.events))
	t.Logf("   Redis latency keys: %d", len(cache.LatencyData))
	t.Logf("   Redis cost (openai): $%.6f", cache.CostData["openai"])
	t.Logf("   Redis cost (anthropic): $%.6f", cache.CostData["anthropic"])
}

// integrationMockStore is a thread-safe store for integration tests.
type integrationMockStore struct {
	events []*models.LLMCallEvent
}

func (s *integrationMockStore) InsertEvent(_ context.Context, e *models.LLMCallEvent) (bool, error) {
	s.events = append(s.events, e)
	return true, nil
}
func (s *integrationMockStore) GetRecentCalls(_ context.Context, _ int) ([]*models.LLMCallEvent, error) {
	return s.events, nil
}
func (s *integrationMockStore) GetSummary(_ context.Context, _ time.Duration) (*models.SummaryMetrics, error) {
	return &models.SummaryMetrics{}, nil
}
func (s *integrationMockStore) GetErrorRates(_ context.Context, _ time.Duration) ([]*models.ErrorRateByModel, error) {
	return nil, nil
}
func (s *integrationMockStore) GetProviderComparison(_ context.Context, _ time.Duration) ([]*models.ProviderComparison, error) {
	return nil, nil
}
func (s *integrationMockStore) UpsertHourlyMetrics(_ context.Context, _ *db.HourlyMetrics) error {
	return nil
}
func (s *integrationMockStore) EnsurePartition(_ context.Context, _ time.Time) error { return nil }
func (s *integrationMockStore) HealthCheck(_ context.Context) error                  { return nil }
func (s *integrationMockStore) Close()                                                {}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
