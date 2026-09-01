package oauth

import (
	"testing"
	"time"

	"github.com/jedarden/tunnel/internal/core"
	"github.com/jedarden/tunnel/internal/providers"
)

func TestNew(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	if provider == nil {
		t.Fatal("New() returned nil")
	}

	if provider.Name() != "oauth" {
		t.Errorf("Expected name 'oauth', got '%s'", provider.Name())
	}

	if provider.Category() != providers.CategoryDirect {
		t.Errorf("Expected category '%s', got '%s'", providers.CategoryDirect, provider.Category())
	}
}

func TestIsInstalled(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	if !provider.IsInstalled() {
		t.Error("OAuth should always be installed")
	}
}

func TestConfigure(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	tests := []struct {
		name    string
		config  *providers.ProviderConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &providers.ProviderConfig{
				Name: "oauth",
				Extra: map[string]string{
					"provider":     "github",
					"client_id":    "test-client-id",
					"redirect_uri": "http://localhost:8080/callback",
				},
			},
			wantErr: false,
		},
		{
			name: "missing client_id",
			config: &providers.ProviderConfig{
				Name: "oauth",
				Extra: map[string]string{
					"provider":     "github",
					"redirect_uri": "http://localhost:8080/callback",
				},
			},
			wantErr: true,
		},
		{
			name: "missing redirect_uri",
			config: &providers.ProviderConfig{
				Name: "oauth",
				Extra: map[string]string{
					"provider":  "github",
					"client_id": "test-client-id",
				},
			},
			wantErr: true,
		},
		{
			name:   "nil config",
			config: nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.Configure(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Configure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	tests := []struct {
		name    string
		config  *providers.ProviderConfig
		wantErr bool
	}{
		{
			name: "valid github config",
			config: &providers.ProviderConfig{
				Name: "oauth",
				Extra: map[string]string{
					"provider":     "github",
					"client_id":    "test-id",
					"redirect_uri": "http://localhost:8080/callback",
				},
			},
			wantErr: false,
		},
		{
			name: "valid google config",
			config: &providers.ProviderConfig{
				Name: "oauth",
				Extra: map[string]string{
					"provider":     "google",
					"client_id":    "test-id",
					"redirect_uri": "https://example.com/callback",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid provider",
			config: &providers.ProviderConfig{
				Name: "oauth",
				Extra: map[string]string{
					"provider":     "invalid",
					"client_id":    "test-id",
					"redirect_uri": "http://localhost:8080/callback",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid redirect_uri",
			config: &providers.ProviderConfig{
				Name: "oauth",
				Extra: map[string]string{
					"provider":     "github",
					"client_id":    "test-id",
					"redirect_uri": "::invalid-url",
				},
			},
			wantErr: true,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseOAuthConfig(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	config := &providers.ProviderConfig{
		Name: "oauth",
		Extra: map[string]string{
			"provider":      "github",
			"client_id":     "test-client",
			"client_secret": "test-secret",
			"redirect_uri":  "http://localhost:8080/callback",
			"scopes":        "read:user user:email",
		},
	}

	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() failed: %v", err)
	}

	oauthConfig, err := provider.parseOAuthConfig(config)
	if err != nil {
		t.Fatalf("parseOAuthConfig() failed: %v", err)
	}

	if oauthConfig.Provider != "github" {
		t.Errorf("Expected provider 'github', got '%s'", oauthConfig.Provider)
	}

	if oauthConfig.ClientID != "test-client" {
		t.Errorf("Expected client_id 'test-client', got '%s'", oauthConfig.ClientID)
	}

	if oauthConfig.ClientSecret != "test-secret" {
		t.Errorf("Expected client_secret 'test-secret', got '%s'", oauthConfig.ClientSecret)
	}

	if oauthConfig.RedirectURI != "http://localhost:8080/callback" {
		t.Errorf("Expected redirect_uri 'http://localhost:8080/callback', got '%s'", oauthConfig.RedirectURI)
	}

	if oauthConfig.Scopes != "read:user user:email" {
		t.Errorf("Expected scopes 'read:user user:email', got '%s'", oauthConfig.Scopes)
	}
}

func TestIsConnected(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	// Initially not connected
	if provider.IsConnected() {
		t.Error("Should not be connected initially")
	}

	// Configure provider
	config := &providers.ProviderConfig{
		Name: "oauth",
		Extra: map[string]string{
			"provider":     "github",
			"client_id":    "test-id",
			"redirect_uri": "http://localhost:8080/callback",
		},
	}

	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() failed: %v", err)
	}

	// Still not connected (no token stored)
	if provider.IsConnected() {
		t.Error("Should not be connected without token")
	}
}

func TestGetConnectionInfo(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	config := &providers.ProviderConfig{
		Name: "oauth",
		Extra: map[string]string{
			"provider":     "github",
			"client_id":    "test-id",
			"redirect_uri": "http://localhost:8080/callback",
		},
	}

	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() failed: %v", err)
	}

	info, err := provider.GetConnectionInfo()
	if err != nil {
		t.Fatalf("GetConnectionInfo() failed: %v", err)
	}

	if info.Status != "disconnected" {
		t.Errorf("Expected status 'disconnected', got '%s'", info.Status)
	}

	if info.Extra == nil {
		t.Fatal("Expected Extra map, got nil")
	}

	if providerVal, ok := info.Extra["provider"]; !ok || providerVal != "github" {
		t.Errorf("Expected provider 'github', got %v", providerVal)
	}

	if authenticated, ok := info.Extra["authenticated"]; !ok || authenticated != false {
		t.Errorf("Expected authenticated false, got %v", authenticated)
	}
}

func TestHealthCheck(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	// Test health check without configuration
	status, err := provider.HealthCheck()
	if err != nil {
		t.Fatalf("HealthCheck() failed: %v", err)
	}

	if status.Healthy {
		t.Error("Expected unhealthy status without config")
	}

	// Configure provider
	config := &providers.ProviderConfig{
		Name: "oauth",
		Extra: map[string]string{
			"provider":     "github",
			"client_id":    "test-id",
			"redirect_uri": "http://localhost:8080/callback",
		},
	}

	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() failed: %v", err)
	}

	// Test health check with configuration but no token
	status, err = provider.HealthCheck()
	if err != nil {
		t.Fatalf("HealthCheck() failed: %v", err)
	}

	if status.Healthy {
		t.Error("Expected unhealthy status without token")
	}

	if status.Status != "not_authenticated" {
		t.Errorf("Expected status 'not_authenticated', got '%s'", status.Status)
	}
}

func TestGetLogs(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	logs, err := provider.GetLogs(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("GetLogs() failed: %v", err)
	}

	if logs == nil {
		t.Fatal("Expected logs slice, got nil")
	}

	if len(logs) != 0 {
		t.Errorf("Expected empty logs, got %d entries", len(logs))
	}
}

func TestGenerateRandomString(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	s1 := provider.generateRandomString(32)
	s2 := provider.generateRandomString(32)

	if len(s1) != 32 {
		t.Errorf("Expected length 32, got %d", len(s1))
	}

	if len(s2) != 32 {
		t.Errorf("Expected length 32, got %d", len(s2))
	}

	if s1 == s2 {
		t.Error("Random strings should be different")
	}
}

func TestDisconnect(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	// Disconnect should not error even with no token
	if err := provider.Disconnect(); err != nil {
		t.Errorf("Disconnect() failed: %v", err)
	}
}

func TestInstallUninstall(t *testing.T) {
	store := core.NewEnvStore("test")
	provider := New(store)

	// Install should be no-op
	if err := provider.Install(); err != nil {
		t.Errorf("Install() failed: %v", err)
	}

	// Uninstall should be no-op
	if err := provider.Uninstall(); err != nil {
		t.Errorf("Uninstall() failed: %v", err)
	}
}
