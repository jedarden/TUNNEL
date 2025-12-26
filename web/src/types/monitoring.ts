/**
 * Monitoring-specific types
 */

/**
 * Log entry
 */
export interface LogEntry {
  id: string
  timestamp: string
  level: 'debug' | 'info' | 'warn' | 'error'
  message: string
  source?: string
  connectionId?: string
  providerId?: string
  metadata?: Record<string, unknown>
}

/**
 * Time-series metrics data point
 */
export interface MetricDataPoint {
  timestamp: string
  value: number
  providerId?: string
  connectionId?: string
}

/**
 * Latency metrics
 */
export interface LatencyMetrics {
  current: number
  avg: number
  min: number
  max: number
  p95: number
  p99: number
  history: MetricDataPoint[]
}

/**
 * Connection event
 */
export interface ConnectionEvent {
  id: string
  timestamp: string
  type: 'connected' | 'disconnected' | 'error' | 'reconnected'
  connectionId: string
  providerId: string
  providerName: string
  providerType: string
  duration?: number
  error?: string
}

/**
 * Real-time metrics snapshot
 */
export interface MetricsSnapshot {
  timestamp: string
  latency: LatencyMetrics
  uptime: number
  uptimePercentage: number
  bytesTransferred: {
    in: number
    out: number
    total: number
  }
  connections: {
    active: number
    total: number
    failed: number
  }
  requests: {
    total: number
    successful: number
    failed: number
    ratePerSecond: number
  }
}

/**
 * Time range for charts
 */
export type TimeRange = '1h' | '6h' | '24h' | '7d' | '30d'

/**
 * Chart data for latency
 */
export interface LatencyChartData {
  timeRange: TimeRange
  data: Array<{
    timestamp: string
    [key: string]: number | string
  }>
  providers: Array<{
    id: string
    name: string
    color: string
  }>
}

/**
 * Log filter options
 */
export interface LogFilter {
  level?: LogEntry['level'][]
  search?: string
  connectionId?: string
  providerId?: string
  startTime?: string
  endTime?: string
}

/**
 * WebSocket log message
 */
export interface WsLogMessage {
  log: LogEntry
}

/**
 * WebSocket metrics message
 */
export interface WsMetricsMessage {
  metrics: MetricsSnapshot
}
