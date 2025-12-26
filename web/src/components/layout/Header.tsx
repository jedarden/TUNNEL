import { Link } from 'react-router-dom'
import { useUIStore } from '@/stores/ui'
import { useMonitoringStore } from '@/stores/monitoring'
import {
  Menu,
  Sun,
  Moon,
  Monitor,
  Activity,
  Zap,
  Network,
  Settings,
} from 'lucide-react'
import { useMemo } from 'react'

/**
 * Top header bar with navigation and global controls
 */
export function Header() {
  const { theme, setTheme, toggleSidebar } = useUIStore()
  const { metrics } = useMonitoringStore()

  // Calculate overall health status
  const healthStatus = useMemo(() => {
    if (!metrics) return 'unknown'

    const activeConnections = metrics.activeConnections || 0
    const errorRate = metrics.errorRate || 0
    const cpu = metrics.cpu || 0
    const memory = metrics.memory || 0

    if (errorRate > 0.1 || cpu > 90 || memory > 90) return 'error'
    if (errorRate > 0.05 || cpu > 70 || memory > 70) return 'warning'
    if (activeConnections > 0) return 'active'
    return 'idle'
  }, [metrics])

  const statusColors = {
    active: 'bg-green-500',
    idle: 'bg-blue-500',
    warning: 'bg-yellow-500',
    error: 'bg-red-500',
    unknown: 'bg-gray-500',
  }

  const cycleTheme = () => {
    const themes: Array<'light' | 'dark' | 'system'> = ['light', 'dark', 'system']
    const currentIndex = themes.indexOf(theme)
    const nextTheme = themes[(currentIndex + 1) % themes.length]
    setTheme(nextTheme)
  }

  const ThemeIcon = theme === 'dark' ? Moon : theme === 'light' ? Sun : Monitor

  return (
    <header className="h-16 border-b border-border bg-card sticky top-0 z-50 shadow-sm">
      <div className="flex items-center justify-between h-full px-4 lg:px-6">
        {/* Left side - Menu and Logo */}
        <div className="flex items-center gap-4">
          {/* Mobile menu button */}
          <button
            onClick={toggleSidebar}
            className="p-2 rounded-lg hover:bg-accent transition-colors lg:hidden"
            aria-label="Toggle menu"
          >
            <Menu className="h-5 w-5" />
          </button>

          {/* Desktop sidebar toggle */}
          <button
            onClick={toggleSidebar}
            className="hidden lg:block p-2 rounded-lg hover:bg-accent transition-colors"
            aria-label="Toggle sidebar"
          >
            <Menu className="h-5 w-5" />
          </button>

          {/* Logo and title */}
          <Link to="/dashboard" className="flex items-center gap-3 group">
            <div className="relative">
              <Zap className="h-7 w-7 text-primary group-hover:text-primary/80 transition-colors" />
              <div className="absolute inset-0 bg-primary/20 blur-xl opacity-0 group-hover:opacity-100 transition-opacity" />
            </div>
            <span className="text-xl font-bold bg-gradient-to-r from-primary to-primary/60 bg-clip-text text-transparent">
              Tunnel Web
            </span>
          </Link>
        </div>

        {/* Center - Main navigation (hidden on mobile) */}
        <nav className="hidden md:flex items-center gap-1">
          <NavLink to="/dashboard" icon={Activity}>
            Dashboard
          </NavLink>
          <NavLink to="/providers" icon={Network}>
            Providers
          </NavLink>
          <NavLink to="/connections" icon={Zap}>
            Connections
          </NavLink>
          <NavLink to="/settings" icon={Settings}>
            Settings
          </NavLink>
        </nav>

        {/* Right side - Status and theme toggle */}
        <div className="flex items-center gap-3">
          {/* Global status indicator */}
          <div className="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-full bg-accent/50">
            <div
              className={`h-2 w-2 rounded-full ${statusColors[healthStatus]} animate-pulse`}
              title={`System status: ${healthStatus}`}
            />
            <span className="text-xs font-medium capitalize">{healthStatus}</span>
          </div>

          {/* Theme toggle */}
          <button
            onClick={cycleTheme}
            className="p-2 rounded-lg hover:bg-accent transition-colors"
            aria-label={`Current theme: ${theme}. Click to change`}
            title={`Theme: ${theme}`}
          >
            <ThemeIcon className="h-5 w-5" />
          </button>
        </div>
      </div>
    </header>
  )
}

interface NavLinkProps {
  to: string
  icon: React.ComponentType<{ className?: string }>
  children: React.ReactNode
}

function NavLink({ to, icon: Icon, children }: NavLinkProps) {
  const isActive = window.location.pathname === to

  return (
    <Link
      to={to}
      className={`
        flex items-center gap-2 px-4 py-2 rounded-lg
        transition-colors font-medium text-sm
        ${
          isActive
            ? 'bg-primary text-primary-foreground'
            : 'text-muted-foreground hover:text-foreground hover:bg-accent'
        }
      `}
    >
      <Icon className="h-4 w-4" />
      <span>{children}</span>
    </Link>
  )
}
