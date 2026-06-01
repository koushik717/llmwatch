package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/llmwatch/llmwatch/internal/models"
	kafkago "github.com/segmentio/kafka-go"
)

// EventProducer is the interface for publishing LLM call events to Kafka.
// It is an interface to allow mocking in tests.
type EventProducer interface {
	Publish(ctx context.Context, event *models.LLMCallEvent) error
	Close() error
}

// KafkaProducer is the concrete Kafka-backed implementation of EventProducer.
type KafkaProducer struct {
	writer *kafkago.Writer
	logger *slog.Logger
	topic  string
}

// ProducerConfig holds the configuration for creating a KafkaProducer.
type ProducerConfig struct {
	Brokers []string
	Topic   string
	Logger  *slog.Logger
}

// NewProducer creates a new KafkaProducer.
func NewProducer(cfg ProducerConfig) (*KafkaProducer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka: at least one broker is required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("kafka: topic is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	w := &kafkago.Writer{
		Addr:                   kafkago.TCP(cfg.Brokers...),
		Topic:                  cfg.Topic,
		Balancer:               &kafkago.Hash{}, // partition by key (provider)
		RequiredAcks:           kafkago.RequireAll,
		MaxAttempts:            5,
		WriteBackoffMin:        100 * time.Millisecond,
		WriteBackoffMax:        1 * time.Second,
		BatchTimeout:           10 * time.Millisecond,
		Compression:            kafkago.Snappy,
		AllowAutoTopicCreation: true,
	}

	return &KafkaProducer{
		writer: w,
		logger: logger,
		topic:  cfg.Topic,
	}, nil
}

// Publish serialises the event to JSON and writes it to Kafka.
// The partition key is set to the provider name so each provider's events
// land on its own partition.
func (p *KafkaProducer) Publish(ctx context.Context, event *models.LLMCallEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("kafka producer: marshal event: %w", err)
	}

	msg := kafkago.Message{
		Key:   []byte(string(event.Provider)),
		Value: payload,
		Headers: []kafkago.Header{
			{Key: "event_id", Value: []byte(event.EventID)},
			{Key: "content-type", Value: []byte("application/json")},
		},
		Time: time.UnixMilli(event.Timestamp),
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka producer: write message: %w", err)
	}

	p.logger.Debug("event published to kafka",
		"event_id", event.EventID,
		"provider", event.Provider,
		"model", event.Model,
		"topic", p.topic,
	)
	return nil
}

// Close shuts down the Kafka writer gracefully.
func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}

// MockProducer is an in-memory EventProducer used in unit tests.
type MockProducer struct {
	Published []*models.LLMCallEvent
	Err       error
}

// Publish stores the event in memory (or returns the configured error).
func (m *MockProducer) Publish(_ context.Context, event *models.LLMCallEvent) error {
	if m.Err != nil {
		return m.Err
	}
	m.Published = append(m.Published, event)
	return nil
}

// Close is a no-op for the mock.
func (m *MockProducer) Close() error { return nil }
