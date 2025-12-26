# Real-time Monitoring Dashboard

The monitoring dashboard provides comprehensive real-time visibility into tunnel connections, performance metrics, and system logs.

## Files Created

### Main Page
- **`/pages/Monitor.tsx`** - Main monitoring dashboard that orchestrates all components

### Components
- **`/components/monitor/MetricsPanel.tsx`** - Live metrics display with sparklines
- **`/components/monitor/LatencyChart.tsx`** - Interactive latency chart using Recharts
- **`/components/monitor/ConnectionTimeline.tsx`** - Connection event timeline
- **`/components/monitor/LogStream.tsx`** - Real-time log streaming with filtering
- **`/components/monitor/StatusIndicator.tsx`** - Reusable status indicator component
- **`/components/monitor/index.ts`** - Barrel export for all monitor components

### Hooks
- **`/hooks/useMetrics.ts`** - Metrics data fetching and WebSocket subscription
- **`/hooks/useLogs.ts`** - Log streaming and buffering management

### Types
- **`/types/monitoring.ts`** - All monitoring-related TypeScript types

## Features

### 1. Live Metrics Panel
- **Current Latency** - Real-time latency with sparkline visualization
  - Current, average, and P95 values
  - Color-coded status (green/yellow/red)
  - 20-point history sparkline
- **Uptime** - System uptime percentage and duration
- **Data Transferred** - Total, incoming, and outgoing bytes
- **Connections** - Active, total, and failed connection counts
- **Request Statistics** - Total requests, success rate, and request rate

Auto-refreshes every 5 seconds (configurable)

### 2. Latency Chart
- **Time-series visualization** using Recharts
- **Multiple time ranges**: 1h, 6h, 24h, 7d, 30d
- **Multi-provider support** - Different lines for each provider
- **Color-coded by status**:
  - Green: < 200ms (good)
  - Amber: 200-500ms (warning)
  - Red: > 500ms (critical)
- **Interactive tooltips** with exact values and timestamps
- **Responsive design** adapts to screen size

### 3. Connection Timeline
- **Event tracking** for all connection state changes
- **Event types**:
  - Connected (green)
  - Disconnected (gray)
  - Error (red)
  - Reconnected (blue)
- **Duration display** between events
- **Provider information** on each event
- **Error highlighting** with detailed error messages
- **Relative timestamps** (e.g., "2 minutes ago")

### 4. Log Stream
- **Real-time streaming** via WebSocket
- **Virtualized list** for performance with large log volumes
- **Level filtering**: Debug, Info, Warning, Error
- **Search/filter** by text, connection ID, provider ID
- **Pause/Resume** - Pause streaming and buffer new logs
- **Auto-scroll toggle** - Automatically scroll to newest logs
- **Export** - Download logs as JSON
- **Clear** - Clear current log buffer
- **Buffered count** indicator when paused
- **Color-coded levels**:
  - Debug: Gray
  - Info: Blue
  - Warning: Yellow
  - Error: Red

### 5. Status Indicator
- **Reusable component** for status display
- **Animated pulse** for active connections
- **Color-coded states**:
  - Active: Green (with pulse animation)
  - Warning: Yellow
  - Error: Red
  - Inactive: Gray
- **Size variants**: sm, md, lg
- **Optional label and tooltip**

## Real-time Updates

The dashboard uses WebSocket connections for real-time updates:

### WebSocket Message Types
- `system.metrics` - System-wide metrics updates
- `log` - Individual log entries
- `connection.status` - Connection status changes
- `connection.metrics` - Connection-specific metrics

### Auto-refresh Strategy
- **Metrics**: Updates every 5 seconds via WebSocket + polling fallback
- **Logs**: Real-time WebSocket stream with buffering when paused
- **Charts**: Refetched when time range changes
- **Timeline**: Refetched on manual refresh

## API Endpoints Expected

The monitoring dashboard expects the following API endpoints:

### Metrics
```typescript
GET /api/metrics
Response: { data: MetricsSnapshot }

GET /api/metrics/latency?range={1h|6h|24h|7d|30d}
Response: { data: LatencyChartData }
```

### Logs
```typescript
GET /api/logs?limit={number}&level={debug,info,warn,error}&connectionId={id}&providerId={id}
Response: { data: LogEntry[] }
```

### Connection Events
```typescript
GET /api/connections/events
Response: { data: ConnectionEvent[] }
```

