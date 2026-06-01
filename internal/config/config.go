package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	APIPort         string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration

	// Kafka
	KafkaBrokers    []string
	KafkaTopic      string
	KafkaGroupID    string
	KafkaPartitions int

	// PostgreSQL
	DatabaseURL   string
	DBMaxConns    int32
	DBMinConns    int32
	DBMaxConnLife time.Duration

	// Redis
	RedisURL      string
	RedisPassword string
	RedisDB       int

	// Observability
	PrometheusPort string

	// Application
	Env      string
	LogLevel string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		APIPort:         getEnv("API_PORT", "8080"),
		ReadTimeout:     getDuration("READ_TIMEOUT", 30*time.Second),
		WriteTimeout:    getDuration("WRITE_TIMEOUT", 30*time.Second),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 30*time.Second),

		KafkaBrokers:    getEnvSlice("KAFKA_BROKERS", []string{"kafka:9092"}),
		KafkaTopic:      getEnv("KAFKA_TOPIC", "llm-events"),
		KafkaGroupID:    getEnv("KAFKA_GROUP_ID", "llm-metrics-consumer"),
		KafkaPartitions: getEnvInt("KAFKA_PARTITIONS", 3),

		DatabaseURL:   getEnvRequired("DATABASE_URL"),
		DBMaxConns:    int32(getEnvInt("DB_MAX_CONNS", 25)),
		DBMinConns:    int32(getEnvInt("DB_MIN_CONNS", 5)),
		DBMaxConnLife: getDuration("DB_MAX_CONN_LIFE", 5*time.Minute),

		RedisURL:      getEnv("REDIS_URL", "redis:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		PrometheusPort: getEnv("PROMETHEUS_PORT", "9090"),

		Env:      getEnv("APP_ENV", "development"),
		LogLevel: getEnv("LOG_LEVEL", "info"),
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvRequired(key string) string {
	val := os.Getenv(key)
	if val == "" {
		// Return empty string; callers validate.
		return ""
	}
	return val
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func getEnvSlice(key string, defaultVal []string) []string {
	if val := os.Getenv(key); val != "" {
		// Comma-separated list.
		result := []string{}
		start := 0
		for i := 0; i <= len(val); i++ {
			if i == len(val) || val[i] == ',' {
				part := val[start:i]
				if part != "" {
					result = append(result, part)
				}
				start = i + 1
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultVal
}

// Validate checks that required configuration is present.
func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if len(c.KafkaBrokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS is required")
	}
	return nil
}
