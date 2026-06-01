import React from 'react'
import clsx from 'clsx'

interface StatusBadgeProps {
  status: string
  className?: string
}

const StatusBadge: React.FC<StatusBadgeProps> = ({ status, className }) => {
  const normalized = status?.toLowerCase()

  const config = {
    success: {
      dot: 'bg-emerald-500',
      text: 'text-emerald-400',
      bg: 'bg-emerald-500/10 border-emerald-500/20',
      label: 'Success',
    },
    error: {
      dot: 'bg-rose-500',
      text: 'text-rose-400',
      bg: 'bg-rose-500/10 border-rose-500/20',
      label: 'Error',
    },
    timeout: {
      dot: 'bg-amber-500',
      text: 'text-amber-400',
      bg: 'bg-amber-500/10 border-amber-500/20',
      label: 'Timeout',
    },
    pending: {
      dot: 'bg-indigo-500 animate-pulse',
      text: 'text-indigo-400',
      bg: 'bg-indigo-500/10 border-indigo-500/20',
      label: 'Pending',
    },
  }

  const style = config[normalized as keyof typeof config] ?? {
    dot: 'bg-slate-500',
    text: 'text-slate-400',
    bg: 'bg-slate-500/10 border-slate-500/20',
    label: status,
  }

  return (
    <span
      className={clsx(
        'badge border',
        style.bg,
        style.text,
        className,
      )}
    >
      <span className={clsx('h-1.5 w-1.5 rounded-full', style.dot)} />
      {style.label}
    </span>
  )
}

export default StatusBadge
