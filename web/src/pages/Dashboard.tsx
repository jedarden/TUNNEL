import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Server, Users, Zap, TrendingUp } from 'lucide-react'
import StatsCard from '../components/dashboard/StatsCard'
import ConnectionCard from '../components/dashboard/ConnectionCard'
import QuickActions from '../components/dashboard/QuickActions'
import ActivityFeed, { ActivityEvent } from '../components/dashboard/ActivityFeed'
import { Connection } from '../types'

// Mock data - replace with actual API calls
const mockConnections: Connection[] = [
  {
    id: '1',
    providerId: 'ngrok-1',
    providerType: 'ngrok',
    localPort: 3000,
    publicUrl: 'https://abc123.ngrok.io',
    protocol: 'https',
    status: 'connected',
    startedAt: new Date(Date.now() - 3600000).toISOString(),
    metrics: {
      requestCount: 1234,
      bytesIn: 5242880,
      bytesOut: 10485760,
      avgResponseTime: 85,
      errorRate: 0.02,
      lastRequestAt: new Date().toISOString(),
    },
  },
  {
    id: '2',
    providerId: 'cloudflare-1',
    providerType: 'cloudflare',
    localPort: 8080,
    publicUrl: 'https://tunnel.example.com',
    protocol: 'https',
    status: 'connected',
    startedAt: new Date(Date.now() - 7200000).toISOString(),
    metrics: {
      requestCount: 5678,
      bytesIn: 20971520,
      bytesOut: 41943040,
      avgResponseTime: 120,
      errorRate: 0.01,
      lastRequestAt: new Date().toISOString(),
    },
  },
  {
    id: '3',
    providerId: 'localhost-1',
    providerType: 'localhost',
    localPort: 4000,
    publicUrl: 'http://localhost:4000',
    protocol: 'http',
    status: 'disconnected',
    startedAt: new Date(Date.now() - 86400000).toISOString(),
  },
]

const mockActivities: ActivityEvent[] = [
  {
    id: '1',
    type: 'connection',
    title: 'Ngrok tunnel connected',
    description: 'Successfully established tunnel on port 3000',
    timestamp: new Date(Date.now() - 300000).toISOString(),
    severity: 'success',
  },
  {
    id: '2',
    type: 'status_change',
    title: 'Cloudflare tunnel reconnected',
    description: 'Tunnel automatically recovered after network interruption',
    timestamp: new Date(Date.now() - 600000).toISOString(),
    severity: 'info',
  },
  {
    id: '3',
    type: 'error',
    title: 'Connection timeout',
    description: 'Failed to connect to localhost:4000 - service not responding',
    timestamp: new Date(Date.now() - 900000).toISOString(),
    severity: 'error',
  },
  {
    id: '4',
    type: 'connection',
    title: 'Cloudflare tunnel connected',
    description: 'Successfully established tunnel on port 8080',
    timestamp: new Date(Date.now() - 7200000).toISOString(),
    severity: 'success',
  },
]

