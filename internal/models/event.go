package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Provider represents an LLM provider.
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderGemini    Provider = "gemini"
)

// Status represents the outcome of an LLM call.
type Status string

const (
	StatusSuccess Status = "success"
	StatusError   Status = "error"
)

// LLMCallEvent is the core data model for a single LLM API call.
// It is produced to Kafka and stored in PostgreSQL.
type LLMCallEvent struct {
	EventID      string   `json:"event_id"`
	Provider     Provider `json:"provider"`
	Model        string   `json:"model"`
	LatencyMS    int64    `json:"latency_ms"`
	InputTokens  int      `json:"input_tokens"`
	OutputTokens int      `json:"output_tokens"`
	CostUSD      float64  `json:"cost_usd"`
	Status       Status   `json:"status"`
	ErrorMessage string   `json:"error_message,omitempty"`
	Timestamp    int64    `json:"timestamp"` // Unix milliseconds
}

// Validate checks that the event has all required fields with valid values.
func (e *LLMCallEvent) Validate() error {
	if e.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	switch e.Provider {
	case ProviderOpenAI, ProviderAnthropic, ProviderGemini:
	default:
		return fmt.Errorf("invalid provider %q: must be one of openai, anthropic, gemini", e.Provider)
	}

	if e.Model == "" {
		return fmt.Errorf("model is required")
	}
	if e.LatencyMS < 0 {
		return fmt.Errorf("latency_ms must be non-negative")
	}
	if e.InputTokens < 0 {
		return fmt.Errorf("input_tokens must be non-negative")
	}
	if e.OutputTokens < 0 {
		return fmt.Errorf("output_tokens must be non-negative")
	}
	switch e.Status {
	case StatusSuccess, StatusError:
	default:
		return fmt.Errorf("invalid status %q: must be success or error", e.Status)
	}

	return nil
}

// Normalize fills in defaults (event_id, timestamp, cost) if not set.
func (e *LLMCallEvent) Normalize() {
	if e.EventID == "" {
		e.EventID = uuid.New().String()
	}
	if e.Timestamp == 0 {
		e.Timestamp = time.Now().UnixMilli()
	}
	if e.CostUSD == 0 {
		e.CostUSD = CalculateCost(e.Provider, e.Model, e.InputTokens, e.OutputTokens)
	}
}

// CalculateCost estimates the cost in USD based on provider/model and token counts.
// Prices are approximate and sourced from public pricing pages.
func CalculateCost(provider Provider, model string, inputTokens, outputTokens int) float64 {
	model = strings.ToLower(model)

	type pricing struct {
		inputPer1M  float64 // USD per 1M input tokens
		outputPer1M float64 // USD per 1M output tokens
	}

	var p pricing

	switch provider {
	case ProviderOpenAI:
		switch {
		case strings.Contains(model, "gpt-4o-mini"):
			p = pricing{0.15, 0.60}
		case strings.Contains(model, "gpt-4o"):
			p = pricing{5.00, 15.00}
		case strings.Contains(model, "gpt-4-turbo"):
			p = pricing{10.00, 30.00}
		case strings.Contains(model, "gpt-4"):
			p = pricing{30.00, 60.00}
		case strings.Contains(model, "gpt-3.5"):
			p = pricing{0.50, 1.50}
		default:
			p = pricing{5.00, 15.00}
		}
	case ProviderAnthropic:
		switch {
		case strings.Contains(model, "claude-3-5-haiku"):
			p = pricing{0.80, 4.00}
		case strings.Contains(model, "claude-3-5-sonnet"):
			p = pricing{3.00, 15.00}
		case strings.Contains(model, "claude-3-opus"):
			p = pricing{15.00, 75.00}
		case strings.Contains(model, "claude-3-sonnet"):
			p = pricing{3.00, 15.00}
		case strings.Contains(model, "claude-3-haiku"):
			p = pricing{0.25, 1.25}
		default:
			p = pricing{3.00, 15.00}
		}
	case ProviderGemini:
		switch {
		case strings.Contains(model, "gemini-2.0-flash"):
			p = pricing{0.075, 0.30}
		case strings.Contains(model, "gemini-1.5-flash"):
			p = pricing{0.075, 0.30}
		case strings.Contains(model, "gemini-1.5-pro"):
			p = pricing{3.50, 10.50}
		case strings.Contains(model, "gemini-2.0-pro"):
			p = pricing{3.50, 10.50}
		default:
			p = pricing{0.075, 0.30}
		}
	}

	inputCost := float64(inputTokens) / 1_000_000 * p.inputPer1M
	outputCost := float64(outputTokens) / 1_000_000 * p.outputPer1M
	return inputCost + outputCost
}

// SummaryMetrics represents aggregated metrics over a time period.
type SummaryMetrics struct {
	TotalCalls   int64   `json:"total_calls"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	AvgLatencyMS float64 `json:"avg_latency_ms"`
	ErrorRate    float64 `json:"error_rate"` // 0.0-1.0
	Period       string  `json:"period"`     // e.g. "24h"
}

// CallsPerMinutePoint is a single data point for the calls/minute time series.
type CallsPerMinutePoint struct {
	Minute    time.Time `json:"minute"`
	CallCount int64     `json:"call_count"`
}

// LatencyPercentiles holds p50/p95/p99 for a specific model.
type LatencyPercentiles struct {
	Provider Provider `json:"provider"`
	Model    string   `json:"model"`
	P50      float64  `json:"p50"`
	P95      float64  `json:"p95"`
	P99      float64  `json:"p99"`
}

// CostByProvider is the cost breakdown per provider.
type CostByProvider struct {
	Provider Provider `json:"provider"`
	TotalUSD float64  `json:"total_usd"`
}

// ErrorRate holds error rate per model.
type ErrorRateByModel struct {
	Provider  Provider `json:"provider"`
	Model     string   `json:"model"`
	Total     int64    `json:"total"`
	Errors    int64    `json:"errors"`
	ErrorRate float64  `json:"error_rate"` // 0.0-1.0
}

// ProviderComparison is a comparative stats row per provider.
type ProviderComparison struct {
	Provider     Provider `json:"provider"`
	TotalCalls   int64    `json:"total_calls"`
	AvgLatencyMS float64  `json:"avg_latency_ms"`
	TotalCostUSD float64  `json:"total_cost_usd"`
	ErrorRate    float64  `json:"error_rate"`
	P95LatencyMS float64  `json:"p95_latency_ms"`
}
