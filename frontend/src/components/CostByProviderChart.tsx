import React, { useState } from 'react'
import {
  PieChart,
  Pie,
  Cell,
  Tooltip,
  ResponsiveContainer,
  Sector,
} from 'recharts'
import type { CostByProvider } from '../types/api'

interface CostByProviderChartProps {
  data: CostByProvider[]
  isLoading?: boolean
}

const CHART_COLORS = [
  '#6366f1', // indigo
  '#10b981', // emerald
  '#f59e0b', // amber
  '#ef4444', // rose
  '#8b5cf6', // violet
  '#06b6d4', // cyan
  '#ec4899', // pink
  '#f97316', // orange
]

const CustomTooltip = ({
  active,
  payload,
}: {
  active?: boolean
  payload?: Array<{ name: string; value: number; payload: { percent: number } }>
}) => {
  if (!active || !payload?.length) return null
  const item = payload[0]

  return (
    <div className="custom-tooltip">
      <p className="label">{item.name}</p>
      <p className="value">${item.value.toFixed(4)}</p>
      <p className="text-[#64748b] text-xs mt-1">
        {(item.payload.percent * 100).toFixed(1)}% of total
      </p>
    </div>
  )
}

const renderActiveShape = (props: {
  cx: number
  cy: number
  innerRadius: number
  outerRadius: number
  startAngle: number
  endAngle: number
  fill: string
  payload: { provider: string }
  value: number
  percent: number
}) => {
  const {
    cx, cy, innerRadius, outerRadius, startAngle, endAngle, fill, payload, value, percent,
  } = props

  return (
    <g>
      <text x={cx} y={cy - 14} textAnchor="middle" fill="#e2e8f0" fontSize={13} fontWeight={600}>
        {payload.provider}
      </text>
      <text x={cx} y={cy + 8} textAnchor="middle" fill="#94a3b8" fontSize={11}>
        ${value.toFixed(4)}
      </text>
      <text x={cx} y={cy + 26} textAnchor="middle" fill="#64748b" fontSize={10}>
        {(percent * 100).toFixed(1)}%
      </text>
      <Sector
        cx={cx}
        cy={cy}
        innerRadius={innerRadius}
        outerRadius={outerRadius + 8}
        startAngle={startAngle}
        endAngle={endAngle}
        fill={fill}
      />
      <Sector
        cx={cx}
        cy={cy}
        innerRadius={outerRadius + 12}
        outerRadius={outerRadius + 14}
        startAngle={startAngle}
        endAngle={endAngle}
        fill={fill}
      />
    </g>
  )
}

const CostByProviderChart: React.FC<CostByProviderChartProps> = ({
  data,
  isLoading,
}) => {
  const [activeIndex, setActiveIndex] = useState<number>(0)

  const total = data.reduce((sum, d) => sum + d.total_usd, 0)

  const chartData = data
    .sort((a, b) => b.total_usd - a.total_usd)
    .map(d => ({
      ...d,
      provider: d.provider.charAt(0).toUpperCase() + d.provider.slice(1),
    }))

  return (
    <div className="glass-card p-6 h-full animate-slide-up" style={{ animationDelay: '250ms' }}>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-base font-semibold text-white">Cost by Provider</h2>
          <p className="text-xs text-[#64748b] mt-0.5">
            Total: <span className="text-emerald-400 font-semibold">${total.toFixed(4)}</span>
          </p>
        </div>
        <div className="px-3 py-1.5 rounded-lg bg-emerald-500/10 border border-emerald-500/20">
          <span className="text-xs font-semibold text-emerald-400">USD</span>
        </div>
      </div>

      {isLoading || chartData.length === 0 ? (
        <div className="h-64 shimmer rounded-xl" />
      ) : (
        <>
          <div className="h-56">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  activeIndex={activeIndex}
                  activeShape={renderActiveShape as unknown as (props: unknown) => React.ReactElement}
                  data={chartData}
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={90}
                  dataKey="total_usd"
                  nameKey="provider"
                  onMouseEnter={(_, index) => setActiveIndex(index)}
                  strokeWidth={0}
                >
                  {chartData.map((_, index) => (
                    <Cell
                      key={`cell-${index}`}
                      fill={CHART_COLORS[index % CHART_COLORS.length]}
                      opacity={activeIndex === index ? 1 : 0.75}
                    />
                  ))}
                </Pie>
                <Tooltip content={<CustomTooltip />} />
              </PieChart>
            </ResponsiveContainer>
          </div>

          {/* Custom legend */}
          <div className="mt-2 space-y-2">
            {chartData.map((entry, index) => (
              <div
                key={entry.provider}
                className="flex items-center justify-between py-1 cursor-pointer"
                onMouseEnter={() => setActiveIndex(index)}
              >
                <div className="flex items-center gap-2">
                  <div
                    className="w-2.5 h-2.5 rounded-full flex-shrink-0"
                    style={{ background: CHART_COLORS[index % CHART_COLORS.length] }}
                  />
                  <span className="text-xs text-[#94a3b8]">{entry.provider}</span>
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-xs text-[#64748b]">
                    {((entry.total_usd / total) * 100).toFixed(1)}%
                  </span>
                  <span className="text-xs font-semibold text-white font-mono">
                    ${entry.total_usd.toFixed(4)}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  )
}

export default CostByProviderChart
