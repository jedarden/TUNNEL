import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Layout } from './components/layout'
import { Providers } from './pages/Providers'
import { Settings } from './pages/Settings'
import {
  AUTH_REQUIRED_EVENT,
  clearAuthToken,
  getAuthToken,
  setAuthToken,
  verifyAuthToken,
} from './api/client'

// Placeholder components - to be implemented
const Dashboard = () => (
  <div>
    <h1 className="text-3xl font-bold text-foreground">Dashboard</h1>
    <p className="mt-4 text-muted-foreground">
      Reverse proxy metrics and status will be displayed here.
    </p>
  </div>
)

const Connections = () => (
  <div>
    <h1 className="text-3xl font-bold text-foreground">Connections</h1>
    <p className="mt-4 text-muted-foreground">
      View and manage active tunnel connections here.
    </p>
  </div>
)

interface AuthenticationPromptProps {
  onAuthenticated: () => void
}

function AuthenticationPrompt({ onAuthenticated }: AuthenticationPromptProps) {
  const [token, setToken] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError(null)
    setSubmitting(true)

    try {
      if (!(await verifyAuthToken(token))) {
        setError('That token was not accepted. Copy it exactly from the TUNNEL startup output.')
        return
      }
      setAuthToken(token)
      onAuthenticated()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to verify the token')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-background p-6 text-foreground">
      <section className="w-full max-w-md rounded-lg border bg-card p-6 shadow-sm">
        <h1 className="text-2xl font-semibold">Unlock TUNNEL</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Paste the API bearer token printed when TUNNEL started. It is kept only for this browser session.
        </p>
        <form className="mt-6 space-y-4" onSubmit={handleSubmit}>
          <div>
            <label className="mb-2 block text-sm font-medium" htmlFor="api-token">
              API bearer token
            </label>
            <input
              id="api-token"
              type="password"
              autoComplete="off"
              autoFocus
              spellCheck={false}
              value={token}
              onChange={(event) => setToken(event.target.value)}
              className="w-full rounded-md border bg-background px-3 py-2 font-mono text-sm outline-none focus:ring-2 focus:ring-ring"
              placeholder="Paste token"
              required
            />
          </div>
          {error && <p className="text-sm text-destructive" role="alert">{error}</p>}
          <button
            type="submit"
            disabled={submitting || token.trim() === ''}
            className="w-full rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:cursor-not-allowed disabled:opacity-50"
          >
            {submitting ? 'Verifying…' : 'Continue'}
          </button>
        </form>
      </section>
    </main>
  )
}

type AuthState = 'checking' | 'required' | 'authenticated'

function App() {
  const [authState, setAuthState] = useState<AuthState>(() =>
    getAuthToken() ? 'checking' : 'required'
  )

  useEffect(() => {
    let active = true
    const requireAuthentication = () => setAuthState('required')
    window.addEventListener(AUTH_REQUIRED_EVENT, requireAuthentication)

    const token = getAuthToken()
    if (token) {
      verifyAuthToken(token)
        .then((valid) => {
          if (!active) return
          if (valid) {
            setAuthState('authenticated')
          } else {
            clearAuthToken()
            setAuthState('required')
          }
        })
        .catch(() => {
          if (active) setAuthState('required')
        })
    }

    return () => {
      active = false
      window.removeEventListener(AUTH_REQUIRED_EVENT, requireAuthentication)
    }
  }, [])

  if (authState === 'checking') {
    return (
      <main className="flex min-h-screen items-center justify-center bg-background text-muted-foreground">
        Verifying API access…
      </main>
    )
  }

  if (authState === 'required') {
    return <AuthenticationPrompt onAuthenticated={() => setAuthState('authenticated')} />
  }

  return (
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/providers" element={<Providers />} />
          <Route path="/connections" element={<Connections />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  )
}

export default App
