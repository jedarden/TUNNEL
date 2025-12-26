import { useEffect } from 'react'
import { useUIStore } from '@/stores/ui'
import {
  CheckCircle,
  Info,
  AlertTriangle,
  XCircle,
  X,
} from 'lucide-react'
import type { NotificationType } from '@/types'

/**
 * Toast notification system
 * Displays notifications from UI store with auto-dismiss
 */
export function Notifications() {
  const { notifications, removeNotification } = useUIStore()

  return (
    <div className="fixed top-20 right-4 z-50 space-y-2 max-w-sm w-full pointer-events-none">
      {notifications.map((notification) => (
        <NotificationToast
          key={notification.id}
          id={notification.id}
          type={notification.type}
          title={notification.title}
          message={notification.message}
          onClose={() => removeNotification(notification.id)}
        />
      ))}
    </div>
  )
}

interface NotificationToastProps {
  id: string
  type: NotificationType
  title: string
  message: string
  onClose: () => void
}

function NotificationToast({ id, type, title, message, onClose }: NotificationToastProps) {
  const { removeNotification } = useUIStore()

  // Auto-dismiss after timeout
  useEffect(() => {
    const timeout = type === 'error' || type === 'warning' ? 8000 : 5000
    const timer = setTimeout(() => {
      removeNotification(id)
    }, timeout)

    return () => clearTimeout(timer)
  }, [id, type, removeNotification])

  const config = getNotificationConfig(type)

  return (
    <div
      className={`
        pointer-events-auto
        flex items-start gap-3 p-4 rounded-lg shadow-lg
        border-l-4 backdrop-blur-sm
        animate-in slide-in-from-right duration-300
        ${config.bgClass} ${config.borderClass}
      `}
      role="alert"
    >
      {/* Icon */}
      <div className={`flex-shrink-0 ${config.iconClass}`}>
        <config.icon className="h-5 w-5" />
      </div>

      {/* Content */}
      <div className="flex-1 min-w-0">
        <h4 className="font-semibold text-sm text-foreground">{title}</h4>
        <p className="mt-1 text-sm text-muted-foreground">{message}</p>
      </div>

      {/* Close button */}
      <button
        onClick={onClose}
        className="flex-shrink-0 p-1 rounded hover:bg-accent transition-colors"
        aria-label="Close notification"
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  )
}

function getNotificationConfig(type: NotificationType) {
  const configs = {
    success: {
      icon: CheckCircle,
      bgClass: 'bg-green-500/10 dark:bg-green-500/20',
      borderClass: 'border-green-500',
      iconClass: 'text-green-500',
    },
    info: {
      icon: Info,
      bgClass: 'bg-blue-500/10 dark:bg-blue-500/20',
      borderClass: 'border-blue-500',
      iconClass: 'text-blue-500',
    },
    warning: {
      icon: AlertTriangle,
      bgClass: 'bg-yellow-500/10 dark:bg-yellow-500/20',
      borderClass: 'border-yellow-500',
      iconClass: 'text-yellow-500',
    },
    error: {
      icon: XCircle,
      bgClass: 'bg-red-500/10 dark:bg-red-500/20',
      borderClass: 'border-red-500',
      iconClass: 'text-red-500',
    },
  }

  return configs[type]
}
