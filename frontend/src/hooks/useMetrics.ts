import { useState, useEffect, useCallback, useRef } from 'react'
import type {
  MetricsSummary,
  CallsPerMinutePoint,
  LatencyPercentile,
  CostByProvider,
  ErrorRate,
  LLMCall,
  ProviderComparison,
  DashboardData,
} from '../types/api'

const apiEnv = import.meta.env.VITE_API_URL
const API_BASE = apiEnv !== undefined && apiEnv !== null ? apiEnv : 'http://localhost:8080'

async function fetchJSON<T>(endpoint: string): Promise<T> {
  const res = await fetch(`${API_BASE}${endpoint}`)
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}: ${res.statusText}`)
  }
  return res.json() as Promise<T>
}

export function useMetrics(intervalMs = 5000): DashboardData {
  const [data, setData] = useState<DashboardData>({
    summary: null,
    callsPerMinute: [],
    latencyPercentiles: [],
    costByProvider: [],
    errorRates: [],
    calls: [],
    providerComparison: [],
    lastUpdated: null,
    isLoading: true,
    error: null,
  })

  const isMounted = useRef(true)

  const fetchAll = useCallback(async () => {
    try {
      const [
        summary,
        callsPerMinute,
        latencyPercentiles,
        costByProvider,
        errorRates,
        calls,
        providerComparison,
      ] = await Promise.all([
        fetchJSON<MetricsSummary>('/api/v1/metrics/summary'),
        fetchJSON<CallsPerMinutePoint[]>('/api/v1/metrics/calls-per-minute'),
        fetchJSON<LatencyPercentile[]>('/api/v1/metrics/latency-percentiles'),
        fetchJSON<CostByProvider[]>('/api/v1/metrics/cost-by-provider'),
        fetchJSON<ErrorRate[]>('/api/v1/metrics/error-rates'),
        fetchJSON<LLMCall[]>('/api/v1/calls'),
        fetchJSON<ProviderComparison[]>('/api/v1/metrics/provider-comparison'),
      ])

      if (!isMounted.current) return

      setData({
        summary,
        callsPerMinute: callsPerMinute ?? [],
        latencyPercentiles: latencyPercentiles ?? [],
        costByProvider: costByProvider ?? [],
        errorRates: errorRates ?? [],
        calls: (calls ?? []).slice(0, 50),
        providerComparison: providerComparison ?? [],
        lastUpdated: new Date(),
        isLoading: false,
        error: null,
      })


    } catch (err) {
      if (!isMounted.current) return
      const message = err instanceof Error ? err.message : 'Unknown error'
      setData(prev => ({
        ...prev,
        isLoading: false,
        error: message,
      }))
    }
  }, [])

  useEffect(() => {
    isMounted.current = true
    fetchAll()

    const timer = setInterval(fetchAll, intervalMs)

    return () => {
      isMounted.current = false
      clearInterval(timer)
    }
  }, [fetchAll, intervalMs])

  return data
}
