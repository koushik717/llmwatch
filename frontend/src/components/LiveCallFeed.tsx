import React, { useEffect, useRef, useState } from 'react'
import { format, parseISO } from 'date-fns'
import type { LLMCall } from '../types/api'
import StatusBadge from './StatusBadge'
import clsx from 'clsx'

interface LiveCallFeedProps {
  calls: LLMCall[]
  sseEvents: LLMCall[]
  isLoading?: boolean
}

const PROVIDER_COLORS: Record<string, string> = {
  openai: 'text-emerald-400 bg-emerald-500/10',
  anthropic: 'text-amber-400 bg-amber-500/10',
  google: 'text-indigo-400 bg-indigo-500/10',
  cohere: 'text-purple-400 bg-purple-500/10',
  mistral: 'text-cyan-400 bg-cyan-500/10',
  default: 'text-slate-400 bg-slate-500/10',
}

const getProviderStyle = (provider: string) =>
  PROVIDER_COLORS[provider.toLowerCase()] ?? PROVIDER_COLORS.default

function formatLatency(ms: number): string {
  if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`
  return `${ms.toFixed(0)}ms`
}

function formatCost(usd: number): string {
  if (usd < 0.0001) return '<$0.0001'
  return `$${usd.toFixed(4)}`
}

function formatTimestamp(ts: string): string {
  try {
    return format(parseISO(ts), 'HH:mm:ss')
  } catch {
    return ts
  }
}

const LiveCallFeed: React.FC<LiveCallFeedProps> = ({
  calls,
  sseEvents,
  isLoading,
}) => {
  const [highlighted, setHighlighted] = useState<Set<string>>(new Set())
  const prevSseCount = useRef(sseEvents.length)
  const tableRef = useRef<HTMLDivElement>(null)

  // Merge SSE events with polled calls, deduplicate by event_id
  const mergedCalls = React.useMemo(() => {
    const seen = new Set<string>()
    const combined = [...sseEvents, ...calls]
    return combined
      .filter(c => {
        if (seen.has(c.event_id)) return false
        seen.add(c.event_id)
        return true
      })
      .slice(0, 50)
  }, [sseEvents, calls])

  // Highlight newly arrived SSE events
  useEffect(() => {
    const newCount = sseEvents.length - prevSseCount.current
    if (newCount > 0) {
      const newIds = new Set(sseEvents.slice(0, newCount).map(e => e.event_id))
      setHighlighted(newIds)
      const timer = setTimeout(() => setHighlighted(new Set()), 2000)
      prevSseCount.current = sseEvents.length
      return () => clearTimeout(timer)
    }
    prevSseCount.current = sseEvents.length
  }, [sseEvents])

  const totalTokens = mergedCalls.reduce(
    (sum, c) => sum + (c.input_tokens || 0) + (c.output_tokens || 0),
    0,
  )
  const totalCost = mergedCalls.reduce((sum, c) => sum + (c.cost_usd || 0), 0)

  return (
    <div className="glass-card animate-slide-up" style={{ animationDelay: '400ms' }}>
      {/* Header */}
      <div className="flex items-center justify-between p-6 border-b border-white/[0.06]">
        <div className="flex items-center gap-3">
          <div>
            <h2 className="text-base font-semibold text-white">Live Call Feed</h2>
            <p className="text-xs text-[#64748b] mt-0.5">
              Last {mergedCalls.length} calls · auto-refreshes
            </p>
          </div>
          {sseEvents.length > 0 && (
            <div className="flex items-center gap-2 px-2.5 py-1 rounded-lg bg-emerald-500/10 border border-emerald-500/20">
              <div className="relative flex h-1.5 w-1.5">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500" />
              </div>
              <span className="text-xs text-emerald-400 font-medium">SSE Active</span>
            </div>
          )}
        </div>

        {/* Summary pills */}
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-white/[0.03] border border-white/[0.06]">
            <span className="text-xs text-[#64748b]">Tokens</span>
            <span className="text-xs font-semibold text-white">{totalTokens.toLocaleString()}</span>
          </div>
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-white/[0.03] border border-white/[0.06]">
            <span className="text-xs text-[#64748b]">Cost</span>
            <span className="text-xs font-semibold text-emerald-400">{formatCost(totalCost)}</span>
          </div>
        </div>
      </div>

      {/* Table */}
      <div ref={tableRef} className="overflow-x-auto">
        {isLoading && mergedCalls.length === 0 ? (
          <div className="p-6 space-y-3">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="h-10 shimmer rounded-lg" />
            ))}
          </div>
        ) : mergedCalls.length === 0 ? (
          <div className="p-12 text-center">
            <div className="w-12 h-12 mx-auto mb-4 rounded-full bg-white/[0.04] flex items-center justify-center">
              <svg className="w-6 h-6 text-[#475569]" fill="none" stroke="currentColor" strokeWidth="1.5" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 9.776c.112-.017.227-.026.344-.026h15.812c.117 0 .232.009.344.026m-16.5 0a2.25 2.25 0 00-1.883 2.542l.857 6a2.25 2.25 0 002.227 1.932H19.05a2.25 2.25 0 002.227-1.932l.857-6a2.25 2.25 0 00-1.883-2.542m-16.5 0V6A2.25 2.25 0 016 3.75h3.879a1.5 1.5 0 011.06.44l2.122 2.12a1.5 1.5 0 001.06.44H18A2.25 2.25 0 0120.25 9v.776" />
              </svg>
            </div>
            <p className="text-[#64748b] text-sm">No calls yet</p>
            <p className="text-[#475569] text-xs mt-1">Waiting for LLM API activity...</p>
          </div>
        ) : (
          <table className="data-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>Provider</th>
                <th>Model</th>
                <th>Status</th>
                <th className="text-right">Latency</th>
                <th className="text-right">Input</th>
                <th className="text-right">Output</th>
                <th className="text-right">Cost</th>
                <th className="max-w-xs">Error</th>
              </tr>
            </thead>
            <tbody>
              {mergedCalls.map(call => {
                const isNew = highlighted.has(call.event_id)
                return (
                  <tr
                    key={call.event_id}
                    className={clsx(
                      'transition-all duration-500',
                      isNew && 'bg-indigo-500/[0.08] border-l-2 border-indigo-500',
                    )}
                  >
                    <td className="font-mono text-xs text-[#64748b] whitespace-nowrap">
                      {formatTimestamp(call.timestamp)}
                    </td>
                    <td>
                      <span
                        className={clsx(
                          'badge text-[10px] px-2 py-0.5 rounded font-medium',
                          getProviderStyle(call.provider),
                        )}
                      >
                        {call.provider}
                      </span>
                    </td>
                    <td className="font-mono text-xs text-[#94a3b8] max-w-[180px] truncate">
                      {call.model}
                    </td>
                    <td>
                      <StatusBadge status={call.status} />
                    </td>
                    <td className="text-right font-mono text-xs">
                      <span
                        className={clsx(
                          call.latency_ms > 5000
                            ? 'text-rose-400'
                            : call.latency_ms > 2000
                            ? 'text-amber-400'
                            : 'text-emerald-400',
                        )}
                      >
                        {formatLatency(call.latency_ms)}
                      </span>
                    </td>
                    <td className="text-right font-mono text-xs text-[#94a3b8]">
                      {(call.input_tokens || 0).toLocaleString()}
                    </td>
                    <td className="text-right font-mono text-xs text-[#94a3b8]">
                      {(call.output_tokens || 0).toLocaleString()}
                    </td>
                    <td className="text-right font-mono text-xs text-emerald-400">
                      {formatCost(call.cost_usd || 0)}
                    </td>
                    <td className="max-w-xs">
                      {call.error_message ? (
                        <span className="text-xs text-rose-400 truncate block max-w-[200px]" title={call.error_message}>
                          {call.error_message}
                        </span>
                      ) : (
                        <span className="text-xs text-[#475569]">—</span>
                      )}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}

export default LiveCallFeed
