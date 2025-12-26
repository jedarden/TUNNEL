# Dashboard Quick Reference

## Import Components

```tsx
import {
  StatsCard,
  ConnectionCard,
  QuickActions,
  ActivityFeed,
} from '@/components/dashboard'

import { Button, Badge, Card } from '@/components/ui'
```

## Component Signatures

### StatsCard
```tsx
<StatsCard
  icon={LucideIcon}
  label="string"
  value={string | number}
  variant?: "default" | "primary" | "success" | "warning" | "error"
  description?: string
  trend?: { value: number, direction: "up" | "down" | "neutral" }
/>
```

### ConnectionCard
```tsx
<ConnectionCard
  connection={Connection}
  onConnect?: (id: string) => void
  onDisconnect?: (id: string) => void
  onConfigure?: (id: string) => void
/>
```

### QuickActions
```tsx
<QuickActions
  onConnectAll?: () => void
  onDisconnectAll?: () => void
  onRunDiagnostics?: () => void
  onOpenSettings?: () => void
  hasActiveConnections?: boolean
  loading?: boolean
/>
```

### ActivityFeed
```tsx
<ActivityFeed
  events={ActivityEvent[]}
  maxItems?: number
  className?: string
/>
```

### Button
```tsx
<Button
  variant?: "primary" | "secondary" | "ghost" | "danger"
  size?: "sm" | "md" | "lg"
  loading?: boolean
/>
```

### Badge
```tsx
<Badge
  variant?: "success" | "warning" | "error" | "info" | "neutral"
  size?: "sm" | "md" | "lg"
/>
```

### Card
```tsx
<Card hover clickable>
  <CardHeader>
    <CardTitle>Title</CardTitle>
    <CardDescription>Description</CardDescription>
  </CardHeader>
  <CardContent>Content</CardContent>
  <CardFooter>Footer</CardFooter>
</Card>
```

## Type Definitions

```typescript
// Connection (from types/index.ts)
interface Connection {
  id: string
  providerId: string
  providerType: 'ngrok' | 'cloudflare' | 'localhost' | 'custom'
  localPort: number
  publicUrl: string
  protocol: 'http' | 'https' | 'tcp'
  status: 'connected' | 'connecting' | 'disconnected' | 'error'
  startedAt: string
  metrics?: ConnectionMetrics
  error?: string
}

// ActivityEvent (from dashboard/ActivityFeed.tsx)
interface ActivityEvent {
  id: string
  type: 'connection' | 'status_change' | 'error' | 'info'
  title: string
  description?: string
  timestamp: string
  severity?: 'success' | 'warning' | 'error' | 'info'
}
```

## Color Coding

### Status
- Connected: Green (`green-600`)
- Connecting: Yellow (`yellow-600`)
- Disconnected: Gray (`gray-600`)
- Error: Red (`red-600`)

### Latency
- < 100ms: Green
- < 300ms: Yellow
- ≥ 300ms: Red

### Variants
- Primary: Blue (`blue-600`)
- Success: Green (`green-600`)
- Warning: Yellow (`yellow-600`)
- Error: Red (`red-600`)
- Neutral: Gray (`gray-600`)

## Responsive Grid

```tsx
// Stats: 1 col mobile, 2 col tablet, 4 col desktop
<div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">

// Main: 1 col mobile, 3 col desktop (2:1 ratio)
<div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
  <div className="lg:col-span-2">Connections</div>
  <div>Sidebar</div>
</div>
```

## React Query Setup

```tsx
const { data: stats } = useQuery({
  queryKey: ['dashboard-stats'],
  queryFn: fetchStats,
  refetchInterval: 5000,
})

const { data: connections } = useQuery({
  queryKey: ['connections'],
  queryFn: fetchConnections,
  refetchInterval: 3000,
})
```

## Icons Used

```tsx
import {
  Server,        // Providers
  Users,         // Connections
  Zap,           // Latency
  TrendingUp,    // Trends
  TrendingDown,  // Trends
  Globe,         // URLs
  Play,          // Connect
  Square,        // Disconnect
  Settings,      // Configure
  Activity,      // Diagnostics
  CheckCircle,   // Success
  XCircle,       // Error
  AlertCircle,   // Warning
  Info,          // Info
  Clock,         // Time
} from 'lucide-react'
```

## File Paths

```
/workspaces/ardenone-cluster/tunnel-web/web/src/
├── pages/Dashboard.tsx
├── components/
│   ├── dashboard/
│   │   ├── StatsCard.tsx
│   │   ├── ConnectionCard.tsx
│   │   ├── QuickActions.tsx
│   │   ├── ActivityFeed.tsx
│   │   └── index.ts
│   └── ui/
│       ├── Button.tsx
│       ├── Badge.tsx
│       ├── Card.tsx
│       └── index.ts
└── types/index.ts
```

## Common Tasks

### Add to Router
```tsx
import Dashboard from '@/pages/Dashboard'

<Route path="/" element={<Dashboard />} />
```

### Connect to API
```tsx
// Replace mock data
const { data: connections } = useConnections()
const { data: activities } = useActivities()
```

### Handle Actions
```tsx
const connect = useConnectTunnel()
const disconnect = useDisconnectTunnel()

<ConnectionCard
  onConnect={(id) => connect.mutate(id)}
  onDisconnect={(id) => disconnect.mutate(id)}
/>
```

### WebSocket Updates
```tsx
useWebSocket({
  onConnectionStatus: (update) => {
    queryClient.invalidateQueries(['connections'])
  }
})
```

## Testing

```tsx
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Dashboard from '@/pages/Dashboard'

const queryClient = new QueryClient()

test('renders dashboard', () => {
  render(
    <QueryClientProvider client={queryClient}>
      <Dashboard />
    </QueryClientProvider>
  )

  expect(screen.getByText('Dashboard')).toBeInTheDocument()
})
```

## Documentation Links

- Component API: `/components/dashboard/README.md`
- Architecture: `/components/dashboard/COMPONENTS.md`
- Visual Guide: `/components/dashboard/VISUAL_GUIDE.md`
- Usage Examples: `/components/dashboard/USAGE.md`
- Integration: `/pages/DashboardExample.md`
- Summary: `/DASHBOARD_SUMMARY.md`
