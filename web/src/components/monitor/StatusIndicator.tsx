import { cn } from '@/lib/utils'

/**
 * Status type
 */
export type Status = 'active' | 'warning' | 'error' | 'inactive'

/**
 * Status indicator props
 */
interface StatusIndicatorProps {
  status: Status
  label?: string
  tooltip?: string
  animated?: boolean
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

/**
 * Status indicator component with color-coded states
 */
export function StatusIndicator({
  status,
  label,
  tooltip,
  animated = true,
  size = 'md',
  className,
}: StatusIndicatorProps) {
  const sizeClasses = {
    sm: 'w-2 h-2',
    md: 'w-3 h-3',
    lg: 'w-4 h-4',
  }

  const statusColors = {
    active: 'bg-green-500',
    warning: 'bg-yellow-500',
    error: 'bg-red-500',
    inactive: 'bg-gray-400',
  }

  const statusLabels = {
    active: 'Active',
    warning: 'Warning',
    error: 'Error',
    inactive: 'Inactive',
  }

  return (
    <div
      className={cn('flex items-center gap-2', className)}
      title={tooltip || statusLabels[status]}
    >
      <div className="relative">
        <div
          className={cn(
            'rounded-full',
            sizeClasses[size],
            statusColors[status]
          )}
        />
        {animated && status === 'active' && (
          <div
            className={cn(
              'absolute inset-0 rounded-full animate-ping opacity-75',
              statusColors[status]
            )}
          />
        )}
      </div>
      {label && (
        <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
          {label}
        </span>
      )}
    </div>
  )
}
