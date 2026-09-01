package totp

import (
	"fmt"
	"time"

	"github.com/jedarden/tunnel/internal/core"
)

const (
	totpService = "totp-auth"
	totpKey     = "secret"
)

// Manager handles TOTP operations with credential storage
type Manager struct {
	provider *Provider
	store    core.CredentialStore
}

// NewManager creates a new TOTP manager
func NewManager(store core.CredentialStore, issuer string) *Manager {
	return &Manager{
		provider: NewProvider(issuer),
		store:    store,
	}
}

// EnrollUser creates a new TOTP enrollment for a user
func (m *Manager) EnrollUser(accountName string) (*EnrollmentBundle, error) {
	// Generate enrollment bundle
	bundle, err := m.provider.GenerateEnrollmentBundle(accountName)
	if err != nil {
		return nil, fmt.Errorf("generate enrollment bundle: %w", err)
	}

	// Store the secret
	if err := m.store.Set(totpService, accountName, []byte(bundle.Secret)); err != nil {
		return nil, fmt.Errorf("store totp secret: %w", err)
	}

	return bundle, nil
}

// ValidateCode validates a TOTP code for a user
func (m *Manager) ValidateCode(accountName, code string, window int) (bool, error) {
	// Retrieve the secret
	secretBytes, err := m.store.Get(totpService, accountName)
	if err != nil {
		if err == core.ErrCredentialNotFound {
			return false, fmt.Errorf("user not enrolled")
		}
		return false, fmt.Errorf("retrieve totp secret: %w", err)
	}

	secret := string(secretBytes)

	// Validate the code
	valid, err := m.provider.ValidateCodeWithWindow(secret, code, window)
	if err != nil {
		return false, fmt.Errorf("validate totp code: %w", err)
	}

	return valid, nil
}

// GetSecret retrieves the TOTP secret for a user (for admin/debug purposes)
func (m *Manager) GetSecret(accountName string) (string, error) {
	secretBytes, err := m.store.Get(totpService, accountName)
	if err != nil {
		if err == core.ErrCredentialNotFound {
			return "", fmt.Errorf("user not enrolled")
		}
		return "", fmt.Errorf("retrieve totp secret: %w", err)
	}

	return string(secretBytes), nil
}

// DeleteUser removes a user's TOTP enrollment
func (m *Manager) DeleteUser(accountName string) error {
	if err := m.store.Delete(totpService, accountName); err != nil {
		return fmt.Errorf("delete totp secret: %w", err)
	}
	return nil
}

// IsEnrolled checks if a user has TOTP enabled
func (m *Manager) IsEnrolled(accountName string) (bool, error) {
	_, err := m.store.Get(totpService, accountName)
	if err != nil {
		if err == core.ErrCredentialNotFound {
			return false, nil
		}
		return false, fmt.Errorf("check totp enrollment: %w", err)
	}
	return true, nil
}

// ListEnrolledUsers returns all account names with TOTP enabled
func (m *Manager) ListEnrolledUsers() ([]string, error) {
	keys, err := m.store.List(totpService)
	if err != nil {
		return nil, fmt.Errorf("list enrolled users: %w", err)
	}
	return keys, nil
}

// RegenerateSecret creates a new secret for an existing user
// This is useful when a user loses access to their authenticator
func (m *Manager) RegenerateSecret(accountName string) (*EnrollmentBundle, error) {
	// Check if user exists
	enrolled, err := m.IsEnrolled(accountName)
	if err != nil {
		return nil, err
	}

	if !enrolled {
		return nil, fmt.Errorf("user not enrolled")
	}

	// Delete old secret
	if err := m.DeleteUser(accountName); err != nil {
		return nil, fmt.Errorf("delete old secret: %w", err)
	}

	// Create new enrollment
	return m.EnrollUser(accountName)
}

// ValidateWithTimeCheck validates a TOTP code with time synchronization check
func (m *Manager) ValidateWithTimeCheck(accountName, code string, window int) (bool, error) {
	// Check time synchronization
	now := time.Now()
	if !m.provider.TimeSyncCheck(now) {
		return false, fmt.Errorf("system time may be out of sync")
	}

	return m.ValidateCode(accountName, code, window)
}

// GenerateCurrentCode generates the current valid TOTP code for a user
// Useful for testing or manual entry scenarios
func (m *Manager) GenerateCurrentCode(accountName string) (string, error) {
	secretBytes, err := m.store.Get(totpService, accountName)
	if err != nil {
		if err == core.ErrCredentialNotFound {
			return "", fmt.Errorf("user not enrolled")
		}
		return "", fmt.Errorf("retrieve totp secret: %w", err)
	}

	return m.provider.GenerateCode(string(secretBytes))
}

// GetTimeRemaining returns the seconds remaining in the current TOTP window
func (m *Manager) GetTimeRemaining(accountName string) (int, error) {
	secretBytes, err := m.store.Get(totpService, accountName)
	if err != nil {
		if err == core.ErrCredentialNotFound {
			return 0, fmt.Errorf("user not enrolled")
		}
		return 0, fmt.Errorf("retrieve totp secret: %w", err)
	}

	window, err := m.provider.GetCurrentWindow(string(secretBytes))
	if err != nil {
		return 0, fmt.Errorf("get current window: %w", err)
	}

	// Calculate remaining time
	// Window changes every 30 seconds
	now := time.Now().Unix()
	windowStart := window * 30
	remaining := 30 - int(now-windowStart)

	if remaining < 0 {
		remaining = 0
	} else if remaining > 30 {
		remaining = 30
	}

	return remaining, nil
}
