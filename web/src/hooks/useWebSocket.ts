import { useEffect, useRef, useState, useCallback } from 'react'
import type { WsMessage, WsMessageType } from '@/types'

/**
 * WebSocket connection options
 */
interface UseWebSocketOptions {
  url?: string
  reconnect?: boolean
  reconnectDelay?: number
  maxReconnectAttempts?: number
  onOpen?: () => void
  onClose?: () => void
  onError?: (error: Event) => void
}

/**
 * WebSocket hook for real-time updates
 */
export function useWebSocket(options: UseWebSocketOptions = {}) {
  const {
    url = '/ws',
    reconnect = true,
    reconnectDelay = 3000,
    maxReconnectAttempts = 5,
    onOpen,
    onClose,
    onError,
  } = options

  const [isConnected, setIsConnected] = useState(false)
  const [reconnectAttempts, setReconnectAttempts] = useState(0)

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout>>()
  const listenersRef = useRef<Map<WsMessageType, Set<(data: unknown) => void>>>(
    new Map()
  )

  /**
   * Connect to WebSocket server
   */
  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      return
    }

    try {
      // Construct WebSocket URL
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const host = window.location.host
      const wsUrl = url.startsWith('ws') ? url : `${protocol}//${host}${url}`

      const ws = new WebSocket(wsUrl)

      ws.onopen = () => {
        console.log('WebSocket connected')
        setIsConnected(true)
        setReconnectAttempts(0)
        onOpen?.()
      }

      ws.onclose = () => {
        console.log('WebSocket disconnected')
        setIsConnected(false)
        wsRef.current = null
        onClose?.()

        // Attempt reconnection
        if (reconnect && reconnectAttempts < maxReconnectAttempts) {
          console.log(
            `Reconnecting in ${reconnectDelay}ms (attempt ${reconnectAttempts + 1}/${maxReconnectAttempts})`
          )
          reconnectTimeoutRef.current = setTimeout(() => {
            setReconnectAttempts((prev) => prev + 1)
            connect()
          }, reconnectDelay)
        }
      }

      ws.onerror = (error) => {
        console.error('WebSocket error:', error)
        onError?.(error)
      }

      ws.onmessage = (event) => {
        try {
          const message: WsMessage = JSON.parse(event.data)

          // Notify all listeners for this message type
          const listeners = listenersRef.current.get(message.type)
          if (listeners) {
            listeners.forEach((callback) => {
              callback(message.data)
            })
          }
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error)
        }
      }

      wsRef.current = ws
    } catch (error) {
      console.error('Failed to create WebSocket connection:', error)
    }
  }, [url, reconnect, reconnectDelay, maxReconnectAttempts, reconnectAttempts, onOpen, onClose, onError])

  /**
   * Disconnect from WebSocket server
   */
  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
    }

    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }

    setIsConnected(false)
  }, [])

  /**
   * Subscribe to a message type
   */
  const subscribe = useCallback(
    <T = unknown>(type: WsMessageType, callback: (data: T) => void) => {
      if (!listenersRef.current.has(type)) {
        listenersRef.current.set(type, new Set())
      }

      listenersRef.current.get(type)!.add(callback as (data: unknown) => void)

      // Return unsubscribe function
      return () => {
        const listeners = listenersRef.current.get(type)
        if (listeners) {
          listeners.delete(callback as (data: unknown) => void)
          if (listeners.size === 0) {
            listenersRef.current.delete(type)
          }
        }
      }
    },
    []
  )

  /**
   * Send a message to the WebSocket server
   */
  const send = useCallback(<T = unknown>(type: WsMessageType, data: T) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      const message: WsMessage<T> = {
        type,
        data,
        timestamp: new Date().toISOString(),
      }
      wsRef.current.send(JSON.stringify(message))
    } else {
      console.warn('WebSocket is not connected')
    }
  }, [])

  // Connect on mount
  useEffect(() => {
    connect()
    return () => {
      disconnect()
    }
  }, [connect, disconnect])

  return {
    isConnected,
    reconnectAttempts,
    connect,
    disconnect,
    subscribe,
    send,
  }
}
