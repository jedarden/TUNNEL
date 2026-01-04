import { useState, useEffect } from 'react'
import { Modal } from '@/components/ui/Modal'
import { FormField, Label, Input, HelpText } from '@/components/ui/Form'
import Button from '@/components/ui/Button'
import { Eye, EyeOff, Plus, Trash2, CheckCircle, XCircle, Save } from 'lucide-react'
import { cn } from '@/lib/utils'
import type { ProviderInfo } from '@/types'

/**
 * Provider instance configuration
 */
export interface ProviderInstance {
  id: string
  name: string
  isDefault: boolean
  config: ProviderInstanceConfig
  createdAt: string
}

interface ProviderInstanceConfig {
  authToken?: string
  region?: string
  subdomain?: string
  tunnelToken?: string
  server?: string
  port?: number
  [key: string]: unknown
}

interface ProviderConfigModalProps {
  provider: ProviderInfo | null
  isOpen: boolean
  onClose: () => void
  onSave: (providerId: string, instances: ProviderInstance[]) => Promise<void>
  onTestConnection: (providerId: string, config: ProviderInstanceConfig) => Promise<boolean>
  existingInstances?: ProviderInstance[]
}

export function ProviderConfigModal({
  provider,
  isOpen,
  onClose,
  onSave,
  onTestConnection,
  existingInstances = [],
}: ProviderConfigModalProps) {
  const [instances, setInstances] = useState<ProviderInstance[]>([])
  const [activeInstanceId, setActiveInstanceId] = useState<string | null>(null)
  const [showTokens, setShowTokens] = useState<Record<string, boolean>>({})
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<'success' | 'error' | null>(null)
  const [saving, setSaving] = useState(false)
  const [hasChanges, setHasChanges] = useState(false)

  // Initialize instances when modal opens
  useEffect(() => {
    if (isOpen && provider) {
      if (existingInstances.length > 0) {
        setInstances(existingInstances)
        setActiveInstanceId(existingInstances[0].id)
      } else {
        // Create default instance
        const defaultInstance: ProviderInstance = {
          id: `${provider.id}-default`,
          name: 'Default',
          isDefault: true,
          config: {},
          createdAt: new Date().toISOString(),
        }
        setInstances([defaultInstance])
        setActiveInstanceId(defaultInstance.id)
      }
      setHasChanges(false)
      setTestResult(null)
    }
  }, [isOpen, provider, existingInstances])

  if (!provider) return null

  const activeInstance = instances.find((i) => i.id === activeInstanceId)

  const handleAddInstance = () => {
    const newInstance: ProviderInstance = {
      id: `${provider.id}-${Date.now()}`,
      name: `Instance ${instances.length + 1}`,
      isDefault: false,
      config: {},
      createdAt: new Date().toISOString(),
    }
    setInstances([...instances, newInstance])
    setActiveInstanceId(newInstance.id)
    setHasChanges(true)
  }

  const handleRemoveInstance = (instanceId: string) => {
    const instance = instances.find((i) => i.id === instanceId)
    if (instance?.isDefault) return // Can't remove default instance

    const newInstances = instances.filter((i) => i.id !== instanceId)
    setInstances(newInstances)
    if (activeInstanceId === instanceId) {
      setActiveInstanceId(newInstances[0]?.id || null)
    }
    setHasChanges(true)
  }

  const handleInstanceNameChange = (instanceId: string, name: string) => {
    setInstances(
      instances.map((i) => (i.id === instanceId ? { ...i, name } : i))
    )
    setHasChanges(true)
  }

  const handleConfigChange = (instanceId: string, key: string, value: unknown) => {
    setInstances(
      instances.map((i) =>
        i.id === instanceId
          ? { ...i, config: { ...i.config, [key]: value } }
          : i
      )
    )
    setHasChanges(true)
    setTestResult(null)
  }

  const handleSetDefault = (instanceId: string) => {
    setInstances(
      instances.map((i) => ({ ...i, isDefault: i.id === instanceId }))
    )
    setHasChanges(true)
  }

  const handleTestConnection = async () => {
    if (!activeInstance) return

    setTesting(true)
    setTestResult(null)

    try {
      const success = await onTestConnection(provider.id, activeInstance.config)
      setTestResult(success ? 'success' : 'error')
    } catch {
      setTestResult('error')
    } finally {
      setTesting(false)
    }

    // Clear result after 5 seconds
    setTimeout(() => setTestResult(null), 5000)
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      await onSave(provider.id, instances)
      setHasChanges(false)
      onClose()
    } catch (error) {
      console.error('Failed to save configuration:', error)
    } finally {
      setSaving(false)
    }
  }

  const toggleShowToken = (instanceId: string) => {
    setShowTokens((prev) => ({ ...prev, [instanceId]: !prev[instanceId] }))
  }

  const renderProviderFields = () => {
    if (!activeInstance) return null

    const config = activeInstance.config
    const showToken = showTokens[activeInstance.id] || false

    switch (provider.id) {
      case 'ngrok':
        return (
          <>
            <FormField>
              <Label htmlFor="ngrok-token" required description="Your ngrok authentication token">
                Auth Token
              </Label>
              <div className="relative">
                <Input
                  id="ngrok-token"
                  type={showToken ? 'text' : 'password'}
                  value={(config.authToken as string) || ''}
                  onChange={(e) =>
                    handleConfigChange(activeInstance.id, 'authToken', e.target.value)
                  }
                  placeholder="Enter your ngrok auth token..."
                />
                <button
                  type="button"
                  onClick={() => toggleShowToken(activeInstance.id)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              <HelpText>
                Get your token from{' '}
                <a
                  href="https://dashboard.ngrok.com/get-started/your-authtoken"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary hover:underline"
                >
                  ngrok dashboard
                </a>
              </HelpText>
            </FormField>

            <FormField>
              <Label htmlFor="ngrok-region" description="Server region for tunnels">
                Region
              </Label>
              <select
                id="ngrok-region"
                value={(config.region as string) || 'us'}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'region', e.target.value)
                }
                className="flex h-10 w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                <option value="us">United States (us)</option>
                <option value="eu">Europe (eu)</option>
                <option value="ap">Asia/Pacific (ap)</option>
                <option value="au">Australia (au)</option>
                <option value="sa">South America (sa)</option>
                <option value="jp">Japan (jp)</option>
                <option value="in">India (in)</option>
              </select>
            </FormField>

            <FormField>
              <Label htmlFor="ngrok-subdomain" description="Custom subdomain (paid plans only)">
                Subdomain
              </Label>
              <Input
                id="ngrok-subdomain"
                value={(config.subdomain as string) || ''}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'subdomain', e.target.value)
                }
                placeholder="my-app (optional)"
              />
              <HelpText>Leave empty for random subdomain</HelpText>
            </FormField>
          </>
        )

      case 'cloudflare':
        return (
          <>
            <FormField>
              <Label htmlFor="cf-token" required description="Cloudflare Tunnel token">
                Tunnel Token
              </Label>
              <div className="relative">
                <Input
                  id="cf-token"
                  type={showToken ? 'text' : 'password'}
                  value={(config.tunnelToken as string) || ''}
                  onChange={(e) =>
                    handleConfigChange(activeInstance.id, 'tunnelToken', e.target.value)
                  }
                  placeholder="Enter Cloudflare tunnel token..."
                />
                <button
                  type="button"
                  onClick={() => toggleShowToken(activeInstance.id)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              <HelpText>
                Create a tunnel in the{' '}
                <a
                  href="https://one.dash.cloudflare.com/"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary hover:underline"
                >
                  Cloudflare Zero Trust dashboard
                </a>
              </HelpText>
            </FormField>
          </>
        )

      case 'tailscale':
        return (
          <>
            <FormField>
              <Label htmlFor="ts-authkey" description="Tailscale auth key for headless login">
                Auth Key
              </Label>
              <div className="relative">
                <Input
                  id="ts-authkey"
                  type={showToken ? 'text' : 'password'}
                  value={(config.authToken as string) || ''}
                  onChange={(e) =>
                    handleConfigChange(activeInstance.id, 'authToken', e.target.value)
                  }
                  placeholder="tskey-auth-..."
                />
                <button
                  type="button"
                  onClick={() => toggleShowToken(activeInstance.id)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
              <HelpText>
                Generate an auth key in the{' '}
                <a
                  href="https://login.tailscale.com/admin/settings/keys"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary hover:underline"
                >
                  Tailscale admin console
                </a>
              </HelpText>
            </FormField>
          </>
        )

      case 'bore':
        return (
          <>
            <FormField>
              <Label htmlFor="bore-server" description="Bore server to connect to">
                Server
              </Label>
              <Input
                id="bore-server"
                value={(config.server as string) || 'bore.pub'}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'server', e.target.value)
                }
                placeholder="bore.pub"
              />
              <HelpText>Default: bore.pub (or run your own server)</HelpText>
            </FormField>
          </>
        )

      case 'wireguard':
        return (
          <>
            <FormField>
              <Label htmlFor="wg-listen-port" description="UDP port for WireGuard to listen on">
                Listen Port
              </Label>
              <Input
                id="wg-listen-port"
                type="number"
                value={(config.listenPort as number) || 51820}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'listenPort', parseInt(e.target.value) || 51820)
                }
                placeholder="51820"
              />
            </FormField>

            <FormField>
              <Label htmlFor="wg-private-key" required description="WireGuard private key for this server">
                Private Key
              </Label>
              <div className="relative">
                <Input
                  id="wg-private-key"
                  type={showToken ? 'text' : 'password'}
                  value={(config.privateKey as string) || ''}
                  onChange={(e) =>
                    handleConfigChange(activeInstance.id, 'privateKey', e.target.value)
                  }
                  placeholder="Generate with: wg genkey"
                />
                <button
                  type="button"
                  onClick={() => toggleShowToken(activeInstance.id)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showToken ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </FormField>

            <FormField>
              <Label htmlFor="wg-address" required description="IP address for this WireGuard interface">
                Interface Address
              </Label>
              <Input
                id="wg-address"
                value={(config.address as string) || ''}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'address', e.target.value)
                }
                placeholder="10.0.0.1/24"
              />
            </FormField>

            <HelpText>
              This configures the WireGuard server on this machine. Clients will connect to this endpoint.
            </HelpText>
          </>
        )

      case 'zerotier':
        return (
          <>
            <FormField>
              <Label htmlFor="zt-network-id" required description="ZeroTier network ID to join">
                Network ID
              </Label>
              <Input
                id="zt-network-id"
                value={(config.networkId as string) || ''}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'networkId', e.target.value)
                }
                placeholder="e.g., 8056c2e21c000001"
              />
              <HelpText>
                Get your network ID from the{' '}
                <a
                  href="https://my.zerotier.com/"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary hover:underline"
                >
                  ZeroTier Central
                </a>
                . This machine will join the network and be accessible to other members.
              </HelpText>
            </FormField>
          </>
        )

      case 'vscode-tunnel':
        return (
          <>
            <FormField>
              <Label htmlFor="vsc-name" description="Name for this tunnel (shown in VS Code)">
                Machine Name
              </Label>
              <Input
                id="vsc-name"
                value={(config.machineName as string) || ''}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'machineName', e.target.value)
                }
                placeholder="my-dev-machine"
              />
              <HelpText>
                This name will appear when connecting via vscode.dev or VS Code Remote.
              </HelpText>
            </FormField>

            <FormField>
              <Label htmlFor="vsc-provider" description="Authentication provider">
                Auth Provider
              </Label>
              <select
                id="vsc-provider"
                value={(config.authProvider as string) || 'github'}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'authProvider', e.target.value)
                }
                className="flex h-10 w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                <option value="github">GitHub</option>
                <option value="microsoft">Microsoft</option>
              </select>
            </FormField>

            <div className="rounded-md bg-muted p-4">
              <p className="text-sm text-muted-foreground">
                VS Code Tunnels makes this machine accessible via vscode.dev. Authentication is handled
                interactively when the tunnel starts.
              </p>
            </div>
          </>
        )

      case 'ssh-forward':
        return (
          <>
            <FormField>
              <Label htmlFor="ssh-listen-port" description="SSH server port on this machine">
                SSH Listen Port
              </Label>
              <Input
                id="ssh-listen-port"
                type="number"
                value={(config.listenPort as number) || 22}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'listenPort', parseInt(e.target.value) || 22)
                }
                placeholder="22"
              />
            </FormField>

            <FormField>
              <Label htmlFor="ssh-expose-ports" description="Local ports to allow forwarding to">
                Allowed Forward Ports
              </Label>
              <Input
                id="ssh-expose-ports"
                value={(config.allowedPorts as string) || ''}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'allowedPorts', e.target.value)
                }
                placeholder="8080, 3000, 5432 (comma-separated)"
              />
              <HelpText>
                Ports that SSH clients can forward to. Leave empty to allow all ports.
              </HelpText>
            </FormField>

            <div className="rounded-md bg-muted p-4">
              <p className="text-sm text-muted-foreground">
                Configures SSH server to accept connections and allow port forwarding to local services.
              </p>
            </div>
          </>
        )

      case 'reverse-ssh':
        return (
          <>
            <FormField>
              <Label htmlFor="rssh-relay" required description="Relay server to connect to">
                Relay Server
              </Label>
              <Input
                id="rssh-relay"
                value={(config.relayHost as string) || ''}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'relayHost', e.target.value)
                }
                placeholder="relay.example.com"
              />
              <HelpText>
                The SSH server that will accept your reverse tunnel and route connections back.
              </HelpText>
            </FormField>

            <FormField>
              <Label htmlFor="rssh-relay-port" description="SSH port on the relay server">
                Relay Port
              </Label>
              <Input
                id="rssh-relay-port"
                type="number"
                value={(config.relayPort as number) || 22}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'relayPort', parseInt(e.target.value) || 22)
                }
                placeholder="22"
              />
            </FormField>

            <FormField>
              <Label htmlFor="rssh-user" required description="Username on the relay server">
                Relay Username
              </Label>
              <Input
                id="rssh-user"
                value={(config.relayUser as string) || ''}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'relayUser', e.target.value)
                }
                placeholder="tunnel-user"
              />
            </FormField>

            <FormField>
              <Label htmlFor="rssh-remote-port" description="Port on relay server that routes to this machine">
                Remote Port
              </Label>
              <Input
                id="rssh-remote-port"
                type="number"
                value={(config.remotePort as number) || 2222}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'remotePort', parseInt(e.target.value) || 2222)
                }
                placeholder="2222"
              />
              <HelpText>
                Clients connect to relay-server:2222 to reach this machine.
              </HelpText>
            </FormField>

            <div className="rounded-md bg-muted p-4">
              <p className="text-sm text-muted-foreground">
                Creates a reverse SSH tunnel to make this machine accessible through a relay server,
                bypassing NAT and firewalls.
              </p>
            </div>
          </>
        )

      case 'bastion':
        return (
          <>
            <FormField>
              <Label htmlFor="bastion-port" description="SSH port for bastion access">
                Listen Port
              </Label>
              <Input
                id="bastion-port"
                type="number"
                value={(config.listenPort as number) || 22}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'listenPort', parseInt(e.target.value) || 22)
                }
                placeholder="22"
              />
            </FormField>

            <FormField>
              <Label htmlFor="bastion-allowed-targets" description="Hosts this bastion can forward to">
                Allowed Target Hosts
              </Label>
              <Input
                id="bastion-allowed-targets"
                value={(config.allowedTargets as string) || ''}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'allowedTargets', e.target.value)
                }
                placeholder="10.0.0.0/8, 192.168.1.0/24"
              />
              <HelpText>
                CIDR ranges or hostnames that clients can jump to through this bastion.
              </HelpText>
            </FormField>

            <FormField>
              <Label htmlFor="bastion-log" description="Enable session logging">
                Session Logging
              </Label>
              <select
                id="bastion-log"
                value={(config.sessionLogging as string) || 'enabled'}
                onChange={(e) =>
                  handleConfigChange(activeInstance.id, 'sessionLogging', e.target.value)
                }
                className="flex h-10 w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                <option value="enabled">Enabled</option>
                <option value="disabled">Disabled</option>
              </select>
            </FormField>

            <div className="rounded-md bg-muted p-4">
              <p className="text-sm text-muted-foreground">
                Configures this machine as a bastion/jump host. Clients SSH here first, then jump to internal targets.
              </p>
            </div>
          </>
        )

      default:
        return (
          <div className="rounded-md bg-muted p-4">
            <p className="text-sm text-muted-foreground">
              This provider doesn't require additional configuration.
            </p>
          </div>
        )
    }
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Configure ${provider.name}`}
      footer={
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            {testResult === 'success' && (
              <div className="flex items-center gap-2 text-sm text-green-600">
                <CheckCircle className="h-4 w-4" />
                <span>Connection successful</span>
              </div>
            )}
            {testResult === 'error' && (
              <div className="flex items-center gap-2 text-sm text-red-600">
                <XCircle className="h-4 w-4" />
                <span>Connection failed</span>
              </div>
            )}
          </div>
          <div className="flex gap-3">
            <Button variant="secondary" onClick={handleTestConnection} loading={testing}>
              Test Connection
            </Button>
            <Button variant="primary" onClick={handleSave} loading={saving} disabled={!hasChanges}>
              <Save className="mr-2 h-4 w-4" />
              Save Configuration
            </Button>
          </div>
        </div>
      }
    >
      <div className="space-y-6">
        {/* Instance Tabs */}
        <div className="border-b border-border">
          <div className="flex items-center gap-2 overflow-x-auto pb-2">
            {instances.map((instance) => (
              <button
                key={instance.id}
                onClick={() => setActiveInstanceId(instance.id)}
                className={cn(
                  'flex items-center gap-2 px-3 py-2 rounded-t-md text-sm font-medium whitespace-nowrap transition-colors',
                  activeInstanceId === instance.id
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:text-foreground hover:bg-accent'
                )}
              >
                {instance.name}
                {instance.isDefault && (
                  <span className="text-xs opacity-70">(default)</span>
                )}
              </button>
            ))}
            <button
              onClick={handleAddInstance}
              className="flex items-center gap-1 px-3 py-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              <Plus className="h-4 w-4" />
              Add Instance
            </button>
          </div>
        </div>

        {/* Instance Configuration */}
        {activeInstance && (
          <div className="space-y-4">
            {/* Instance Name */}
            <FormField>
              <Label htmlFor="instance-name" description="Name for this configuration instance">
                Instance Name
              </Label>
              <div className="flex gap-2">
                <Input
                  id="instance-name"
                  value={activeInstance.name}
                  onChange={(e) =>
                    handleInstanceNameChange(activeInstance.id, e.target.value)
                  }
                  placeholder="Instance name"
                />
                {!activeInstance.isDefault && (
                  <>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => handleSetDefault(activeInstance.id)}
                    >
                      Set Default
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleRemoveInstance(activeInstance.id)}
                    >
                      <Trash2 className="h-4 w-4 text-red-500" />
                    </Button>
                  </>
                )}
              </div>
            </FormField>

            {/* Provider-specific fields */}
            {renderProviderFields()}
          </div>
        )}

        {/* Multi-instance info */}
        {instances.length > 1 && (
          <div className="rounded-lg border border-blue-500/30 bg-blue-500/10 p-4">
            <p className="text-sm text-blue-900 dark:text-blue-100">
              You have {instances.length} configuration instances. The default instance will be
              used when connecting without specifying an instance.
            </p>
          </div>
        )}
      </div>
    </Modal>
  )
}