const Dashboard = () => {
  const [connections, setConnections] = useState<Connection[]>(mockConnections)
  const [activities] = useState<ActivityEvent[]>(mockActivities)

  // In production, replace with actual API calls
  const { data: stats } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: async () => {
      // Simulated API call
      return {
        totalProviders: 3,
        activeConnections: connections.filter((c) => c.status === 'connected').length,
        avgLatency: Math.round(
          connections
            .filter((c) => c.metrics?.avgResponseTime)
            .reduce((acc, c) => acc + (c.metrics?.avgResponseTime || 0), 0) /
            connections.filter((c) => c.metrics?.avgResponseTime).length
        ),
        totalRequests: connections.reduce((acc, c) => acc + (c.metrics?.requestCount || 0), 0),
      }
    },
    refetchInterval: 5000, // Refresh every 5 seconds
  })

  const handleConnect = (connectionId: string) => {
    console.log('Connect:', connectionId)
    // Implement connection logic
    setConnections((prev) =>
      prev.map((c) => (c.id === connectionId ? { ...c, status: 'connecting' as const } : c))
    )
    // Simulate connection
    setTimeout(() => {
      setConnections((prev) =>
        prev.map((c) => (c.id === connectionId ? { ...c, status: 'connected' as const } : c))
      )
    }, 2000)
  }

  const handleDisconnect = (connectionId: string) => {
    console.log('Disconnect:', connectionId)
    setConnections((prev) =>
      prev.map((c) => (c.id === connectionId ? { ...c, status: 'disconnected' as const } : c))
    )
  }

  const handleConfigure = (connectionId: string) => {
    console.log('Configure:', connectionId)
    // Navigate to configuration page or open modal
  }

  const handleConnectAll = () => {
    console.log('Connect all')
    connections.forEach((c) => {
      if (c.status === 'disconnected') handleConnect(c.id)
    })
  }

  const handleDisconnectAll = () => {
    console.log('Disconnect all')
    connections.forEach((c) => {
      if (c.status === 'connected') handleDisconnect(c.id)
    })
  }

  const handleRunDiagnostics = () => {
    console.log('Run diagnostics')
    // Implement diagnostics logic
  }

  const handleOpenSettings = () => {
    console.log('Open settings')
    // Navigate to settings page
  }

  const hasActiveConnections = connections.some((c) => c.status === 'connected')

  return (
    <div className="space-y-6 p-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-gray-900 dark:text-gray-100">Dashboard</h1>
        <p className="text-gray-500 dark:text-gray-400 mt-1">
          Monitor and manage your tunnel connections
        </p>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatsCard
          icon={Server}
          label="Total Providers"
          value={stats?.totalProviders || 0}
          variant="primary"
          description="Configured"
        />
        <StatsCard
          icon={Users}
          label="Active Connections"
          value={stats?.activeConnections || 0}
          variant="success"
          description="Currently running"
          trend={{ value: 12, direction: 'up' }}
        />
        <StatsCard
          icon={Zap}
          label="Avg Latency"
          value={`${stats?.avgLatency || 0}ms`}
          variant="warning"
          description="Response time"
          trend={{ value: 5, direction: 'down' }}
        />
        <StatsCard
          icon={TrendingUp}
          label="Total Requests"
          value={stats?.totalRequests?.toLocaleString() || '0'}
          variant="default"
          description="All time"
          trend={{ value: 23, direction: 'up' }}
        />
      </div>

      {/* Main Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Connection Cards - Takes 2 columns */}
        <div className="lg:col-span-2 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold text-gray-900 dark:text-gray-100">
              Connections
            </h2>
            <span className="text-sm text-gray-500 dark:text-gray-400">
              {connections.length} total
            </span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {connections.map((connection) => (
              <ConnectionCard
                key={connection.id}
                connection={connection}
                onConnect={handleConnect}
                onDisconnect={handleDisconnect}
                onConfigure={handleConfigure}
              />
            ))}
          </div>

          {connections.length === 0 && (
            <div className="text-center py-12 text-gray-500 dark:text-gray-400">
              <Server className="h-12 w-12 mx-auto mb-3 opacity-50" />
              <p>No connections configured</p>
              <p className="text-sm mt-1">Add a provider to get started</p>
            </div>
          )}
        </div>

        {/* Sidebar - Takes 1 column */}
        <div className="space-y-4">
          {/* Quick Actions */}
          <QuickActions
            onConnectAll={handleConnectAll}
            onDisconnectAll={handleDisconnectAll}
            onRunDiagnostics={handleRunDiagnostics}
            onOpenSettings={handleOpenSettings}
            hasActiveConnections={hasActiveConnections}
          />

          {/* Activity Feed */}
          <ActivityFeed events={activities} maxItems={8} />
        </div>
      </div>
    </div>
  )
}

export default Dashboard
