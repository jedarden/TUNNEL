import { useState, useEffect } from 'react'
import { FormField, Label, Input, HelpText } from '../ui/Form'
import Button from '../ui/Button'
import { Eye, EyeOff, CheckCircle, XCircle } from 'lucide-react'
import type { ProviderType } from '@/types'

interface ProviderConfig {
  authKey?: string
  region?: string
  subdomain?: string
  port?: number
  protocol?: string
  [key: string]: unknown
}

interface ProviderSettingsProps {
  providerType: ProviderType
  providerName: string
  config: ProviderConfig | null
  onConfigChange: (config: ProviderConfig) => void
  onTestConnection?: () => Promise<boolean>
}

export function ProviderSettings({
  providerType,
  providerName,
  config,
  onConfigChange,
  onTestConnection,
}: ProviderSettingsProps) {
  const [authKey, setAuthKey] = useState('')
  const [showAuthKey, setShowAuthKey] = useState(false)
  const [region, setRegion] = useState('us')
  const [subdomain, setSubdomain] = useState('')
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<'success' | 'error' | null>(null)

  useEffect(() => {
    if (config) {
      setAuthKey(config.authKey || '')
      setRegion(config.region || 'us')
      setSubdomain(config.subdomain || '')
    }
  }, [config])

  const handleAuthKeyChange = (value: string) => {
    setAuthKey(value)
    onConfigChange({
      ...config,
      authKey: value,
    })
  }

  const handleRegionChange = (value: string) => {
    setRegion(value)
    onConfigChange({
      ...config,
      region: value,
    })
  }

  const handleSubdomainChange = (value: string) => {
    setSubdomain(value)
    onConfigChange({
      ...config,
      subdomain: value,
    })
  }

  const handleTestConnection = async () => {
    if (!onTestConnection) return

    setTesting(true)
    setTestResult(null)

    try {
      const success = await onTestConnection()
      setTestResult(success ? 'success' : 'error')
    } catch (error) {
      setTestResult('error')
    } finally {
      setTesting(false)
    }

    // Clear test result after 5 seconds
    setTimeout(() => setTestResult(null), 5000)
  }

  const renderProviderSpecificFields = () => {
    switch (providerType) {
      case 'ngrok':
        return (
          <>
            <FormField>
              <Label
                htmlFor="ngrok-auth"
                required
                description="Your ngrok authentication token"
              >
                Auth Token
              </Label>
              <div className="relative">
                <Input
                  id="ngrok-auth"
                  type={showAuthKey ? 'text' : 'password'}
                  value={authKey}
                  onChange={(e) => handleAuthKeyChange(e.target.value)}
                  placeholder="Enter your ngrok auth token..."
                />
                <button
                  type="button"
                  onClick={() => setShowAuthKey(!showAuthKey)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showAuthKey ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                </button>
              </div>
              <HelpText>
                Get your auth token from{' '}
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
              <Label htmlFor="ngrok-region" description="Choose the server region">
                Region
              </Label>
              <Input
                id="ngrok-region"
                value={region}
                onChange={(e) => handleRegionChange(e.target.value)}
                placeholder="us, eu, ap, au, sa, jp, in"
              />
            </FormField>

            <FormField>
              <Label
                htmlFor="ngrok-subdomain"
                description="Custom subdomain (requires paid plan)"
              >
                Subdomain
              </Label>
              <Input
                id="ngrok-subdomain"
                value={subdomain}
                onChange={(e) => handleSubdomainChange(e.target.value)}
                placeholder="my-app"
              />
              <HelpText>Leave empty for random subdomain</HelpText>
            </FormField>
          </>
        )

      case 'cloudflare':
        return (
          <>
            <FormField>
              <Label
                htmlFor="cf-token"
                description="Cloudflare Tunnel token"
              >
                Tunnel Token
              </Label>
              <div className="relative">
                <Input
                  id="cf-token"
                  type={showAuthKey ? 'text' : 'password'}
                  value={authKey}
                  onChange={(e) => handleAuthKeyChange(e.target.value)}
                  placeholder="Enter Cloudflare tunnel token..."
                />
                <button
                  type="button"
                  onClick={() => setShowAuthKey(!showAuthKey)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showAuthKey ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
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

      case 'localhost':
        return (
          <div className="rounded-md bg-muted p-4">
            <p className="text-sm text-muted-foreground">
              Localhost mode doesn't require authentication. Configure port settings
              when creating a connection.
            </p>
          </div>
        )

      case 'custom':
        return (
          <>
            <FormField>
              <Label htmlFor="custom-key" description="Authentication key or token">
                Auth Key
              </Label>
              <div className="relative">
                <Input
                  id="custom-key"
                  type={showAuthKey ? 'text' : 'password'}
                  value={authKey}
                  onChange={(e) => handleAuthKeyChange(e.target.value)}
                  placeholder="Enter authentication key..."
                />
                <button
                  type="button"
                  onClick={() => setShowAuthKey(!showAuthKey)}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  {showAuthKey ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                </button>
              </div>
            </FormField>
          </>
        )

      default:
        return null
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <h2 className="text-2xl font-bold text-foreground">{providerName} Settings</h2>
        <p className="mt-1 text-sm text-muted-foreground">
          Configure authentication and provider-specific options.
        </p>
      </div>

      <div className="space-y-4">
        {renderProviderSpecificFields()}

        {onTestConnection && providerType !== 'localhost' && (
          <FormField>
            <div className="flex items-center gap-3">
              <Button
                variant="secondary"
                onClick={handleTestConnection}
                loading={testing}
                disabled={!authKey}
              >
                Test Connection
              </Button>

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
            <HelpText>
              Verify that your credentials are valid and the provider is accessible
            </HelpText>
          </FormField>
        )}
      </div>
    </div>
  )
}
