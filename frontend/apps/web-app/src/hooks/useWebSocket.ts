import { useEffect, useRef, useCallback } from 'react'

type WebSocketStatus = 'connecting' | 'open' | 'closed' | 'error'

interface UseWebSocketOptions {
  onMessage: (data: string) => void
  onStatusChange?: (status: WebSocketStatus) => void
  reconnectDelayMs?: number
}

/**
 * useWebSocket — connects to a WebSocket endpoint and auto-reconnects on close.
 *
 * Usage (Phase 3 upgrade — swap out usePolling in LogTable):
 *   const { close } = useWebSocket('/ws/logs', { onMessage: (data) => ... })
 *
 * The Go backend needs a matching handler:
 *   r.Get("/ws/logs", wsHandler.ServeHTTP)
 */
export function useWebSocket(url: string, options: UseWebSocketOptions) {
  const { onMessage, onStatusChange, reconnectDelayMs = 3000 } = options
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const isMounted = useRef(true)

  const connect = useCallback(() => {
    if (!isMounted.current) return

    // Build absolute ws:// URL from relative path
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}${url}`

    onStatusChange?.('connecting')
    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => onStatusChange?.('open')

    ws.onmessage = (event) => onMessage(event.data)

    ws.onerror = () => onStatusChange?.('error')

    ws.onclose = () => {
      onStatusChange?.('closed')
      if (isMounted.current) {
        reconnectTimer.current = setTimeout(connect, reconnectDelayMs)
      }
    }
  }, [url, onMessage, onStatusChange, reconnectDelayMs])

  useEffect(() => {
    isMounted.current = true
    connect()
    return () => {
      isMounted.current = false
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
    }
  }, [connect])

  return {
    close: () => {
      isMounted.current = false
      wsRef.current?.close()
    },
  }
}
