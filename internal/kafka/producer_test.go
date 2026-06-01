package kafka_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/llmwatch/llmwatch/internal/kafka"
	"github.com/llmwatch/llmwatch/internal/models"
)

func TestMockProducer_Publish_Success(t *testing.T) {
	mock := &kafka.MockProducer{}

	event := &models.LLMCallEvent{
		EventID:   "prod-test-001",
		Provider:  models.ProviderOpenAI,
		Model:     "gpt-4o-mini",
		LatencyMS: 300,
		Status:    models.StatusSuccess,
		Timestamp: time.Now().UnixMilli(),
	}

	if err := mock.Publish(context.Background(), event); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(mock.Published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(mock.Published))
	}
	if mock.Published[0].EventID != "prod-test-001" {
		t.Errorf("unexpected event_id: %s", mock.Published[0].EventID)
	}
}

func TestMockProducer_Publish_Error(t *testing.T) {
	expectedErr := context.DeadlineExceeded
	mock := &kafka.MockProducer{Err: expectedErr}

	event := &models.LLMCallEvent{
		EventID:  "prod-test-002",
		Provider: models.ProviderGemini,
		Model:    "gemini-2.0-flash",
		Status:   models.StatusSuccess,
	}

	err := mock.Publish(context.Background(), event)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != expectedErr {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
	if len(mock.Published) != 0 {
		t.Error("expected no events published on error")
	}
}

func TestMockProducer_Close(t *testing.T) {
	mock := &kafka.MockProducer{}
	if err := mock.Close(); err != nil {
		t.Errorf("unexpected error on Close: %v", err)
	}
}

func TestMockProducer_MultipleBatchPublish(t *testing.T) {
	mock := &kafka.MockProducer{}
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		event := &models.LLMCallEvent{
			EventID:   fmt.Sprintf("batch-event-%d", i),
			Provider:  models.ProviderAnthropic,
			Model:     "claude-3-5-sonnet-20241022",
			LatencyMS: int64(100 * (i + 1)),
			Status:    models.StatusSuccess,
			Timestamp: time.Now().UnixMilli(),
		}
		if err := mock.Publish(ctx, event); err != nil {
			t.Fatalf("publish %d: %v", i, err)
		}
	}

	if len(mock.Published) != 10 {
		t.Errorf("expected 10 published events, got %d", len(mock.Published))
	}
}
