package fido2

import (
	"testing"
)

func TestDefaultProviderConfig(t *testing.T) {
	config := DefaultProviderConfig()

	if config.RPID != "localhost" {
		t.Errorf("expected RPID 'localhost', got '%s'", config.RPID)
	}

	if config.RPOrigin != "http://localhost:8080" {
		t.Errorf("expected RPOrigin 'http://localhost:8080', got '%s'", config.RPOrigin)
	}

	if config.RPDisplayName != "TUNNEL" {
		t.Errorf("expected RPDisplayName 'TUNNEL', got '%s'", config.RPDisplayName)
	}

	if config.Timeout != 60000 {
		t.Errorf("expected Timeout 60000, got %d", config.Timeout)
	}
}

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  *ProviderConfig
		wantErr bool
	}{
		{
			name:   "default config",
			config: DefaultProviderConfig(),
			wantErr: false,
		},
		{
			name:   "nil config uses defaults",
			config: nil,
			wantErr: false,
		},
		{
			name: "custom config",
			config: &ProviderConfig{
				RPID:          "example.com",
				RPOrigin:      "https://example.com",
				RPDisplayName: "Example App",
				Timeout:       30000,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if provider == nil {
					t.Error("expected provider to be non-nil")
				}

				if provider.config == nil {
					t.Error("expected provider config to be non-nil")
				}
			}
		})
	}
}

func TestNewUser(t *testing.T) {
	accountName := "testuser"
	user := NewUser(accountName)

	if user == nil {
		t.Fatal("expected user to be non-nil")
	}

	if string(user.WebAuthnID()) != accountName {
		t.Errorf("expected ID '%s', got '%s'", accountName, string(user.WebAuthnID()))
	}

	if user.WebAuthnName() != accountName {
		t.Errorf("expected Username '%s', got '%s'", accountName, user.WebAuthnName())
	}

	if user.WebAuthnDisplayName() != accountName {
		t.Errorf("expected DisplayName '%s', got '%s'", accountName, user.WebAuthnDisplayName())
	}

	if len(user.WebAuthnCredentials()) != 0 {
		t.Errorf("expected no credentials, got %d", len(user.WebAuthnCredentials()))
	}

	if user.WebAuthnIcon() != "" {
		t.Errorf("expected empty icon, got '%s'", user.WebAuthnIcon())
	}
}

func TestProviderGetConfig(t *testing.T) {
	config := &ProviderConfig{
		RPID:          "test.com",
		RPOrigin:      "https://test.com",
		RPDisplayName: "Test",
		Timeout:       120000,
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	retrievedConfig := provider.GetConfig()
	if retrievedConfig == nil {
		t.Fatal("expected config to be non-nil")
	}

	if retrievedConfig.RPID != config.RPID {
		t.Errorf("expected RPID '%s', got '%s'", config.RPID, retrievedConfig.RPID)
	}

	if retrievedConfig.RPOrigin != config.RPOrigin {
		t.Errorf("expected RPOrigin '%s', got '%s'", config.RPOrigin, retrievedConfig.RPOrigin)
	}

	if retrievedConfig.RPDisplayName != config.RPDisplayName {
		t.Errorf("expected RPDisplayName '%s', got '%s'", config.RPDisplayName, retrievedConfig.RPDisplayName)
	}

	if retrievedConfig.Timeout != config.Timeout {
		t.Errorf("expected Timeout %d, got %d", config.Timeout, retrievedConfig.Timeout)
	}
}

func TestValidateUserVerification(t *testing.T) {
	provider, err := NewProvider(DefaultProviderConfig())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Should return true with default config (timeout > 0)
	if !provider.ValidateUserVerification() {
		t.Error("expected ValidateUserVerification to return true")
	}
}

func TestBeginRegistrationWithNoCredentials(t *testing.T) {
	provider, err := NewProvider(DefaultProviderConfig())
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	user := NewUser("testuser")

	_, err = provider.BeginAuthentication(user)
	if err == nil {
		t.Error("expected error when beginning authentication with no credentials")
	}

	if err != nil && err.Error() != "user has no registered credentials" {
		t.Errorf("expected 'user has no registered credentials' error, got '%v'", err)
	}
}
