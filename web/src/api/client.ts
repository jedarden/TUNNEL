import type {
  ApiResponse,
  ApiError,
  Provider,
  Connection,
  SystemMetrics,
  Config
} from '@/types'

/**
 * API client configuration
 */
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api'
const AUTH_TOKEN_STORAGE_KEY = 'tunnel_auth_token'

export const AUTH_REQUIRED_EVENT = 'tunnel:auth-required'
export const WEBSOCKET_AUTH_PROTOCOL = 'tunnel-auth'

/**
 * Authentication token storage
 */
let authToken: string | null = null

/**
 * Set the authentication token
 */
export function setAuthToken(token: string): void {
  authToken = token.trim()
  sessionStorage.setItem(AUTH_TOKEN_STORAGE_KEY, authToken)
}

/**
 * Get the current authentication token
 */
export function getAuthToken(): string | null {
  if (!authToken) {
    authToken = sessionStorage.getItem(AUTH_TOKEN_STORAGE_KEY)
  }
  return authToken
}

/**
 * Clear the authentication token
 */
export function clearAuthToken(): void {
  authToken = null
  sessionStorage.removeItem(AUTH_TOKEN_STORAGE_KEY)
  // Remove values written by pre-release builds that used persistent storage.
  localStorage.removeItem(AUTH_TOKEN_STORAGE_KEY)
}

/**
 * Custom error class for API errors
 */
export class APIError extends Error {
  constructor(
    public code: string,
    message: string,
    public details?: Record<string, unknown>
  ) {
    super(message)
    this.name = 'APIError'
  }
}

function notifyAuthenticationRequired(): void {
  clearAuthToken()
  window.dispatchEvent(new Event(AUTH_REQUIRED_EVENT))
}

/**
 * Check a user-provided token without storing it first.
 */
export async function verifyAuthToken(token: string): Promise<boolean> {
  const candidate = token.trim()
  if (!candidate) return false

  const response = await fetch(`${API_BASE_URL}/system/status`, {
    headers: { Authorization: `Bearer ${candidate}` },
  })
  if (response.status === 401) return false
  if (!response.ok) {
    throw new Error(`Unable to verify token: HTTP ${response.status}`)
  }
  return true
}

/**
 * Fetch wrapper for direct API calls outside the typed client.
 */
export async function authenticatedFetch(
  input: RequestInfo | URL,
  options: RequestInit = {}
): Promise<Response> {
  const token = getAuthToken()
  if (!token) {
    notifyAuthenticationRequired()
    throw new APIError('AUTH_REQUIRED', 'API authentication is required')
  }

  const headers = new Headers(options.headers)
  headers.set('Authorization', `Bearer ${token}`)
  const response = await fetch(input, { ...options, headers })

  if (response.status === 401) {
    notifyAuthenticationRequired()
  }

  return response
}

/**
 * Generic fetch wrapper with error handling
 */
async function fetchAPI<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`

  try {
    const response = await authenticatedFetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
    })

    if (!response.ok) {
      let error: ApiError
      try {
        const errorData = await response.json()
        error = errorData.error || {
          code: 'UNKNOWN_ERROR',
          message: response.statusText,
        }
      } catch {
        error = {
          code: 'PARSE_ERROR',
          message: `HTTP ${response.status}: ${response.statusText}`,
        }
      }
      throw new APIError(error.code, error.message, error.details)
    }

    const data: ApiResponse<T> = await response.json()

    if (!data.success) {
      throw new APIError(
        data.error || 'REQUEST_FAILED',
        data.error || 'Request failed'
      )
    }

    return data.data as T
  } catch (error) {
    if (error instanceof APIError) {
      throw error
    }
    if (error instanceof Error) {
      throw new APIError('NETWORK_ERROR', error.message)
    }
    throw new APIError('UNKNOWN_ERROR', 'An unknown error occurred')
  }
}

/**
 * Provider API methods
 */
export const providersAPI = {
  list: () => fetchAPI<Provider[]>('/providers'),

  get: (id: string) => fetchAPI<Provider>(`/providers/${id}`),

  create: (data: Partial<Provider>) =>
    fetchAPI<Provider>('/providers', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: Partial<Provider>) =>
    fetchAPI<Provider>(`/providers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  delete: (id: string) =>
    fetchAPI<void>(`/providers/${id}`, {
      method: 'DELETE',
    }),
}

/**
 * Connection API methods
 */
export const connectionsAPI = {
  list: () => fetchAPI<Connection[]>('/connections'),

  get: (id: string) => fetchAPI<Connection>(`/connections/${id}`),

  create: (data: {
    providerId: string
    localPort: number
    protocol?: 'http' | 'https' | 'tcp'
  }) =>
    fetchAPI<Connection>('/connections', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  disconnect: (id: string) =>
    fetchAPI<void>(`/connections/${id}`, {
      method: 'DELETE',
    }),

  reconnect: (id: string) =>
    fetchAPI<Connection>(`/connections/${id}/reconnect`, {
      method: 'POST',
    }),
}

/**
 * Metrics API methods
 */
export const metricsAPI = {
  system: () => fetchAPI<SystemMetrics>('/metrics/system'),

  connection: (id: string) =>
    fetchAPI<Connection['metrics']>(`/metrics/connections/${id}`),

  // Get historical metrics for a connection
  history: (id: string, params?: { limit?: number; since?: string }) => {
    const query = new URLSearchParams()
    if (params?.limit) query.set('limit', params.limit.toString())
    if (params?.since) query.set('since', params.since)
    const queryString = query.toString()
    return fetchAPI<any>(`/connections/${id}/history${queryString ? `?${queryString}` : ''}`)
  },

  // Get connection stats (uptime %, failover count, MTTR)
  stats: (id: string, params?: { since?: string }) => {
    const query = new URLSearchParams()
    if (params?.since) query.set('since', params.since)
    const queryString = query.toString()
    return fetchAPI<any>(`/connections/${id}/stats${queryString ? `?${queryString}` : ''}`)
  },

  // Get stats for all connections
  allStats: (params?: { since?: string }) => {
    const query = new URLSearchParams()
    if (params?.since) query.set('since', params.since)
    const queryString = query.toString()
    return fetchAPI<any>(`/metrics/connections/stats${queryString ? `?${queryString}` : ''}`)
  },

  // Get failover events
  failoverEvents: (params?: { limit?: number; since?: string }) => {
    const query = new URLSearchParams()
    if (params?.limit) query.set('limit', params.limit.toString())
    if (params?.since) query.set('since', params.since)
    const queryString = query.toString()
    return fetchAPI<any>(`/failover/events${queryString ? `?${queryString}` : ''}`)
  },
}

/**
 * Config API methods
 */
export const configAPI = {
  get: () => fetchAPI<Config>('/config'),

  update: (data: Partial<Config>) =>
    fetchAPI<Config>('/config', {
      method: 'PUT',
      body: JSON.stringify(data),
    }),
}

/**
 * Health check
 */
export const healthAPI = {
  check: () => fetchAPI<{ status: string; version: string }>('/health'),
}
