import { useState, useEffect, useCallback } from 'react'
import { useWebSocket } from './useWebSocket'
import type { MetricsSnapshot, TimeRange, LatencyChartData } from '@/types/monitoring'

/**
 * Metrics hook options
 */
interface UseMetricsOptions {
  refreshInterval?: number
  autoRefresh?: boolean
  timeRange?: TimeRange
}

/**
 * Hook for fetching and subscribing to metrics data
 */
export function useMetrics(options: UseMetricsOptions = {}) {
  const {
    refreshInterval = 5000,
    autoRefresh = true,
    timeRange = '1h',
  } = options

  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null)
  const [chartData, setChartData] = useState<LatencyChartData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const { subscribe, isConnected } = useWebSocket()

  /**
   * Fetch current metrics snapshot
   */
  const fetchMetrics = useCallback(async () => {
    try {
      setError(null)
      const response = await fetch('/api/metrics')

      if (!response.ok) {
        throw new Error(`Failed to fetch metrics: ${response.statusText}`)
      }

      const data = await response.json()
      setMetrics(data.data)
      setLoading(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch metrics')
      setLoading(false)
    }
  }, [])

  /**
   * Fetch historical chart data
   */
  const fetchChartData = useCallback(async (range: TimeRange) => {
    try {
      const response = await fetch(`/api/metrics/latency?range=${range}`)

      if (!response.ok) {
        throw new Error(`Failed to fetch chart data: ${response.statusText}`)
      }

      const data = await response.json()
      setChartData(data.data)
    } catch (err) {
      console.error('Failed to fetch chart data:', err)
    }
  }, [])

  /**
   * Refresh all metrics data
   */
  const refresh = useCallback(async () => {
    await Promise.all([
      fetchMetrics(),
      fetchChartData(timeRange),
    ])
  }, [fetchMetrics, fetchChartData, timeRange])

  // Subscribe to real-time metrics updates via WebSocket
  useEffect(() => {
    if (!isConnected) return

    const unsubscribe = subscribe<{ metrics: MetricsSnapshot }>('system.metrics', (data) => {
      setMetrics(data.metrics)
    })

    return unsubscribe
  }, [subscribe, isConnected])

  // Initial fetch
  useEffect(() => {
    refresh()
  }, [refresh])

  // Auto-refresh interval
  useEffect(() => {
    if (!autoRefresh) return

    const interval = setInterval(() => {
      fetchMetrics()
    }, refreshInterval)

    return () => clearInterval(interval)
  }, [autoRefresh, refreshInterval, fetchMetrics])

  // Refetch chart data when time range changes
  useEffect(() => {
    fetchChartData(timeRange)
  }, [timeRange, fetchChartData])

  return {
    metrics,
    chartData,
    loading,
    error,
    refresh,
    fetchChartData,
    isConnected,
  }
}
