import { useState, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowUpDown } from 'lucide-react'
import { ProviderCard } from '@/components/providers/ProviderCard'
import { ProviderDetail } from '@/components/providers/ProviderDetail'
import { ProviderSearch } from '@/components/providers/ProviderSearch'
import { CategoryTabs } from '@/components/providers/CategoryTabs'
import { cn } from '@/lib/utils'
import type { ProviderInfo, ProviderCategory } from '@/types'

// Mock provider data
const mockProviders: ProviderInfo[] = [
  // VPN/Mesh
  {
    id: 'tailscale',
    name: 'Tailscale',
    category: 'vpn-mesh',
    description: 'Zero-config VPN for building secure networks. Connect devices anywhere.',
    icon: '🔒',
    features: [
      'Zero-config mesh VPN',
      'End-to-end encryption',
      'Built on WireGuard',
      'Multi-platform support',
      'Free for personal use',
    ],
    installed: true,
    status: 'connected',
    latency: 12,
  },
  {
    id: 'wireguard',
    name: 'WireGuard',
    category: 'vpn-mesh',
    description: 'Fast, modern, secure VPN tunnel with state-of-the-art cryptography.',
    icon: '⚡',
    features: [
      'Extremely fast and lightweight',
      'Modern cryptography',
      'Simple configuration',
      'Cross-platform',
      'Open source',
    ],
    installed: true,
    status: 'available',
    latency: 8,
  },
  {
    id: 'zerotier',
    name: 'ZeroTier',
    category: 'vpn-mesh',
    description: 'Global area networking - combine the capabilities of VPN and SD-WAN.',
    icon: '🌐',
    features: [
      'Software-defined networking',
      'Peer-to-peer mesh network',
      'Easy network management',
      'Cross-platform support',
      'Free tier available',
    ],
    installed: false,
    status: 'available',
    latency: 18,
  },
  // Tunnels
  {
    id: 'cloudflare',
    name: 'Cloudflare Tunnel',
    category: 'tunnels',
    description: 'Secure tunnel from Cloudflare to your origin without opening firewall ports.',
    icon: '☁️',
    features: [
      'No open ports required',
      'DDoS protection',
      'Access control policies',
      'Load balancing',
      'Free tier available',
    ],
    installed: true,
    status: 'connected',
    latency: 15,
  },
  {
    id: 'ngrok',
    name: 'ngrok',
    category: 'tunnels',
    description: 'Instant, secure URL to your localhost server. Perfect for webhooks and demos.',
    icon: '🚇',
    features: [
      'Instant public URLs',
      'HTTPS tunnels',
      'Request inspection',
      'Custom domains',
      'Webhook testing',
    ],
    installed: true,
    status: 'available',
    latency: 22,
  },
  {
    id: 'bore',
    name: 'bore',
    category: 'tunnels',
    description: 'Simple CLI tool for exposing local ports to the internet.',
    icon: '🔧',
    features: [
      'Lightweight and fast',
      'No account required',
      'Open source',
      'Simple CLI interface',
      'TCP tunneling',
    ],
    installed: false,
    status: 'available',
    latency: 28,
  },
  // SSH
  {
    id: 'vscode-tunnel',
    name: 'VS Code Tunnels',
    category: 'ssh',
    description: 'Securely connect to a remote machine through a VS Code tunnel.',
    icon: '💻',
    features: [
      'Integrated with VS Code',
      'No SSH configuration needed',
      'GitHub authentication',
      'Cross-platform',
      'Free for VS Code users',
    ],
    installed: true,
    status: 'available',
    latency: 10,
  },
  {
    id: 'ssh-forward',
    name: 'SSH Port Forward',
    category: 'ssh',
    description: 'Traditional SSH port forwarding for secure remote access.',
    icon: '🔐',
    features: [
      'Standard SSH protocol',
      'Highly secure',
      'Wide compatibility',
      'Local and remote forwarding',
      'Free and open source',
    ],
    installed: true,
    status: 'available',
    latency: 14,
  },
  {
    id: 'reverse-ssh',
    name: 'Reverse SSH',
    category: 'ssh',
    description: 'Access machines behind NAT/firewall using reverse SSH tunnels.',
    icon: '🔄',
    features: [
      'Bypass NAT/firewalls',
      'No port forwarding needed',
      'Standard SSH protocol',
      'Persistent connections',
      'Free and open source',
    ],
    installed: false,
    status: 'available',
    latency: 16,
  },
  {
    id: 'bastion',
    name: 'Bastion',
    category: 'ssh',
    description: 'Jump host for accessing internal resources securely.',
    icon: '🏰',
    features: [
      'Centralized access point',
      'Audit logging',
      'Access control',
      'Session recording',
      'Compliance ready',
    ],
    installed: false,
    status: 'available',
    latency: 20,
  },
]

type SortOption = 'name' | 'status' | 'latency'

