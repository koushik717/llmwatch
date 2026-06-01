import { useState, useEffect, useRef, useCallback } from 'react'
import type { LLMCallEvent } from '../types/api'

let API_BASE = import.meta.env.VITE_API_URL
if (!API_BASE || API_BASE === '""' || API_BASE === "''") {
  API_BASE = import.meta.env.DEV ? 'http://localhost:8080' : ''
}
const MAX_EVENTS = 50

export interface SSEState {
  events: LLMCallEvent[]
  connected: boolean
  error: string | null
}

export function useSSE(): SSEState {
  const [state, setState] = useState<SSEState>({
    events: [],
    connected: false,
    error: null,
  })

  const esRef = useRef<EventSource | null>(null)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const isMounted = useRef(true)

  const connect = useCallback(() => {
    if (!isMounted.current) return

    // Clean up any existing connection
    if (esRef.current) {
      esRef.current.close()
      esRef.current = null
    }

    try {
      const es = new EventSource(`${API_BASE}/events/stream`)
      esRef.current = es

      es.onopen = () => {
        if (!isMounted.current) return
        setState(prev => ({ ...prev, connected: true, error: null }))
      }

      es.onmessage = (event: MessageEvent) => {
        if (!isMounted.current) return
        try {
          const data = JSON.parse(event.data) as LLMCallEvent
          setState(prev => ({
            ...prev,
            events: [data, ...prev.events].slice(0, MAX_EVENTS),
          }))
        } catch {
          // Ignore parse errors for individual events
        }
      }

      es.addEventListener('llm_call', (event: MessageEvent) => {
        if (!isMounted.current) return
        try {
          const data = JSON.parse(event.data) as LLMCallEvent
          setState(prev => ({
            ...prev,
            events: [data, ...prev.events].slice(0, MAX_EVENTS),
          }))
        } catch {
          // Ignore parse errors
        }
      })

      es.onerror = () => {
        if (!isMounted.current) return
        es.close()
        esRef.current = null
        setState(prev => ({
          ...prev,
          connected: false,
          error: 'SSE connection lost. Reconnecting...',
        }))
        // Reconnect after 3 seconds
        reconnectTimer.current = setTimeout(() => {
          if (isMounted.current) connect()
        }, 3000)
      }
    } catch (err) {
      if (!isMounted.current) return
      const message = err instanceof Error ? err.message : 'Failed to connect'
      setState(prev => ({ ...prev, connected: false, error: message }))
    }
  }, [])

  useEffect(() => {
    isMounted.current = true
    connect()

    return () => {
      isMounted.current = false
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      if (esRef.current) {
        esRef.current.close()
        esRef.current = null
      }
    }
  }, [connect])

  return state
}
