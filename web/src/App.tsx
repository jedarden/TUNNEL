import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Layout } from './components/layout'
import { Providers } from './pages/Providers'
import { Settings } from './pages/Settings'

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

function App() {
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
