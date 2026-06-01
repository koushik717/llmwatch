import React from 'react'
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
  ReferenceLine,
} from 'recharts'
import type { ErrorRate } from '../types/api'

interface ErrorRateChartProps {
  data: ErrorRate[]
  isLoading?: boolean
}

const CustomTooltip = ({
  active,
  payload,
  label,
}: {
  active?: boolean
  payload?: Array<{ value: number }>
  label?: string
}) => {
  if (!active || !payload?.length) return null

  const rate = payload[0]?.value ?? 0
  return (
    <div className="custom-tooltip">
      <p className="label mb-1">{label}</p>
      <div className="flex items-center gap-2">
        <span
          className="inline-block w-2 h-2 rounded-full"
          style={{
            background: rate > 10 ? '#ef4444' : rate > 5 ? '#f59e0b' : '#10b981',
          }}
        />
        <p className="value">{rate.toFixed(2)}%</p>
      </div>
    </div>
  )
}

const ErrorRateChart: React.FC<ErrorRateChartProps> = ({
  data,
  isLoading,
}) => {
  // Aggregate: sum errors and total by provider+model key
  const chartData = data
    .map(d => ({
      name: `${d.provider} / ${d.model.split('-').slice(0, 2).join('-')}`,
      rate: d.error_rate * 100,
      errors: d.errors,
      total: d.total,
    }))
    .sort((a, b) => b.rate - a.rate)
    .slice(0, 10)

  const getBarColor = (rate: number) => {
    if (rate > 10) return '#ef4444'
    if (rate > 5) return '#f59e0b'
    return '#10b981'
  }

  return (
    <div className="glass-card p-6 h-full animate-slide-up" style={{ animationDelay: '300ms' }}>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-base font-semibold text-white">Error Rates</h2>
          <p className="text-xs text-[#64748b] mt-0.5">By provider &amp; model · top 10</p>
        </div>
        <div className="flex items-center gap-3 text-xs">
          <div className="flex items-center gap-1.5">
            <div className="w-2 h-2 rounded-full bg-emerald-500" />
            <span className="text-[#64748b]">&lt;5%</span>
          </div>
          <div className="flex items-center gap-1.5">
            <div className="w-2 h-2 rounded-full bg-amber-500" />
            <span className="text-[#64748b]">5-10%</span>
          </div>
          <div className="flex items-center gap-1.5">
            <div className="w-2 h-2 rounded-full bg-rose-500" />
            <span className="text-[#64748b]">&gt;10%</span>
          </div>
        </div>
      </div>

      {isLoading || chartData.length === 0 ? (
        <div className="h-64 shimmer rounded-xl" />
      ) : (
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart
              data={chartData}
              layout="vertical"
              margin={{ top: 0, right: 30, left: 10, bottom: 0 }}
            >
              <CartesianGrid
                strokeDasharray="3 3"
                stroke="rgba(255,255,255,0.05)"
                horizontal={false}
              />
              <XAxis
                type="number"
                tick={{ fill: '#64748b', fontSize: 11 }}
                axisLine={false}
                tickLine={false}
                unit="%"
                domain={[0, 'dataMax + 2']}
              />
              <YAxis
                type="category"
                dataKey="name"
                tick={{ fill: '#94a3b8', fontSize: 10 }}
                axisLine={false}
                tickLine={false}
                width={140}
              />
              <Tooltip
                content={<CustomTooltip />}
                cursor={{ fill: 'rgba(255,255,255,0.03)' }}
              />
              <ReferenceLine
                x={5}
                stroke="rgba(245, 158, 11, 0.3)"
                strokeDasharray="4 4"
              />
              <ReferenceLine
                x={10}
                stroke="rgba(239, 68, 68, 0.3)"
                strokeDasharray="4 4"
              />
              <Bar dataKey="rate" radius={[0, 4, 4, 0]} maxBarSize={20}>
                {chartData.map((entry, index) => (
                  <Cell
                    key={`cell-${index}`}
                    fill={getBarColor(entry.rate)}
                    opacity={0.85}
                  />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}

export default ErrorRateChart
