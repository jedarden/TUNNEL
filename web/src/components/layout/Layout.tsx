import { useEffect } from 'react'
import { useUIStore } from '@/stores/ui'
import { Header } from './Header'
import { Sidebar } from './Sidebar'
import { MobileNav } from './MobileNav'
import { Notifications } from './Notifications'

interface LayoutProps {
  children: React.ReactNode
}

/**
 * Main application layout wrapper
 * Handles theme, sidebar state, and overall structure
 */
export function Layout({ children }: LayoutProps) {
  const { theme, sidebarOpen } = useUIStore()

  // Apply theme to document root
  useEffect(() => {
    const root = document.documentElement

    // Handle system theme preference
    if (theme === 'system') {
      const systemTheme = window.matchMedia('(prefers-color-scheme: dark)').matches
        ? 'dark'
        : 'light'
      root.classList.toggle('dark', systemTheme === 'dark')
    } else {
      root.classList.toggle('dark', theme === 'dark')
    }
  }, [theme])

  // Listen for system theme changes when theme is 'system'
  useEffect(() => {
    if (theme !== 'system') return

    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = (e: MediaQueryListEvent) => {
      document.documentElement.classList.toggle('dark', e.matches)
    }

    mediaQuery.addEventListener('change', handler)
    return () => mediaQuery.removeEventListener('change', handler)
  }, [theme])

  return (
    <div className="min-h-screen bg-background text-foreground">
      {/* Header - fixed at top */}
      <Header />

      {/* Main layout container */}
      <div className="flex h-[calc(100vh-4rem)]">
        {/* Sidebar - hidden on mobile, toggleable on desktop */}
        <Sidebar />

        {/* Mobile navigation drawer */}
        <MobileNav />

        {/* Main content area */}
        <main
          className={`
            flex-1 overflow-y-auto transition-all duration-300
            ${sidebarOpen ? 'lg:ml-64' : 'lg:ml-0'}
          `}
        >
          <div className="container mx-auto p-6 lg:p-8">
            {children}
          </div>
        </main>
      </div>

      {/* Toast notifications */}
      <Notifications />
    </div>
  )
}
