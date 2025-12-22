package ngrok

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

func TestNew(t *testing.T) {
	provider := New()
	if provider == nil {
		t.Fatal("New() returned nil")
	}
	if provider.BaseProvider == nil {
		t.Fatal("BaseProvider is nil")
	}
	if provider.apiURL == "" {
		t.Error("apiURL is empty")
	}
	expectedAPIURL := "http://localhost:4040/api"
	if provider.apiURL != expectedAPIURL {
		t.Errorf("apiURL = %q, want %q", provider.apiURL, expectedAPIURL)
	}
}

func TestName(t *testing.T) {
	provider := New()
	expected := "ngrok"
	if got := provider.Name(); got != expected {
		t.Errorf("Name() = %q, want %q", got, expected)
	}
}

func TestCategory(t *testing.T) {
	provider := New()
	expected := providers.CategoryTunnel
	if got := provider.Category(); got != expected {
		t.Errorf("Category() = %q, want %q", got, expected)
	}
}

func TestValidateConfig(t *testing.T) {
	provider := New()

	tests := []struct {
		name    string
		config  *providers.ProviderConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
			errMsg:  "invalid configuration",
		},
		{
			name: "missing name",
			config: &providers.ProviderConfig{
				LocalPort: 22,
			},
			wantErr: true,
			errMsg:  "provider name is required",
		},
		{
			name: "valid config with name only",
			config: &providers.ProviderConfig{
				Name: "ngrok",
			},
			wantErr: false,
		},
		{
			name: "valid config with auth token",
			config: &providers.ProviderConfig{
				Name:      "ngrok",
				AuthToken: "test-token-123",
			},
			wantErr: false,
		},
		{
			name: "valid config with local port",
			config: &providers.ProviderConfig{
				Name:      "ngrok",
				LocalPort: 8080,
				AuthToken: "test-token",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("ValidateConfig() error message = %q, want %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestGetConnectionInfo_Disconnected(t *testing.T) {
	provider := New()

	// Configure the provider
	config := &providers.ProviderConfig{
		Name:      "ngrok",
		LocalPort: 22,
	}
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Get connection info
	// This will fail if ngrok is not installed, which is expected
	info, err := provider.GetConnectionInfo()
	if err != nil {
		// If ngrok is not installed, we should get ErrNotInstalled
		if err != providers.ErrNotInstalled {
			t.Logf("GetConnectionInfo() error = %v (expected if ngrok not installed)", err)
		}
		return
	}

	if info == nil {
		t.Fatal("GetConnectionInfo() returned nil without error")
	}

	// Status should be disconnected (assuming ngrok is not running in test environment)
	expectedStatus := "disconnected"
	if info.Status != expectedStatus && info.Status != "connected" {
		t.Errorf("GetConnectionInfo() status = %q, want %q or 'connected'", info.Status, expectedStatus)
	}

	// Extra map should exist
	if info.Extra == nil {
		t.Error("GetConnectionInfo() Extra map is nil")
	}
}

func TestGetTunnels_ErrorHandling(t *testing.T) {
	provider := New()

	tests := []struct {
		name       string
		apiURL     string
		wantErr    bool
		setupMock  func() *httptest.Server
		closeMock  bool
	}{
		{
			name:    "connection refused",
			apiURL:  "http://localhost:9999",
			wantErr: true,
		},
		{
			name:   "invalid json response",
			wantErr: true,
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("invalid json"))
				}))
			},
			closeMock: true,
		},
		{
			name:   "valid empty response",
			wantErr: false,
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					resp := NgrokAPIResponse{
						Tunnels: []NgrokTunnel{},
					}
					_ = json.NewEncoder(w).Encode(resp)
				}))
			},
			closeMock: true,
		},
		{
			name:   "valid response with tunnel",
			wantErr: false,
			setupMock: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					resp := NgrokAPIResponse{
						Tunnels: []NgrokTunnel{
							{
								Name:      "command_line",
								PublicURL: "tcp://0.tcp.ngrok.io:12345",
								Proto:     "tcp",
							},
						},
					}
					_ = json.NewEncoder(w).Encode(resp)
				}))
			},
			closeMock: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.setupMock != nil {
				server = tt.setupMock()
				if tt.closeMock {
					defer server.Close()
				}
				provider.apiURL = server.URL
			} else {
				provider.apiURL = tt.apiURL
			}

			tunnels, err := provider.getTunnels()
			if (err != nil) != tt.wantErr {
				t.Errorf("getTunnels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tunnels == nil {
				t.Error("getTunnels() returned nil tunnels without error")
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	provider := New()

	// Configure the provider
	config := &providers.ProviderConfig{
		Name:      "ngrok",
		LocalPort: 22,
	}
	if err := provider.Configure(config); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	health, err := provider.HealthCheck()
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}

	if health == nil {
		t.Fatal("HealthCheck() returned nil")
	}

	// Verify LastCheck is recent
	if time.Since(health.LastCheck) > time.Second {
		t.Errorf("HealthCheck() LastCheck is too old: %v", health.LastCheck)
	}

	// Status should be one of the expected values
	validStatuses := map[string]bool{
		"not_installed": true,
		"disconnected":  true,
		"connected":     true,
	}
	if !validStatuses[health.Status] {
		t.Errorf("HealthCheck() status = %q, want one of: not_installed, disconnected, connected", health.Status)
	}

	// Message should not be empty
	if health.Message == "" {
		t.Error("HealthCheck() message is empty")
	}

	// If not installed, healthy should be false
	if health.Status == "not_installed" && health.Healthy {
		t.Error("HealthCheck() healthy = true when status is not_installed")
	}

	// If disconnected, healthy should be false
	if health.Status == "disconnected" && health.Healthy {
		t.Error("HealthCheck() healthy = true when status is disconnected")
	}
}

