package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/llmwatch/llmwatch/internal/config"
	"github.com/llmwatch/llmwatch/internal/db"
	kafkaclient "github.com/llmwatch/llmwatch/internal/kafka"
	"github.com/llmwatch/llmwatch/internal/metrics"
	"github.com/llmwatch/llmwatch/internal/models"
	redisclient "github.com/llmwatch/llmwatch/internal/redis"
)

// Prometheus metrics for the consumer.
var (
	eventsProcessedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llmwatch_consumer_events_processed_total",
		Help: "Total events processed by the Kafka consumer.",
	}, []string{"provider", "status", "result"})

	processingDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "llmwatch_consumer_processing_duration_seconds",
		Help:    "Time taken to process a single Kafka event.",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
	})

	duplicateEventsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "llmwatch_consumer_duplicate_events_total",
		Help: "Total duplicate events skipped.",
	})
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "llmwatch-consumer: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// ── Logging ────────────────────────────────────────────────────────────
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting llmwatch Kafka consumer")

	// ── Configuration ──────────────────────────────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// ── Context with shutdown signal ───────────────────────────────────────
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// ── PostgreSQL ─────────────────────────────────────────────────────────
	logger.Info("connecting to PostgreSQL")
	store, err := db.NewPostgresStore(ctx, db.StoreConfig{
		DatabaseURL: cfg.DatabaseURL,
		MaxConns:    cfg.DBMaxConns,
		MinConns:    cfg.DBMinConns,
		MaxConnLife: cfg.DBMaxConnLife,
		Logger:      logger,
	})
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer store.Close()

	// ── Redis ──────────────────────────────────────────────────────────────
	logger.Info("connecting to Redis")
	cache, err := redisclient.NewRedisCache(ctx, redisclient.CacheConfig{
		Addr:     cfg.RedisURL,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
		Logger:   logger,
	})
	if err != nil {
		return fmt.Errorf("connect to redis: %w", err)
	}
	defer cache.Close()

	// ── Metrics Processor ──────────────────────────────────────────────────
	processor := metrics.NewProcessor(store, cache, logger)

	// ── Kafka Consumer ─────────────────────────────────────────────────────
	logger.Info("creating Kafka consumer", "brokers", cfg.KafkaBrokers, "group", cfg.KafkaGroupID)
	consumer, err := kafkaclient.NewConsumer(kafkaclient.ConsumerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTopic,
		GroupID: cfg.KafkaGroupID,
		Logger:  logger,
	})
	if err != nil {
		return fmt.Errorf("create kafka consumer: %w", err)
	}
	defer consumer.Close()

	// ── Hourly Rollup Ticker ───────────────────────────────────────────────
	// Pre-create tomorrow's partition every hour.
	go runHourlyMaintenance(ctx, store, logger)

	// ── Consume ────────────────────────────────────────────────────────────
	logger.Info("starting Kafka consume loop")
	handler := buildHandler(processor, logger)
	if err := consumer.Consume(ctx, handler); err != nil {
		return fmt.Errorf("consumer error: %w", err)
	}

	logger.Info("llmwatch consumer stopped")
	return nil
}

// buildHandler wraps the metrics processor in the Kafka MessageHandler signature.
func buildHandler(processor *metrics.Processor, logger *slog.Logger) kafkaclient.MessageHandler {
	return func(ctx context.Context, event *models.LLMCallEvent) error {
		start := time.Now()

		result, err := processor.Process(ctx, event)
		duration := time.Since(start)
		processingDuration.Observe(duration.Seconds())

		if err != nil {
			logger.Error("processor error",
				"event_id", event.EventID,
				"error", err,
				"duration_ms", duration.Milliseconds(),
			)
			eventsProcessedTotal.WithLabelValues(string(event.Provider), string(event.Status), "error").Inc()
			return err
		}

		if result.Duplicate {
			duplicateEventsTotal.Inc()
			eventsProcessedTotal.WithLabelValues(string(event.Provider), string(event.Status), "duplicate").Inc()
			return nil
		}

		eventsProcessedTotal.WithLabelValues(string(event.Provider), string(event.Status), "ok").Inc()
		return nil
	}
}

// runHourlyMaintenance creates future partitions and runs other periodic tasks.
func runHourlyMaintenance(ctx context.Context, store db.Store, logger *slog.Logger) {
	// Run immediately on startup, then every hour.
	doMaintenance(ctx, store, logger)

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			doMaintenance(ctx, store, logger)
		}
	}
}

func doMaintenance(ctx context.Context, store db.Store, logger *slog.Logger) {
	now := time.Now()
	// Ensure partitions exist for today, tomorrow, and the day after.
	for _, offset := range []int{0, 1, 2} {
		d := now.Add(time.Duration(offset) * 24 * time.Hour)
		if err := store.EnsurePartition(ctx, d); err != nil {
			logger.Warn("maintenance: ensure partition failed",
				"date", d.Format("2006-01-02"),
				"error", err,
			)
		}
	}
}
