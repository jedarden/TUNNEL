import { Search } from 'lucide-react'
import { Input } from '@/components/ui/Input'
import { cn } from '@/lib/utils'

interface ProviderSearchProps {
  value: string
  onChange: (value: string) => void
  statusFilter: 'all' | 'connected' | 'available'
  onStatusFilterChange: (status: 'all' | 'connected' | 'available') => void
  className?: string
}

export function ProviderSearch({
  value,
  onChange,
  statusFilter,
  onStatusFilterChange,
  className,
}: ProviderSearchProps) {
  const filterOptions: Array<{ value: 'all' | 'connected' | 'available'; label: string }> = [
    { value: 'all', label: 'All' },
    { value: 'connected', label: 'Connected' },
    { value: 'available', label: 'Available' },
  ]

  return (
    <div className={cn('space-y-4', className)}>
      {/* Search Input */}
      <Input
        type="search"
        placeholder="Search providers..."
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onClear={() => onChange('')}
        icon={<Search className="w-4 h-4" />}
      />

      {/* Status Filter Chips */}
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-sm text-muted-foreground">Status:</span>
        {filterOptions.map((option) => (
          <button
            key={option.value}
            onClick={() => onStatusFilterChange(option.value)}
            className={cn(
              'px-3 py-1.5 rounded-full text-sm font-medium transition-colors',
              statusFilter === option.value
                ? 'bg-primary text-primary-foreground'
                : 'bg-muted text-muted-foreground hover:bg-muted/80'
            )}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  )
}
