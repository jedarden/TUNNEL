import { cn } from '@/lib/utils'
import {
  Settings,
  Globe,
  Key,
  Palette,
  Info,
  type LucideIcon,
} from 'lucide-react'

export type SettingsSection =
  | 'general'
  | 'ngrok'
  | 'cloudflare'
  | 'localhost'
  | 'custom'
  | 'credentials'
  | 'appearance'
  | 'about'

interface SettingsSidebarProps {
  activeSection: SettingsSection
  onSectionChange: (section: SettingsSection) => void
  providers: Array<{ type: string; name: string }>
  className?: string
}

interface NavItem {
  id: SettingsSection
  label: string
  icon: LucideIcon
  group?: 'general' | 'providers'
}

export function SettingsSidebar({
  activeSection,
  onSectionChange,
  providers,
  className,
}: SettingsSidebarProps) {
  const generalItems: NavItem[] = [
    { id: 'general', label: 'General', icon: Settings, group: 'general' },
    { id: 'credentials', label: 'Credentials', icon: Key, group: 'general' },
    { id: 'appearance', label: 'Appearance', icon: Palette, group: 'general' },
    { id: 'about', label: 'About', icon: Info, group: 'general' },
  ]

  // Map provider types to nav items
  const providerNavItems: NavItem[] = providers.map(provider => ({
    id: provider.type as SettingsSection,
    label: provider.name,
    icon: Globe,
    group: 'providers' as const,
  }))

  return (
    <aside
      className={cn(
        'w-64 border-r border-border bg-card p-4',
        className
      )}
    >
      <nav className="space-y-6">
        {/* General Settings */}
        <div>
          <h3 className="mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            General
          </h3>
          <ul className="space-y-1">
            {generalItems.map((item) => (
              <li key={item.id}>
                <button
                  onClick={() => onSectionChange(item.id)}
                  className={cn(
                    'flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                    'hover:bg-muted',
                    activeSection === item.id
                      ? 'bg-primary/10 text-primary'
                      : 'text-foreground'
                  )}
                >
                  <item.icon className="h-4 w-4" />
                  <span>{item.label}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>

        {/* Provider Settings */}
        {providerNavItems.length > 0 && (
          <div>
            <h3 className="mb-2 px-3 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Providers
            </h3>
            <ul className="space-y-1">
              {providerNavItems.map((item) => (
                <li key={item.id}>
                  <button
                    onClick={() => onSectionChange(item.id)}
                    className={cn(
                      'flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
                      'hover:bg-muted',
                      activeSection === item.id
                        ? 'bg-primary/10 text-primary'
                        : 'text-foreground'
                    )}
                  >
                    <item.icon className="h-4 w-4" />
                    <span>{item.label}</span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}
      </nav>
    </aside>
  )
}
