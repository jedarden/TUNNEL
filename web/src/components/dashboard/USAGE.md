# Dashboard Components Usage Guide

## Quick Start

### 1. Basic Import

```tsx
// Import individual components
import StatsCard from '@/components/dashboard/StatsCard'
import ConnectionCard from '@/components/dashboard/ConnectionCard'
import QuickActions from '@/components/dashboard/QuickActions'
import ActivityFeed from '@/components/dashboard/ActivityFeed'

// Or use barrel export
import {
  StatsCard,
  ConnectionCard,
  QuickActions,
  ActivityFeed,
} from '@/components/dashboard'
```

### 2. Import with Types

```tsx
import {
  StatsCard,
  ConnectionCard,
  QuickActions,
  ActivityFeed,
  type StatsCardProps,
  type ConnectionCardProps,
  type QuickActionsProps,
  type ActivityFeedProps,
  type ActivityEvent,
} from '@/components/dashboard'
```

## Component Examples

### StatsCard

```tsx
import { Server, TrendingUp } from 'lucide-react'
import { StatsCard } from '@/components/dashboard'

function MyComponent() {
  return (
    <>
      {/* Basic usage */}
      <StatsCard
        icon={Server}
        label="Total Providers"
        value={5}
      />

      {/* With description */}
      <StatsCard
        icon={Server}
        label="Total Providers"
        value={5}
        description="Configured"
      />

      {/* With trend */}
      <StatsCard
        icon={Server}
        label="Total Providers"
        value={5}
        variant="primary"
        trend={{ value: 12, direction: 'up' }}
      />

      {/* All options */}
      <StatsCard
        icon={TrendingUp}
        label="Total Requests"
        value="6,912"
        variant="success"
        description="All time"
        trend={{ value: 23, direction: 'up' }}
      />
    </>
  )
}
```

### ConnectionCard

```tsx
import { ConnectionCard } from '@/components/dashboard'
import { Connection } from '@/types'

function MyComponent() {
  const connection: Connection = {
    id: 'conn-1',
    providerId: 'ngrok-1',
    providerType: 'ngrok',
    localPort: 3000,
    publicUrl: 'https://abc123.ngrok.io',
    protocol: 'https',
    status: 'connected',
    startedAt: new Date().toISOString(),
    metrics: {
      requestCount: 1234,
      bytesIn: 5242880,
      bytesOut: 10485760,
      avgResponseTime: 85,
      errorRate: 0.02,
    },
  }

  return (
    <ConnectionCard
      connection={connection}
      onConnect={(id) => console.log('Connect:', id)}
      onDisconnect={(id) => console.log('Disconnect:', id)}
      onConfigure={(id) => console.log('Configure:', id)}
    />
  )
}
```

### ConnectionCard with React Query

```tsx
import { ConnectionCard } from '@/components/dashboard'
import { useConnectTunnel, useDisconnectTunnel } from '@/hooks/useConnections'

function MyComponent({ connection }) {
  const connectMutation = useConnectTunnel()
  const disconnectMutation = useDisconnectTunnel()

  return (
    <ConnectionCard
      connection={connection}
      onConnect={(id) => connectMutation.mutate(id)}
      onDisconnect={(id) => disconnectMutation.mutate(id)}
      onConfigure={(id) => router.push(`/settings/connections/${id}`)}
    />
  )
}
```

### QuickActions

```tsx
import { QuickActions } from '@/components/dashboard'

function MyComponent() {
  const hasActiveConnections = true

  return (
    <QuickActions
      onConnectAll={() => console.log('Connect all')}
      onDisconnectAll={() => console.log('Disconnect all')}
      onRunDiagnostics={() => console.log('Run diagnostics')}
      onOpenSettings={() => console.log('Open settings')}
      hasActiveConnections={hasActiveConnections}
      loading={false}
    />
  )
}
```

### QuickActions with Router

```tsx
import { QuickActions } from '@/components/dashboard'
import { useNavigate } from 'react-router-dom'

function MyComponent() {
  const navigate = useNavigate()
  const { mutate: connectAll, isLoading } = useConnectAllTunnels()

  return (
    <QuickActions
      onConnectAll={() => connectAll()}
      onDisconnectAll={() => disconnectAll()}
      onRunDiagnostics={() => runDiagnostics()}
      onOpenSettings={() => navigate('/settings')}
      hasActiveConnections={hasActiveConnections}
      loading={isLoading}
    />
  )
}
```

### ActivityFeed

```tsx
import { ActivityFeed, type ActivityEvent } from '@/components/dashboard'

function MyComponent() {
  const events: ActivityEvent[] = [
    {
      id: '1',
      type: 'connection',
      title: 'Ngrok tunnel connected',
      description: 'Successfully established tunnel on port 3000',
      timestamp: new Date().toISOString(),
      severity: 'success',
    },
    {
      id: '2',
      type: 'error',
      title: 'Connection failed',
      description: 'Failed to connect to localhost:4000',
      timestamp: new Date(Date.now() - 300000).toISOString(),
      severity: 'error',
    },
  ]

  return (
    <>
      {/* Basic usage */}
      <ActivityFeed events={events} />

      {/* With custom max items */}
      <ActivityFeed events={events} maxItems={5} />

      {/* With custom className */}
      <ActivityFeed
        events={events}
        maxItems={8}
        className="lg:col-span-1"
      />
    </>
  )
}
```

### ActivityFeed with Real-time Updates

