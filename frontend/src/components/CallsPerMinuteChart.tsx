import React from 'react'
import {
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Area,
  AreaChart,
  ReferenceLine,
} from 'recharts'
import { format, parseISO } from 'date-fns'
import type { CallsPerMinutePoint } from '../types/api'

interface CallsPerMinuteChartProps {
  data: CallsPerMinutePoint[]
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

  let displayTime = label ?? ''
  try {
    displayTime = format(parseISO(label ?? ''), 'HH:mm')
  } catch {
    // use raw label
  }

  return (
    <div className="custom-tooltip">
      <p className="label">{displayTime}</p>
      <p className="value">{payload[0]?.value ?? 0} calls</p>
    </div>
  )
}

const CallsPerMinuteChart: React.FC<CallsPerMinuteChartProps> = ({
  data,
  isLoading,
}) => {
  const formattedData = data.map(d => {
    let label = d.minute
    try {
      label = format(parseISO(d.minute), 'HH:mm')
    } catch {
      // use raw
    }
    return { ...d, label }
  })

  const maxVal = Math.max(...data.map(d => d.call_count), 1)
  const avg = data.length
    ? Math.round(data.reduce((s, d) => s + d.call_count, 0) / data.length)
    : 0

  return (
    <div className="glass-card p-6 animate-slide-up" style={{ animationDelay: '100ms' }}>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-base font-semibold text-white">Calls Per Minute</h2>
          <p className="text-xs text-[#64748b] mt-0.5">Last 60 minutes · rolling window</p>
        </div>
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <div className="w-6 h-0.5 bg-gradient-to-r from-indigo-500 to-purple-500 rounded" />
            <span className="text-xs text-[#94a3b8]">Call Volume</span>
          </div>
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-indigo-500/10 border border-indigo-500/20">
            <span className="text-xs text-[#94a3b8]">avg</span>
            <span className="text-xs font-semibold text-indigo-400">{avg}/min</span>
          </div>
        </div>
      </div>

      {isLoading ? (
        <div className="h-64 shimmer rounded-xl" />
      ) : (
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={formattedData} margin={{ top: 5, right: 10, left: -20, bottom: 0 }}>
              <defs>
                <linearGradient id="callsGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#6366f1" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#6366f1" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid
                strokeDasharray="3 3"
                stroke="rgba(255,255,255,0.05)"
                vertical={false}
              />
              <XAxis
                dataKey="label"
                tick={{ fill: '#64748b', fontSize: 11 }}
                axisLine={false}
                tickLine={false}
                interval={Math.floor(formattedData.length / 8)}
              />
              <YAxis
                tick={{ fill: '#64748b', fontSize: 11 }}
                axisLine={false}
                tickLine={false}
                domain={[0, maxVal * 1.2]}
              />
              <Tooltip content={<CustomTooltip />} />
              {avg > 0 && (
                <ReferenceLine
                  y={avg}
                  stroke="rgba(99, 102, 241, 0.4)"
                  strokeDasharray="4 4"
                  label={{
                    value: `avg ${avg}`,
                    position: 'insideTopRight',
                    fill: '#6366f1',
                    fontSize: 10,
                  }}
                />
              )}
              <Area
                type="monotone"
                dataKey="call_count"
                stroke="#6366f1"
                strokeWidth={2.5}
                fill="url(#callsGradient)"
                dot={false}
                activeDot={{
                  r: 5,
                  fill: '#6366f1',
                  stroke: '#a5b4fc',
                  strokeWidth: 2,
                }}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  )
}

export default CallsPerMinuteChart
