import { LucideIcon, TrendingUp, TrendingDown, Minus } from 'lucide-react'
import { Card, CardContent, CardHeader } from '../ui/Card'
import { cn } from '../../lib/utils'

export interface StatsCardProps {
  icon: LucideIcon
  label: string
  value: string | number
  trend?: {
    value: number
    direction: 'up' | 'down' | 'neutral'
  }
  variant?: 'default' | 'primary' | 'success' | 'warning' | 'error'
  description?: string
}

const StatsCard = ({
  icon: Icon,
  label,
  value,
  trend,
  variant = 'default',
  description,
}: StatsCardProps) => {
  const iconColors = {
    default: 'text-gray-600 bg-gray-100 dark:text-gray-400 dark:bg-gray-800',
    primary: 'text-blue-600 bg-blue-100 dark:text-blue-400 dark:bg-blue-900/30',
    success: 'text-green-600 bg-green-100 dark:text-green-400 dark:bg-green-900/30',
    warning: 'text-yellow-600 bg-yellow-100 dark:text-yellow-400 dark:bg-yellow-900/30',
    error: 'text-red-600 bg-red-100 dark:text-red-400 dark:bg-red-900/30',
  }

  const TrendIcon = trend?.direction === 'up' ? TrendingUp : trend?.direction === 'down' ? TrendingDown : Minus

  const trendColors = {
    up: 'text-green-600 dark:text-green-400',
    down: 'text-red-600 dark:text-red-400',
    neutral: 'text-gray-600 dark:text-gray-400',
  }

  return (
    <Card hover>
      <CardHeader className="flex flex-row items-center justify-between pb-2 space-y-0">
        <div className="flex items-center space-x-3">
          <div className={cn('p-2 rounded-lg', iconColors[variant])}>
            <Icon className="h-5 w-5" />
          </div>
          <div className="flex flex-col">
            <p className="text-sm font-medium text-gray-500 dark:text-gray-400">{label}</p>
            {description && (
              <p className="text-xs text-gray-400 dark:text-gray-500">{description}</p>
            )}
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="flex items-end justify-between">
          <div className="text-3xl font-bold text-gray-900 dark:text-gray-100">
            {value}
          </div>
          {trend && (
            <div className={cn('flex items-center text-sm font-medium', trendColors[trend.direction])}>
              <TrendIcon className="h-4 w-4 mr-1" />
              {Math.abs(trend.value)}%
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  )
}

export default StatsCard
