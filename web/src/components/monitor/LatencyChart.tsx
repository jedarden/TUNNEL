import { useState } from 'react'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import type { TimeRange, LatencyChartData } from '@/types/monitoring'
import { cn } from '@/lib/utils'

/**
 * Latency chart props
 */
interface LatencyChartProps {
  data: LatencyChartData | null
  loading?: boolean
  onTimeRangeChange?: (range: TimeRange) => void
  className?: string
}

/**
 * Time range options
 */
const TIME_RANGES: Array<{ value: TimeRange; label: string }> = [
  { value: '1h', label: '1 Hour' },
  { value: '6h', label: '6 Hours' },
  { value: '24h', label: '24 Hours' },
  { value: '7d', label: '7 Days' },
  { value: '30d', label: '30 Days' },
]

/**
 * Provider colors
 */
const PROVIDER_COLORS = [
  '#3b82f6', // blue
  '#10b981', // green
  '#f59e0b', // amber
  '#ef4444', // red
  '#8b5cf6', // violet
  '#ec4899', // pink
]

/**
 * Custom tooltip
 */
function CustomTooltip({ active, payload, label }: any) {
  if (!active || !payload || !payload.length) {
    return null
  }

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg p-3">
      <div className="text-xs text-gray-500 dark:text-gray-400 mb-2">
        {new Date(label).toLocaleString()}
      </div>
      <div className="space-y-1">
        {payload.map((entry: any, index: number) => (
          <div key={index} className="flex items-center gap-2">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: entry.color }}
            />
            <span className="text-sm text-gray-700 dark:text-gray-300">
              {entry.name}:
            </span>
            <span className="text-sm font-semibold text-gray-900 dark:text-white">
              {entry.value}ms
            </span>
          </div>
        ))}
      </div>
    </div>
  )
}

/**
 * Latency chart with time range selector
 */
export function LatencyChart({
  data,
  loading,
  onTimeRangeChange,
  className,
}: LatencyChartProps) {
  const [selectedRange, setSelectedRange] = useState<TimeRange>('1h')

  const handleRangeChange = (range: TimeRange) => {
    setSelectedRange(range)
    onTimeRangeChange?.(range)
  }

  const formatXAxis = (timestamp: string) => {
    const date = new Date(timestamp)

    if (selectedRange === '1h' || selectedRange === '6h') {
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    }

    if (selectedRange === '24h') {
      return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    }

    return date.toLocaleDateString([], { month: 'short', day: 'numeric' })
  }

  if (loading && !data) {
    return (
      <div className={cn('bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6', className)}>
        <div className="animate-pulse">
          <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-1/4 mb-4" />
          <div className="h-64 bg-gray-200 dark:bg-gray-700 rounded" />
        </div>
      </div>
    )
  }

  if (!data || !data.data.length) {
    return (
      <div className={cn('bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6', className)}>
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          Latency Over Time
        </h3>
        <div className="bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-lg p-4">
          <p className="text-sm text-yellow-800 dark:text-yellow-200">
            No latency data available
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className={cn('bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6', className)}>
      <div className="flex items-center justify-between mb-6">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
          Latency Over Time
        </h3>

        <div className="flex gap-2">
          {TIME_RANGES.map((range) => (
            <button
              key={range.value}
              onClick={() => handleRangeChange(range.value)}
              className={cn(
                'px-3 py-1.5 text-xs font-medium rounded-md transition-colors',
                selectedRange === range.value
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
              )}
            >
              {range.label}
            </button>
          ))}
        </div>
      </div>

      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={data.data}>
          <CartesianGrid
            strokeDasharray="3 3"
            stroke="#e5e7eb"
            className="dark:stroke-gray-700"
          />
          <XAxis
            dataKey="timestamp"
            tickFormatter={formatXAxis}
            stroke="#9ca3af"
            className="dark:stroke-gray-400"
            style={{ fontSize: '12px' }}
          />
          <YAxis
            stroke="#9ca3af"
            className="dark:stroke-gray-400"
            style={{ fontSize: '12px' }}
            label={{ value: 'Latency (ms)', angle: -90, position: 'insideLeft' }}
          />
          <Tooltip content={<CustomTooltip />} />
          <Legend
            wrapperStyle={{ fontSize: '12px' }}
            iconType="line"
          />
          {data.providers.map((provider, index) => (
            <Line
              key={provider.id}
              type="monotone"
              dataKey={provider.id}
              name={provider.name}
              stroke={provider.color || PROVIDER_COLORS[index % PROVIDER_COLORS.length]}
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 4 }}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>

      <div className="mt-4 flex items-center justify-center gap-6 text-xs">
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 rounded-full bg-green-500" />
          <span className="text-gray-600 dark:text-gray-400">Good (&lt; 200ms)</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 rounded-full bg-amber-500" />
          <span className="text-gray-600 dark:text-gray-400">Warning (200-500ms)</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 rounded-full bg-red-500" />
          <span className="text-gray-600 dark:text-gray-400">Critical (&gt; 500ms)</span>
        </div>
      </div>
    </div>
  )
}