```tsx
import { ActivityFeed } from '@/components/dashboard'
import { useActivities } from '@/hooks/useActivities'
import { useWebSocket } from '@/hooks/useWebSocket'

function MyComponent() {
  const { data: activities = [] } = useActivities()

  // WebSocket will automatically update React Query cache
  useWebSocket({
    onEvent: (event) => {
      console.log('New activity:', event)
    },
  })

  return <ActivityFeed events={activities} maxItems={10} />
}
```

## Full Dashboard Page Example

```tsx
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Server, Users, Zap, TrendingUp } from 'lucide-react'
import {
  StatsCard,
  ConnectionCard,
  QuickActions,
  ActivityFeed,
} from '@/components/dashboard'

export default function Dashboard() {
  // Fetch data
  const { data: connections = [] } = useQuery({
    queryKey: ['connections'],
    queryFn: fetchConnections,
    refetchInterval: 3000,
  })

  const { data: stats } = useQuery({
    queryKey: ['stats'],
    queryFn: fetchStats,
    refetchInterval: 5000,
  })

  const { data: activities = [] } = useQuery({
    queryKey: ['activities'],
    queryFn: fetchActivities,
  })

  // Event handlers
  const handleConnect = async (id: string) => {
    await api.connect(id)
  }

  const handleDisconnect = async (id: string) => {
    await api.disconnect(id)
  }

  return (
    <div className="space-y-6 p-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Dashboard</h1>
        <p className="text-gray-500">Monitor and manage your tunnel connections</p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatsCard
          icon={Server}
          label="Total Providers"
          value={stats?.totalProviders || 0}
          variant="primary"
        />
        <StatsCard
          icon={Users}
          label="Active Connections"
          value={stats?.activeConnections || 0}
          variant="success"
          trend={{ value: 12, direction: 'up' }}
        />
        <StatsCard
          icon={Zap}
          label="Avg Latency"
          value={`${stats?.avgLatency || 0}ms`}
          variant="warning"
        />
        <StatsCard
          icon={TrendingUp}
          label="Total Requests"
          value={stats?.totalRequests?.toLocaleString() || '0'}
        />
      </div>

      {/* Main Content */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Connections */}
        <div className="lg:col-span-2 space-y-4">
          <h2 className="text-xl font-semibold">Connections</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {connections.map((connection) => (
              <ConnectionCard
                key={connection.id}
                connection={connection}
                onConnect={handleConnect}
                onDisconnect={handleDisconnect}
                onConfigure={(id) => router.push(`/settings/${id}`)}
              />
            ))}
          </div>
        </div>

        {/* Sidebar */}
        <div className="space-y-4">
          <QuickActions
            onConnectAll={() => connectAll()}
            onDisconnectAll={() => disconnectAll()}
            onRunDiagnostics={() => runDiagnostics()}
            onOpenSettings={() => router.push('/settings')}
            hasActiveConnections={connections.some(c => c.status === 'connected')}
          />
          <ActivityFeed events={activities} maxItems={8} />
        </div>
      </div>
    </div>
  )
}
```

## Styling Customization

### Custom Colors

```tsx
// Override default colors with Tailwind classes
<StatsCard
  className="border-purple-500 shadow-purple-100"
  icon={Server}
  label="Custom Stat"
  value={100}
/>
```

### Custom Icon Colors

```tsx
<StatsCard
  icon={Server}
  label="Custom Stat"
  value={100}
  variant="primary" // Uses blue by default
  className="[&_.text-blue-600]:text-purple-600" // Override icon color
/>
```

### Custom Card Spacing

```tsx
<ConnectionCard
  className="p-8" // More padding
  connection={connection}
/>
```

## TypeScript Usage

```tsx
import type {
  StatsCardProps,
  ConnectionCardProps,
  QuickActionsProps,
  ActivityFeedProps,
  ActivityEvent,
} from '@/components/dashboard'

// Custom wrapper with additional props
interface CustomStatsCardProps extends StatsCardProps {
  refreshInterval?: number
}

function CustomStatsCard({ refreshInterval, ...props }: CustomStatsCardProps) {
  // Custom logic here
  return <StatsCard {...props} />
}
```

## Testing

```tsx
import { render, screen, fireEvent } from '@testing-library/react'
import { ConnectionCard } from '@/components/dashboard'

describe('ConnectionCard', () => {
  it('calls onConnect when connect button is clicked', () => {
    const onConnect = jest.fn()
    const connection = createMockConnection({ status: 'disconnected' })

    render(<ConnectionCard connection={connection} onConnect={onConnect} />)

    fireEvent.click(screen.getByText('Connect'))
    expect(onConnect).toHaveBeenCalledWith(connection.id)
  })
})
```

## Common Patterns

### Loading State

```tsx
function Dashboard() {
  const { data, isLoading } = useQuery(['connections'], fetchConnections)

  if (isLoading) {
    return <DashboardSkeleton />
  }

  return (
    // ... render components
  )
}
```

### Error Handling

```tsx
function Dashboard() {
  const { data, error } = useQuery(['connections'], fetchConnections)

  if (error) {
    return <ErrorMessage error={error} />
  }

  return (
    // ... render components
  )
}
```

### Empty States

```tsx
function Dashboard() {
  const { data: connections = [] } = useQuery(['connections'], fetchConnections)

  return (
    <div>
      {connections.length === 0 ? (
        <EmptyState
          title="No connections"
          description="Add a provider to get started"
          action={<Button onClick={addProvider}>Add Provider</Button>}
        />
      ) : (
        connections.map(conn => <ConnectionCard key={conn.id} connection={conn} />)
      )}
    </div>
  )
}
```
