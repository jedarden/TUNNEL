# Tunnel Web - Frontend

React frontend for the Tunnel Web reverse proxy management system.

## Tech Stack

- **React 18** - UI library
- **TypeScript** - Type safety
- **Vite** - Build tool and dev server
- **Tailwind CSS** - Styling with dark mode support
- **React Router** - Client-side routing
- **TanStack Query** - Server state management
- **Zustand** - Client state management
- **Lucide React** - Icons
- **Recharts** - Charts and visualizations

## Project Structure

```
src/
├── api/          # API client and endpoints
├── hooks/        # Custom React hooks
├── stores/       # Zustand stores
├── lib/          # Utility functions
├── types/        # TypeScript type definitions
├── App.tsx       # Main application component
├── main.tsx      # Application entry point
└── index.css     # Global styles
```

## Getting Started

### Install Dependencies

```bash
npm install
```

### Development

```bash
npm run dev
```

The dev server will start at http://localhost:3000 with:
- Hot module replacement
- API proxy to Go backend at http://localhost:8080
- WebSocket proxy for real-time updates

### Build

```bash
npm run build
```

Builds the app for production to the `dist/` folder.

### Preview

```bash
npm run preview
```

Preview the production build locally.

## Configuration

### Environment Variables

Create a `.env.local` file for local development:

```env
VITE_API_BASE_URL=/api
VITE_WS_URL=/ws
```

### API Proxy

The Vite dev server proxies API requests to the Go backend:
- `/api/*` → `http://localhost:8080/api/*`
- `/ws` → `ws://localhost:8080/ws`

## Features

### API Client

Type-safe API client in `src/api/client.ts`:
- Automatic error handling
- Request/response typing
- Base URL configuration
- Support for all REST methods

### WebSocket Hook

Real-time updates via `src/hooks/useWebSocket.ts`:
- Auto-reconnect with exponential backoff
- Type-safe message subscription
- Connection state management

### UI Store

Persistent UI state using Zustand:
- Theme management (light/dark)
- Sidebar state
- Notifications
- Loading states

### Utilities

Helper functions in `src/lib/utils.ts`:
- `cn()` - Class name merging
- `formatBytes()` - Byte formatting
- `formatDuration()` - Duration formatting
- `formatRelativeTime()` - Relative time formatting

## Type System

Comprehensive TypeScript types in `src/types/index.ts`:
- Provider, Connection, Metrics
- API responses and errors
- WebSocket messages
- UI state

## Styling

Tailwind CSS with CSS variables for theming:
- Dark mode support via `class` strategy
- Custom color palette
- Responsive design utilities
- Consistent spacing and typography

## Next Steps

Build out the UI components:
1. Dashboard with metrics and charts
2. Provider management interface
3. Connection list and details
4. Settings panel
5. Real-time status updates
6. Error handling and notifications
