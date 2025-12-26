import { useState, useEffect } from 'react'
import { SettingsSidebar, type SettingsSection } from '../components/config/SettingsSidebar'
import { GeneralSettings } from '../components/config/GeneralSettings'
import { ProviderSettings } from '../components/config/ProviderSettings'
import { CredentialsManager } from '../components/config/CredentialsManager'
import { AppearanceSettings } from '../components/config/AppearanceSettings'
import Button from '../components/ui/Button'
import { configAPI, providersAPI } from '@/api/client'
import { useUIStore } from '@/stores/ui'
import { Save, RotateCcw, AlertCircle } from 'lucide-react'
import type { Config, ProviderType, Provider } from '@/types'

interface Credential {
  id: string
  provider: ProviderType
  name: string
  key: string
  createdAt: string
}

export function Settings() {
  const { addNotification } = useUIStore()
  const [activeSection, setActiveSection] = useState<SettingsSection>('general')
  const [config, setConfig] = useState<Config | null>(null)
  const [providers, setProviders] = useState<Provider[]>([])
  const [credentials, setCredentials] = useState<Credential[]>([])
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  // Load initial data
  useEffect(() => {
    loadData()
  }, [])

  const loadData = async () => {
    try {
      setLoading(true)
      const [configData, providersData] = await Promise.all([
        configAPI.get(),
        providersAPI.list(),
      ])

      setConfig(configData)
      setProviders(providersData)

      // Load credentials from localStorage (in a real app, this would come from a secure API)
      const savedCredentials = localStorage.getItem('tunnel-credentials')
      if (savedCredentials) {
        setCredentials(JSON.parse(savedCredentials))
      }
    } catch (error) {
      addNotification('error', 'Failed to load settings', 'Please try again later.')
      console.error('Failed to load settings:', error)
    } finally {
      setLoading(false)
    }
  }

  // Warn about unsaved changes
  useEffect(() => {
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (hasUnsavedChanges) {
        e.preventDefault()
        e.returnValue = ''
      }
    }

    window.addEventListener('beforeunload', handleBeforeUnload)
    return () => window.removeEventListener('beforeunload', handleBeforeUnload)
  }, [hasUnsavedChanges])

  const handleConfigChange = (updates: Partial<Config>) => {
    setConfig(prev => prev ? { ...prev, ...updates } : null)
    setHasUnsavedChanges(true)
  }

  const handleSave = async () => {
    if (!config) return

    try {
      setSaving(true)
      await configAPI.update(config)
      setHasUnsavedChanges(false)
      addNotification('success', 'Settings saved', 'Your changes have been saved successfully.')
    } catch (error) {
      addNotification('error', 'Failed to save settings', 'Please try again.')
      console.error('Failed to save settings:', error)
    } finally {
      setSaving(false)
    }
  }

  const handleReset = async () => {
    if (confirm('Are you sure you want to discard all unsaved changes?')) {
      await loadData()
      setHasUnsavedChanges(false)
      addNotification('info', 'Changes discarded', 'Settings have been reset to saved values.')
    }
  }

  const handleAddCredential = (credential: Omit<Credential, 'id' | 'createdAt'>) => {
    const newCredential: Credential = {
      ...credential,
      id: `cred-${Date.now()}`,
      createdAt: new Date().toISOString(),
    }

    const updated = [...credentials, newCredential]
    setCredentials(updated)
    localStorage.setItem('tunnel-credentials', JSON.stringify(updated))
    addNotification('success', 'Credential added', 'The credential has been saved securely.')
  }

  const handleUpdateCredential = (id: string, updates: Partial<Credential>) => {
    const updated = credentials.map(cred =>
      cred.id === id ? { ...cred, ...updates } : cred
    )
    setCredentials(updated)
    localStorage.setItem('tunnel-credentials', JSON.stringify(updated))
    addNotification('success', 'Credential updated', 'The credential has been updated.')
  }

  const handleDeleteCredential = (id: string) => {
    const updated = credentials.filter(cred => cred.id !== id)
    setCredentials(updated)
    localStorage.setItem('tunnel-credentials', JSON.stringify(updated))
    addNotification('success', 'Credential deleted', 'The credential has been removed.')
  }

  const handleImportFromEnv = async () => {
    // Mock implementation - in a real app, this would call an API endpoint
    return new Promise<void>((resolve) => {
      setTimeout(() => {
        addNotification('info', 'Import completed', 'No environment credentials found.')
        resolve()
      }, 1000)
    })
  }

  const handleTestConnection = async (_providerType: ProviderType) => {
    // Mock implementation - in a real app, this would test the actual connection
    return new Promise<boolean>((resolve) => {
      setTimeout(() => {
        resolve(Math.random() > 0.3) // 70% success rate for demo
      }, 1500)
    })
  }

  const renderContent = () => {
    if (loading) {
      return (
        <div className="flex h-full items-center justify-center">
          <div className="text-center">
            <div className="mx-auto h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
            <p className="mt-4 text-sm text-muted-foreground">Loading settings...</p>
          </div>
        </div>
      )
    }

    switch (activeSection) {
      case 'general':
        return (
          <GeneralSettings
            config={config}
            onConfigChange={handleConfigChange}
            availableProviders={['ngrok', 'cloudflare', 'localhost', 'custom']}
          />
        )

      case 'ngrok':
      case 'cloudflare':
      case 'localhost':
      case 'custom': {
        const provider = providers.find(p => p.type === activeSection)
        const providerName = activeSection.charAt(0).toUpperCase() + activeSection.slice(1)

        return (
          <ProviderSettings
            providerType={activeSection}
            providerName={providerName}
            config={provider?.config || null}
            onConfigChange={(_providerConfig) => {
              // In a real app, this would update the provider config via API
              setHasUnsavedChanges(true)
            }}
            onTestConnection={() => handleTestConnection(activeSection)}
          />
        )
      }

      case 'credentials':
        return (
          <CredentialsManager
            credentials={credentials}
            onAdd={handleAddCredential}
            onUpdate={handleUpdateCredential}
            onDelete={handleDeleteCredential}
            onImportFromEnv={handleImportFromEnv}
          />
        )

      case 'appearance':
        return <AppearanceSettings />

      case 'about':
        return (
          <div className="space-y-6">
            <div>
              <h2 className="text-2xl font-bold text-foreground">About Tunnel Web</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                Version and application information.
              </p>
            </div>

            <div className="space-y-4">
              <div className="rounded-lg border border-border bg-card p-6">
                <h3 className="text-lg font-semibold text-foreground">Version</h3>
                <p className="mt-2 text-sm text-muted-foreground">v0.1.0</p>
              </div>

              <div className="rounded-lg border border-border bg-card p-6">
                <h3 className="text-lg font-semibold text-foreground">Description</h3>
                <p className="mt-2 text-sm text-muted-foreground">
                  A unified web interface for managing multiple tunnel providers including
                  ngrok, Cloudflare Tunnel, and custom solutions.
                </p>
              </div>

              <div className="rounded-lg border border-border bg-card p-6">
                <h3 className="text-lg font-semibold text-foreground">Links</h3>
                <div className="mt-2 space-y-2">
                  <a
                    href="#"
                    className="block text-sm text-primary hover:underline"
                  >
                    Documentation
                  </a>
                  <a
                    href="#"
                    className="block text-sm text-primary hover:underline"
                  >
                    GitHub Repository
                  </a>
                  <a
                    href="#"
                    className="block text-sm text-primary hover:underline"
                  >
                    Report an Issue
                  </a>
                </div>
              </div>
            </div>
          </div>
        )

      default:
        return null
    }
  }

  const providerList = [
    { type: 'ngrok', name: 'ngrok' },
    { type: 'cloudflare', name: 'Cloudflare' },
    { type: 'localhost', name: 'Localhost' },
    { type: 'custom', name: 'Custom' },
  ]

  return (
    <div className="flex h-[calc(100vh-73px)] overflow-hidden">
      <SettingsSidebar
        activeSection={activeSection}
        onSectionChange={setActiveSection}
        providers={providerList}
      />

      <div className="flex flex-1 flex-col overflow-hidden">
        {/* Unsaved Changes Warning */}
        {hasUnsavedChanges && (
          <div className="border-b border-yellow-500/30 bg-yellow-500/10 px-6 py-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <AlertCircle className="h-4 w-4 text-yellow-600" />
                <span className="text-sm font-medium text-yellow-900 dark:text-yellow-100">
                  You have unsaved changes
                </span>
              </div>
              <div className="flex gap-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleReset}
                  disabled={saving}
                >
                  <RotateCcw className="mr-2 h-3 w-3" />
                  Reset
                </Button>
                <Button
                  variant="primary"
                  size="sm"
                  onClick={handleSave}
                  loading={saving}
                >
                  <Save className="mr-2 h-3 w-3" />
                  Save Changes
                </Button>
              </div>
            </div>
          </div>
        )}

        {/* Content Area */}
        <div className="flex-1 overflow-y-auto p-8">
          <div className="mx-auto max-w-4xl">
            {renderContent()}
          </div>
        </div>

        {/* Footer Actions */}
        {(activeSection === 'general' ||
          activeSection === 'ngrok' ||
          activeSection === 'cloudflare' ||
          activeSection === 'custom') && (
          <div className="border-t border-border bg-card px-8 py-4">
            <div className="mx-auto flex max-w-4xl items-center justify-end gap-3">
              <Button
                variant="secondary"
                onClick={handleReset}
                disabled={!hasUnsavedChanges || saving}
              >
                <RotateCcw className="mr-2 h-4 w-4" />
                Reset
              </Button>
              <Button
                variant="primary"
                onClick={handleSave}
                disabled={!hasUnsavedChanges}
                loading={saving}
              >
                <Save className="mr-2 h-4 w-4" />
                Save Changes
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
