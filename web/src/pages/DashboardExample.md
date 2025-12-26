# Dashboard Integration Example

## Adding Dashboard to Your App

To integrate the Dashboard into your application, add it to your router configuration:

```tsx
// App.tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Dashboard from './pages/Dashboard'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
    },
  },
})

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<Dashboard />} />
          {/* Other routes */}
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  )
}

export default App
```

## Connecting to Real API

Replace the mock data in `Dashboard.tsx` with actual API calls:

```tsx
import { useConnections } from '../hooks/useConnections'
import { useActivities } from '../hooks/useActivities'

const Dashboard = () => {
  // Replace mock data with real hooks
  const { data: connections, isLoading } = useConnections()
  const { data: activities } = useActivities()

  const { data: stats } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: async () => {
      const response = await fetch('/api/stats')
      return response.json()
    },
    refetchInterval: 5000,
  })

  // Rest of component...
}
```

## Creating API Hooks

Create custom hooks for data fetching:

```tsx
// hooks/useConnections.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Connection } from '../types'

export function useConnections() {
  return useQuery({
    queryKey: ['connections'],
    queryFn: async () => {
      const response = await fetch('/api/connections')
      const data = await response.json()
      return data as Connection[]
    },
    refetchInterval: 3000, // Refresh every 3 seconds
  })
}

export function useConnectTunnel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (connectionId: string) => {
      const response = await fetch(`/api/connections/${connectionId}/connect`, {
        method: 'POST',
      })
      return response.json()
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connections'] })
    },
  })
}

export function useDisconnectTunnel() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (connectionId: string) => {
      const response = await fetch(`/api/connections/${connectionId}/disconnect`, {
        method: 'POST',
      })
      return response.json()
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connections'] })
    },
  })
}
```

## WebSocket Integration

For real-time updates, integrate WebSocket:

```tsx
import { useWebSocket } from '../hooks/useWebSocket'

const Dashboard = () => {
  const { data: connections } = useConnections()

  // Subscribe to real-time updates
  useWebSocket({
    onConnectionStatus: (update) => {
      console.log('Connection status changed:', update)
      // Update will be reflected via React Query cache invalidation
    },
    onMetrics: (metrics) => {
      console.log('Metrics update:', metrics)
    },
  })

  // Rest of component...
}
```

## Example API Response Format

Your backend should return data in this format:

```json
// GET /api/connections
[
  {
    "id": "conn-1",
    "providerId": "ngrok-1",
    "providerType": "ngrok",
    "localPort": 3000,
    "publicUrl": "https://abc123.ngrok.io",
    "protocol": "https",
    "status": "connected",
    "startedAt": "2024-01-15T10:30:00Z",
    "metrics": {
      "requestCount": 1234,
      "bytesIn": 5242880,
      "bytesOut": 10485760,
      "avgResponseTime": 85,
      "errorRate": 0.02,
      "lastRequestAt": "2024-01-15T12:45:00Z"
    }
  }
]

// GET /api/stats
{
  "totalProviders": 3,
  "activeConnections": 2,
  "avgLatency": 102,
  "totalRequests": 6912
}

// GET /api/activities
[
  {
    "id": "evt-1",
    "type": "connection",
    "title": "Ngrok tunnel connected",
    "description": "Successfully established tunnel on port 3000",
    "timestamp": "2024-01-15T12:45:00Z",
    "severity": "success"
  }
]
```

## Customization

### Custom Styling

Override Tailwind classes:

```tsx
<StatsCard
  className="shadow-lg border-2"
  icon={Server}
  label="Custom Stat"
  value={100}
/>
```

### Custom Provider Icons

Replace emoji icons with actual logos:

```tsx
// In ConnectionCard.tsx
const providerIcons: Record<string, React.ReactNode> = {
  ngrok: <img src="/logos/ngrok.svg" alt="Ngrok" className="w-8 h-8" />,
  cloudflare: <img src="/logos/cloudflare.svg" alt="Cloudflare" className="w-8 h-8" />,
  localhost: <Server className="w-8 h-8" />,
  custom: <Wrench className="w-8 h-8" />,
}
```

### Custom Actions

Add more quick actions:

```tsx
// In QuickActions.tsx
<Button
  variant="ghost"
  onClick={onExportMetrics}
  className="w-full"
>
  <Download className="h-4 w-4 mr-2" />
  Export Metrics
</Button>
```

## Testing

Example test for Dashboard component:

```tsx
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Dashboard from './Dashboard'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
})

describe('Dashboard', () => {
  it('renders stats cards', () => {
    render(
      <QueryClientProvider client={queryClient}>
        <Dashboard />
      </QueryClientProvider>
    )

    expect(screen.getByText('Total Providers')).toBeInTheDocument()
    expect(screen.getByText('Active Connections')).toBeInTheDocument()
  })
})
```
