import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { Notification, NotificationType } from '@/types'

/**
 * UI state interface
 */
interface UIState {
  // Theme
  theme: 'light' | 'dark' | 'system'
  setTheme: (theme: 'light' | 'dark' | 'system') => void
  toggleTheme: () => void

  // Sidebar
  sidebarOpen: boolean
  setSidebarOpen: (open: boolean) => void
  toggleSidebar: () => void

  // Notifications
  notifications: Notification[]
  addNotification: (
    type: NotificationType,
    title: string,
    message: string
  ) => void
  removeNotification: (id: string) => void
  markNotificationAsRead: (id: string) => void
  clearNotifications: () => void

  // Loading states
  isLoading: boolean
  setIsLoading: (loading: boolean) => void
}

/**
 * UI store with persistence
 */
export const useUIStore = create<UIState>()(
  persist(
    (set, get) => ({
      // Theme state
      theme: 'system',
      setTheme: (theme) => set({ theme }),
      toggleTheme: () =>
        set((state) => ({
          theme: state.theme === 'dark' ? 'light' : 'dark',
        })),

      // Sidebar state
      sidebarOpen: true,
      setSidebarOpen: (open) => set({ sidebarOpen: open }),
      toggleSidebar: () =>
        set((state) => ({ sidebarOpen: !state.sidebarOpen })),

      // Notifications state
      notifications: [],
      addNotification: (type, title, message) => {
        const notification: Notification = {
          id: `${Date.now()}-${Math.random()}`,
          type,
          title,
          message,
          timestamp: new Date().toISOString(),
          read: false,
        }
        set((state) => ({
          notifications: [notification, ...state.notifications],
        }))

        // Auto-remove after 5 seconds for success/info notifications
        if (type === 'success' || type === 'info') {
          setTimeout(() => {
            get().removeNotification(notification.id)
          }, 5000)
        }
      },
      removeNotification: (id) =>
        set((state) => ({
          notifications: state.notifications.filter((n) => n.id !== id),
        })),
      markNotificationAsRead: (id) =>
        set((state) => ({
          notifications: state.notifications.map((n) =>
            n.id === id ? { ...n, read: true } : n
          ),
        })),
      clearNotifications: () => set({ notifications: [] }),

      // Loading state
      isLoading: false,
      setIsLoading: (loading) => set({ isLoading: loading }),
    }),
    {
      name: 'tunnel-web-ui',
      partialize: (state) => ({
        theme: state.theme,
        sidebarOpen: state.sidebarOpen,
      }),
    }
  )
)
