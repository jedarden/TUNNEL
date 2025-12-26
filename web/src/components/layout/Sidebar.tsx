import { Link, useLocation } from 'react-router-dom'
import { useUIStore } from '@/stores/ui'
import { useMonitoringStore } from '@/stores/monitoring'
import {
  Activity,
  Network,
  Zap,
  Settings,
  ChevronLeft,
  Cpu,
  HardDrive,
  Clock,
} from 'lucide-react'
import { useMemo } from 'react'

/**
 * Side navigation panel
 * Collapsible, with navigation items and status overview
 */
export function Sidebar() {
  const location = useLocation()
  const { sidebarOpen, toggleSidebar } = useUIStore()
  const { metrics } = useMonitoringStore()

  // Format uptime
  const uptimeText = useMemo(() => {
    if (!metrics?.uptime) return '0m'

    const seconds = metrics.uptime
    const days = Math.floor(seconds / 86400)
    const hours = Math.floor((seconds % 86400) / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)

    if (days > 0) return `${days}d ${hours}h`
    if (hours > 0) return `${hours}h ${minutes}m`
    return `${minutes}m`
  }, [metrics?.uptime])

  return (
    <>
      {/* Sidebar - hidden on mobile, fixed on desktop */}
      <aside
        className={`
          fixed top-16 left-0 z-40
          h-[calc(100vh-4rem)] w-64
          bg-card border-r border-border
          transform transition-transform duration-300
          lg:transform-none
          ${sidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0 lg:hidden'}
        `}
      >
        <div className="flex flex-col h-full">
          {/* Navigation items */}
          <nav className="flex-1 p-4 space-y-1">
            <SidebarLink
              to="/dashboard"
              icon={Activity}
              label="Dashboard"
              isActive={location.pathname === '/dashboard'}
            />
            <SidebarLink
              to="/providers"
              icon={Network}
              label="Providers"
              isActive={location.pathname === '/providers'}
            />
            <SidebarLink
              to="/connections"
              icon={Zap}
              label="Connections"
              isActive={location.pathname === '/connections'}
              badge={metrics?.activeConnections}
            />
            <SidebarLink
              to="/settings"
              icon={Settings}
              label="Settings"
              isActive={location.pathname === '/settings'}
            />
          </nav>

          {/* Status overview */}
          {metrics && (
            <div className="p-4 border-t border-border space-y-3">
              <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                System Status
              </div>

              <StatusItem
                icon={Cpu}
                label="CPU"
                value={`${Math.round(metrics.cpu || 0)}%`}
                status={getStatusColor(metrics.cpu || 0, 70, 90)}
              />

              <StatusItem
                icon={HardDrive}
                label="Memory"
                value={`${Math.round(metrics.memory || 0)}%`}
                status={getStatusColor(metrics.memory || 0, 70, 90)}
              />

              <StatusItem
                icon={Clock}
                label="Uptime"
                value={uptimeText}
                status="normal"
              />
            </div>
          )}

          {/* Version info */}
          <div className="p-4 border-t border-border">
            <div className="text-xs text-muted-foreground">
              <div className="font-semibold">Tunnel Web</div>
              <div className="mt-0.5">v1.0.0</div>
            </div>
          </div>

          {/* Collapse button (desktop only) */}
          <button
            onClick={toggleSidebar}
            className="hidden lg:flex items-center justify-center p-3 border-t border-border hover:bg-accent transition-colors"
            aria-label="Collapse sidebar"
          >
            <ChevronLeft className="h-4 w-4" />
          </button>
        </div>
      </aside>

      {/* Backdrop overlay for mobile */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-30 lg:hidden"
          onClick={toggleSidebar}
          aria-label="Close sidebar"
        />
      )}
    </>
  )
}

interface SidebarLinkProps {
  to: string
  icon: React.ComponentType<{ className?: string }>
  label: string
  isActive: boolean
  badge?: number
}

function SidebarLink({ to, icon: Icon, label, isActive, badge }: SidebarLinkProps) {
  return (
    <Link
      to={to}
      className={`
        flex items-center justify-between gap-3 px-3 py-2.5 rounded-lg
        transition-colors font-medium text-sm
        ${
          isActive
            ? 'bg-primary text-primary-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground hover:bg-accent'
        }
      `}
    >
      <div className="flex items-center gap-3">
        <Icon className="h-5 w-5 flex-shrink-0" />
        <span>{label}</span>
      </div>
      {badge !== undefined && badge > 0 && (
        <span
          className={`
            px-2 py-0.5 rounded-full text-xs font-semibold
            ${isActive ? 'bg-primary-foreground/20' : 'bg-primary text-primary-foreground'}
          `}
        >
          {badge}
        </span>
      )}
    </Link>
  )
}

interface StatusItemProps {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: string
  status: 'normal' | 'warning' | 'error'
}

function StatusItem({ icon: Icon, label, value, status }: StatusItemProps) {
  const colors = {
    normal: 'text-green-500',
    warning: 'text-yellow-500',
    error: 'text-red-500',
  }

  return (
    <div className="flex items-center justify-between text-sm">
      <div className="flex items-center gap-2 text-muted-foreground">
        <Icon className={`h-4 w-4 ${colors[status]}`} />
        <span>{label}</span>
      </div>
      <span className="font-semibold text-foreground">{value}</span>
    </div>
  )
}

function getStatusColor(
  value: number,
  warningThreshold: number,
  errorThreshold: number
): 'normal' | 'warning' | 'error' {
  if (value >= errorThreshold) return 'error'
  if (value >= warningThreshold) return 'warning'
  return 'normal'
}
