// All TypeScript interfaces matching the Go backend API responses

export interface MetricsSummary {
  total_calls: number
  total_cost_usd: number
  avg_latency_ms: number
  error_rate: number
  period: string
}

export interface CallsPerMinutePoint {
  minute: string
  call_count: number
}

export interface LatencyPercentile {
  provider: string
  model: string
  p50: number
  p95: number
  p99: number
}

export interface CostByProvider {
  provider: string
  total_usd: number
}

export interface ErrorRate {
  provider: string
  model: string
  total: number
  errors: number
  error_rate: number
}

export interface LLMCall {
  event_id: string
  provider: string
  model: string
  latency_ms: number
  input_tokens: number
  output_tokens: number
  cost_usd: number
  status: 'success' | 'error' | string
  error_message: string
  timestamp: string
}

export interface ProviderComparison {
  provider: string
  total_calls: number
  avg_latency_ms: number
  total_cost_usd: number
  error_rate: number
  p95_latency_ms: number
}

// SSE Event — same shape as LLMCall
export type LLMCallEvent = LLMCall

export interface DashboardData {
  summary: MetricsSummary | null
  callsPerMinute: CallsPerMinutePoint[]
  latencyPercentiles: LatencyPercentile[]
  costByProvider: CostByProvider[]
  errorRates: ErrorRate[]
  calls: LLMCall[]
  providerComparison: ProviderComparison[]
  lastUpdated: Date | null
  isLoading: boolean
  error: string | null
}
