import { useCallback, useEffect, useRef, useState } from 'react'
import type { WSMessage } from '../types'

// WebSocket hook with resilient reconnects
// Note: We intentionally do NOT batch messages here because:
// 1. React 18 already batches state updates automatically
// 2. Each WebSocket message (health_check, node_status, node_metrics) needs to be processed
// 3. Using requestAnimationFrame batching causes message loss when multiple arrive in same frame
export function useMonitorWebSocket(accountId?: string, token?: string) {
  const [connected, setConnected] = useState(false)
  const [lastMessage, setLastMessage] = useState<WSMessage | null>(null)

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const reconnectAttemptsRef = useRef(0)
  const shouldReconnectRef = useRef(true)
  const connectRef = useRef<() => void>(() => {})

  const scheduleReconnect = useCallback(() => {
    if (!shouldReconnectRef.current) return
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
    }
    const attempt = reconnectAttemptsRef.current
    const baseDelay = 1000
    const maxDelay = 30000
    const backoff = Math.min(maxDelay, baseDelay * 2 ** attempt)
    const jitter = backoff * 0.3 * Math.random()
    const delay = backoff + jitter
    reconnectTimeoutRef.current = setTimeout(() => {
      reconnectAttemptsRef.current += 1
      connectRef.current()
    }, delay)
  }, [])

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const params = new URLSearchParams()
    if (accountId) params.set('account_id', accountId)
    if (token) params.set('token', token)
    const query = params.toString()
    const url = `${protocol}//${host}/api/monitor/ws${query ? `?${query}` : ''}`

    try {
      if (wsRef.current) {
        wsRef.current.close()
      }

      const ws = new WebSocket(url)

      ws.onopen = () => {
        reconnectAttemptsRef.current = 0
        setConnected(true)
      }

      ws.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data) as WSMessage
          setLastMessage(message)
        } catch (err) {
          console.error('[WS] Failed to parse message:', err)
        }
      }

      ws.onerror = (error) => {
        console.error('[WS] Error:', error)
      }

      ws.onclose = () => {
        setConnected(false)
        wsRef.current = null
        scheduleReconnect()
      }

      wsRef.current = ws
    } catch (err) {
      console.error('[WS] Failed to connect:', err)
      scheduleReconnect()
    }
  }, [accountId, scheduleReconnect, token])

  useEffect(() => {
    connectRef.current = connect
  }, [connect])

  useEffect(() => {
    shouldReconnectRef.current = true
    connect()

    return () => {
      shouldReconnectRef.current = false
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
      if (wsRef.current) {
        wsRef.current.close()
      }
    }
  }, [connect])

  return { connected, lastMessage }
}
