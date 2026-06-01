package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/llmwatch/llmwatch/internal/api"
	"github.com/llmwatch/llmwatch/internal/config"
	"github.com/llmwatch/llmwatch/internal/db"
	"github.com/llmwatch/llmwatch/internal/kafka"
	redisclient "github.com/llmwatch/llmwatch/internal/redis"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "llmwatch-api: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// ── Logging ────────────────────────────────────────────────────────────
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting llmwatch API server")

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

	// Run migrations.
	logger.Info("running database migrations")
	if err := db.RunMigrations(ctx, cfg.DatabaseURL, logger); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Pre-create partitions for today and tomorrow.
	now := time.Now()
	for _, d := range []time.Time{now, now.Add(24 * time.Hour)} {
		if err := store.EnsurePartition(ctx, d); err != nil {
			logger.Warn("failed to ensure partition on startup", "date", d.Format("2006-01-02"), "error", err)
		}
	}

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

	// ── Kafka Producer ─────────────────────────────────────────────────────
	logger.Info("creating Kafka producer", "brokers", cfg.KafkaBrokers)
	producer, err := kafka.NewProducer(kafka.ProducerConfig{
		Brokers: cfg.KafkaBrokers,
		Topic:   cfg.KafkaTopic,
		Logger:  logger,
	})
	if err != nil {
		return fmt.Errorf("create kafka producer: %w", err)
	}
	defer producer.Close()

	// ── HTTP Server ────────────────────────────────────────────────────────
	handlers := api.NewHandlers(producer, store, cache, logger)
	router := api.NewRouter(handlers)

	srv := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	// Run the HTTP server in a goroutine.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("HTTP server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("http server: %w", err)
		}
	}()

	// Wait for shutdown signal or server error.
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	// Graceful shutdown with timeout.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	logger.Info("shutting down HTTP server", "timeout", cfg.ShutdownTimeout)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http server shutdown: %w", err)
	}

	logger.Info("llmwatch API server stopped")
	return nil
}
