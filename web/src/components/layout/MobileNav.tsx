import { Link, useLocation } from 'react-router-dom'
import { useUIStore } from '@/stores/ui'
import {
  Activity,
  Network,
  Zap,
  Settings,
  X,
} from 'lucide-react'

/**
 * Mobile navigation drawer
 * Slide-out menu for small screens
 */
export function MobileNav() {
  const location = useLocation()
  const { sidebarOpen, setSidebarOpen } = useUIStore()

  const closeNav = () => setSidebarOpen(false)

  return (
    <>
      {/* Navigation drawer */}
      <div
        className={`
          fixed top-0 right-0 z-50
          h-screen w-80 max-w-[85vw]
          bg-card border-l border-border
          transform transition-transform duration-300
          lg:hidden
          ${sidebarOpen ? 'translate-x-0' : 'translate-x-full'}
        `}
      >
        <div className="flex flex-col h-full">
          {/* Header with close button */}
          <div className="flex items-center justify-between p-4 border-b border-border">
            <h2 className="text-lg font-semibold">Menu</h2>
            <button
              onClick={closeNav}
              className="p-2 rounded-lg hover:bg-accent transition-colors"
              aria-label="Close menu"
            >
              <X className="h-5 w-5" />
            </button>
          </div>

          {/* Navigation items */}
          <nav className="flex-1 p-4 space-y-1">
            <MobileNavLink
              to="/dashboard"
              icon={Activity}
              label="Dashboard"
              isActive={location.pathname === '/dashboard'}
              onClick={closeNav}
            />
            <MobileNavLink
              to="/providers"
              icon={Network}
              label="Providers"
              isActive={location.pathname === '/providers'}
              onClick={closeNav}
            />
            <MobileNavLink
              to="/connections"
              icon={Zap}
              label="Connections"
              isActive={location.pathname === '/connections'}
              onClick={closeNav}
            />
            <MobileNavLink
              to="/settings"
              icon={Settings}
              label="Settings"
              isActive={location.pathname === '/settings'}
              onClick={closeNav}
            />
          </nav>

          {/* Footer */}
          <div className="p-4 border-t border-border">
            <div className="text-sm text-muted-foreground">
              <div className="font-semibold">Tunnel Web</div>
              <div className="mt-0.5">v1.0.0</div>
            </div>
          </div>
        </div>
      </div>

      {/* Backdrop overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-40 lg:hidden"
          onClick={closeNav}
          aria-label="Close menu"
        />
      )}
    </>
  )
}

interface MobileNavLinkProps {
  to: string
  icon: React.ComponentType<{ className?: string }>
  label: string
  isActive: boolean
  onClick: () => void
}

function MobileNavLink({ to, icon: Icon, label, isActive, onClick }: MobileNavLinkProps) {
  return (
    <Link
      to={to}
      onClick={onClick}
      className={`
        flex items-center gap-4 px-4 py-3 rounded-lg
        transition-colors font-medium
        ${
          isActive
            ? 'bg-primary text-primary-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground hover:bg-accent'
        }
      `}
    >
      <Icon className="h-5 w-5" />
      <span>{label}</span>
    </Link>
  )
}
