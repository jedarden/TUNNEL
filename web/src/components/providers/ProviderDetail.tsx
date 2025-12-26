import { useState } from 'react'
import {
  CheckCircle2,
  Circle,
  AlertCircle,
  Clock,
  Activity,
  Settings as SettingsIcon,
  Play,
  History,
} from 'lucide-react'
import { Modal } from '@/components/ui/Modal'
import { cn } from '@/lib/utils'
import { formatRelativeTime } from '@/lib/utils'
import type { ProviderInfo } from '@/types'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'

interface ProviderDetailProps {
  provider: ProviderInfo | null
  isOpen: boolean
  onClose: () => void
  onConfigure: (provider: ProviderInfo) => void
  onConnect: (provider: ProviderInfo) => void
}

// Mock latency data for the chart
const generateLatencyData = (baseLatency: number) => {
  return Array.from({ length: 20 }, (_, i) => ({
    time: new Date(Date.now() - (19 - i) * 60000).toLocaleTimeString('en-US', {
      hour: '2-digit',
      minute: '2-digit',
    }),
    latency: Math.max(0, baseLatency + Math.random() * 20 - 10),
  }))
}

// Mock connection history
const mockConnectionHistory = [
  { id: '1', timestamp: new Date(Date.now() - 3600000).toISOString(), duration: 1800000, status: 'completed' },
  { id: '2', timestamp: new Date(Date.now() - 7200000).toISOString(), duration: 3600000, status: 'completed' },
  { id: '3', timestamp: new Date(Date.now() - 10800000).toISOString(), duration: 900000, status: 'failed' },
]

