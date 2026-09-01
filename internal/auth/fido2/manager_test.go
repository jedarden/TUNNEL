package fido2

import (
	"testing"

	"github.com/jedarden/tunnel/internal/core"
)

// mockCredentialStore implements core.CredentialStore for testing
type mockCredentialStore struct {
	data map[string]map[string][]byte
}

func newMockCredentialStore() *mockCredentialStore {
	return &mockCredentialStore{
		data: make(map[string]map[string][]byte),
	}
}

func (m *mockCredentialStore) Get(service, key string) ([]byte, error) {
	if serviceData, ok := m.data[service]; ok {
		if value, ok := serviceData[key]; ok {
			return value, nil
		}
	}
	return nil, core.ErrCredentialNotFound
}

func (m *mockCredentialStore) Set(service, key string, value []byte) error {
	if _, ok := m.data[service]; !ok {
		m.data[service] = make(map[string][]byte)
	}
	m.data[service][key] = value
	return nil
}

func (m *mockCredentialStore) Delete(service, key string) error {
	if serviceData, ok := m.data[service]; ok {
		delete(serviceData, key)
	}
	return nil
}

func (m *mockCredentialStore) List(service string) ([]string, error) {
	var keys []string
	if serviceData, ok := m.data[service]; ok {
		for key := range serviceData {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func TestNewManager(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()

	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.provider == nil {
		t.Error("Expected provider to be initialized")
	}

	if manager.store != store {
		t.Error("Expected manager to have the provided store")
	}
}

func TestNewManagerNilConfig(t *testing.T) {
	store := newMockCredentialStore()

	// Should use default config
	manager, err := NewManager(store, nil)
	if err != nil {
		t.Fatalf("Failed to create manager with nil config: %v", err)
	}

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.provider == nil {
		t.Error("Expected provider to be initialized with default config")
	}
}

func TestManager_CreateUser(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	accountName := "testuser@example.com"
	user, err := manager.CreateUser(accountName)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user == nil {
		t.Fatal("Expected non-nil user")
	}

	if user.Username != accountName {
		t.Errorf("Expected username %s, got %s", accountName, user.Username)
	}

	if user.DisplayName != accountName {
		t.Errorf("Expected display name %s, got %s", accountName, user.DisplayName)
	}

	// Check that user was stored
	exists, err := manager.UserExists(accountName)
	if err != nil {
		t.Fatalf("Failed to check user existence: %v", err)
	}

	if !exists {
		t.Error("Expected user to exist after creation")
	}
}

func TestManager_GetUser(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	accountName := "testuser@example.com"

	// Create user first
	_, err = manager.CreateUser(accountName)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Get user
	user, err := manager.GetUser(accountName)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if user == nil {
		t.Fatal("Expected non-nil user")
	}

	if user.Username != accountName {
		t.Errorf("Expected username %s, got %s", accountName, user.Username)
	}

	// Test getting non-existent user
	_, err = manager.GetUser("nonexistent@example.com")
	if err == nil {
		t.Error("Expected error when getting non-existent user")
	}

	if err.Error() != "user not found" {
		t.Errorf("Expected 'user not found' error, got: %v", err)
	}
}

func TestManager_UserExists(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	accountName := "testuser@example.com"

	// User shouldn't exist initially
	exists, err := manager.UserExists(accountName)
	if err != nil {
		t.Fatalf("Failed to check user existence: %v", err)
	}

	if exists {
		t.Error("Expected user to not exist initially")
	}

	// Create user
	_, err = manager.CreateUser(accountName)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Now user should exist
	exists, err = manager.UserExists(accountName)
	if err != nil {
		t.Fatalf("Failed to check user existence: %v", err)
	}

	if !exists {
		t.Error("Expected user to exist after creation")
	}
}

func TestManager_SaveCredential(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	accountName := "testuser@example.com"

	// Create user first
	_, err = manager.CreateUser(accountName)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create test credential
	cred := &StoredCredential{
		ID:          "test-credential-id",
		Type:        "public-key",
		Attestation: "none",
		AAGUID:      "12345678-1234-1234-1234-123456789abc",
		PublicKey:   "base64encodedpublickey",
		SignCount:   0,
		Transport:   []string{"internal", "hybrid"},
		Flags:       []string{"userPresent", "userVerified"},
	}

	err = manager.SaveCredential(accountName, cred)
	if err != nil {
		t.Fatalf("Failed to save credential: %v", err)
	}

	// Verify credential was saved
	userCreds, err := manager.GetUserCredentials(accountName)
	if err != nil {
		t.Fatalf("Failed to get user credentials: %v", err)
	}

	if len(userCreds.Credentials) != 1 {
		t.Errorf("Expected 1 credential, got %d", len(userCreds.Credentials))
	}

	savedCred := userCreds.Credentials[0]
	if savedCred.ID != cred.ID {
		t.Errorf("Expected credential ID %s, got %s", cred.ID, savedCred.ID)
	}

	if savedCred.Type != cred.Type {
		t.Errorf("Expected credential type %s, got %s", cred.Type, savedCred.Type)
	}
}

func TestManager_UpdateCredential(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	accountName := "testuser@example.com"

	// Create user first
	_, err = manager.CreateUser(accountName)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create test credential
	cred := &StoredCredential{
		ID:          "test-credential-id",
		Type:        "public-key",
		Attestation: "none",
		AAGUID:      "12345678-1234-1234-1234-123456789abc",
		PublicKey:   "base64encodedpublickey",
		SignCount:   0,
		Transport:   []string{"internal"},
		Flags:       []string{"userPresent"},
	}

	err = manager.SaveCredential(accountName, cred)
	if err != nil {
		t.Fatalf("Failed to save credential: %v", err)
	}

	// Update credential sign count
	err = manager.UpdateCredential(accountName, cred.ID, func(c *StoredCredential) {
		c.SignCount = 42
		c.Transport = []string{"internal", "hybrid"}
	})
	if err != nil {
		t.Fatalf("Failed to update credential: %v", err)
	}

	// Verify update
	userCreds, err := manager.GetUserCredentials(accountName)
	if err != nil {
		t.Fatalf("Failed to get user credentials: %v", err)
	}

	updatedCred := userCreds.Credentials[0]
	if updatedCred.SignCount != 42 {
		t.Errorf("Expected sign count 42, got %d", updatedCred.SignCount)
	}

	if len(updatedCred.Transport) != 2 {
		t.Errorf("Expected 2 transport entries, got %d", len(updatedCred.Transport))
	}
}

func TestManager_DeleteCredential(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	accountName := "testuser@example.com"

	// Create user first
	_, err = manager.CreateUser(accountName)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create test credentials
	cred1 := &StoredCredential{
		ID:         "cred-1",
		Type:       "public-key",
		Attestation: "none",
		AAGUID:     "12345678-1234-1234-1234-123456789abc",
		PublicKey:  "base64encodedpublickey1",
		SignCount:  0,
	}

	cred2 := &StoredCredential{
		ID:         "cred-2",
		Type:       "public-key",
		Attestation: "none",
		AAGUID:     "87654321-4321-4321-4321-cba987654321",
		PublicKey:  "base64encodedpublickey2",
		SignCount:  0,
	}

	err = manager.SaveCredential(accountName, cred1)
	if err != nil {
		t.Fatalf("Failed to save credential 1: %v", err)
	}

	err = manager.SaveCredential(accountName, cred2)
	if err != nil {
		t.Fatalf("Failed to save credential 2: %v", err)
	}

	// Delete one credential
	err = manager.DeleteCredential(accountName, "cred-1")
	if err != nil {
		t.Fatalf("Failed to delete credential: %v", err)
	}

	// Verify deletion
	userCreds, err := manager.GetUserCredentials(accountName)
	if err != nil {
		t.Fatalf("Failed to get user credentials: %v", err)
	}

	if len(userCreds.Credentials) != 1 {
		t.Errorf("Expected 1 credential after deletion, got %d", len(userCreds.Credentials))
	}

	if userCreds.Credentials[0].ID != "cred-2" {
		t.Errorf("Expected remaining credential to be cred-2, got %s", userCreds.Credentials[0].ID)
	}

	// Try to delete non-existent credential
	err = manager.DeleteCredential(accountName, "nonexistent")
	if err == nil {
		t.Error("Expected error when deleting non-existent credential")
	}
}

func TestManager_DeleteUser(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	accountName := "testuser@example.com"

	// Create user first
	_, err = manager.CreateUser(accountName)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify user exists
	exists, err := manager.UserExists(accountName)
	if err != nil {
		t.Fatalf("Failed to check user existence: %v", err)
	}

	if !exists {
		t.Error("Expected user to exist before deletion")
	}

	// Delete user
	err = manager.DeleteUser(accountName)
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	// Verify user was deleted
	exists, err = manager.UserExists(accountName)
	if err != nil {
		t.Fatalf("Failed to check user existence after deletion: %v", err)
	}

	if exists {
		t.Error("Expected user to not exist after deletion")
	}
}

func TestManager_ListUsers(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Initially no users
	users, err := manager.ListUsers()
	if err != nil {
		t.Fatalf("Failed to list users: %v", err)
	}

	if len(users) != 0 {
		t.Errorf("Expected 0 users initially, got %d", len(users))
	}

	// Create multiple users
	testUsers := []string{
		"user1@example.com",
		"user2@example.com",
		"user3@example.com",
	}

	for _, user := range testUsers {
		_, err = manager.CreateUser(user)
		if err != nil {
			t.Fatalf("Failed to create user %s: %v", user, err)
		}
	}

	// List users
	users, err = manager.ListUsers()
	if err != nil {
		t.Fatalf("Failed to list users: %v", err)
	}

	if len(users) != len(testUsers) {
		t.Errorf("Expected %d users, got %d", len(testUsers), len(users))
	}

	// Verify all users are present
	userMap := make(map[string]bool)
	for _, user := range users {
		userMap[user] = true
	}

	for _, expectedUser := range testUsers {
		if !userMap[expectedUser] {
			t.Errorf("Expected user %s not found in list", expectedUser)
		}
	}
}

func TestManager_GetUserCredentials(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	accountName := "testuser@example.com"

	// Create user first
	_, err = manager.CreateUser(accountName)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Get credentials (should be empty)
	userCreds, err := manager.GetUserCredentials(accountName)
	if err != nil {
		t.Fatalf("Failed to get user credentials: %v", err)
	}

	if userCreds == nil {
		t.Fatal("Expected non-nil user credentials")
	}

	if userCreds.Metadata == nil {
		t.Fatal("Expected non-nil metadata")
	}

	if userCreds.Metadata.Username != accountName {
		t.Errorf("Expected username %s, got %s", accountName, userCreds.Metadata.Username)
	}

	if len(userCreds.Credentials) != 0 {
		t.Errorf("Expected 0 credentials initially, got %d", len(userCreds.Credentials))
	}

	// Test with non-existent user
	_, err = manager.GetUserCredentials("nonexistent@example.com")
	if err == nil {
		t.Error("Expected error when getting credentials for non-existent user")
	}

	if err.Error() != "user not found" {
		t.Errorf("Expected 'user not found' error, got: %v", err)
	}
}

func TestManager_BeginRegistration(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	accountName := "testuser@example.com"

	// Begin registration (should create user automatically)
	req, err := manager.BeginRegistration(accountName)
	if err != nil {
		t.Fatalf("Failed to begin registration: %v", err)
	}

	if req == nil {
		t.Fatal("Expected non-nil registration request")
	}

	if req.User == nil {
		t.Error("Expected non-nil user in registration request")
	}

	if req.SessionData == nil {
		t.Error("Expected non-nil session data")
	}

	if req.Challenge == "" {
		t.Error("Expected non-empty challenge")
	}

	if req.JSON == "" {
		t.Error("Expected non-empty JSON options")
	}

	// Verify user was created
	exists, err := manager.UserExists(accountName)
	if err != nil {
		t.Fatalf("Failed to check user existence: %v", err)
	}

	if !exists {
		t.Error("Expected user to be created during registration")
	}
}

func TestManager_BeginAuthentication(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	accountName := "testuser@example.com"

	// Create user first
	_, err = manager.CreateUser(accountName)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Try authentication with no credentials (should fail)
	_, err = manager.BeginAuthentication(accountName)
	if err == nil {
		t.Error("Expected error when beginning authentication with no credentials")
	}

	if err.Error() != "user has no registered credentials" {
		t.Errorf("Expected 'user has no registered credentials' error, got: %v", err)
	}
}

// BenchmarkManager_CreateUser benchmarks user creation
func BenchmarkManager_CreateUser(b *testing.B) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		accountName := "benchuser@example.com"
		_, err := manager.CreateUser(accountName)
		if err != nil {
			b.Fatalf("Failed to create user: %v", err)
		}
	}
}

// BenchmarkManager_GetUser benchmarks user retrieval
func BenchmarkManager_GetUser(b *testing.B) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}

	accountName := "benchuser@example.com"
	_, err = manager.CreateUser(accountName)
	if err != nil {
		b.Fatalf("Failed to create user: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := manager.GetUser(accountName)
		if err != nil {
			b.Fatalf("Failed to get user: %v", err)
		}
	}
}
