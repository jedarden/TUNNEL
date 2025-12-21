package providers_test

import (
	"testing"

	"github.com/jedarden/tunnel/internal/providers"
)

func TestBaseProvider(t *testing.T) {
	base := providers.NewBaseProvider("test", providers.CategoryTunnel)

	if base.Name() != "test" {
		t.Errorf("expected name 'test', got '%s'", base.Name())
	}

	if base.Category() != providers.CategoryTunnel {
		t.Errorf("expected category 'tunnel', got '%s'", base.Category())
	}
}

func TestProviderConfig(t *testing.T) {
	base := providers.NewBaseProvider("test", providers.CategoryVPN)

	config := &providers.ProviderConfig{
		Name:      "test",
		AuthToken: "token123",
	}

	if err := base.Configure(config); err != nil {
		t.Errorf("failed to configure: %v", err)
	}

	retrievedConfig, err := base.GetConfig()
	if err != nil {
		t.Errorf("failed to get config: %v", err)
	}

	if retrievedConfig.AuthToken != "token123" {
		t.Errorf("expected token 'token123', got '%s'", retrievedConfig.AuthToken)
	}
}

func TestValidateConfig(t *testing.T) {
	base := providers.NewBaseProvider("test", providers.CategoryDirect)

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
			name:    "missing name",
			config:  &providers.ProviderConfig{},
			wantErr: true,
		},
		{
			name: "valid config",
			config: &providers.ProviderConfig{
				Name: "test",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := base.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
