import { CheckCircle, XCircle, AlertCircle, Info, Clock } from 'lucide-react'
import { Card, CardHeader, CardTitle, CardContent } from '../ui/Card'
import { formatRelativeTime } from '../../lib/utils'
import { cn } from '../../lib/utils'

export interface ActivityEvent {
  id: string
  type: 'connection' | 'status_change' | 'error' | 'info'
  title: string
  description?: string
  timestamp: string
  severity?: 'success' | 'warning' | 'error' | 'info'
}

export interface ActivityFeedProps {
  events: ActivityEvent[]
  maxItems?: number
  className?: string
}

const ActivityFeed = ({ events, maxItems = 10, className }: ActivityFeedProps) => {
  const displayEvents = events.slice(0, maxItems)

  const getEventIcon = (type: ActivityEvent['type'], severity?: ActivityEvent['severity']) => {
    if (severity === 'error') return XCircle
    if (severity === 'warning') return AlertCircle
    if (severity === 'success') return CheckCircle
    if (type === 'connection') return CheckCircle
    return Info
  }

  const getEventColor = (severity?: ActivityEvent['severity']) => {
    switch (severity) {
      case 'success':
        return 'text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900/30'
      case 'warning':
        return 'text-yellow-600 dark:text-yellow-400 bg-yellow-100 dark:bg-yellow-900/30'
      case 'error':
        return 'text-red-600 dark:text-red-400 bg-red-100 dark:bg-red-900/30'
      default:
        return 'text-blue-600 dark:text-blue-400 bg-blue-100 dark:bg-blue-900/30'
    }
  }

  if (displayEvents.length === 0) {
    return (
      <Card className={className}>
        <CardHeader>
          <CardTitle>Recent Activity</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex flex-col items-center justify-center py-8 text-center">
            <Clock className="h-12 w-12 text-gray-300 dark:text-gray-700 mb-3" />
            <p className="text-sm text-gray-500 dark:text-gray-400">No recent activity</p>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card className={className}>
      <CardHeader>
        <CardTitle>Recent Activity</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {displayEvents.map((event, index) => {
            const Icon = getEventIcon(event.type, event.severity)
            const isLast = index === displayEvents.length - 1

            return (
              <div key={event.id} className="relative">
                <div className="flex gap-3">
                  {/* Icon */}
                  <div className={cn('flex-shrink-0 p-2 rounded-lg', getEventColor(event.severity))}>
                    <Icon className="h-4 w-4" />
                  </div>

                  {/* Content */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex-1">
                        <p className="text-sm font-medium text-gray-900 dark:text-gray-100">
                          {event.title}
                        </p>
                        {event.description && (
                          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                            {event.description}
                          </p>
                        )}
                      </div>
                      <time className="text-xs text-gray-500 dark:text-gray-400 flex-shrink-0">
                        {formatRelativeTime(event.timestamp)}
                      </time>
                    </div>
                  </div>
                </div>

                {/* Timeline connector */}
                {!isLast && (
                  <div className="absolute left-5 top-10 bottom-0 w-px bg-gray-200 dark:bg-gray-800" />
                )}
              </div>
            )
          })}
        </div>
      </CardContent>
    </Card>
  )
}

export default ActivityFeed
