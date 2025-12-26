import { useState, useEffect, useCallback, useRef } from 'react'
import { useWebSocket } from './useWebSocket'
import type { LogEntry, LogFilter } from '@/types/monitoring'

/**
 * Logs hook options
 */
interface UseLogsOptions {
  limit?: number
  autoScroll?: boolean
  bufferSize?: number
  filter?: LogFilter
}

/**
 * Hook for fetching and streaming logs
 */
export function useLogs(options: UseLogsOptions = {}) {
  const {
    limit = 500,
    autoScroll = true,
    bufferSize = 1000,
    filter = {},
  } = options

  const [logs, setLogs] = useState<LogEntry[]>([])
  const [filteredLogs, setFilteredLogs] = useState<LogEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [isPaused, setIsPaused] = useState(false)
  const [shouldAutoScroll, setShouldAutoScroll] = useState(autoScroll)

  const { subscribe, isConnected } = useWebSocket()
  const bufferRef = useRef<LogEntry[]>([])

  /**
   * Fetch initial logs
   */
  const fetchLogs = useCallback(async () => {
    try {
      setError(null)
      setLoading(true)

      const params = new URLSearchParams()
      params.append('limit', limit.toString())

      if (filter.level?.length) {
        params.append('level', filter.level.join(','))
      }
      if (filter.connectionId) {
        params.append('connectionId', filter.connectionId)
      }
      if (filter.providerId) {
        params.append('providerId', filter.providerId)
      }

      const response = await fetch(`/api/logs?${params}`)

      if (!response.ok) {
        throw new Error(`Failed to fetch logs: ${response.statusText}`)
      }

      const data = await response.json()
      setLogs(data.data || [])
      setLoading(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch logs')
      setLoading(false)
    }
  }, [limit, filter])

  /**
   * Add new log entry
   */
  const addLog = useCallback((log: LogEntry) => {
    if (isPaused) {
      bufferRef.current.push(log)
      return
    }

    setLogs((prev) => {
      const updated = [log, ...prev]
      // Keep buffer size limit
      if (updated.length > bufferSize) {
        return updated.slice(0, bufferSize)
      }
      return updated
    })
  }, [isPaused, bufferSize])

  /**
   * Apply filters to logs
   */
  const applyFilter = useCallback((logs: LogEntry[], filter: LogFilter): LogEntry[] => {
    let filtered = [...logs]

    // Filter by level
    if (filter.level?.length) {
      filtered = filtered.filter((log) => filter.level!.includes(log.level))
    }

    // Filter by search text
    if (filter.search) {
      const search = filter.search.toLowerCase()
      filtered = filtered.filter((log) =>
        log.message.toLowerCase().includes(search) ||
        log.source?.toLowerCase().includes(search)
      )
    }

    // Filter by connection ID
    if (filter.connectionId) {
      filtered = filtered.filter((log) => log.connectionId === filter.connectionId)
    }

    // Filter by provider ID
    if (filter.providerId) {
      filtered = filtered.filter((log) => log.providerId === filter.providerId)
    }

    // Filter by time range
    if (filter.startTime) {
      const startTime = new Date(filter.startTime).getTime()
      filtered = filtered.filter((log) => new Date(log.timestamp).getTime() >= startTime)
    }
    if (filter.endTime) {
      const endTime = new Date(filter.endTime).getTime()
      filtered = filtered.filter((log) => new Date(log.timestamp).getTime() <= endTime)
    }

    return filtered
  }, [])

  /**
   * Pause log streaming
   */
  const pause = useCallback(() => {
    setIsPaused(true)
    setShouldAutoScroll(false)
  }, [])

  /**
   * Resume log streaming
   */
  const resume = useCallback(() => {
    setIsPaused(false)
    setShouldAutoScroll(autoScroll)

    // Flush buffer
    if (bufferRef.current.length > 0) {
      setLogs((prev) => {
        const updated = [...bufferRef.current, ...prev]
        bufferRef.current = []
        if (updated.length > bufferSize) {
          return updated.slice(0, bufferSize)
        }
        return updated
      })
    }
  }, [autoScroll, bufferSize])

  /**
   * Clear all logs
   */
  const clear = useCallback(() => {
    setLogs([])
    bufferRef.current = []
  }, [])

  /**
   * Export logs to JSON
   */
  const exportLogs = useCallback((filename = 'logs.json') => {
    const dataStr = JSON.stringify(filteredLogs, null, 2)
    const dataUri = 'data:application/json;charset=utf-8,' + encodeURIComponent(dataStr)

    const exportFileDefaultName = filename
    const linkElement = document.createElement('a')
    linkElement.setAttribute('href', dataUri)
    linkElement.setAttribute('download', exportFileDefaultName)
    linkElement.click()
  }, [filteredLogs])

  // Subscribe to real-time log updates via WebSocket
  useEffect(() => {
    if (!isConnected) return

    const unsubscribe = subscribe<{ log: LogEntry }>('log', (data) => {
      addLog(data.log)
    })

    return unsubscribe
  }, [subscribe, isConnected, addLog])

  // Apply filters whenever logs or filter changes
  useEffect(() => {
    const filtered = applyFilter(logs, filter)
    setFilteredLogs(filtered)
  }, [logs, filter, applyFilter])

  // Initial fetch
  useEffect(() => {
    fetchLogs()
  }, [fetchLogs])

  return {
    logs: filteredLogs,
    allLogs: logs,
    loading,
    error,
    isPaused,
    shouldAutoScroll,
    isConnected,
    bufferedCount: bufferRef.current.length,
    pause,
    resume,
    clear,
    refresh: fetchLogs,
    exportLogs,
  }
}