export function ProviderDetail({
  provider,
  isOpen,
  onClose,
  onConfigure,
  onConnect,
}: ProviderDetailProps) {
  const [activeTab, setActiveTab] = useState<'overview' | 'history' | 'latency'>('overview')

  if (!provider) return null

  const StatusIcon = {
    connected: CheckCircle2,
    available: Circle,
    error: AlertCircle,
  }[provider.status]

  const statusColors = {
    connected: 'text-green-500',
    available: 'text-muted-foreground',
    error: 'text-destructive',
  }

  const latencyData = provider.latency ? generateLatencyData(provider.latency) : []

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={provider.name}
      footer={
        <div className="flex gap-3 justify-end">
          <button
            onClick={() => {
              onConfigure(provider)
              onClose()
            }}
            className="px-4 py-2 rounded-md border border-border bg-background hover:bg-accent transition-colors text-sm font-medium flex items-center gap-2"
          >
            <SettingsIcon className="w-4 h-4" />
            Configure
          </button>
          <button
            onClick={() => {
              onConnect(provider)
              onClose()
            }}
            disabled={provider.status === 'error'}
            className="px-4 py-2 rounded-md bg-primary text-primary-foreground hover:bg-primary/90 transition-colors text-sm font-medium flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Play className="w-4 h-4" />
            {provider.status === 'connected' ? 'Connected' : 'Connect'}
          </button>
        </div>
      }
    >
      <div className="space-y-6">
        {/* Status & Info */}
        <div className="flex items-start justify-between">
          <div className="space-y-2">
            <div className="flex items-center gap-2">
              <StatusIcon className={cn('w-5 h-5', statusColors[provider.status])} />
              <span className="text-sm font-medium capitalize">{provider.status}</span>
            </div>
            {provider.latency !== undefined && (
              <div className="flex items-center gap-2 text-sm text-muted-foreground">
                <Clock className="w-4 h-4" />
                <span>{provider.latency}ms latency</span>
              </div>
            )}
          </div>
          {provider.installed && (
            <span className="px-3 py-1 rounded-full text-xs font-medium bg-green-500/10 text-green-500 border border-green-500/20">
              Installed
            </span>
          )}
        </div>

        {/* Tabs */}
        <div className="border-b border-border">
          <div className="flex gap-6">
            {[
              { value: 'overview', label: 'Overview', icon: Activity },
              { value: 'history', label: 'History', icon: History },
              { value: 'latency', label: 'Latency', icon: Clock },
            ].map((tab) => {
              const Icon = tab.icon
              return (
                <button
                  key={tab.value}
                  onClick={() => setActiveTab(tab.value as typeof activeTab)}
                  className={cn(
                    'flex items-center gap-2 px-1 py-3 text-sm font-medium border-b-2 -mb-px transition-colors',
                    activeTab === tab.value
                      ? 'border-primary text-foreground'
                      : 'border-transparent text-muted-foreground hover:text-foreground'
                  )}
                >
                  <Icon className="w-4 h-4" />
                  {tab.label}
                </button>
              )
            })}
          </div>
        </div>

        {/* Tab Content */}
        {activeTab === 'overview' && (
          <div className="space-y-4">
            {/* Description */}
            <div>
              <h3 className="text-sm font-medium text-foreground mb-2">Description</h3>
              <p className="text-sm text-muted-foreground">{provider.description}</p>
            </div>

            {/* Features */}
            <div>
              <h3 className="text-sm font-medium text-foreground mb-2">Features</h3>
              <ul className="space-y-2">
                {provider.features.map((feature, index) => (
                  <li key={index} className="flex items-start gap-2 text-sm text-muted-foreground">
                    <CheckCircle2 className="w-4 h-4 text-green-500 mt-0.5 flex-shrink-0" />
                    <span>{feature}</span>
                  </li>
                ))}
              </ul>
            </div>

            {/* Configuration */}
            {provider.config && Object.keys(provider.config).length > 0 && (
              <div>
                <h3 className="text-sm font-medium text-foreground mb-2">Configuration</h3>
                <div className="space-y-2">
                  {Object.entries(provider.config).map(([key, value]) => (
                    <div
                      key={key}
                      className="flex justify-between items-center text-sm p-2 rounded-md bg-muted/50"
                    >
                      <span className="text-muted-foreground capitalize">
                        {key.replace(/([A-Z])/g, ' $1').trim()}
                      </span>
                      <span className="text-foreground font-mono">
                        {typeof value === 'boolean' ? (value ? 'Yes' : 'No') : String(value)}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {activeTab === 'history' && (
          <div className="space-y-3">
            <h3 className="text-sm font-medium text-foreground">Recent Connections</h3>
            {mockConnectionHistory.length > 0 ? (
              <div className="space-y-2">
                {mockConnectionHistory.map((connection) => (
                  <div
                    key={connection.id}
                    className="p-3 rounded-lg border border-border bg-card/50"
                  >
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-sm text-muted-foreground">
                        {formatRelativeTime(connection.timestamp)}
                      </span>
                      <span
                        className={cn(
                          'text-xs px-2 py-0.5 rounded-full',
                          connection.status === 'completed'
                            ? 'bg-green-500/10 text-green-500'
                            : 'bg-red-500/10 text-red-500'
                        )}
                      >
                        {connection.status}
                      </span>
                    </div>
                    <div className="text-xs text-muted-foreground">
                      Duration: {Math.floor(connection.duration / 60000)} minutes
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">No connection history available</p>
            )}
          </div>
        )}

        {activeTab === 'latency' && (
          <div className="space-y-3">
            <h3 className="text-sm font-medium text-foreground">Latency Chart</h3>
            {latencyData.length > 0 ? (
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={latencyData}>
                    <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
                    <XAxis
                      dataKey="time"
                      stroke="hsl(var(--muted-foreground))"
                      fontSize={12}
                    />
                    <YAxis
                      stroke="hsl(var(--muted-foreground))"
                      fontSize={12}
                      label={{ value: 'ms', angle: -90, position: 'insideLeft' }}
                    />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: 'hsl(var(--card))',
                        border: '1px solid hsl(var(--border))',
                        borderRadius: '0.5rem',
                      }}
                    />
                    <Line
                      type="monotone"
                      dataKey="latency"
                      stroke="hsl(var(--primary))"
                      strokeWidth={2}
                      dot={false}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <p className="text-sm text-muted-foreground">No latency data available</p>
            )}
          </div>
        )}
      </div>
    </Modal>
  )
}
