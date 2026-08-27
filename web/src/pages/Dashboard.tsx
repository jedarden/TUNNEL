import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Server, Users, Zap, TrendingUp, Activity } from 'lucide-react'
import StatsCard from '../components/dashboard/StatsCard'
import ConnectionCard from '../components/dashboard/ConnectionCard'
import QuickActions from '../components/dashboard/QuickActions'
import ActivityFeed, { ActivityEvent } from '../components/dashboard/ActivityFeed'
import { Connection } from '../types'
import { connectionsAPI, metricsAPI } from '../api/client'

const Dashboard = () => {
  const [connections, setConnections] = useState<Connection[]>([])

  // Fetch real connections from API
  const { data: connectionsData, isLoading: connectionsLoading, error: connectionsError } = useQuery({
    queryKey: ['connections'],
    queryFn: async () => {
      const data = await connectionsAPI.list()
      setConnections(data)
      return data
    },
    refetchInterval: 5000, // Refresh every 5 seconds
  })

  // Fetch global stats from API
  const { data: globalMetrics } = useQuery({
    queryKey: ['global-metrics'],
    queryFn: async () => {
      return await metricsAPI.allStats({ since: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString() })
    },
    refetchInterval: 10000, // Refresh every 10 seconds
  })

  // Fetch failover events for activity feed
  const { data: failoverEvents } = useQuery({
    queryKey: ['failover-events'],
    queryFn: async () => {
      return await metricsAPI.failoverEvents({ limit: 20, since: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString() })
    },
    refetchInterval: 15000, // Refresh every 15 seconds
  })

  // Calculate aggregate stats
  const activeConnections = connections.filter((c) => c.status === 'connected').length
  const avgLatency = connections.length > 0
    ? Math.round(
        connections
          .filter((c) => c.metrics?.avgResponseTime)
          .reduce((acc, c) => acc + (c.metrics?.avgResponseTime || 0), 0) /
          connections.filter((c) => c.metrics?.avgResponseTime).length
      )
    : 0

  const totalRequests = connections.reduce((acc, c) => acc + (c.metrics?.requestCount || 0), 0)

  // Calculate uptime percentage from global metrics
  const avgUptimePercentage = globalMetrics?.stats
    ? Object.values(globalMetrics.stats).reduce((sum: number, stats: any) => sum + (stats?.uptime_percentage || 0), 0) / Object.keys(globalMetrics.stats).length
    : 0

  // Calculate total failovers from events
  const totalFailovers = failoverEvents?.events?.length || 0

  // Convert failover events to activity feed format
  const activities: ActivityEvent[] = (failoverEvents?.events || []).map((event: any, index: number) => ({
    id: `failover-${index}`,
    type: event.event_type === 'failover' ? 'status_change' : 'connection',
    title: event.event_type === 'failover' ? 'Failover Event' : 'Connection Event',
    description: event.reason || event.details?.trigger || 'Connection state changed',
    timestamp: event.timestamp || new Date().toISOString(),
    severity: event.event_type === 'failover' ? 'warning' as const : 'info' as const,
  }))

  const stats = {
    totalProviders: connections.length,
    activeConnections,
    avgLatency,
    totalRequests,
    avgUptimePercentage: Math.round(avgUptimePercentage),
    totalFailovers,
  }

  const handleConnect = (connectionId: string) => {
    console.log('Connect:', connectionId)
    // Implement connection logic via API
    setConnections((prev) =>
      prev.map((c) => (c.id === connectionId ? { ...c, status: 'connecting' as const } : c))
    )
    // Simulate connection for now
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

  if (connectionsError) {
    return (
      <div className="space-y-6 p-6">
        <div className="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4">
          <p className="text-red-800 dark:text-red-200">Failed to load connections. Please check your connection.</p>
        </div>
      </div>
    )
  }

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
          value={stats.totalProviders || 0}
          variant="primary"
          description="Configured"
        />
        <StatsCard
          icon={Users}
          label="Active Connections"
          value={stats.activeConnections || 0}
          variant="success"
          description="Currently running"
          trend={{ value: 12, direction: 'up' }}
        />
        <StatsCard
          icon={Zap}
          label="Avg Latency"
          value={`${stats.avgLatency || 0}ms`}
          variant="warning"
          description="Response time"
          trend={{ value: 5, direction: 'down' }}
        />
        <StatsCard
          icon={TrendingUp}
          label="Uptime"
          value={`${stats.avgUptimePercentage || 0}%`}
          variant="default"
          description="7-day average"
          trend={{ value: 3, direction: 'up' }}
        />
      </div>

      {/* Additional Stats Row */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <StatsCard
          icon={Activity}
          label="Total Requests"
          value={stats.totalRequests?.toLocaleString() || '0'}
          variant="default"
          description="All time"
        />
        <StatsCard
          icon={TrendingUp}
          label="Failovers (7d)"
          value={stats.totalFailovers || 0}
          variant={stats.totalFailovers > 0 ? 'warning' : 'success'}
          description="Automatic switches"
        />
        <StatsCard
          icon={Server}
          label="MTTR"
          value={globalMetrics?.stats ? `${Object.values(globalMetrics.stats).reduce((sum: number, s: any) => sum + (s?.mttr_seconds || 0), 0) / Math.max(Object.keys(globalMetrics.stats).length, 1) / 60 | 0}m` : 'N/A'}
          variant="default"
          description="Mean Time To Recover"
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

          {connectionsLoading ? (
            <div className="text-center py-12 text-gray-500 dark:text-gray-400">
              <Server className="h-12 w-12 mx-auto mb-3 opacity-50 animate-pulse" />
              <p>Loading connections...</p>
            </div>
          ) : (
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
          )}

          {!connectionsLoading && connections.length === 0 && (
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
