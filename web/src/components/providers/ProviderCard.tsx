import { Settings, Play, CheckCircle2, Circle, AlertCircle, Clock } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { ProviderInfo } from '@/types'

interface ProviderCardProps {
  provider: ProviderInfo
  onConfigure: (provider: ProviderInfo) => void
  onConnect: (provider: ProviderInfo) => void
  onViewDetails: (provider: ProviderInfo) => void
}

const categoryColors: Record<string, string> = {
  'vpn-mesh': 'bg-blue-500/10 text-blue-500 border-blue-500/20',
  'tunnels': 'bg-purple-500/10 text-purple-500 border-purple-500/20',
  'ssh': 'bg-green-500/10 text-green-500 border-green-500/20',
}

const categoryLabels: Record<string, string> = {
  'vpn-mesh': 'VPN/Mesh',
  'tunnels': 'Tunnel',
  'ssh': 'SSH',
}

export function ProviderCard({
  provider,
  onConfigure,
  onConnect,
  onViewDetails,
}: ProviderCardProps) {
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

  return (
    <div
      className={cn(
        'group relative p-6 rounded-lg border border-border bg-card',
        'hover:border-primary/50 hover:shadow-lg transition-all duration-200',
        'cursor-pointer'
      )}
      onClick={() => onViewDetails(provider)}
    >
      {/* Status Badge */}
      <div className="absolute top-4 right-4">
        <StatusIcon className={cn('w-5 h-5', statusColors[provider.status])} />
      </div>

      {/* Provider Icon & Name */}
      <div className="mb-4">
        {provider.icon ? (
          <div className="w-12 h-12 rounded-lg bg-muted flex items-center justify-center mb-3">
            <span className="text-2xl">{provider.icon}</span>
          </div>
        ) : (
          <div className="w-12 h-12 rounded-lg bg-gradient-to-br from-primary/20 to-primary/5 flex items-center justify-center mb-3">
            <span className="text-xl font-bold text-primary">
              {provider.name.charAt(0)}
            </span>
          </div>
        )}
        <h3 className="text-lg font-semibold text-foreground">{provider.name}</h3>
      </div>

      {/* Category Badge */}
      <div className="mb-3">
        <span
          className={cn(
            'inline-flex px-2.5 py-1 rounded-full text-xs font-medium border',
            categoryColors[provider.category] || 'bg-muted text-muted-foreground border-border'
          )}
        >
          {categoryLabels[provider.category] || provider.category}
        </span>
      </div>

      {/* Description */}
      <p className="text-sm text-muted-foreground line-clamp-2 mb-4">
        {provider.description}
      </p>

      {/* Latency */}
      {provider.latency !== undefined && (
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground mb-4">
          <Clock className="w-3.5 h-3.5" />
          <span>{provider.latency}ms</span>
        </div>
      )}

      {/* Installed Badge */}
      {provider.installed && (
        <div className="flex items-center gap-1.5 text-xs text-green-500 mb-4">
          <CheckCircle2 className="w-3.5 h-3.5" />
          <span>Installed</span>
        </div>
      )}

      {/* Action Buttons */}
      <div className="flex gap-2 mt-auto">
        <button
          onClick={(e) => {
            e.stopPropagation()
            onConfigure(provider)
          }}
          className={cn(
            'flex-1 px-3 py-2 rounded-md text-sm font-medium',
            'border border-border bg-background',
            'hover:bg-accent transition-colors',
            'flex items-center justify-center gap-2'
          )}
        >
          <Settings className="w-4 h-4" />
          Configure
        </button>
        <button
          onClick={(e) => {
            e.stopPropagation()
            onConnect(provider)
          }}
          disabled={provider.status === 'error'}
          className={cn(
            'flex-1 px-3 py-2 rounded-md text-sm font-medium',
            'bg-primary text-primary-foreground',
            'hover:bg-primary/90 transition-colors',
            'flex items-center justify-center gap-2',
            'disabled:opacity-50 disabled:cursor-not-allowed'
          )}
        >
          <Play className="w-4 h-4" />
          {provider.status === 'connected' ? 'Connected' : 'Connect'}
        </button>
      </div>
    </div>
  )
}
