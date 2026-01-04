package registry

import (
	"fmt"
	"sync"

	"github.com/jedarden/tunnel/internal/providers"
	"github.com/jedarden/tunnel/internal/providers/bastion"
	"github.com/jedarden/tunnel/internal/providers/bore"
	"github.com/jedarden/tunnel/internal/providers/cloudflare"
	"github.com/jedarden/tunnel/internal/providers/ngrok"
	"github.com/jedarden/tunnel/internal/providers/reversessh"
	"github.com/jedarden/tunnel/internal/providers/sshforward"
	"github.com/jedarden/tunnel/internal/providers/tailscale"
	"github.com/jedarden/tunnel/internal/providers/vscodetunnel"
	"github.com/jedarden/tunnel/internal/providers/wireguard"
	"github.com/jedarden/tunnel/internal/providers/zerotier"
)

// Registry manages all available providers
type Registry struct {
	mu        sync.RWMutex
	providers map[string]providers.Provider
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	r := &Registry{
		providers: make(map[string]providers.Provider),
	}
	r.registerDefaultProviders()
	return r
}

// registerDefaultProviders registers all built-in providers
func (r *Registry) registerDefaultProviders() {
	// VPN providers
	r.Register(tailscale.New())
	r.Register(wireguard.New())
	r.Register(zerotier.New())

	// Tunnel providers
	r.Register(cloudflare.New())
	r.Register(ngrok.New())
	r.Register(bore.New())

	// SSH providers
	r.Register(vscodetunnel.New())
	r.Register(sshforward.New())
	r.Register(reversessh.New())
	r.Register(bastion.New())
}

// Register adds a provider to the registry
func (r *Registry) Register(provider providers.Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.Name()] = provider
}

// Unregister removes a provider from the registry
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, name)
}

// GetProvider retrieves a provider by name
func (r *Registry) GetProvider(name string) (providers.Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("%w: %s", providers.ErrProviderNotFound, name)
	}

	return provider, nil
}

// ListProviders returns all registered providers
func (r *Registry) ListProviders() []providers.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providerList := make([]providers.Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		providerList = append(providerList, provider)
	}

	return providerList
}

// ListByCategory returns all providers in a specific category
func (r *Registry) ListByCategory(category providers.Category) []providers.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providerList := make([]providers.Provider, 0)
	for _, provider := range r.providers {
		if provider.Category() == category {
			providerList = append(providerList, provider)
		}
	}

	return providerList
}

// GetInstalledProviders returns all providers that are currently installed
func (r *Registry) GetInstalledProviders() []providers.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	installed := make([]providers.Provider, 0)
	for _, provider := range r.providers {
		if provider.IsInstalled() {
			installed = append(installed, provider)
		}
	}

	return installed
}

// GetConnectedProviders returns all providers that are currently connected
func (r *Registry) GetConnectedProviders() []providers.Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connected := make([]providers.Provider, 0)
	for _, provider := range r.providers {
		if provider.IsConnected() {
			connected = append(connected, provider)
		}
	}

	return connected
}

// ProviderInfo contains summary information about a provider
type ProviderInfo struct {
	Name      string             `json:"name"`
	Category  providers.Category `json:"category"`
	Installed bool               `json:"installed"`
	Connected bool               `json:"connected"`
}

// GetProviderInfo returns summary information for all providers
func (r *Registry) GetProviderInfo() []ProviderInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info := make([]ProviderInfo, 0, len(r.providers))
	for _, provider := range r.providers {
		info = append(info, ProviderInfo{
			Name:      provider.Name(),
			Category:  provider.Category(),
			Installed: provider.IsInstalled(),
			Connected: provider.IsConnected(),
		})
	}

	return info
}

// Default global registry instance
var defaultRegistry = NewRegistry()

// GetProvider retrieves a provider from the default registry
func GetProvider(name string) (providers.Provider, error) {
	return defaultRegistry.GetProvider(name)
}

// ListProviders returns all providers from the default registry
func ListProviders() []providers.Provider {
	return defaultRegistry.ListProviders()
}

// ListByCategory returns providers by category from the default registry
func ListByCategory(category providers.Category) []providers.Provider {
	return defaultRegistry.ListByCategory(category)
}

// GetInstalledProviders returns installed providers from the default registry
func GetInstalledProviders() []providers.Provider {
	return defaultRegistry.GetInstalledProviders()
}

// GetConnectedProviders returns connected providers from the default registry
func GetConnectedProviders() []providers.Provider {
	return defaultRegistry.GetConnectedProviders()
}

// GetProviderInfo returns provider info from the default registry
func GetProviderInfo() []ProviderInfo {
	return defaultRegistry.GetProviderInfo()
}
