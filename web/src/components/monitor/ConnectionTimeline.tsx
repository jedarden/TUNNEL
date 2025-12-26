import { useEffect, useState } from 'react'
import { CheckCircle, XCircle, AlertCircle, RefreshCw } from 'lucide-react'
import { formatRelativeTime, formatDuration } from '@/lib/utils'
import type { ConnectionEvent } from '@/types/monitoring'
import { cn } from '@/lib/utils'

/**
 * Connection timeline props
 */
interface ConnectionTimelineProps {
  events: ConnectionEvent[]
  loading?: boolean
  className?: string
}

/**
 * Get icon for event type
 */
function getEventIcon(type: ConnectionEvent['type']) {
  switch (type) {
    case 'connected':
      return <CheckCircle className="w-4 h-4 text-green-500" />
    case 'disconnected':
      return <XCircle className="w-4 h-4 text-gray-500" />
    case 'error':
      return <AlertCircle className="w-4 h-4 text-red-500" />
    case 'reconnected':
      return <RefreshCw className="w-4 h-4 text-blue-500" />
  }
}

/**
 * Get color for event type
 */
function getEventColor(type: ConnectionEvent['type']) {
  switch (type) {
    case 'connected':
      return 'border-green-500 bg-green-50 dark:bg-green-900/20'
    case 'disconnected':
      return 'border-gray-300 bg-gray-50 dark:bg-gray-800/50'
    case 'error':
      return 'border-red-500 bg-red-50 dark:bg-red-900/20'
    case 'reconnected':
      return 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
  }
}

/**
 * Get label for event type
 */
function getEventLabel(type: ConnectionEvent['type']) {
  switch (type) {
    case 'connected':
      return 'Connected'
    case 'disconnected':
      return 'Disconnected'
    case 'error':
      return 'Error'
    case 'reconnected':
      return 'Reconnected'
  }
}

/**
 * Connection timeline component
 */
export function ConnectionTimeline({
  events,
  loading,
  className,
}: ConnectionTimelineProps) {
  const [sortedEvents, setSortedEvents] = useState<ConnectionEvent[]>([])

  useEffect(() => {
    // Sort events by timestamp (newest first)
    const sorted = [...events].sort(
      (a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
    )
    setSortedEvents(sorted)
  }, [events])

  if (loading && events.length === 0) {
    return (
      <div className={cn('bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6', className)}>
        <div className="animate-pulse space-y-4">
          {[...Array(5)].map((_, i) => (
            <div key={i} className="flex gap-4">
              <div className="w-4 h-4 bg-gray-200 dark:bg-gray-700 rounded-full" />
              <div className="flex-1 space-y-2">
                <div className="h-4 bg-gray-200 dark:bg-gray-700 rounded w-1/4" />
                <div className="h-3 bg-gray-200 dark:bg-gray-700 rounded w-3/4" />
              </div>
            </div>
          ))}
        </div>
      </div>
    )
  }

  if (events.length === 0) {
    return (
      <div className={cn('bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6', className)}>
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          Connection Timeline
        </h3>
        <div className="bg-gray-50 dark:bg-gray-900/50 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
          <p className="text-sm text-gray-600 dark:text-gray-400">
            No connection events yet
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className={cn('bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-6', className)}>
      <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-6">
        Connection Timeline
      </h3>

      <div className="space-y-4">
        {sortedEvents.map((event, index) => {
          const isLast = index === sortedEvents.length - 1
          const nextEvent = sortedEvents[index + 1]
          const duration =
            nextEvent && event.type === 'connected'
              ? new Date(nextEvent.timestamp).getTime() -
                new Date(event.timestamp).getTime()
              : event.duration

          return (
            <div key={event.id} className="relative">
              <div className="flex gap-4">
                {/* Icon */}
                <div className="relative flex-shrink-0">
                  <div className="flex items-center justify-center w-8 h-8 rounded-full border-2 bg-white dark:bg-gray-800">
                    {getEventIcon(event.type)}
                  </div>
                  {!isLast && (
                    <div className="absolute left-1/2 top-8 bottom-0 w-0.5 bg-gray-200 dark:bg-gray-700 -translate-x-1/2" />
                  )}
                </div>

                {/* Content */}
                <div className="flex-1 pb-8">
                  <div
                    className={cn(
                      'rounded-lg border p-4',
                      getEventColor(event.type)
                    )}
                  >
                    <div className="flex items-start justify-between mb-2">
                      <div>
                        <div className="font-medium text-gray-900 dark:text-white">
                          {getEventLabel(event.type)}
                        </div>
                        <div className="text-sm text-gray-600 dark:text-gray-400 mt-1">
                          {event.providerName} ({event.providerType})
                        </div>
                      </div>
                      <div className="text-xs text-gray-500 dark:text-gray-400">
                        {formatRelativeTime(event.timestamp)}
                      </div>
                    </div>

                    {duration && duration > 0 && (
                      <div className="text-sm text-gray-600 dark:text-gray-400 mt-2">
                        Duration: {formatDuration(duration)}
                      </div>
                    )}

                    {event.error && (
                      <div className="mt-3 p-2 bg-red-100 dark:bg-red-900/30 border border-red-200 dark:border-red-800 rounded text-sm text-red-800 dark:text-red-200">
                        {event.error}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
