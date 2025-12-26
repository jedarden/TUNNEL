import { useEffect, useState } from 'react'
import { Activity, TrendingUp, Database, Link } from 'lucide-react'
import { formatBytes, formatDuration } from '@/lib/utils'
import type { MetricsSnapshot } from '@/types/monitoring'
import { StatusIndicator } from './StatusIndicator'

/**
 * Metrics panel props
 */
interface MetricsPanelProps {
  metrics: MetricsSnapshot | null
  loading?: boolean
  className?: string
}

/**
 * Individual metric card
 */
interface MetricCardProps {
  title: string
  value: string | number
  subtitle?: string
  icon: React.ReactNode
  sparkline?: number[]
  trend?: 'up' | 'down' | 'neutral'
  status?: 'active' | 'warning' | 'error' | 'inactive'
}

function MetricCard({ title, value, subtitle, icon, sparkline, trend, status }: MetricCardProps) {
  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
      <div className="flex items-start justify-between mb-2">
        <div className="flex items-center gap-2">
          <div className="text-gray-500 dark:text-gray-400">{icon}</div>
          <h3 className="text-sm font-medium text-gray-600 dark:text-gray-300">
            {title}
          </h3>
        </div>
        {status && <StatusIndicator status={status} size="sm" />}
      </div>

      <div className="flex items-end justify-between">
        <div>
          <div className="text-2xl font-bold text-gray-900 dark:text-white">
            {value}
          </div>
          {subtitle && (
            <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
              {subtitle}
            </div>
          )}
        </div>

        {sparkline && sparkline.length > 0 && (
          <div className="flex items-end gap-0.5 h-8">
            {sparkline.slice(-10).map((val, i) => {
              const max = Math.max(...sparkline)
              const height = max > 0 ? (val / max) * 100 : 0
              return (
                <div
                  key={i}
                  className="w-1 bg-blue-500 dark:bg-blue-400 rounded-t opacity-70"
                  style={{ height: `${height}%` }}
                />
              )
            })}
          </div>
        )}

        {trend && (
          <div
            className={`flex items-center gap-1 text-xs ${
              trend === 'up'
                ? 'text-green-600 dark:text-green-400'
                : trend === 'down'
                ? 'text-red-600 dark:text-red-400'
                : 'text-gray-500 dark:text-gray-400'
            }`}
          >
            <TrendingUp
              className={`w-3 h-3 ${
                trend === 'down' ? 'rotate-180' : ''
              }`}
            />
          </div>
        )}
      </div>
    </div>
  )
}

/**
 * Live metrics panel with auto-refresh
 */
export function MetricsPanel({ metrics, loading, className }: MetricsPanelProps) {
  const [latencyHistory, setLatencyHistory] = useState<number[]>([])

  // Track latency history for sparkline
  useEffect(() => {
    if (metrics?.latency.current) {
      setLatencyHistory((prev) => [...prev, metrics.latency.current].slice(-20))
    }
  }, [metrics?.latency.current])

  if (loading && !metrics) {
    return (
      <div className={className}>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {[...Array(4)].map((_, i) => (
            <div
              key={i}
              className="bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4 animate-pulse"
            >
              <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-1/2 mb-4" />
              <div className="h-8 bg-gray-200 dark:bg-gray-700 rounded w-3/4" />
            </div>
          ))}
        </div>
      </div>
    )
  }

  if (!metrics) {
    return (
      <div className={className}>
        <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
          <p className="text-sm text-yellow-800 dark:text-yellow-200">
            No metrics data available
          </p>
        </div>
      </div>
    )
  }

  const getConnectionStatus = () => {
    if (metrics.connections.active === 0) return 'inactive'
    if (metrics.connections.failed > 0) return 'warning'
    return 'active'
  }

  const getLatencyStatus = () => {
    if (metrics.latency.current > 500) return 'error'
    if (metrics.latency.current > 200) return 'warning'
    return 'active'
  }

  return (
    <div className={className}>
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <MetricCard
          title="Current Latency"
          value={`${metrics.latency.current}ms`}
          subtitle={`Avg: ${metrics.latency.avg}ms | P95: ${metrics.latency.p95}ms`}
          icon={<Activity className="w-4 h-4" />}
          sparkline={latencyHistory}
          status={getLatencyStatus()}
        />

        <MetricCard
          title="Uptime"
          value={`${metrics.uptimePercentage.toFixed(2)}%`}
          subtitle={formatDuration(metrics.uptime)}
          icon={<TrendingUp className="w-4 h-4" />}
          status={metrics.uptimePercentage >= 99 ? 'active' : 'warning'}
        />

        <MetricCard
          title="Data Transferred"
          value={formatBytes(metrics.bytesTransferred.total)}
          subtitle={`↓ ${formatBytes(metrics.bytesTransferred.in)} | ↑ ${formatBytes(
            metrics.bytesTransferred.out
          )}`}
          icon={<Database className="w-4 h-4" />}
        />

        <MetricCard
          title="Connections"
          value={metrics.connections.active}
          subtitle={`Total: ${metrics.connections.total} | Failed: ${metrics.connections.failed}`}
          icon={<Link className="w-4 h-4" />}
          status={getConnectionStatus()}
        />
      </div>

      <div className="mt-4 bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-4">
        <h3 className="text-sm font-medium text-gray-600 dark:text-gray-300 mb-3">
          Request Statistics
        </h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div>
            <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">
              Total Requests
            </div>
            <div className="text-xl font-semibold text-gray-900 dark:text-white">
              {metrics.requests.total.toLocaleString()}
            </div>
          </div>
          <div>
            <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">
              Success Rate
            </div>
            <div className="text-xl font-semibold text-green-600 dark:text-green-400">
              {metrics.requests.total > 0
                ? ((metrics.requests.successful / metrics.requests.total) * 100).toFixed(1)
                : 0}
              %
            </div>
          </div>
          <div>
            <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">
              Rate
            </div>
            <div className="text-xl font-semibold text-blue-600 dark:text-blue-400">
              {metrics.requests.ratePerSecond.toFixed(1)} req/s
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