### WebSocket
```typescript
ws://localhost:PORT/ws

Messages:
- { type: 'system.metrics', data: { metrics: MetricsSnapshot }, timestamp: string }
- { type: 'log', data: { log: LogEntry }, timestamp: string }
```

## Usage Example

```tsx
import { Monitor } from '@/pages/Monitor'

// In your router
<Route path="/monitor" element={<Monitor />} />
```

## Component Props

### MetricsPanel
```typescript
interface MetricsPanelProps {
  metrics: MetricsSnapshot | null
  loading?: boolean
  className?: string
}
```

### LatencyChart
```typescript
interface LatencyChartProps {
  data: LatencyChartData | null
  loading?: boolean
  onTimeRangeChange?: (range: TimeRange) => void
  className?: string
}
```

### ConnectionTimeline
```typescript
interface ConnectionTimelineProps {
  events: ConnectionEvent[]
  loading?: boolean
  className?: string
}
```

### LogStream
```typescript
interface LogStreamProps {
  logs: LogEntry[]
  loading?: boolean
  isPaused?: boolean
  shouldAutoScroll?: boolean
  bufferedCount?: number
  onPause?: () => void
  onResume?: () => void
  onClear?: () => void
  onExport?: () => void
  onFilterChange?: (filter: LogFilter) => void
  className?: string
}
```

### StatusIndicator
```typescript
interface StatusIndicatorProps {
  status: 'active' | 'warning' | 'error' | 'inactive'
  label?: string
  tooltip?: string
  animated?: boolean
  size?: 'sm' | 'md' | 'lg'
  className?: string
}
```

## Hook Usage

### useMetrics
```typescript
const {
  metrics,        // Current metrics snapshot
  chartData,      // Chart data for current time range
  loading,        // Loading state
  error,          // Error message if any
  refresh,        // Manual refresh function
  fetchChartData, // Fetch chart data for specific range
  isConnected,    // WebSocket connection status
} = useMetrics({
  refreshInterval: 5000,    // Auto-refresh interval in ms
  autoRefresh: true,        // Enable auto-refresh
  timeRange: '1h',          // Initial time range
})
```

### useLogs
```typescript
const {
  logs,            // Filtered logs
  allLogs,         // All logs (unfiltered)
  loading,         // Loading state
  error,           // Error message if any
  isPaused,        // Pause state
  shouldAutoScroll,// Auto-scroll state
  isConnected,     // WebSocket connection status
  bufferedCount,   // Number of buffered logs when paused
  pause,           // Pause streaming
  resume,          // Resume streaming
  clear,           // Clear logs
  refresh,         // Refetch logs
  exportLogs,      // Export to JSON
} = useLogs({
  limit: 500,              // Max logs to fetch
  autoScroll: true,        // Enable auto-scroll
  bufferSize: 1000,        // Max buffer size
  filter: {                // Initial filter
    level: ['info', 'error'],
    search: 'tunnel',
  },
})
```

## Performance Considerations

1. **Log Buffering**: When paused, new logs are buffered to prevent UI updates
2. **Buffer Limits**: Logs are capped at `bufferSize` (default 1000) to prevent memory issues
3. **Sparkline History**: Latency sparklines keep only last 20 data points
4. **Chart Data**: Time-series data is fetched on-demand when range changes
5. **WebSocket Reconnection**: Automatic reconnection with exponential backoff

## Styling

All components use Tailwind CSS with dark mode support:
- Light theme: `bg-white`, `text-gray-900`
- Dark theme: `dark:bg-gray-800`, `dark:text-white`

## Dependencies

- **react**: ^18.2.0
- **recharts**: ^2.10.3 (for charts)
- **lucide-react**: ^0.303.0 (for icons)
- **tailwind-merge**: ^2.2.0 (for className merging)
- **clsx**: ^2.1.0 (for conditional classes)

## Future Enhancements

Potential improvements:
1. **Alerts Configuration** - Set up threshold-based alerts
2. **Custom Dashboards** - Drag-and-drop dashboard customization
3. **Historical Analysis** - Deep dive into historical metrics
4. **Comparison View** - Compare metrics across time periods
5. **Export to CSV/Excel** - Export metrics for analysis
6. **Real-time Notifications** - Browser notifications for critical events
7. **Log Aggregation** - Group similar log entries
8. **Advanced Filtering** - Regex support, saved filters
