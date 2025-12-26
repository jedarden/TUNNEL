import { Play, Square, Activity, Settings } from 'lucide-react'
import { Card, CardHeader, CardTitle, CardContent } from '../ui/Card'
import Button from '../ui/Button'

export interface QuickActionsProps {
  onConnectAll?: () => void
  onDisconnectAll?: () => void
  onRunDiagnostics?: () => void
  onOpenSettings?: () => void
  hasActiveConnections?: boolean
  loading?: boolean
}

const QuickActions = ({
  onConnectAll,
  onDisconnectAll,
  onRunDiagnostics,
  onOpenSettings,
  hasActiveConnections = false,
  loading = false,
}: QuickActionsProps) => {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Quick Actions</CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Button
          variant="primary"
          onClick={onConnectAll}
          disabled={loading || hasActiveConnections}
          loading={loading}
          className="w-full"
        >
          <Play className="h-4 w-4 mr-2" />
          Connect All
        </Button>

        <Button
          variant="secondary"
          onClick={onDisconnectAll}
          disabled={loading || !hasActiveConnections}
          loading={loading}
          className="w-full"
        >
          <Square className="h-4 w-4 mr-2" />
          Disconnect All
        </Button>

        <Button
          variant="ghost"
          onClick={onRunDiagnostics}
          disabled={loading}
          className="w-full"
        >
          <Activity className="h-4 w-4 mr-2" />
          Run Diagnostics
        </Button>

        <Button
          variant="ghost"
          onClick={onOpenSettings}
          disabled={loading}
          className="w-full"
        >
          <Settings className="h-4 w-4 mr-2" />
          Settings
        </Button>
      </CardContent>
    </Card>
  )
}

export default QuickActions
