import { create } from 'zustand'
import type { SystemMetrics } from '@/types'

/**
 * Monitoring state interface
 */
interface MonitoringState {
  // System metrics
  metrics: SystemMetrics | null
  setMetrics: (metrics: SystemMetrics) => void

  // Error tracking
  errorRate: number
  setErrorRate: (rate: number) => void
}

/**
 * Monitoring store for system metrics and health
 */
export const useMonitoringStore = create<MonitoringState>((set) => ({
  // Metrics state
  metrics: null,
  setMetrics: (metrics) => set({ metrics }),

  // Error tracking
  errorRate: 0,
  setErrorRate: (rate) => set({ errorRate: rate }),
}))
