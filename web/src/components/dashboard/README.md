# Dashboard Components

This directory contains all the components used in the tunnel-web Dashboard page.

## Components

### StatsCard
Displays a statistical metric with an icon, label, value, and optional trend indicator.

**Props:**
- `icon`: LucideIcon - Icon component to display
- `label`: string - Label for the metric
- `value`: string | number - The metric value
- `trend?`: object - Optional trend data with value and direction
- `variant?`: 'default' | 'primary' | 'success' | 'warning' | 'error' - Color variant
- `description?`: string - Optional description text

**Example:**
```tsx
<StatsCard
  icon={Server}
  label="Total Providers"
  value={5}
  variant="primary"
  description="Configured"
  trend={{ value: 12, direction: 'up' }}
/>
```

### ConnectionCard
Displays a tunnel connection with status, metrics, and quick actions.

**Props:**
- `connection`: Connection - Connection object from types
- `onConnect?`: (connectionId: string) => void - Connect handler
- `onDisconnect?`: (connectionId: string) => void - Disconnect handler
- `onConfigure?`: (connectionId: string) => void - Configure handler

**Features:**
- Click to expand for detailed metrics
- Status badge with color coding
- Latency display with color coding (green < 100ms, yellow < 300ms, red >= 300ms)
- Provider icon display
- Error message display
- Quick action buttons (Connect/Disconnect/Configure)

**Example:**
```tsx
<ConnectionCard
  connection={connection}
  onConnect={(id) => console.log('Connect', id)}
  onDisconnect={(id) => console.log('Disconnect', id)}
  onConfigure={(id) => console.log('Configure', id)}
/>
```

### QuickActions
Panel with common dashboard actions.

**Props:**
- `onConnectAll?`: () => void - Connect all handler
- `onDisconnectAll?`: () => void - Disconnect all handler
- `onRunDiagnostics?`: () => void - Run diagnostics handler
- `onOpenSettings?`: () => void - Open settings handler
- `hasActiveConnections?`: boolean - Whether any connections are active
- `loading?`: boolean - Loading state

**Features:**
- Automatically disables buttons based on state
- Connect All disabled when connections are already active
- Disconnect All disabled when no active connections

**Example:**
```tsx
<QuickActions
  onConnectAll={() => console.log('Connect all')}
  onDisconnectAll={() => console.log('Disconnect all')}
  hasActiveConnections={true}
/>
```

### ActivityFeed
Displays a timeline of recent events and activities.

**Props:**
- `events`: ActivityEvent[] - Array of activity events
- `maxItems?`: number - Maximum items to display (default: 10)
- `className?`: string - Additional CSS classes

**ActivityEvent Interface:**
```ts
interface ActivityEvent {
  id: string
  type: 'connection' | 'status_change' | 'error' | 'info'
  title: string
  description?: string
  timestamp: string
  severity?: 'success' | 'warning' | 'error' | 'info'
}
```

**Features:**
- Automatic icon selection based on type and severity
- Color-coded events
- Relative timestamps (e.g., "5 minutes ago")
- Timeline connector between events
- Empty state display

**Example:**
```tsx
<ActivityFeed
  events={[
    {
      id: '1',
      type: 'connection',
      title: 'Ngrok tunnel connected',
      description: 'Successfully established on port 3000',
      timestamp: new Date().toISOString(),
      severity: 'success'
    }
  ]}
  maxItems={5}
/>
```

## Usage in Dashboard

The Dashboard page combines all these components:

```tsx
import { Dashboard } from '../pages/Dashboard'

// The Dashboard component handles:
// - Fetching connection data via React Query
// - Managing connection states
// - Handling user actions
// - Organizing components in a responsive grid layout
```

## Styling

All components use:
- **Tailwind CSS** for styling
- **Dark mode** support via `dark:` variants
- **Lucide React** for icons
- **clsx + tailwind-merge** via the `cn()` utility for class merging

## Integration with React Query

The Dashboard page uses `@tanstack/react-query` for data fetching:

```tsx
const { data: stats } = useQuery({
  queryKey: ['dashboard-stats'],
  queryFn: async () => {
    // Fetch stats from API
  },
  refetchInterval: 5000, // Auto-refresh every 5 seconds
})
```

## Future Enhancements

Potential improvements:
- WebSocket integration for real-time updates
- Connection filtering and search
- Sortable connection grid
- Export metrics to CSV
- Custom date range for activity feed
- Connection grouping by provider type
