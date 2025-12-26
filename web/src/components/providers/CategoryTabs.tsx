import { cn } from '@/lib/utils'
import type { ProviderCategory } from '@/types'

interface CategoryTabsProps {
  activeCategory: ProviderCategory
  onCategoryChange: (category: ProviderCategory) => void
  counts?: Record<ProviderCategory, number>
  className?: string
}

const categories: Array<{ value: ProviderCategory; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'vpn-mesh', label: 'VPN/Mesh' },
  { value: 'tunnels', label: 'Tunnels' },
  { value: 'ssh', label: 'SSH' },
]

export function CategoryTabs({
  activeCategory,
  onCategoryChange,
  counts,
  className,
}: CategoryTabsProps) {
  return (
    <div className={cn('border-b border-border', className)}>
      <div className="flex space-x-8">
        {categories.map((category) => {
          const count = counts?.[category.value] ?? 0
          const isActive = activeCategory === category.value

          return (
            <button
              key={category.value}
              onClick={() => onCategoryChange(category.value)}
              className={cn(
                'relative px-1 py-4 text-sm font-medium transition-colors',
                'border-b-2 -mb-px',
                isActive
                  ? 'border-primary text-foreground'
                  : 'border-transparent text-muted-foreground hover:text-foreground'
              )}
            >
              <span className="flex items-center gap-2">
                {category.label}
                {count > 0 && (
                  <span
                    className={cn(
                      'px-2 py-0.5 rounded-full text-xs',
                      isActive
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted text-muted-foreground'
                    )}
                  >
                    {count}
                  </span>
                )}
              </span>
            </button>
          )
        })}
      </div>
    </div>
  )
}
