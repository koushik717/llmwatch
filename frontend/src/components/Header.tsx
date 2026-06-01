import React from 'react'
import { format } from 'date-fns'

interface HeaderProps {
  lastUpdated: Date | null
  isConnected: boolean
  isLoading: boolean
  error: string | null
}

const Header: React.FC<HeaderProps> = ({
  lastUpdated,
  isConnected,
  isLoading,
  error,
}) => {
  return (
    <header className="relative z-10 border-b border-white/[0.06] bg-[#0a0f1e]/80 backdrop-blur-xl sticky top-0">
      <div className="max-w-[1600px] mx-auto px-6 py-4">
        <div className="flex items-center justify-between gap-4">
          {/* Logo + Title */}
          <div className="flex items-center gap-3">
            <div className="relative">
              <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center shadow-lg shadow-indigo-500/25">
                <svg
                  className="w-6 h-6 text-white"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
                </svg>
              </div>
              {/* Glow behind logo */}
              <div className="absolute inset-0 w-10 h-10 rounded-xl bg-indigo-500/30 blur-lg -z-10" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-white tracking-tight">
                LLM<span className="text-indigo-400">Watch</span>
              </h1>
              <p className="text-[10px] text-[#64748b] font-medium uppercase tracking-widest -mt-0.5">
                AI Observability Platform
              </p>
            </div>
          </div>

          {/* Center: Navigation Hint */}
          <div className="hidden md:flex items-center gap-1 px-4 py-2 rounded-xl bg-white/[0.03] border border-white/[0.06]">
            <span className="text-xs text-[#64748b]">Monitoring</span>
            <span className="text-[#475569] mx-1">/</span>
            <span className="text-xs text-[#94a3b8] font-medium">Live Dashboard</span>
          </div>

          {/* Right: Status + Updated */}
          <div className="flex items-center gap-4">
            {error && (
              <div className="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-rose-500/10 border border-rose-500/20">
                <svg className="w-3.5 h-3.5 text-rose-400" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z"
                    clipRule="evenodd"
                  />
                </svg>
                <span className="text-xs text-rose-400 font-medium">API Error</span>
              </div>
            )}

            {/* Last updated */}
            {lastUpdated && (
              <div className="hidden sm:flex items-center gap-2 text-xs text-[#64748b]">
                <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" strokeWidth="2" viewBox="0 0 24 24">
                  <circle cx="12" cy="12" r="10" />
                  <polyline points="12 6 12 12 16 14" />
                </svg>
                <span>Updated {format(lastUpdated, 'HH:mm:ss')}</span>
              </div>
            )}

            {/* Refresh indicator */}
            {isLoading && !lastUpdated && (
              <div className="flex items-center gap-2 text-xs text-[#64748b]">
                <div className="w-3.5 h-3.5 border-2 border-indigo-500/30 border-t-indigo-500 rounded-full animate-spin" />
                <span>Loading...</span>
              </div>
            )}

            {/* Live indicator */}
            <div className="flex items-center gap-2.5 px-3 py-1.5 rounded-xl bg-white/[0.03] border border-white/[0.06]">
              <div className="relative flex h-2 w-2">
                {isConnected ? (
                  <>
                    <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                    <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
                  </>
                ) : (
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-[#64748b]" />
                )}
              </div>
              <span
                className={`text-xs font-medium ${
                  isConnected ? 'text-emerald-400' : 'text-[#64748b]'
                }`}
              >
                {isConnected ? 'Live' : 'Offline'}
              </span>
            </div>
          </div>
        </div>
      </div>
    </header>
  )
}

export default Header
