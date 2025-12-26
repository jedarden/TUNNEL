/**
 * Provider types (ngrok, cloudflare, etc.)
 */
export type ProviderType = 'ngrok' | 'cloudflare' | 'localhost' | 'custom'

/**
 * Provider categories for organization
 */
export type ProviderCategory = 'vpn-mesh' | 'tunnels' | 'ssh' | 'all'

/**
 * Provider information for browsing
 */
export interface ProviderInfo {
  id: string
  name: string
  category: ProviderCategory
  description: string
  icon?: string
  features: string[]
  installed: boolean
  status: 'connected' | 'available' | 'error'
  latency?: number
  config?: Record<string, unknown>
}

export interface Provider {
  id: string
  name: string
  type: ProviderType
  status: 'active' | 'inactive' | 'error'
  config: Record<string, unknown>
  createdAt: string
  updatedAt: string
}

/**
 * Tunnel connection
 */
export interface Connection {
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

/**
 * Connection metrics
 */
export interface ConnectionMetrics {
  requestCount: number
  bytesIn: number
  bytesOut: number
  avgResponseTime: number
  errorRate: number
  lastRequestAt?: string
}

/**
 * System metrics
 */
export interface SystemMetrics {
  cpu: number
  memory: number
  uptime: number
  activeConnections: number
  totalRequests: number
  errorRate?: number
  timestamp: string
}

/**
 * Application configuration
 */
export interface Config {
  server: {
    port: number
    host: string
  }
  tunnel: {
    defaultProvider: ProviderType
    autoReconnect: boolean
    reconnectDelay: number
  }
  ui: {
    theme: 'light' | 'dark' | 'system'
    refreshInterval: number
  }
}

/**
 * API response wrapper
 */
export interface ApiResponse<T> {
  success: boolean
  data?: T
  error?: string
  timestamp: string
}

/**
 * API error
 */
export interface ApiError {
  code: string
  message: string
  details?: Record<string, unknown>
}

/**
 * WebSocket message types
 */
export type WsMessageType =
  | 'connection.status'
  | 'connection.metrics'
  | 'system.metrics'
  | 'provider.status'
  | 'log'
  | 'error'

export interface WsMessage<T = unknown> {
  type: WsMessageType
  data: T
  timestamp: string
}

/**
 * WebSocket connection status update
 */
export interface WsConnectionStatus {
  connectionId: string
  status: Connection['status']
  error?: string
}

/**
 * WebSocket metrics update
 */
export interface WsMetricsUpdate {
  connectionId: string
  metrics: ConnectionMetrics
}

/**
 * WebSocket system metrics update
 */
export interface WsSystemMetrics {
  metrics: SystemMetrics
}

/**
 * WebSocket provider status update
 */
export interface WsProviderStatus {
  providerId: string
  status: Provider['status']
  error?: string
}

/**
 * Notification types
 */
export type NotificationType = 'info' | 'success' | 'warning' | 'error'

export interface Notification {
  id: string
  type: NotificationType
  title: string
  message: string
  timestamp: string
  read: boolean
}

/**
 * Export monitoring types
 */
export * from './monitoring'