export function Providers() {
  const [searchQuery, setSearchQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'connected' | 'available'>('all')
  const [activeCategory, setActiveCategory] = useState<ProviderCategory>('all')
  const [sortBy, setSortBy] = useState<SortOption>('name')
  const [selectedProvider, setSelectedProvider] = useState<ProviderInfo | null>(null)
  const [isDetailOpen, setIsDetailOpen] = useState(false)

  // In a real app, this would fetch from the API
  const { data: providers = mockProviders, isLoading } = useQuery({
    queryKey: ['providers'],
    queryFn: async () => {
      // Simulate API call
      await new Promise((resolve) => setTimeout(resolve, 500))
      return mockProviders
    },
  })

  // Filter and sort providers
  const filteredProviders = useMemo(() => {
    let filtered = providers

    // Filter by search query
    if (searchQuery) {
      const query = searchQuery.toLowerCase()
      filtered = filtered.filter(
        (p) =>
          p.name.toLowerCase().includes(query) ||
          p.description.toLowerCase().includes(query) ||
          p.features.some((f) => f.toLowerCase().includes(query))
      )
    }

    // Filter by status
    if (statusFilter !== 'all') {
      filtered = filtered.filter((p) => p.status === statusFilter)
    }

    // Filter by category
    if (activeCategory !== 'all') {
      filtered = filtered.filter((p) => p.category === activeCategory)
    }

    // Sort
    filtered = [...filtered].sort((a, b) => {
      switch (sortBy) {
        case 'name':
          return a.name.localeCompare(b.name)
        case 'status':
          const statusOrder = { connected: 0, available: 1, error: 2 }
          return statusOrder[a.status] - statusOrder[b.status]
        case 'latency':
          return (a.latency ?? 999) - (b.latency ?? 999)
        default:
          return 0
      }
    })

    return filtered
  }, [providers, searchQuery, statusFilter, activeCategory, sortBy])

  // Count providers by category
  const categoryCounts = useMemo(() => {
    const counts: Record<ProviderCategory, number> = {
      all: providers.length,
      'vpn-mesh': 0,
      tunnels: 0,
      ssh: 0,
    }

    providers.forEach((p) => {
      counts[p.category]++
    })

    return counts
  }, [providers])

  const handleConfigure = (provider: ProviderInfo) => {
    console.log('Configure provider:', provider.id)
    // TODO: Navigate to configuration page or open config modal
  }

  const handleConnect = (provider: ProviderInfo) => {
    console.log('Connect to provider:', provider.id)
    // TODO: Initiate connection
  }

  const handleViewDetails = (provider: ProviderInfo) => {
    setSelectedProvider(provider)
    setIsDetailOpen(true)
  }

  const sortOptions: Array<{ value: SortOption; label: string }> = [
    { value: 'name', label: 'Name' },
    { value: 'status', label: 'Status' },
    { value: 'latency', label: 'Latency' },
  ]

  return (
    <div className="py-8 px-4 max-w-7xl mx-auto">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground mb-2">Tunnel Providers</h1>
        <p className="text-muted-foreground">
          Browse and connect to tunnel providers for secure remote access
        </p>
      </div>

      {/* Search and Filters */}
      <div className="mb-6">
        <ProviderSearch
          value={searchQuery}
          onChange={setSearchQuery}
          statusFilter={statusFilter}
          onStatusFilterChange={setStatusFilter}
        />
      </div>

      {/* Category Tabs */}
      <CategoryTabs
        activeCategory={activeCategory}
        onCategoryChange={setActiveCategory}
        counts={categoryCounts}
        className="mb-6"
      />

      {/* Sort Options */}
      <div className="flex items-center justify-between mb-6">
        <p className="text-sm text-muted-foreground">
          {filteredProviders.length} {filteredProviders.length === 1 ? 'provider' : 'providers'}{' '}
          found
        </p>
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">Sort by:</span>
          <div className="flex gap-1 border border-border rounded-lg p-1">
            {sortOptions.map((option) => (
              <button
                key={option.value}
                onClick={() => setSortBy(option.value)}
                className={cn(
                  'px-3 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-1.5',
                  sortBy === option.value
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                <ArrowUpDown className="w-3.5 h-3.5" />
                {option.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Provider Grid */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {Array.from({ length: 6 }).map((_, i) => (
            <div
              key={i}
              className="h-80 rounded-lg border border-border bg-card animate-pulse"
            />
          ))}
        </div>
      ) : filteredProviders.length > 0 ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {filteredProviders.map((provider) => (
            <ProviderCard
              key={provider.id}
              provider={provider}
              onConfigure={handleConfigure}
              onConnect={handleConnect}
              onViewDetails={handleViewDetails}
            />
          ))}
        </div>
      ) : (
        <div className="text-center py-12">
          <p className="text-muted-foreground">No providers found matching your criteria</p>
        </div>
      )}

      {/* Provider Detail Modal */}
      <ProviderDetail
        provider={selectedProvider}
        isOpen={isDetailOpen}
        onClose={() => setIsDetailOpen(false)}
        onConfigure={handleConfigure}
        onConnect={handleConnect}
      />
    </div>
  )
}
