import React from 'react'
import type { ProviderComparison } from '../types/api'
import clsx from 'clsx'

interface ProviderComparisonTableProps {
  data: ProviderComparison[]
  isLoading?: boolean
}

const PROVIDER_ICONS: Record<string, string> = {
  openai: '⚡',
  anthropic: '🔮',
  google: '🌐',
  cohere: '🚀',
  mistral: '💨',
  default: '🤖',
}

const getProviderIcon = (provider: string) =>
  PROVIDER_ICONS[provider.toLowerCase()] ?? PROVIDER_ICONS.default

function formatLatency(ms: number): string {
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${Math.round(ms)}ms`
}

function formatCost(usd: number): string {
  if (usd === 0) return '$0'
  if (usd < 0.001) return `$${usd.toFixed(6)}`
  return `$${usd.toFixed(4)}`
}

function getLatencyClass(ms: number): string {
  if (ms > 5000) return 'text-rose-400'
  if (ms > 2000) return 'text-amber-400'
  return 'text-emerald-400'
}

function getErrorRateClass(rate: number): string {
  if (rate > 0.1) return 'text-rose-400'
  if (rate > 0.05) return 'text-amber-400'
  return 'text-emerald-400'
}

function ProgressBar({
  value,
  max,
  color,
}: {
  value: number
  max: number
  color: string
}) {
  const pct = max > 0 ? Math.min((value / max) * 100, 100) : 0
  return (
    <div className="w-full h-1 bg-white/[0.06] rounded-full overflow-hidden">
      <div
        className={clsx('h-full rounded-full transition-all duration-500', color)}
        style={{ width: `${pct}%` }}
      />
    </div>
  )
}

const ProviderComparisonTable: React.FC<ProviderComparisonTableProps> = ({
  data,
  isLoading,
}) => {
  const maxCalls = Math.max(...data.map(d => d.total_calls), 1)
  const maxLatency = Math.max(...data.map(d => d.avg_latency_ms), 1)

  return (
    <div className="glass-card h-full animate-slide-up" style={{ animationDelay: '350ms' }}>
      <div className="flex items-center justify-between p-6 border-b border-white/[0.06]">
        <div>
          <h2 className="text-base font-semibold text-white">Provider Comparison</h2>
          <p className="text-xs text-[#64748b] mt-0.5">Performance &amp; cost overview</p>
        </div>
        <div className="px-3 py-1.5 rounded-lg bg-indigo-500/10 border border-indigo-500/20">
          <span className="text-xs font-medium text-indigo-400">{data.length} providers</span>
        </div>
      </div>

      {isLoading || data.length === 0 ? (
        <div className="p-6 space-y-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-16 shimmer rounded-xl" />
          ))}
        </div>
      ) : (
        <div className="divide-y divide-white/[0.04]">
          {data
            .sort((a, b) => b.total_calls - a.total_calls)
            .map(provider => (
              <div
                key={provider.provider}
                className="p-4 hover:bg-white/[0.02] transition-colors duration-200"
              >
                <div className="flex items-start gap-3">
                  {/* Icon + Name */}
                  <div className="flex-shrink-0 w-9 h-9 rounded-xl bg-white/[0.04] border border-white/[0.08] flex items-center justify-center text-base">
                    {getProviderIcon(provider.provider)}
                  </div>

                  <div className="flex-1 min-w-0">
                    <div className="flex items-center justify-between mb-2">
                      <h3 className="text-sm font-semibold text-white capitalize">
                        {provider.provider}
                      </h3>
                      <span className="text-xs text-[#64748b] font-mono">
                        {provider.total_calls.toLocaleString()} calls
                      </span>
                    </div>

                    {/* Metrics row */}
                    <div className="grid grid-cols-4 gap-3 mb-2">
                      <div>
                        <p className="text-[10px] text-[#475569] uppercase tracking-wider mb-0.5">Avg Latency</p>
                        <p className={clsx('text-xs font-semibold font-mono', getLatencyClass(provider.avg_latency_ms))}>
                          {formatLatency(provider.avg_latency_ms)}
                        </p>
                      </div>
                      <div>
                        <p className="text-[10px] text-[#475569] uppercase tracking-wider mb-0.5">P95</p>
                        <p className={clsx('text-xs font-semibold font-mono', getLatencyClass(provider.p95_latency_ms))}>
                          {formatLatency(provider.p95_latency_ms)}
                        </p>
                      </div>
                      <div>
                        <p className="text-[10px] text-[#475569] uppercase tracking-wider mb-0.5">Error Rate</p>
                        <p className={clsx('text-xs font-semibold', getErrorRateClass(provider.error_rate))}>
                          {(provider.error_rate * 100).toFixed(1)}%
                        </p>
                      </div>
                      <div>
                        <p className="text-[10px] text-[#475569] uppercase tracking-wider mb-0.5">Total Cost</p>
                        <p className="text-xs font-semibold text-emerald-400 font-mono">
                          {formatCost(provider.total_cost_usd)}
                        </p>
                      </div>
                    </div>

                    {/* Progress bars */}
                    <div className="space-y-1">
                      <ProgressBar value={provider.total_calls} max={maxCalls} color="bg-indigo-500" />
                      <ProgressBar value={provider.avg_latency_ms} max={maxLatency} color="bg-amber-500" />
                    </div>
                  </div>
                </div>
              </div>
            ))}
        </div>
      )}
    </div>
  )
}

export default ProviderComparisonTable
