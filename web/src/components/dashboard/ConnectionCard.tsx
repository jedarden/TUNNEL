import { useState } from 'react'
import { Globe, Zap, Settings, Play, Square, AlertCircle } from 'lucide-react'
import { Card, CardContent, CardFooter, CardHeader } from '../ui/Card'
import Badge from '../ui/Badge'
import Button from '../ui/Button'
import { Connection } from '../../types'
import { formatRelativeTime } from '../../lib/utils'
import { cn } from '../../lib/utils'

export interface ConnectionCardProps {
  connection: Connection
  onConnect?: (connectionId: string) => void
  onDisconnect?: (connectionId: string) => void
  onConfigure?: (connectionId: string) => void
}

const ConnectionCard = ({
  connection,
  onConnect,
  onDisconnect,
  onConfigure,
}: ConnectionCardProps) => {
  const [expanded, setExpanded] = useState(false)

  const statusConfig = {
    connected: { variant: 'success' as const, label: 'Connected' },
    connecting: { variant: 'warning' as const, label: 'Connecting' },
    disconnected: { variant: 'neutral' as const, label: 'Disconnected' },
    error: { variant: 'error' as const, label: 'Error' },
  }

  const getLatencyColor = (latency?: number) => {
    if (!latency) return 'text-gray-400'
    if (latency < 100) return 'text-green-600 dark:text-green-400'
    if (latency < 300) return 'text-yellow-600 dark:text-yellow-400'
    return 'text-red-600 dark:text-red-400'
  }

  const providerIcons: Record<string, string> = {
    ngrok: '🚇',
    cloudflare: '☁️',
    localhost: '💻',
    custom: '🔧',
  }

  const latency = connection.metrics?.avgResponseTime

  return (
    <Card clickable onClick={() => setExpanded(!expanded)}>
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between">
          <div className="flex items-center space-x-3">
            <div className="text-3xl">{providerIcons[connection.providerType] || '🔌'}</div>
            <div>
              <h3 className="font-semibold text-gray-900 dark:text-gray-100 capitalize">
                {connection.providerType}
              </h3>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Port {connection.localPort}
              </p>
            </div>
          </div>
          <Badge variant={statusConfig[connection.status].variant} size="sm">
            {statusConfig[connection.status].label}
          </Badge>
        </div>
      </CardHeader>

      <CardContent className="space-y-3">
        {/* Public URL */}
        <div className="flex items-center space-x-2 text-sm">
          <Globe className="h-4 w-4 text-gray-400" />
          <a
            href={connection.publicUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="text-blue-600 dark:text-blue-400 hover:underline truncate"
            onClick={(e) => e.stopPropagation()}
          >
            {connection.publicUrl}
          </a>
        </div>

        {/* Latency */}
        {latency !== undefined && (
          <div className="flex items-center space-x-2 text-sm">
            <Zap className={cn('h-4 w-4', getLatencyColor(latency))} />
            <span className={cn('font-medium', getLatencyColor(latency))}>
              {latency.toFixed(0)}ms
            </span>
            <span className="text-gray-500 dark:text-gray-400">latency</span>
          </div>
        )}

        {/* Error message */}
        {connection.error && (
          <div className="flex items-start space-x-2 text-sm text-red-600 dark:text-red-400">
            <AlertCircle className="h-4 w-4 mt-0.5 flex-shrink-0" />
            <span className="break-words">{connection.error}</span>
          </div>
        )}

        {/* Expanded details */}
        {expanded && (
          <div className="pt-3 mt-3 border-t border-gray-200 dark:border-gray-800 space-y-2">
            <div className="grid grid-cols-2 gap-3 text-sm">
              <div>
                <p className="text-gray-500 dark:text-gray-400">Protocol</p>
                <p className="font-medium text-gray-900 dark:text-gray-100 uppercase">
                  {connection.protocol}
                </p>
              </div>
              <div>
                <p className="text-gray-500 dark:text-gray-400">Started</p>
                <p className="font-medium text-gray-900 dark:text-gray-100">
                  {formatRelativeTime(connection.startedAt)}
                </p>
              </div>
              {connection.metrics && (
                <>
                  <div>
                    <p className="text-gray-500 dark:text-gray-400">Requests</p>
                    <p className="font-medium text-gray-900 dark:text-gray-100">
                      {connection.metrics.requestCount.toLocaleString()}
                    </p>
                  </div>
                  <div>
                    <p className="text-gray-500 dark:text-gray-400">Error Rate</p>
                    <p className="font-medium text-gray-900 dark:text-gray-100">
                      {(connection.metrics.errorRate * 100).toFixed(1)}%
                    </p>
                  </div>
                </>
              )}
            </div>
          </div>
        )}
      </CardContent>

      <CardFooter className="flex gap-2 pt-3">
        {connection.status === 'disconnected' || connection.status === 'error' ? (
          <Button
            size="sm"
            variant="primary"
            onClick={(e) => {
              e.stopPropagation()
              onConnect?.(connection.id)
            }}
            className="flex-1"
          >
            <Play className="h-4 w-4 mr-1" />
            Connect
          </Button>
        ) : (
          <Button
            size="sm"
            variant="secondary"
            onClick={(e) => {
              e.stopPropagation()
              onDisconnect?.(connection.id)
            }}
            className="flex-1"
          >
            <Square className="h-4 w-4 mr-1" />
            Disconnect
          </Button>
        )}
        <Button
          size="sm"
          variant="ghost"
          onClick={(e) => {
            e.stopPropagation()
            onConfigure?.(connection.id)
          }}
        >
          <Settings className="h-4 w-4" />
        </Button>
      </CardFooter>
    </Card>
  )
}

export default ConnectionCard
