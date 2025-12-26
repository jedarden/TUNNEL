# Dashboard Components Overview

## Component Hierarchy

```
Dashboard (Page)
├── StatsCard × 4
│   ├── Icon
│   ├── Label & Description
│   ├── Value
│   └── Trend Indicator (optional)
│
├── ConnectionCard × N (grid)
│   ├── Provider Icon
│   ├── Status Badge
│   ├── Connection Details
│   │   ├── Public URL
│   │   ├── Latency Display
│   │   └── Error Message (if any)
│   ├── Expanded Metrics (on click)
│   │   ├── Protocol & Started Time
│   │   ├── Request Count
│   │   └── Error Rate
│   └── Action Buttons
│       ├── Connect/Disconnect
│       └── Configure
│
├── QuickActions (sidebar)
│   ├── Connect All
│   ├── Disconnect All
│   ├── Run Diagnostics
│   └── Open Settings
│
└── ActivityFeed (sidebar)
    └── Activity Events × N
        ├── Icon (status-based)
        ├── Title & Description
        └── Timestamp
```

## Base UI Components Used

### From `components/ui/`:

1. **Button** (`Button.tsx`)
   - Variants: primary, secondary, ghost, danger
   - Sizes: sm, md, lg
   - Loading state support
   - Used in: ConnectionCard, QuickActions

2. **Badge** (`Badge.tsx`)
   - Variants: success, warning, error, info, neutral
   - Sizes: sm, md, lg
   - Used in: ConnectionCard (status badges)

3. **Card** (`Card.tsx`)
   - Base container with header, content, footer
   - Hover and clickable states
   - Used in: All dashboard components

## Component Files

### Dashboard Components (`components/dashboard/`)

| File | Export | Description | Props |
|------|--------|-------------|-------|
| `StatsCard.tsx` | `StatsCard` | Metric display card | icon, label, value, trend, variant, description |
| `ConnectionCard.tsx` | `ConnectionCard` | Provider connection card | connection, onConnect, onDisconnect, onConfigure |
| `QuickActions.tsx` | `QuickActions` | Quick action buttons panel | onConnectAll, onDisconnectAll, onRunDiagnostics, onOpenSettings, hasActiveConnections, loading |
| `ActivityFeed.tsx` | `ActivityFeed` | Recent events timeline | events, maxItems, className |
| `index.ts` | All | Barrel export | - |

### Page (`pages/`)

| File | Export | Description |
|------|--------|-------------|
| `Dashboard.tsx` | `Dashboard` | Main dashboard page component |

## Data Flow

```
Dashboard Page
    │
    ├─→ React Query (useQuery)
    │   ├─ Fetch stats
    │   ├─ Fetch connections
    │   └─ Fetch activities
    │
    ├─→ State Management (useState)
    │   ├─ connections
    │   └─ activities
    │
    └─→ Event Handlers
        ├─ handleConnect
        ├─ handleDisconnect
        ├─ handleConfigure
        ├─ handleConnectAll
        ├─ handleDisconnectAll
        ├─ handleRunDiagnostics
        └─ handleOpenSettings
```

## Type Definitions

### From `types/index.ts`:

```typescript
Connection {
  id: string
  providerId: string
  providerType: ProviderType
  localPort: number
  publicUrl: string
  protocol: 'http' | 'https' | 'tcp'
  status: 'connected' | 'connecting' | 'disconnected' | 'error'
  startedAt: string
  metrics?: ConnectionMetrics
  error?: string
}

ConnectionMetrics {
  requestCount: number
  bytesIn: number
  bytesOut: number
  avgResponseTime: number
  errorRate: number
  lastRequestAt?: string
}
```

### Dashboard-specific:

```typescript
ActivityEvent {
  id: string
  type: 'connection' | 'status_change' | 'error' | 'info'
  title: string
  description?: string
  timestamp: string
  severity?: 'success' | 'warning' | 'error' | 'info'
}
```

## Styling System

### Tailwind CSS Classes

All components use Tailwind with dark mode support:

- **Layout**: grid, flex, space-y, gap
- **Colors**: gray-*, blue-*, green-*, yellow-*, red-*
- **Dark mode**: `dark:` prefix for all color classes
- **Responsive**: sm:, md:, lg: breakpoints
- **Transitions**: transition-colors, transition-shadow, transition-all

### Color Coding Standards

**Status Colors:**
- Connected: green-600
- Connecting: yellow-600
- Disconnected: gray-600
- Error: red-600

**Latency Colors:**
- Good (<100ms): green-600
- Fair (<300ms): yellow-600
- Poor (≥300ms): red-600

**Variant Colors:**
- Primary: blue-600
- Success: green-600
- Warning: yellow-600
- Error: red-600
- Neutral: gray-600

## Responsive Breakpoints

```css
/* Mobile first approach */
default: < 640px (1 column)
sm: ≥ 640px (2 columns for stats, 1 for connections)
md: ≥ 768px (2 columns for stats and connections)
lg: ≥ 1024px (4 stats, 2 connections, sidebar)
```

## Icon Usage (Lucide React)

| Component | Icons Used | Purpose |
|-----------|------------|---------|
| StatsCard | Server, Users, Zap, TrendingUp, TrendingDown, Minus | Stats display & trends |
| ConnectionCard | Globe, Zap, Settings, Play, Square, AlertCircle | Connection info & actions |
| QuickActions | Play, Square, Activity, Settings | Action buttons |
| ActivityFeed | CheckCircle, XCircle, AlertCircle, Info, Clock | Event status |

## Dependencies

```json
{
  "react": "^18.2.0",
  "react-dom": "^18.2.0",
  "@tanstack/react-query": "^5.17.0",
  "lucide-react": "^0.303.0",
  "clsx": "^2.1.0",
  "tailwind-merge": "^2.2.0"
}
```

## File Size Summary

```
StatsCard.tsx:      ~2.4 KB
ConnectionCard.tsx: ~6.3 KB
QuickActions.tsx:   ~1.8 KB
ActivityFeed.tsx:   ~4.0 KB
Dashboard.tsx:      ~8.4 KB
-----------------------------------
Total:              ~22.9 KB
```

## Performance Considerations

1. **React Query** automatically handles:
   - Caching
   - Background refetching
   - Stale data management
   - Loading states

2. **Memoization** opportunities:
   - Provider icon mapping
   - Color calculation functions
   - Event filtering

3. **Virtual scrolling** recommended for:
   - Large connection lists (>50 items)
   - Long activity feeds (>100 items)

4. **Optimization tips**:
   - Use React.memo for ConnectionCard if list is large
   - Debounce real-time updates
   - Implement pagination for activity feed
   - Use WebSocket for live metrics instead of polling
