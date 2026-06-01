package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/llmwatch/llmwatch/internal/models"
)

// modelDef represents a model that the simulator can generate events for.
type modelDef struct {
	provider     models.Provider
	model        string
	avgLatencyMS int64
	jitterMS     int64
	avgInputTok  int
	avgOutputTok int
	errorRate    float64 // 0.0-1.0
}

// modelPool defines the pool of realistic LLM models to simulate.
var modelPool = []modelDef{
	{models.ProviderOpenAI, "gpt-4o", 800, 300, 1200, 800, 0.02},
	{models.ProviderOpenAI, "gpt-4o-mini", 350, 150, 900, 500, 0.01},
	{models.ProviderOpenAI, "gpt-3.5-turbo", 200, 100, 600, 300, 0.015},
	{models.ProviderAnthropic, "claude-3-5-sonnet-20241022", 1100, 400, 1500, 900, 0.02},
	{models.ProviderAnthropic, "claude-3-haiku-20240307", 400, 200, 800, 400, 0.01},
	{models.ProviderAnthropic, "claude-3-opus-20240229", 2000, 600, 2000, 1200, 0.03},
	{models.ProviderGemini, "gemini-2.0-flash", 250, 100, 700, 400, 0.01},
	{models.ProviderGemini, "gemini-1.5-pro", 900, 300, 1300, 700, 0.02},
}

func main() {
	// CLI flags.
	apiURL := flag.String("api", getEnv("LLMWATCH_API_URL", "http://localhost:8080"), "LLMWatch API URL")
	rps := flag.Float64("rps", 2.0, "events per second to generate")
	duration := flag.Duration("duration", 0, "how long to run (0 = forever)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("llmwatch simulator starting",
		"api_url", *apiURL,
		"rps", *rps,
		"duration", *duration,
	)

	client := &http.Client{Timeout: 10 * time.Second}

	// Shutdown signal.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Optional duration cutoff.
	if *duration > 0 {
		go func() {
			time.Sleep(*duration)
			cancel()
		}()
	}

	interval := time.Duration(float64(time.Second) / *rps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var sent, failed int

	for {
		select {
		case <-ctx.Done():
			logger.Info("simulator stopped", "sent", sent, "failed", failed)
			return
		case <-ticker.C:
			event := generateEvent()
			if err := sendEvent(client, *apiURL, event); err != nil {
				failed++
				logger.Warn("failed to send event", "error", err, "event_id", event.EventID)
			} else {
				sent++
				logger.Info("event sent",
					"provider", event.Provider,
					"model", event.Model,
					"latency_ms", event.LatencyMS,
					"cost_usd", fmt.Sprintf("$%.6f", event.CostUSD),
					"status", event.Status,
					"total_sent", sent,
				)
			}
		}
	}
}

// generateEvent creates a realistic synthetic LLM call event.
func generateEvent() *models.LLMCallEvent {
	// Pick a random model (weighted towards popular ones).
	m := modelPool[rand.Intn(len(modelPool))]

	// Generate realistic latency with jitter.
	latency := m.avgLatencyMS + int64(rand.NormFloat64()*float64(m.jitterMS))
	if latency < 50 {
		latency = 50
	}

	// Generate realistic token counts with variance.
	inputTokens := m.avgInputTok + int(rand.NormFloat64()*float64(m.avgInputTok/3))
	outputTokens := m.avgOutputTok + int(rand.NormFloat64()*float64(m.avgOutputTok/3))
	if inputTokens < 10 {
		inputTokens = 10
	}
	if outputTokens < 5 {
		outputTokens = 5
	}

	// Determine status.
	status := models.StatusSuccess
	errorMsg := ""
	if rand.Float64() < m.errorRate {
		status = models.StatusError
		errorMsg = randomErrorMessage()
	}

	event := &models.LLMCallEvent{
		EventID:      uuid.New().String(),
		Provider:     m.provider,
		Model:        m.model,
		LatencyMS:    latency,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Status:       status,
		ErrorMessage: errorMsg,
		Timestamp:    time.Now().UnixMilli(),
	}
	event.CostUSD = models.CalculateCost(m.provider, m.model, inputTokens, outputTokens)
	return event
}

// sendEvent sends an event to the LLMWatch API.
func sendEvent(client *http.Client, apiURL string, event *models.LLMCallEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	resp, err := client.Post(apiURL+"/api/v1/events", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

var errorMessages = []string{
	"rate_limit_exceeded: too many requests",
	"context_length_exceeded: prompt too long",
	"server_error: internal server error from provider",
	"timeout: request timed out after 30s",
	"invalid_api_key: authentication failed",
	"model_overloaded: model is at capacity",
	"content_policy_violation: request blocked by content filter",
}

func randomErrorMessage() string {
	return errorMessages[rand.Intn(len(errorMessages))]
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func init() {
	// Re-seed random for Go < 1.20 (no-op in 1.20+).
	rand.New(rand.NewSource(time.Now().UnixNano()))
}
