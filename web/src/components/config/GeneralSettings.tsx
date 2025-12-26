import { useState, useEffect } from 'react'
import { FormField, Label, Input, HelpText } from '../ui/Form'
import { Select, type SelectOption } from '../ui/Select'
import { Switch } from '../ui/Switch'
import type { Config, ProviderType } from '@/types'

interface GeneralSettingsProps {
  config: Config | null
  onConfigChange: (config: Partial<Config>) => void
  availableProviders: ProviderType[]
}

const loggingLevels: SelectOption[] = [
  { value: 'debug', label: 'Debug' },
  { value: 'info', label: 'Info' },
  { value: 'warn', label: 'Warning' },
  { value: 'error', label: 'Error' },
]

export function GeneralSettings({
  config,
  onConfigChange,
  availableProviders,
}: GeneralSettingsProps) {
  const [autoConnect, setAutoConnect] = useState(false)
  const [autoReconnect, setAutoReconnect] = useState(true)
  const [connectionTimeout, setConnectionTimeout] = useState(30)
  const [loggingLevel, setLoggingLevel] = useState('info')
  const [defaultProvider, setDefaultProvider] = useState<ProviderType>('ngrok')

  // Initialize from config
  useEffect(() => {
    if (config) {
      setAutoReconnect(config.tunnel?.autoReconnect ?? true)
      setDefaultProvider(config.tunnel?.defaultProvider ?? 'ngrok')
      setConnectionTimeout((config.tunnel?.reconnectDelay ?? 30000) / 1000)
    }
  }, [config])

  const handleAutoConnectChange = (checked: boolean) => {
    setAutoConnect(checked)
    // This would be stored in an extended config
  }

  const handleAutoReconnectChange = (checked: boolean) => {
    setAutoReconnect(checked)
    onConfigChange({
      tunnel: {
        ...config?.tunnel,
        defaultProvider: config?.tunnel?.defaultProvider ?? 'ngrok',
        autoReconnect: checked,
        reconnectDelay: config?.tunnel?.reconnectDelay ?? 30000,
      },
    })
  }

  const handleConnectionTimeoutChange = (value: string) => {
    const timeout = parseInt(value, 10)
    if (!isNaN(timeout) && timeout > 0) {
      setConnectionTimeout(timeout)
      onConfigChange({
        tunnel: {
          ...config?.tunnel,
          defaultProvider: config?.tunnel?.defaultProvider ?? 'ngrok',
          autoReconnect: config?.tunnel?.autoReconnect ?? true,
          reconnectDelay: timeout * 1000,
        },
      })
    }
  }

  const handleLoggingLevelChange = (value: string | string[]) => {
    if (typeof value === 'string') {
      setLoggingLevel(value)
      // Would be stored in extended config
    }
  }

  const handleDefaultProviderChange = (value: string | string[]) => {
    if (typeof value === 'string') {
      const provider = value as ProviderType
      setDefaultProvider(provider)
      onConfigChange({
        tunnel: {
          ...config?.tunnel,
          defaultProvider: provider,
          autoReconnect: config?.tunnel?.autoReconnect ?? true,
          reconnectDelay: config?.tunnel?.reconnectDelay ?? 30000,
        },
      })
    }
  }

  const providerOptions: SelectOption[] = availableProviders.map(provider => ({
    value: provider,
    label: provider.charAt(0).toUpperCase() + provider.slice(1),
  }))

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-foreground">General Settings</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Configure global application settings and defaults.
        </p>
      </div>

      <div className="space-y-4">
        <FormField>
          <Label
            htmlFor="default-provider"
            description="The default tunnel provider to use for new connections"
          >
            Default Provider
          </Label>
          <Select
            id="default-provider"
            options={providerOptions}
            value={defaultProvider}
            onChange={handleDefaultProviderChange}
            placeholder="Select a provider..."
          />
        </FormField>

        <FormField>
          <Switch
            id="auto-connect"
            checked={autoConnect}
            onChange={handleAutoConnectChange}
            label="Auto-connect on startup"
            description="Automatically establish tunnel connections when the application starts"
          />
        </FormField>

        <FormField>
          <Switch
            id="auto-reconnect"
            checked={autoReconnect}
            onChange={handleAutoReconnectChange}
            label="Auto-reconnect"
            description="Automatically reconnect when connection is lost"
          />
        </FormField>

        <FormField>
          <Label
            htmlFor="connection-timeout"
            description="Time in seconds before connection attempts timeout"
          >
            Connection Timeout
          </Label>
          <Input
            id="connection-timeout"
            type="number"
            min="5"
            max="300"
            value={connectionTimeout}
            onChange={(e) => handleConnectionTimeoutChange(e.target.value)}
            placeholder="30"
          />
          <HelpText>Recommended: 30 seconds</HelpText>
        </FormField>

        <FormField>
          <Label
            htmlFor="logging-level"
            description="Set the verbosity of application logs"
          >
            Logging Level
          </Label>
          <Select
            id="logging-level"
            options={loggingLevels}
            value={loggingLevel}
            onChange={handleLoggingLevelChange}
          />
        </FormField>
      </div>
    </div>
  )
}
