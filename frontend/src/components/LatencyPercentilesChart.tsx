import React from 'react'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import type { LatencyPercentile } from '../types/api'

interface LatencyPercentilesChartProps {
  data: LatencyPercentile[]
  isLoading?: boolean
}

const COLORS = {
  p50: '#6366f1',
  p95: '#f59e0b',
  p99: '#ef4444',
}

const CustomTooltip = ({
  active,
  payload,
  label,
}: {
  active?: boolean
  payload?: Array<{ name: string; value: number; color: string }>
  label?: string
}) => {
  if (!active || !payload?.length) return null

  return (
    <div className="custom-tooltip">
      <p className="label mb-2">{label}</p>
      {payload.map(entry => (
        <div key={entry.name} className="flex items-center gap-2 mb-1">
          <span
            className="inline-block w-2.5 h-2.5 rounded-sm"
            style={{ background: entry.color }}
          />
          <span className="text-[#94a3b8] text-xs">{entry.name}:</span>
          <span className="text-white text-xs font-semibold">{entry.value.toFixed(0)}ms</span>
        </div>
      ))}
    </div>
  )
}

const LatencyPercentilesChart: React.FC<LatencyPercentilesChartProps> = ({
  data,
  isLoading,
}) => {
  // Aggregate by provider (average across models)
  const providerMap = new Map<string, { p50: number[]; p95: number[]; p99: number[] }>()
  data.forEach(d => {
    if (!providerMap.has(d.provider)) {
      providerMap.set(d.provider, { p50: [], p95: [], p99: [] })
    }
    const entry = providerMap.get(d.provider)!
    entry.p50.push(d.p50)
    entry.p95.push(d.p95)
    entry.p99.push(d.p99)
  })

  const chartData = Array.from(providerMap.entries()).map(([provider, vals]) => ({
    provider,
    P50: Math.round(vals.p50.reduce((a, b) => a + b, 0) / vals.p50.length),
    P95: Math.round(vals.p95.reduce((a, b) => a + b, 0) / vals.p95.length),
    P99: Math.round(vals.p99.reduce((a, b) => a + b, 0) / vals.p99.length),
  }))

  return (
    <div className="glass-card p-6 h-full animate-slide-up" style={{ animationDelay: '200ms' }}>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-base font-semibold text-white">Latency Percentiles</h2>
          <p className="text-xs text-[#64748b] mt-0.5">P50 · P95 · P99 by provider</p>
        </div>
        <div className="flex items-center gap-3">
          {Object.entries(COLORS).map(([key, color]) => (
            <div key={key} className="flex items-center gap-1.5">
              <div
                className="w-2.5 h-2.5 rounded-sm"
                style={{ background: color }}
              />
              <span className="text-xs text-[#94a3b8] uppercase">{key}</span>
            </div>
          ))}
        </div>
      </div>

      {isLoading || chartData.length === 0 ? (
        <div className="h-64 shimmer rounded-xl" />
      ) : (
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={chartData}
              margin={{ top: 5, right: 10, left: -20, bottom: 0 }}
              barCategoryGap="30%"
              barGap={3}
            >
              <CartesianGrid
                strokeDasharray="3 3"
                stroke="rgba(255,255,255,0.05)"
                vertical={false}
              />
              <XAxis
                dataKey="provider"
                tick={{ fill: '#94a3b8', fontSize: 12 }}
                axisLine={false}
                tickLine={false}
              />
              <YAxis
                tick={{ fill: '#64748b', fontSize: 11 }}
                axisLine={false}
                tickLine={false}
                unit="ms"
              />
              <Tooltip content={<CustomTooltip />} cursor={{ fill: 'rgba(255,255,255,0.03)' }} />
              <Bar dataKey="P50" fill={COLORS.p50} radius={[4, 4, 0, 0]} maxBarSize={40} />
              <Bar dataKey="P95" fill={COLORS.p95} radius={[4, 4, 0, 0]} maxBarSize={40} />
              <Bar dataKey="P99" fill={COLORS.p99} radius={[4, 4, 0, 0]} maxBarSize={40} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}

export default LatencyPercentilesChart
