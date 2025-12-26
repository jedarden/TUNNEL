import { useState, useRef, useEffect } from 'react'
import { Search, Pause, Play, Trash2, Download, Filter } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { LogEntry, LogFilter } from '@/types/monitoring'

/**
 * Log stream props
 */
interface LogStreamProps {
  logs: LogEntry[]
  loading?: boolean
  isPaused?: boolean
  shouldAutoScroll?: boolean
  bufferedCount?: number
  onPause?: () => void
  onResume?: () => void
  onClear?: () => void
  onExport?: () => void
  onFilterChange?: (filter: LogFilter) => void
  className?: string
}

/**
 * Log level colors
 */
const LOG_LEVEL_COLORS = {
  debug: 'text-gray-600 dark:text-gray-400 bg-gray-100 dark:bg-gray-800',
  info: 'text-blue-600 dark:text-blue-400 bg-blue-100 dark:bg-blue-900/30',
  warn: 'text-yellow-600 dark:text-yellow-400 bg-yellow-100 dark:bg-yellow-900/30',
  error: 'text-red-600 dark:text-red-400 bg-red-100 dark:bg-red-900/30',
}

/**
 * Log level filter options
 */
const LOG_LEVELS: Array<{ value: LogEntry['level']; label: string }> = [
  { value: 'debug', label: 'Debug' },
  { value: 'info', label: 'Info' },
  { value: 'warn', label: 'Warning' },
  { value: 'error', label: 'Error' },
]

/**
 * Log entry component
 */
function LogEntryItem({ log }: { log: LogEntry }) {
  const timestamp = new Date(log.timestamp).toLocaleTimeString()

  return (
    <div className="flex gap-3 px-4 py-2 hover:bg-gray-50 dark:hover:bg-gray-800/50 border-b border-gray-100 dark:border-gray-800 font-mono text-sm">
      <div className="text-gray-500 dark:text-gray-400 text-xs flex-shrink-0 w-20">
        {timestamp}
      </div>
      <div className="flex-shrink-0">
        <span
          className={cn(
            'px-2 py-0.5 rounded text-xs font-medium uppercase',
            LOG_LEVEL_COLORS[log.level]
          )}
        >
          {log.level}
        </span>
      </div>
      {log.source && (
        <div className="text-gray-600 dark:text-gray-400 text-xs flex-shrink-0 min-w-[100px]">
          [{log.source}]
        </div>
      )}
      <div className="flex-1 text-gray-900 dark:text-gray-100 break-words">
        {log.message}
      </div>
    </div>
  )
}

/**
 * Real-time log stream with filtering and controls
 */
export function LogStream({
  logs,
  loading,
  isPaused,
  shouldAutoScroll = true,
  bufferedCount = 0,
  onPause,
  onResume,
  onClear,
  onExport,
  onFilterChange,
  className,
}: LogStreamProps) {
  const [search, setSearch] = useState('')
  const [selectedLevels, setSelectedLevels] = useState<LogEntry['level'][]>([])
  const [showFilters, setShowFilters] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)
  const prevLogsLengthRef = useRef(logs.length)

  // Auto-scroll to bottom when new logs arrive
  useEffect(() => {
    if (shouldAutoScroll && scrollRef.current && logs.length > prevLogsLengthRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
    prevLogsLengthRef.current = logs.length
  }, [logs, shouldAutoScroll])

  // Apply filters
  useEffect(() => {
    onFilterChange?.({
      level: selectedLevels.length > 0 ? selectedLevels : undefined,
      search: search || undefined,
    })
  }, [search, selectedLevels, onFilterChange])

  const toggleLevel = (level: LogEntry['level']) => {
    setSelectedLevels((prev) =>
      prev.includes(level) ? prev.filter((l) => l !== level) : [...prev, level]
    )
  }

  return (
    <div className={cn('bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 flex flex-col', className)}>
      {/* Header */}
      <div className="border-b border-gray-200 dark:border-gray-700 p-4">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
            Live Logs
          </h3>
          <div className="flex items-center gap-2">
            {bufferedCount > 0 && (
              <div className="text-xs text-gray-600 dark:text-gray-400 bg-yellow-100 dark:bg-yellow-900/30 px-2 py-1 rounded">
                {bufferedCount} buffered
              </div>
            )}
            <button
              onClick={isPaused ? onResume : onPause}
              className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              title={isPaused ? 'Resume' : 'Pause'}
            >
              {isPaused ? (
                <Play className="w-4 h-4 text-gray-700 dark:text-gray-300" />
              ) : (
                <Pause className="w-4 h-4 text-gray-700 dark:text-gray-300" />
              )}
            </button>
            <button
              onClick={onClear}
              className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              title="Clear logs"
            >
              <Trash2 className="w-4 h-4 text-gray-700 dark:text-gray-300" />
            </button>
            <button
              onClick={onExport}
              className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
              title="Export logs"
            >
              <Download className="w-4 h-4 text-gray-700 dark:text-gray-300" />
            </button>
            <button
              onClick={() => setShowFilters(!showFilters)}
              className={cn(
                'p-2 rounded-lg transition-colors',
                showFilters
                  ? 'bg-blue-100 dark:bg-blue-900/30 text-blue-600 dark:text-blue-400'
                  : 'hover:bg-gray-100 dark:hover:bg-gray-700 text-gray-700 dark:text-gray-300'
              )}
              title="Toggle filters"
            >
              <Filter className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Search */}
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search logs..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full pl-10 pr-4 py-2 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-900 text-gray-900 dark:text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        {/* Filters */}
        {showFilters && (
          <div className="mt-4 p-3 bg-gray-50 dark:bg-gray-900/50 rounded-lg">
            <div className="text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">
              Log Levels
            </div>
            <div className="flex flex-wrap gap-2">
              {LOG_LEVELS.map((level) => (
                <button
                  key={level.value}
                  onClick={() => toggleLevel(level.value)}
                  className={cn(
                    'px-3 py-1 text-xs rounded-md transition-colors',
                    selectedLevels.includes(level.value)
                      ? LOG_LEVEL_COLORS[level.value]
                      : 'bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-600'
                  )}
                >
                  {level.label}
                </button>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Log entries */}
      <div
        ref={scrollRef}
        className="flex-1 overflow-y-auto min-h-[400px] max-h-[600px]"
      >
        {loading && logs.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <div className="text-sm text-gray-500 dark:text-gray-400">
              Loading logs...
            </div>
          </div>
        ) : logs.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <div className="text-sm text-gray-500 dark:text-gray-400">
              No logs to display
            </div>
          </div>
        ) : (
          <div>
            {logs.map((log) => (
              <LogEntryItem key={log.id} log={log} />
            ))}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="border-t border-gray-200 dark:border-gray-700 px-4 py-2 bg-gray-50 dark:bg-gray-900/50">
        <div className="flex items-center justify-between text-xs text-gray-600 dark:text-gray-400">
          <div>
            Showing {logs.length} log{logs.length !== 1 ? 's' : ''}
          </div>
          {!shouldAutoScroll && (
            <div className="text-yellow-600 dark:text-yellow-400">
              Auto-scroll disabled
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
