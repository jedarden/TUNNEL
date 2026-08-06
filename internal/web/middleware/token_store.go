package middleware

import (
	"fmt"

	"github.com/jedarden/tunnel/internal/core"
)

// CredentialTokenStore implements TokenStore using the core credential store
type CredentialTokenStore struct {
	store core.CredentialStore
}

// NewCredentialTokenStore creates a new token store backed by the credential store
func NewCredentialTokenStore(store core.CredentialStore) *CredentialTokenStore {
	return &CredentialTokenStore{
		store: store,
	}
}

// GetToken retrieves the stored bearer token
func (c *CredentialTokenStore) GetToken() (string, error) {
	data, err := c.store.Get(AuthService, AuthKey)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}
	return string(data), nil
}

// SetToken stores the bearer token
func (c *CredentialTokenStore) SetToken(token string) error {
	return c.store.Set(AuthService, AuthKey, []byte(token))
}

// GetOrCreateToken retrieves an existing token or generates a new one if none exists
func (c *CredentialTokenStore) GetOrCreateToken() (string, error) {
	token, err := c.GetToken()
	if err == nil {
		return token, nil
	}

	// Token doesn't exist, generate a new one
	newToken, err := GenerateRandomToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate new token: %w", err)
	}

	if err := c.SetToken(newToken); err != nil {
		return "", fmt.Errorf("failed to store new token: %w", err)
	}

	return newToken, nil
}
