import { useState } from 'react'
import { FormField, Label, Input } from '../ui/Form'
import Button from '../ui/Button'
import {
  Trash2,
  Edit,
  Plus,
  Upload,
  Eye,
  EyeOff,
  CheckCircle,
} from 'lucide-react'
// cn utility available if needed
import type { ProviderType } from '@/types'

interface Credential {
  id: string
  provider: ProviderType
  name: string
  key: string
  createdAt: string
}

interface CredentialsManagerProps {
  credentials: Credential[]
  onAdd: (credential: Omit<Credential, 'id' | 'createdAt'>) => void
  onUpdate: (id: string, credential: Partial<Credential>) => void
  onDelete: (id: string) => void
  onImportFromEnv: () => Promise<void>
}

export function CredentialsManager({
  credentials,
  onAdd,
  onUpdate: _onUpdate,
  onDelete,
  onImportFromEnv,
}: CredentialsManagerProps) {
  const [isAdding, setIsAdding] = useState(false)
  const [_editingId, setEditingId] = useState<string | null>(null)
  const [showKeys, setShowKeys] = useState<Record<string, boolean>>({})
  const [importing, setImporting] = useState(false)

  const [newCredential, setNewCredential] = useState({
    provider: 'ngrok' as ProviderType,
    name: '',
    key: '',
  })

  const handleAdd = () => {
    if (newCredential.name && newCredential.key) {
      onAdd(newCredential)
      setNewCredential({ provider: 'ngrok', name: '', key: '' })
      setIsAdding(false)
    }
  }

  const handleDelete = (id: string) => {
    if (confirm('Are you sure you want to delete this credential?')) {
      onDelete(id)
    }
  }

  const toggleShowKey = (id: string) => {
    setShowKeys(prev => ({ ...prev, [id]: !prev[id] }))
  }

  const handleImport = async () => {
    setImporting(true)
    try {
      await onImportFromEnv()
    } finally {
      setImporting(false)
    }
  }

  const groupedCredentials = credentials.reduce((acc, cred) => {
    if (!acc[cred.provider]) {
      acc[cred.provider] = []
    }
    acc[cred.provider].push(cred)
    return acc
  }, {} as Record<ProviderType, Credential[]>)

  const maskKey = (key: string) => {
    if (key.length <= 8) return '*'.repeat(key.length)
    return key.slice(0, 4) + '*'.repeat(key.length - 8) + key.slice(-4)
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold text-foreground">Credentials Manager</h2>
          <p className="mt-1 text-sm text-muted-foreground">
            Manage authentication credentials for tunnel providers.
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={handleImport}
            loading={importing}
          >
            <Upload className="mr-2 h-4 w-4" />
            Import from Env
          </Button>
          <Button variant="primary" size="sm" onClick={() => setIsAdding(true)}>
            <Plus className="mr-2 h-4 w-4" />
            Add Credential
          </Button>
        </div>
      </div>

      {/* Add New Credential Form */}
      {isAdding && (
        <div className="rounded-lg border border-border bg-card p-4">
          <h3 className="mb-4 font-semibold text-foreground">New Credential</h3>
          <div className="space-y-4">
            <FormField>
              <Label htmlFor="new-provider">Provider</Label>
              <select
                id="new-provider"
                value={newCredential.provider}
                onChange={(e) =>
                  setNewCredential({
                    ...newCredential,
                    provider: e.target.value as ProviderType,
                  })
                }
                className="flex h-10 w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
              >
                <option value="ngrok">ngrok</option>
                <option value="cloudflare">Cloudflare</option>
                <option value="localhost">Localhost</option>
                <option value="custom">Custom</option>
              </select>
            </FormField>

            <FormField>
              <Label htmlFor="new-name">Name</Label>
              <Input
                id="new-name"
                value={newCredential.name}
                onChange={(e) =>
                  setNewCredential({ ...newCredential, name: e.target.value })
                }
                placeholder="e.g., Production Account"
              />
            </FormField>

            <FormField>
              <Label htmlFor="new-key">Auth Key / Token</Label>
              <Input
                id="new-key"
                type="password"
                value={newCredential.key}
                onChange={(e) =>
                  setNewCredential({ ...newCredential, key: e.target.value })
                }
                placeholder="Enter authentication key or token..."
              />
            </FormField>

            <div className="flex justify-end gap-2">
              <Button
                variant="secondary"
                onClick={() => {
                  setIsAdding(false)
                  setNewCredential({ provider: 'ngrok', name: '', key: '' })
                }}
              >
                Cancel
              </Button>
              <Button variant="primary" onClick={handleAdd}>Add Credential</Button>
            </div>
          </div>
        </div>
      )}

      {/* Credentials List */}
      <div className="space-y-6">
        {Object.keys(groupedCredentials).length === 0 ? (
          <div className="rounded-lg border border-dashed border-border p-8 text-center">
            <p className="text-sm text-muted-foreground">
              No credentials stored. Add a credential to get started.
            </p>
          </div>
        ) : (
          Object.entries(groupedCredentials).map(([provider, creds]) => (
            <div key={provider}>
              <h3 className="mb-3 text-sm font-semibold uppercase tracking-wider text-muted-foreground">
                {provider}
              </h3>
              <div className="space-y-2">
                {creds.map((cred) => (
                  <div
                    key={cred.id}
                    className="flex items-center justify-between rounded-lg border border-border bg-card p-4"
                  >
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <h4 className="font-medium text-foreground">{cred.name}</h4>
                        {cred.provider === 'ngrok' && (
                          <span className="rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">
                            Default
                          </span>
                        )}
                      </div>
                      <div className="mt-1 flex items-center gap-2 text-sm text-muted-foreground">
                        <code className="rounded bg-muted px-2 py-0.5 font-mono text-xs">
                          {showKeys[cred.id] ? cred.key : maskKey(cred.key)}
                        </code>
                        <button
                          onClick={() => toggleShowKey(cred.id)}
                          className="text-muted-foreground hover:text-foreground"
                        >
                          {showKeys[cred.id] ? (
                            <EyeOff className="h-3 w-3" />
                          ) : (
                            <Eye className="h-3 w-3" />
                          )}
                        </button>
                      </div>
                      <p className="mt-1 text-xs text-muted-foreground">
                        Added {new Date(cred.createdAt).toLocaleDateString()}
                      </p>
                    </div>

                    <div className="flex items-center gap-2">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setEditingId(cred.id)}
                      >
                        <Edit className="h-4 w-4" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleDelete(cred.id)}
                      >
                        <Trash2 className="h-4 w-4 text-red-500" />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))
        )}
      </div>

      {/* Security Notice */}
      <div className="rounded-lg border border-yellow-500/30 bg-yellow-500/10 p-4">
        <div className="flex items-start gap-3">
          <CheckCircle className="h-5 w-5 text-yellow-600" />
          <div className="flex-1">
            <h4 className="font-medium text-yellow-900 dark:text-yellow-100">
              Secure Storage
            </h4>
            <p className="mt-1 text-sm text-yellow-800 dark:text-yellow-200">
              Credentials are encrypted and stored securely. They are never sent to
              third parties except when authenticating with the respective tunnel
              providers.
            </p>
          </div>
        </div>
      </div>
    </div>
  )
}
