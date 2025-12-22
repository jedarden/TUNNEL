package registry

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// instanceCounter is used to generate unique instance IDs
var instanceCounter uint64

// generateInstanceID creates a unique instance ID
func generateInstanceID(providerName string) string {
	count := atomic.AddUint64(&instanceCounter, 1)
	return fmt.Sprintf("%s-%d-%d", providerName, time.Now().Unix(), count)
}

// ProviderInstance represents a single instance of a provider
type ProviderInstance struct {
	mu           sync.RWMutex
	ID           string                    `json:"id"`
	ProviderName string                    `json:"provider_name"`
	DisplayName  string                    `json:"display_name"`
	Config       *providers.ProviderConfig `json:"config"`
	Provider     providers.Provider        `json:"-"`
	CreatedAt    time.Time                 `json:"created_at"`
	ConnectedAt  *time.Time                `json:"connected_at,omitempty"`
	Status       string                    `json:"status"` // "disconnected", "connecting", "connected", "error"
	LastError    string                    `json:"last_error,omitempty"`
}

// NewProviderInstance creates a new provider instance
func NewProviderInstance(provider providers.Provider, displayName string, config *providers.ProviderConfig) *ProviderInstance {
	instance := &ProviderInstance{
		ID:           generateInstanceID(provider.Name()),
		ProviderName: provider.Name(),
		DisplayName:  displayName,
		Config:       config,
		Provider:     provider,
		CreatedAt:    time.Now(),
		Status:       "disconnected",
	}

	if displayName == "" {
		instance.DisplayName = instance.ID
	}

	return instance
}

// Connect attempts to connect this instance
func (pi *ProviderInstance) Connect() error {
	pi.mu.Lock()
	pi.Status = "connecting"
	pi.LastError = ""
	pi.mu.Unlock()

	// Configure the provider with instance-specific config
	if pi.Config != nil {
		if err := pi.Provider.Configure(pi.Config); err != nil {
			pi.mu.Lock()
			pi.Status = "error"
			pi.LastError = err.Error()
			pi.mu.Unlock()
			return fmt.Errorf("configuration failed: %w", err)
		}
	}

	// Connect
	if err := pi.Provider.Connect(); err != nil {
		pi.mu.Lock()
		pi.Status = "error"
		pi.LastError = err.Error()
		pi.mu.Unlock()
		return fmt.Errorf("connection failed: %w", err)
	}

	pi.mu.Lock()
	pi.Status = "connected"
	now := time.Now()
	pi.ConnectedAt = &now
	pi.mu.Unlock()

	return nil
}

// Disconnect disconnects this instance
func (pi *ProviderInstance) Disconnect() error {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	if err := pi.Provider.Disconnect(); err != nil {
		pi.LastError = err.Error()
		return err
	}

	pi.Status = "disconnected"
	pi.ConnectedAt = nil
	return nil
}

// IsConnected returns whether this instance is connected
func (pi *ProviderInstance) IsConnected() bool {
	pi.mu.RLock()
	defer pi.mu.RUnlock()
	return pi.Status == "connected" && pi.Provider.IsConnected()
}

// GetStatus returns the current status
func (pi *ProviderInstance) GetStatus() string {
	pi.mu.RLock()
	defer pi.mu.RUnlock()
	return pi.Status
}

// GetConnectionInfo returns connection info for this instance
func (pi *ProviderInstance) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	return pi.Provider.GetConnectionInfo()
}

// InstanceManager manages multiple instances of providers
type InstanceManager struct {
	mu        sync.RWMutex
	instances map[string]*ProviderInstance // keyed by instance ID
	registry  *Registry
}

// NewInstanceManager creates a new instance manager
func NewInstanceManager(registry *Registry) *InstanceManager {
	return &InstanceManager{
		instances: make(map[string]*ProviderInstance),
		registry:  registry,
	}
}

// CreateInstance creates a new provider instance
func (im *InstanceManager) CreateInstance(providerName, displayName string, config *providers.ProviderConfig) (*ProviderInstance, error) {
	// Get the provider template from registry
	provider, err := im.registry.GetProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}

	// Check if provider is installed
	if !provider.IsInstalled() {
		return nil, fmt.Errorf("provider %s is not installed", providerName)
	}

	// Create a new instance
	instance := NewProviderInstance(provider, displayName, config)

	im.mu.Lock()
	im.instances[instance.ID] = instance
	im.mu.Unlock()

	return instance, nil
}

// GetInstance retrieves an instance by ID
func (im *InstanceManager) GetInstance(instanceID string) (*ProviderInstance, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	instance, exists := im.instances[instanceID]
	if !exists {
		return nil, fmt.Errorf("instance not found: %s", instanceID)
	}

	return instance, nil
}

// ListInstances returns all instances
func (im *InstanceManager) ListInstances() []*ProviderInstance {
	im.mu.RLock()
	defer im.mu.RUnlock()

	instances := make([]*ProviderInstance, 0, len(im.instances))
	for _, instance := range im.instances {
		instances = append(instances, instance)
	}

	return instances
}

// ListInstancesByProvider returns all instances of a specific provider type
func (im *InstanceManager) ListInstancesByProvider(providerName string) []*ProviderInstance {
	im.mu.RLock()
	defer im.mu.RUnlock()

	instances := make([]*ProviderInstance, 0)
	for _, instance := range im.instances {
		if instance.ProviderName == providerName {
			instances = append(instances, instance)
		}
	}

	return instances
}