func TestGetLogs(t *testing.T) {
	provider := New()

	// GetLogs should not return an error even if logs are empty
	logs, err := provider.GetLogs(time.Now().Add(-1 * time.Hour))
	if err != nil {
		t.Fatalf("GetLogs() error = %v", err)
	}

	if logs == nil {
		t.Fatal("GetLogs() returned nil")
	}

	// For now, logs are expected to be empty (simplified implementation)
	if len(logs) != 0 {
		t.Logf("GetLogs() returned %d logs (expected 0 for simplified implementation)", len(logs))
	}
}

func TestConfigure(t *testing.T) {
	provider := New()

	tests := []struct {
		name    string
		config  *providers.ProviderConfig
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "valid config",
			config: &providers.ProviderConfig{
				Name:      "ngrok",
				LocalPort: 8080,
				AuthToken: "test-token",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.Configure(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Configure() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify config was set
				config, err := provider.GetConfig()
				if err != nil {
					t.Errorf("GetConfig() error = %v", err)
				}
				if config == nil {
					t.Error("GetConfig() returned nil after Configure()")
				}
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	provider := New()

	// GetConfig should return a default config (name only)
	config, err := provider.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if config == nil {
		t.Fatal("GetConfig() returned nil")
	}

	if config.Name != "ngrok" {
		t.Errorf("GetConfig() name = %q, want %q", config.Name, "ngrok")
	}
}

func TestNgrokTunnel_Marshal(t *testing.T) {
	// Test that NgrokTunnel can be marshaled/unmarshaled
	tunnel := NgrokTunnel{
		Name:      "test",
		PublicURL: "tcp://0.tcp.ngrok.io:12345",
		Proto:     "tcp",
	}
	tunnel.Config.Addr = "localhost:22"

	data, err := json.Marshal(tunnel)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded NgrokTunnel
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Name != tunnel.Name {
		t.Errorf("decoded Name = %q, want %q", decoded.Name, tunnel.Name)
	}
	if decoded.PublicURL != tunnel.PublicURL {
		t.Errorf("decoded PublicURL = %q, want %q", decoded.PublicURL, tunnel.PublicURL)
	}
	if decoded.Proto != tunnel.Proto {
		t.Errorf("decoded Proto = %q, want %q", decoded.Proto, tunnel.Proto)
	}
}
