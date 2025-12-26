import { useState, useEffect } from 'react'
import { Activity, RefreshCw } from 'lucide-react'
import { useMetrics } from '@/hooks/useMetrics'
import { useLogs } from '@/hooks/useLogs'
import { MetricsPanel } from '@/components/monitor/MetricsPanel'
import { LatencyChart } from '@/components/monitor/LatencyChart'
import { ConnectionTimeline } from '@/components/monitor/ConnectionTimeline'
import { LogStream } from '@/components/monitor/LogStream'
import { StatusIndicator } from '@/components/monitor/StatusIndicator'
import type { ConnectionEvent, LogFilter, TimeRange } from '@/types/monitoring'
import { cn } from '@/lib/utils'

/**
 * Monitor page - Real-time monitoring dashboard
 */
export function Monitor() {
  const [timeRange, setTimeRange] = useState<TimeRange>('1h')
  const [connectionEvents, setConnectionEvents] = useState<ConnectionEvent[]>([])
  const [logFilter, setLogFilter] = useState<LogFilter>({})

  const {
    metrics,
    chartData,
    loading: metricsLoading,
    error: metricsError,
    refresh: refreshMetrics,
    fetchChartData,
    isConnected,
  } = useMetrics({ timeRange })

  const {
    logs,
    loading: logsLoading,
    isPaused,
    shouldAutoScroll,
    bufferedCount,
    pause,
    resume,
    clear,
    refresh: refreshLogs,
    exportLogs,
  } = useLogs({ filter: logFilter })

  /**
   * Fetch connection events
   */
  const fetchConnectionEvents = async () => {
    try {
      const response = await fetch('/api/connections/events')
      if (!response.ok) {
        throw new Error('Failed to fetch connection events')
      }
      const data = await response.json()
      setConnectionEvents(data.data || [])
    } catch (err) {
      console.error('Failed to fetch connection events:', err)
    }
  }

  /**
   * Handle time range change
   */
  const handleTimeRangeChange = (range: TimeRange) => {
    setTimeRange(range)
    fetchChartData(range)
  }

  /**
   * Refresh all data
   */
  const refreshAll = async () => {
    await Promise.all([
      refreshMetrics(),
      fetchConnectionEvents(),
      refreshLogs(),
    ])
  }

  // Fetch connection events on mount
  useEffect(() => {
    fetchConnectionEvents()
  }, [])

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
            <Activity className="w-6 h-6 text-blue-600 dark:text-blue-400" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
              Real-time Monitoring
            </h1>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              Live metrics, charts, and logs
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <StatusIndicator
            status={isConnected ? 'active' : 'error'}
            label={isConnected ? 'Connected' : 'Disconnected'}
            tooltip={isConnected ? 'WebSocket connected' : 'WebSocket disconnected'}
          />
          <button
            onClick={refreshAll}
            className={cn(
              'flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors',
              metricsLoading && 'opacity-50 cursor-not-allowed'
            )}
            disabled={metricsLoading}
          >
            <RefreshCw className={cn('w-4 h-4', metricsLoading && 'animate-spin')} />
            Refresh All
          </button>
        </div>
      </div>

      {/* Error Message */}
      {metricsError && (
        <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4">
          <div className="flex items-start gap-3">
            <div className="text-red-600 dark:text-red-400">
              <Activity className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-sm font-medium text-red-800 dark:text-red-200">
                Failed to load metrics
              </h3>
              <p className="text-sm text-red-700 dark:text-red-300 mt-1">
                {metricsError}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Metrics Panel */}
      <MetricsPanel
        metrics={metrics}
        loading={metricsLoading}
      />

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Latency Chart */}
        <LatencyChart
          data={chartData}
          loading={metricsLoading}
          onTimeRangeChange={handleTimeRangeChange}
        />

        {/* Connection Timeline */}
        <ConnectionTimeline
          events={connectionEvents}
          loading={false}
        />
      </div>

      {/* Log Stream */}
      <LogStream
        logs={logs}
        loading={logsLoading}
        isPaused={isPaused}
        shouldAutoScroll={shouldAutoScroll}
        bufferedCount={bufferedCount}
        onPause={pause}
        onResume={resume}
        onClear={clear}
        onExport={exportLogs}
        onFilterChange={setLogFilter}
      />
    </div>
  )
}

export default Monitor