// GetConnectedInstances returns all connected instances
func (im *InstanceManager) GetConnectedInstances() []*ProviderInstance {
	im.mu.RLock()
	defer im.mu.RUnlock()

	connected := make([]*ProviderInstance, 0)
	for _, instance := range im.instances {
		if instance.IsConnected() {
			connected = append(connected, instance)
		}
	}

	return connected
}

// ConnectInstance connects a specific instance
func (im *InstanceManager) ConnectInstance(instanceID string) error {
	instance, err := im.GetInstance(instanceID)
	if err != nil {
		return err
	}

	return instance.Connect()
}

// DisconnectInstance disconnects a specific instance
func (im *InstanceManager) DisconnectInstance(instanceID string) error {
	instance, err := im.GetInstance(instanceID)
	if err != nil {
		return err
	}

	return instance.Disconnect()
}

// DeleteInstance removes an instance (disconnects first if connected)
func (im *InstanceManager) DeleteInstance(instanceID string) error {
	instance, err := im.GetInstance(instanceID)
	if err != nil {
		return err
	}

	// Disconnect if connected
	if instance.IsConnected() {
		if err := instance.Disconnect(); err != nil {
			// Log but continue with deletion
			instance.mu.Lock()
			instance.LastError = fmt.Sprintf("disconnect error during delete: %v", err)
			instance.mu.Unlock()
		}
	}

	im.mu.Lock()
	delete(im.instances, instanceID)
	im.mu.Unlock()

	return nil
}

// ConnectAll connects all instances concurrently
func (im *InstanceManager) ConnectAll() map[string]error {
	im.mu.RLock()
	instances := make([]*ProviderInstance, 0, len(im.instances))
	for _, instance := range im.instances {
		instances = append(instances, instance)
	}
	im.mu.RUnlock()

	var wg sync.WaitGroup
	errors := make(map[string]error)
	var errorsMu sync.Mutex

	for _, instance := range instances {
		wg.Add(1)
		go func(inst *ProviderInstance) {
			defer wg.Done()
			if err := inst.Connect(); err != nil {
				errorsMu.Lock()
				errors[inst.ID] = err
				errorsMu.Unlock()
			}
		}(instance)
	}

	wg.Wait()
	return errors
}

// DisconnectAll disconnects all instances concurrently
func (im *InstanceManager) DisconnectAll() map[string]error {
	im.mu.RLock()
	instances := make([]*ProviderInstance, 0, len(im.instances))
	for _, instance := range im.instances {
		if instance.IsConnected() {
			instances = append(instances, instance)
		}
	}
	im.mu.RUnlock()

	var wg sync.WaitGroup
	errors := make(map[string]error)
	var errorsMu sync.Mutex

	for _, instance := range instances {
		wg.Add(1)
		go func(inst *ProviderInstance) {
			defer wg.Done()
			if err := inst.Disconnect(); err != nil {
				errorsMu.Lock()
				errors[inst.ID] = err
				errorsMu.Unlock()
			}
		}(instance)
	}

	wg.Wait()
	return errors
}

// ConnectMultiple connects multiple instances concurrently by ID
func (im *InstanceManager) ConnectMultiple(instanceIDs []string) map[string]error {
	var wg sync.WaitGroup
	errors := make(map[string]error)
	var errorsMu sync.Mutex

	for _, id := range instanceIDs {
		wg.Add(1)
		go func(instanceID string) {
			defer wg.Done()
			if err := im.ConnectInstance(instanceID); err != nil {
				errorsMu.Lock()
				errors[instanceID] = err
				errorsMu.Unlock()
			}
		}(id)
	}

	wg.Wait()
	return errors
}

// InstanceCount returns the total number of instances
func (im *InstanceManager) InstanceCount() int {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return len(im.instances)
}

// ConnectedCount returns the number of connected instances
func (im *InstanceManager) ConnectedCount() int {
	im.mu.RLock()
	defer im.mu.RUnlock()

	count := 0
	for _, instance := range im.instances {
		if instance.IsConnected() {
			count++
		}
	}
	return count
}

// InstanceInfo contains summary information about an instance
type InstanceInfo struct {
	ID           string     `json:"id"`
	ProviderName string     `json:"provider_name"`
	DisplayName  string     `json:"display_name"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	ConnectedAt  *time.Time `json:"connected_at,omitempty"`
	LastError    string     `json:"last_error,omitempty"`
}

// GetInstanceInfo returns summary information for all instances
func (im *InstanceManager) GetInstanceInfo() []InstanceInfo {
	im.mu.RLock()
	defer im.mu.RUnlock()

	info := make([]InstanceInfo, 0, len(im.instances))
	for _, instance := range im.instances {
		instance.mu.RLock()
		info = append(info, InstanceInfo{
			ID:           instance.ID,
			ProviderName: instance.ProviderName,
			DisplayName:  instance.DisplayName,
			Status:       instance.Status,
			CreatedAt:    instance.CreatedAt,
			ConnectedAt:  instance.ConnectedAt,
			LastError:    instance.LastError,
		})
		instance.mu.RUnlock()
	}

	return info
}
