import React from 'react'
import { useMetrics } from '../hooks/useMetrics'
import { useSSE } from '../hooks/useSSE'
import Header from './Header'
import MetricCard from './MetricCard'
import CallsPerMinuteChart from './CallsPerMinuteChart'
import LatencyPercentilesChart from './LatencyPercentilesChart'
import CostByProviderChart from './CostByProviderChart'
import ErrorRateChart from './ErrorRateChart'
import LiveCallFeed from './LiveCallFeed'
import ProviderComparisonTable from './ProviderComparisonTable'

// SVG Icons
const icons = {
  calls: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
    </svg>
  ),
  cost: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <path d="M16 8h-6a2 2 0 100 4h4a2 2 0 110 4H8" />
      <path d="M12 18V6" />
    </svg>
  ),
  latency: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <polyline points="12 6 12 12 16 14" />
    </svg>
  ),
  error: (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
      <line x1="12" y1="9" x2="12" y2="13" />
      <line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
  ),
}

function formatNumber(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`
  return n.toLocaleString()
}

function formatLatency(ms: number): string {
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${Math.round(ms)}ms`
}

const Dashboard: React.FC = () => {
  const metrics = useMetrics(5000)
  const sse = useSSE()

  const { summary, isLoading } = metrics

  return (
    <div className="min-h-screen bg-[#0a0f1e] grid-bg">
      {/* Ambient background glows */}
      <div className="fixed inset-0 pointer-events-none overflow-hidden">
        <div className="absolute top-[-10%] left-[20%] w-[600px] h-[600px] rounded-full bg-indigo-500/[0.04] blur-[120px]" />
        <div className="absolute bottom-[-5%] right-[10%] w-[500px] h-[500px] rounded-full bg-purple-500/[0.03] blur-[100px]" />
        <div className="absolute top-[40%] left-[-5%] w-[400px] h-[400px] rounded-full bg-emerald-500/[0.02] blur-[100px]" />
      </div>

      {/* Header */}
      <Header
        lastUpdated={metrics.lastUpdated}
        isConnected={sse.connected}
        isLoading={isLoading}
        error={metrics.error}
      />

      {/* Main content */}
      <main className="relative max-w-[1600px] mx-auto px-6 py-8 space-y-6">

        {/* Error banner */}
        {metrics.error && (
          <div className="animate-fade-in flex items-center gap-3 px-5 py-3.5 rounded-xl bg-rose-500/10 border border-rose-500/20">
            <svg className="w-4 h-4 text-rose-400 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
              <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
            </svg>
            <p className="text-sm text-rose-300">
              <span className="font-semibold">API connection error:</span> {metrics.error}
              {' '}— Dashboard will retry automatically.
            </p>
          </div>
        )}

        {/* Row 1: Metric Cards */}
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-4">
          <MetricCard
            title="Total Calls"
            value={summary ? formatNumber(summary.total_calls) : '—'}
            subtitle={summary?.period ? `Period: ${summary.period}` : undefined}
            icon={icons.calls}
            accentColor="indigo"
            isLoading={isLoading && !summary}
            animationDelay={0}
          />
          <MetricCard
            title="Total Cost"
            value={summary ? `$${summary.total_cost_usd.toFixed(4)}` : '—'}
            subtitle="USD · all providers"
            icon={icons.cost}
            accentColor="emerald"
            isLoading={isLoading && !summary}
            animationDelay={100}
          />
          <MetricCard
            title="Avg Latency"
            value={summary ? formatLatency(summary.avg_latency_ms) : '—'}
            subtitle="Across all models"
            icon={icons.latency}
            accentColor="amber"
            isLoading={isLoading && !summary}
            animationDelay={200}
          />
          <MetricCard
            title="Error Rate"
            value={summary ? `${(summary.error_rate * 100).toFixed(2)}%` : '—'}
            subtitle={
              summary
                ? summary.error_rate > 0.1
                  ? '⚠ Above threshold'
                  : '✓ Within normal range'
                : undefined
            }
            icon={icons.error}
            accentColor={
              summary
                ? summary.error_rate > 0.1
                  ? 'rose'
                  : summary.error_rate > 0.05
                  ? 'amber'
                  : 'emerald'
                : 'rose'
            }
            isLoading={isLoading && !summary}
            animationDelay={300}
          />
        </div>

        {/* Row 2: Calls Per Minute (full width) */}
        <CallsPerMinuteChart
          data={metrics.callsPerMinute}
          isLoading={isLoading && metrics.callsPerMinute.length === 0}
        />

        {/* Row 3: Latency (60%) + Cost Pie (40%) */}
        <div className="grid grid-cols-1 lg:grid-cols-5 gap-6">
          <div className="lg:col-span-3">
            <LatencyPercentilesChart
              data={metrics.latencyPercentiles}
              isLoading={isLoading && metrics.latencyPercentiles.length === 0}
            />
          </div>
          <div className="lg:col-span-2">
            <CostByProviderChart
              data={metrics.costByProvider}
              isLoading={isLoading && metrics.costByProvider.length === 0}
            />
          </div>
        </div>

        {/* Row 4: Error Rate (50%) + Provider Comparison (50%) */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <ErrorRateChart
            data={metrics.errorRates}
            isLoading={isLoading && metrics.errorRates.length === 0}
          />
          <ProviderComparisonTable
            data={metrics.providerComparison}
            isLoading={isLoading && metrics.providerComparison.length === 0}
          />
        </div>

        {/* Row 5: Live Call Feed (full width) */}
        <LiveCallFeed
          calls={metrics.calls}
          sseEvents={sse.events}
          isLoading={isLoading && metrics.calls.length === 0}
        />

        {/* Footer */}
        <footer className="pt-4 pb-8 text-center">
          <p className="text-xs text-[#334155]">
            LLMWatch · AI Observability Platform · Polling every 5s
            {sse.connected && ' · SSE stream active'}
          </p>
        </footer>
      </main>
    </div>
  )
}

export default Dashboard
