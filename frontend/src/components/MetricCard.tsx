import React, { ReactNode } from 'react'
import clsx from 'clsx'

interface MetricCardProps {
  title: string
  value: string | number
  subtitle?: string
  icon: ReactNode
  trend?: {
    value: number
    label: string
    positive?: boolean
  }
  accentColor?: 'indigo' | 'emerald' | 'amber' | 'rose'
  isLoading?: boolean
  className?: string
  animationDelay?: number
}

const accentMap = {
  indigo: {
    icon: 'text-indigo-400',
    iconBg: 'bg-indigo-500/10',
    border: 'border-indigo-500/20',
    glow: 'hover:shadow-[0_0_30px_rgba(99,102,241,0.15)]',
    trend: 'text-indigo-400',
  },
  emerald: {
    icon: 'text-emerald-400',
    iconBg: 'bg-emerald-500/10',
    border: 'border-emerald-500/20',
    glow: 'hover:shadow-[0_0_30px_rgba(16,185,129,0.15)]',
    trend: 'text-emerald-400',
  },
  amber: {
    icon: 'text-amber-400',
    iconBg: 'bg-amber-500/10',
    border: 'border-amber-500/20',
    glow: 'hover:shadow-[0_0_30px_rgba(245,158,11,0.15)]',
    trend: 'text-amber-400',
  },
  rose: {
    icon: 'text-rose-400',
    iconBg: 'bg-rose-500/10',
    border: 'border-rose-500/20',
    glow: 'hover:shadow-[0_0_30px_rgba(239,68,68,0.15)]',
    trend: 'text-rose-400',
  },
}

const MetricCard: React.FC<MetricCardProps> = ({
  title,
  value,
  subtitle,
  icon,
  trend,
  accentColor = 'indigo',
  isLoading = false,
  className,
  animationDelay = 0,
}) => {
  const accent = accentMap[accentColor]

  return (
    <div
      className={clsx(
        'glass-card-hover p-6 animate-slide-up cursor-default',
        accent.glow,
        className,
      )}
      style={{ animationDelay: `${animationDelay}ms` }}
    >
      <div className="flex items-start justify-between">
        <div className="flex-1 min-w-0">
          <p className="text-xs font-medium text-[#94a3b8] uppercase tracking-wider mb-1">
            {title}
          </p>
          {isLoading ? (
            <div className="space-y-2 mt-2">
              <div className="h-8 w-32 rounded-lg shimmer" />
              <div className="h-3 w-20 rounded shimmer" />
            </div>
          ) : (
            <>
              <p className="text-3xl font-bold text-white tracking-tight leading-none mt-2">
                {value}
              </p>
              {subtitle && (
                <p className="text-xs text-[#94a3b8] mt-1.5">{subtitle}</p>
              )}
              {trend && (
                <div className="flex items-center gap-1.5 mt-3">
                  <span
                    className={clsx(
                      'text-xs font-medium',
                      trend.positive !== false
                        ? 'text-emerald-400'
                        : 'text-rose-400',
                    )}
                  >
                    {trend.positive !== false ? '↑' : '↓'} {Math.abs(trend.value)}%
                  </span>
                  <span className="text-xs text-[#64748b]">{trend.label}</span>
                </div>
              )}
            </>
          )}
        </div>
        <div
          className={clsx(
            'flex items-center justify-center w-12 h-12 rounded-xl ml-4 flex-shrink-0',
            accent.iconBg,
          )}
        >
          <span className={clsx('w-6 h-6', accent.icon)}>{icon}</span>
        </div>
      </div>
    </div>
  )
}

export default MetricCard
