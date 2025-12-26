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

/**
 * Generic fetch wrapper with error handling
 */
async function fetchAPI<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`

  try {
    const response = await fetch(url, {
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
