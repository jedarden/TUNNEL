package fido2

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jedarden/tunnel/internal/core"
)

const (
	fido2Service = "fido2-auth"
	credKey      = "credentials"
	metadataKey  = "metadata"
)

// Manager handles FIDO2 operations with credential storage
type Manager struct {
	provider *Provider
	store    core.CredentialStore
}

// StoredCredential represents a credential stored in the credential store
type StoredCredential struct {
	ID           string              `json:"id"`
	Type         string              `json:"type"`
	Attestation  string              `json:"attestation_type"`
	AAGUID       string              `json:"aaguid"`
	SignCount    uint32              `json:"sign_count"`
	PublicKey    string              `json:"public_key"`
	Transport    []string            `json:"transport,omitempty"`
	Flags        []string            `json:"flags,omitempty"`
	RegisteredAt time.Time           `json:"registered_at"`
	LastUsedAt   time.Time           `json:"last_used_at"`
	Metadata     map[string]string   `json:"metadata,omitempty"`
}

// StoredMetadata represents user metadata stored in the credential store
type StoredMetadata struct {
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	UserID      string    `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserCredentials combines user metadata with their stored credentials
type UserCredentials struct {
	Metadata    *StoredMetadata
	Credentials []StoredCredential
}

// NewManager creates a new FIDO2 manager
func NewManager(store core.CredentialStore, config *ProviderConfig) (*Manager, error) {
	provider, err := NewProvider(config)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	return &Manager{
		provider: provider,
		store:    store,
	}, nil
}

// CreateUser creates a new FIDO2 user (metadata only)
func (m *Manager) CreateUser(accountName string) (*User, error) {
	metadata := &StoredMetadata{
		Username:    accountName,
		DisplayName: accountName,
		UserID:      base64.RawURLEncoding.EncodeToString([]byte(accountName)),
		CreatedAt:   time.Now(),
	}

	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	if err := m.store.Set(fido2Service, makeMetadataKey(accountName), metadataBytes); err != nil {
		return nil, fmt.Errorf("store user metadata: %w", err)
	}

	return &User{
		ID:          []byte(accountName),
		Username:    accountName,
		DisplayName: accountName,
		Credentials: []webauthn.Credential{},
	}, nil
}

// GetUser retrieves a user from the credential store
func (m *Manager) GetUser(accountName string) (*User, error) {
	// Check if user exists
	metadataBytes, err := m.store.Get(fido2Service, makeMetadataKey(accountName))
	if err != nil {
		if err == core.ErrCredentialNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("retrieve user metadata: %w", err)
	}

	var metadata StoredMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	// Load user's credentials
	user := &User{
		ID:          []byte(metadata.UserID),
		Username:    metadata.Username,
		DisplayName: metadata.DisplayName,
		Credentials: []webauthn.Credential{},
	}

	credsBytes, err := m.store.Get(fido2Service, makeCredKey(accountName))
	if err != nil && err != core.ErrCredentialNotFound {
		return nil, fmt.Errorf("retrieve credentials: %w", err)
	}

	if credsBytes != nil {
		var storedCreds []StoredCredential
		if err := json.Unmarshal(credsBytes, &storedCreds); err != nil {
			return nil, fmt.Errorf("unmarshal credentials: %w", err)
		}

		// Convert stored credentials to webauthn.Credential
		for _, cred := range storedCreds {
			wc, err := convertToWebAuthnCredential(cred)
			if err != nil {
				return nil, fmt.Errorf("convert credential: %w", err)
			}
			user.Credentials = append(user.Credentials, *wc)
		}
	}

	return user, nil
}

// SaveCredential saves a new credential for a user
func (m *Manager) SaveCredential(accountName string, credential *StoredCredential) error {
	// Load existing credentials
	var creds []StoredCredential
	credsBytes, err := m.store.Get(fido2Service, makeCredKey(accountName))
	if err == nil {
		if err := json.Unmarshal(credsBytes, &creds); err != nil {
			return fmt.Errorf("unmarshal existing credentials: %w", err)
		}
	} else if err != core.ErrCredentialNotFound {
		return fmt.Errorf("retrieve credentials: %w", err)
	}

	// Set registration time
	credential.RegisteredAt = time.Now()
	credential.LastUsedAt = time.Now()

	// Add new credential
	creds = append(creds, *credential)

	// Marshal and save
	credsBytes, err = json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	if err := m.store.Set(fido2Service, makeCredKey(accountName), credsBytes); err != nil {
		return fmt.Errorf("store credentials: %w", err)
	}

	return nil
}

// UpdateCredential updates an existing credential (e.g., sign count)
func (m *Manager) UpdateCredential(accountName, credentialID string, updateFn func(*StoredCredential)) error {
	// Load existing credentials
	credsBytes, err := m.store.Get(fido2Service, makeCredKey(accountName))
	if err != nil {
		return fmt.Errorf("retrieve credentials: %w", err)
	}

	var creds []StoredCredential
	if err := json.Unmarshal(credsBytes, &creds); err != nil {
		return fmt.Errorf("unmarshal credentials: %w", err)
	}

	// Find and update credential
	found := false
	for i, cred := range creds {
		if cred.ID == credentialID {
			updateFn(&creds[i])
			creds[i].LastUsedAt = time.Now()
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("credential not found")
	}

	// Marshal and save
	credsBytes, err = json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	if err := m.store.Set(fido2Service, makeCredKey(accountName), credsBytes); err != nil {
		return fmt.Errorf("store credentials: %w", err)
	}

	return nil
}

// DeleteCredential removes a credential from a user
func (m *Manager) DeleteCredential(accountName, credentialID string) error {
	// Load existing credentials
	credsBytes, err := m.store.Get(fido2Service, makeCredKey(accountName))
	if err != nil {
		return fmt.Errorf("retrieve credentials: %w", err)
	}

	var creds []StoredCredential
	if err := json.Unmarshal(credsBytes, &creds); err != nil {
		return fmt.Errorf("unmarshal credentials: %w", err)
	}

	// Remove credential
	newCreds := make([]StoredCredential, 0, len(creds)-1)
	found := false
	for _, cred := range creds {
		if cred.ID != credentialID {
			newCreds = append(newCreds, cred)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("credential not found")
	}

	// If no credentials left, delete the key
	if len(newCreds) == 0 {
		if err := m.store.Delete(fido2Service, makeCredKey(accountName)); err != nil {
			return fmt.Errorf("delete credentials: %w", err)
		}
		return nil
	}

	// Marshal and save remaining
	credsBytes, err = json.Marshal(newCreds)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	if err := m.store.Set(fido2Service, makeCredKey(accountName), credsBytes); err != nil {
		return fmt.Errorf("store credentials: %w", err)
	}

	return nil
}

// DeleteUser removes a user and all their credentials
func (m *Manager) DeleteUser(accountName string) error {
	// Delete metadata
	if err := m.store.Delete(fido2Service, makeMetadataKey(accountName)); err != nil {
		return fmt.Errorf("delete metadata: %w", err)
	}

	// Delete credentials
	if err := m.store.Delete(fido2Service, makeCredKey(accountName)); err != nil {
		return fmt.Errorf("delete credentials: %w", err)
	}

	return nil
}

// UserExists checks if a user exists in the credential store
func (m *Manager) UserExists(accountName string) (bool, error) {
	_, err := m.store.Get(fido2Service, makeMetadataKey(accountName))
	if err != nil {
		if err == core.ErrCredentialNotFound {
			return false, nil
		}
		return false, fmt.Errorf("check user existence: %w", err)
	}
	return true, nil
}

// ListUsers returns all account names with FIDO2 enabled
func (m *Manager) ListUsers() ([]string, error) {
	keys, err := m.store.List(fido2Service)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	// Filter for metadata keys only
	var users []string
	for _, key := range keys {
		if isMetadataKey(key) {
			accountName := extractAccountNameFromMetadata(key)
			users = append(users, accountName)
		}
	}

	return users, nil
}

// GetUserCredentials returns all credentials for a user
func (m *Manager) GetUserCredentials(accountName string) (*UserCredentials, error) {
	// Load metadata
	metadataBytes, err := m.store.Get(fido2Service, makeMetadataKey(accountName))
	if err != nil {
		if err == core.ErrCredentialNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("retrieve metadata: %w", err)
	}

	var metadata StoredMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	// Load credentials
	var creds []StoredCredential
	credsBytes, err := m.store.Get(fido2Service, makeCredKey(accountName))
	if err == nil {
		if err := json.Unmarshal(credsBytes, &creds); err != nil {
			return nil, fmt.Errorf("unmarshal credentials: %w", err)
		}
	} else if err != core.ErrCredentialNotFound {
		return nil, fmt.Errorf("retrieve credentials: %w", err)
	}

	return &UserCredentials{
		Metadata:    &metadata,
		Credentials: creds,
	}, nil
}

// BeginRegistration starts a registration ceremony
func (m *Manager) BeginRegistration(accountName string) (*RegistrationRequest, error) {
	// Get or create user
	user, err := m.GetUser(accountName)
	if err != nil {
		// User doesn't exist, create them
		user, err = m.CreateUser(accountName)
		if err != nil {
			return nil, fmt.Errorf("create user: %w", err)
		}
	}

	req, err := m.provider.BeginRegistration(user, protocol.AuthenticatorSelection{
		UserVerification: protocol.VerificationRequired,
	})
	if err != nil {
		return nil, err
	}

	// Store session data for later verification
	sessionDataBytes, err := json.Marshal(req.SessionData)
	if err != nil {
		return nil, fmt.Errorf("marshal session data: %w", err)
	}

	sessionKey := makeSessionKey(accountName)
	if err := m.store.Set(fido2Service, sessionKey, sessionDataBytes); err != nil {
		return nil, fmt.Errorf("store session data: %w", err)
	}

	return req, nil
}

// FinishRegistration completes a registration ceremony
func (m *Manager) FinishRegistration(accountName string, response *http.Request) (*StoredCredential, error) {
	user, err := m.GetUser(accountName)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Retrieve session data
	sessionDataJSON, err := m.store.Get(fido2Service, makeSessionKey(accountName))
	if err != nil {
		return nil, fmt.Errorf("retrieve session data: %w", err)
	}

	var sessionData webauthn.SessionData
	if err := json.Unmarshal(sessionDataJSON, &sessionData); err != nil {
		return nil, fmt.Errorf("unmarshal session data: %w", err)
	}

	// Complete registration
	reg, err := m.provider.FinishRegistration(user, &sessionData, response)
	if err != nil {
		return nil, fmt.Errorf("finish registration: %w", err)
	}

	// Convert to stored credential
	storedCred := &StoredCredential{
		ID:        reg.ID,
		Type:      reg.Type,
		Attestation: reg.Attestation,
		AAGUID:    reg.AAGUID,
		PublicKey: reg.PublicKey,
		SignCount: reg.SignCount,
		Transport: reg.Transport,
		Flags:     reg.Flags,
	}

	if err := m.SaveCredential(accountName, storedCred); err != nil {
		return nil, fmt.Errorf("save credential: %w", err)
	}

	// Clean up session data
	_ = m.store.Delete(fido2Service, makeSessionKey(accountName))

	return storedCred, nil
}

// BeginAuthentication starts an authentication ceremony
func (m *Manager) BeginAuthentication(accountName string) (*AuthenticationRequest, error) {
	user, err := m.GetUser(accountName)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	req, err := m.provider.BeginAuthentication(user)
	if err != nil {
		return nil, err
	}

	// Store session data for later verification
	sessionDataBytes, err := json.Marshal(req.SessionData)
	if err != nil {
		return nil, fmt.Errorf("marshal session data: %w", err)
	}

	sessionKey := makeSessionKey(accountName)
	if err := m.store.Set(fido2Service, sessionKey, sessionDataBytes); err != nil {
		return nil, fmt.Errorf("store session data: %w", err)
	}

	return req, nil
}

// FinishAuthentication completes an authentication ceremony
func (m *Manager) FinishAuthentication(accountName string, response *http.Request) (*CredentialAssertion, error) {
	user, err := m.GetUser(accountName)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// Retrieve session data
	sessionDataJSON, err := m.store.Get(fido2Service, makeSessionKey(accountName))
	if err != nil {
		return nil, fmt.Errorf("retrieve session data: %w", err)
	}

	var sessionData webauthn.SessionData
	if err := json.Unmarshal(sessionDataJSON, &sessionData); err != nil {
		return nil, fmt.Errorf("unmarshal session data: %w", err)
	}

	assertion, err := m.provider.FinishAuthentication(user, &sessionData, response)
	if err != nil {
		return nil, fmt.Errorf("finish authentication: %w", err)
	}

	// Update sign count
	if err := m.UpdateCredential(accountName, assertion.ID, func(cred *StoredCredential) {
		cred.SignCount = assertion.SignCount
	}); err != nil {
		return nil, fmt.Errorf("update credential: %w", err)
	}

	// Clean up session data
	_ = m.store.Delete(fido2Service, makeSessionKey(accountName))

	return assertion, nil
}

// Helper functions

func makeMetadataKey(accountName string) string {
	return fmt.Sprintf("user:%s:metadata", accountName)
}

func makeCredKey(accountName string) string {
	return fmt.Sprintf("user:%s:credentials", accountName)
}

func makeSessionKey(accountName string) string {
	return fmt.Sprintf("user:%s:session", accountName)
}

func isMetadataKey(key string) bool {
	// Check if key ends with ":metadata"
	return len(key) > 9 && key[len(key)-9:] == ":metadata"
}

func extractAccountNameFromMetadata(key string) string {
	// Extract account name from "user:{accountName}:metadata"
	parts := splitKey(key)
	if len(parts) >= 3 && parts[0] == "user" && parts[2] == "metadata" {
		return parts[1]
	}
	return ""
}

func splitKey(key string) []string {
	var parts []string
	current := ""
	for _, c := range key {
		if c == ':' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func convertToWebAuthnCredential(cred StoredCredential) (*webauthn.Credential, error) {
	// Decode AAGUID
	aaguid, err := base64.RawURLEncoding.DecodeString(cred.AAGUID)
	if err != nil {
		return nil, fmt.Errorf("decode aaguid: %w", err)
	}

	// Decode public key
	publicKey, err := base64.RawURLEncoding.DecodeString(cred.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}

	return &webauthn.Credential{
		ID:              cred.ID,
		Type:            cred.Type,
		AttestationType: cred.Attestation,
		AAGUID:          aaguid,
		PublicKey:       publicKey,
		Transport:       cred.SignCount,
		Flags: webauthn.CredentialFlags{
			UserPresent:    contains(cred.Flags, "userPresent"),
			UserVerified:   contains(cred.Flags, "userVerified"),
			BackupEligible: contains(cred.Flags, "backupEligible"),
			BackupState:    contains(cred.Flags, "backupState"),
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:    aaguid,
			Attachment: "",
			Transport:  cred.Transport,
		},
	}, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
