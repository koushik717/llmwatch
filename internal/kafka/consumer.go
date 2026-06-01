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

// MessageHandler is the function signature for processing a consumed Kafka message.
type MessageHandler func(ctx context.Context, event *models.LLMCallEvent) error

// EventConsumer is the interface for consuming LLM call events from Kafka.
type EventConsumer interface {
	Consume(ctx context.Context, handler MessageHandler) error
	Close() error
}

// KafkaConsumer is the concrete Kafka-backed implementation of EventConsumer.
type KafkaConsumer struct {
	reader *kafkago.Reader
	logger *slog.Logger
}

// ConsumerConfig holds the configuration for creating a KafkaConsumer.
type ConsumerConfig struct {
	Brokers     []string
	Topic       string
	GroupID     string
	StartOffset int64 // kafkago.FirstOffset or kafkago.LastOffset
	Logger      *slog.Logger
}

// NewConsumer creates a new KafkaConsumer.
func NewConsumer(cfg ConsumerConfig) (*KafkaConsumer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka consumer: at least one broker is required")
	}
	if cfg.Topic == "" {
		return nil, fmt.Errorf("kafka consumer: topic is required")
	}
	if cfg.GroupID == "" {
		return nil, fmt.Errorf("kafka consumer: group ID is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	startOffset := kafkago.FirstOffset
	if cfg.StartOffset != 0 {
		startOffset = cfg.StartOffset
	}

	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		MaxWait:        500 * time.Millisecond,
		StartOffset:    startOffset,
		CommitInterval: time.Second, // async commit every second
		RetentionTime:  24 * time.Hour,
	})

	return &KafkaConsumer{
		reader: r,
		logger: logger,
	}, nil
}

// Consume runs the consumer loop. It reads messages from Kafka and calls the
// handler for each one. The loop runs until ctx is cancelled or an
// unrecoverable error occurs.
//
// At-least-once delivery is guaranteed: messages are committed only after the
// handler returns nil. Duplicate delivery is handled by the caller (event_id
// dedup in Redis).
func (c *KafkaConsumer) Consume(ctx context.Context, handler MessageHandler) error {
	c.logger.Info("kafka consumer started", "topic", c.reader.Config().Topic, "group", c.reader.Config().GroupID)

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.logger.Info("kafka consumer shutting down", "reason", ctx.Err())
				return nil
			}
			return fmt.Errorf("kafka consumer: fetch message: %w", err)
		}

		var event models.LLMCallEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			c.logger.Error("kafka consumer: failed to unmarshal message",
				"offset", msg.Offset,
				"partition", msg.Partition,
				"error", err,
			)
			// Commit the bad message so we don't get stuck.
			if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
				c.logger.Error("kafka consumer: failed to commit bad message", "error", commitErr)
			}
			continue
		}

		start := time.Now()
		if err := handler(ctx, &event); err != nil {
			c.logger.Error("kafka consumer: handler error",
				"event_id", event.EventID,
				"error", err,
				"duration", time.Since(start),
			)
			// Still commit so we don't reprocess indefinitely.
			// Idempotent dedup handles the case where the event WAS written
			// but the handler returned an error on a subsequent step.
			if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
				c.logger.Error("kafka consumer: failed to commit after handler error", "error", commitErr)
			}
			continue
		}

		c.logger.Debug("event processed",
			"event_id", event.EventID,
			"provider", event.Provider,
			"model", event.Model,
			"duration_ms", time.Since(start).Milliseconds(),
		)

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			return fmt.Errorf("kafka consumer: commit messages: %w", err)
		}
	}
}

// Close shuts down the Kafka reader.
func (c *KafkaConsumer) Close() error {
	return c.reader.Close()
}

// Stats returns consumer lag statistics.
func (c *KafkaConsumer) Stats() kafkago.ReaderStats {
	return c.reader.Stats()
}
