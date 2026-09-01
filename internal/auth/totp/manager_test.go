package totp

import (
	"testing"

	"github.com/jedarden/tunnel/internal/core"
)

// mockCredentialStore implements CredentialStore for testing
type mockCredentialStore struct {
	data map[string]map[string][]byte
}

func newMockCredentialStore() *mockCredentialStore {
	return &mockCredentialStore{
		data: make(map[string]map[string][]byte),
	}
}

func (m *mockCredentialStore) Set(service, key string, value []byte) error {
	if m.data[service] == nil {
		m.data[service] = make(map[string][]byte)
	}
	m.data[service][key] = value
	return nil
}

func (m *mockCredentialStore) Get(service, key string) ([]byte, error) {
	if m.data[service] == nil {
		return nil, core.ErrCredentialNotFound
	}
	value, ok := m.data[service][key]
	if !ok {
		return nil, core.ErrCredentialNotFound
	}
	return value, nil
}

func (m *mockCredentialStore) Delete(service, key string) error {
	if m.data[service] == nil {
		return nil
	}
	delete(m.data[service], key)
	return nil
}

func (m *mockCredentialStore) List(service string) ([]string, error) {
	if m.data[service] == nil {
		return []string{}, nil
	}
	keys := make([]string, 0, len(m.data[service]))
	for key := range m.data[service] {
		keys = append(keys, key)
	}
	return keys, nil
}

func TestManagerEnrollUser(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")

	accountName := "user@example.com"
	bundle, err := manager.EnrollUser(accountName)
	if err != nil {
		t.Fatalf("EnrollUser() error = %v", err)
	}

	if bundle == nil {
		t.Fatal("EnrollUser() returned nil bundle")
	}
	if bundle.Secret == "" {
		t.Error("EnrollUser().Secret is empty")
	}
	if bundle.Account != accountName {
		t.Errorf("EnrollUser().Account = %v, want %v", bundle.Account, accountName)
	}
	if bundle.Issuer != "test-issuer" {
		t.Errorf("EnrollUser().Issuer = %v, want 'test-issuer'", bundle.Issuer)
	}

	// Verify secret was stored
	storedSecret, err := store.Get(totpService, accountName)
	if err != nil {
		t.Errorf("Secret not stored: %v", err)
	}
	if string(storedSecret) != bundle.Secret {
		t.Errorf("Stored secret = %v, want %v", string(storedSecret), bundle.Secret)
	}
}

func TestManagerValidateCode(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")

	accountName := "user@example.com"

	// Enroll user first
	bundle, err := manager.EnrollUser(accountName)
	if err != nil {
		t.Fatalf("EnrollUser() error = %v", err)
	}

	// Generate current valid code
	validCode, err := manager.GenerateCurrentCode(accountName)
	if err != nil {
		t.Fatalf("GenerateCurrentCode() error = %v", err)
	}

	tests := []struct {
		name        string
		accountName string
		code        string
		window      int
		wantValid   bool
		wantErr     bool
	}{
		{
			name:        "valid code",
			accountName: accountName,
			code:        validCode,
			window:      1,
			wantValid:   true,
			wantErr:     false,
		},
		{
			name:        "invalid code",
			accountName: accountName,
			code:        "000000",
			window:      1,
			wantValid:   false,
			wantErr:     false,
		},
		{
			name:        "user not enrolled",
			accountName: "other@example.com",
			code:        validCode,
			window:      1,
			wantValid:   false,
			wantErr:     true,
		},
		{
			name:        "valid code with window 0",
			accountName: accountName,
			code:        validCode,
			window:      0,
			wantValid:   true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := manager.ValidateCode(tt.accountName, tt.code, tt.window)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && valid != tt.wantValid {
				t.Errorf("ValidateCode() = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

func TestManagerGetSecret(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")

	accountName := "user@example.com"

	// Enroll user first
	bundle, err := manager.EnrollUser(accountName)
	if err != nil {
		t.Fatalf("EnrollUser() error = %v", err)
	}

	// Get secret
	secret, err := manager.GetSecret(accountName)
	if err != nil {
		t.Fatalf("GetSecret() error = %v", err)
	}

	if secret != bundle.Secret {
		t.Errorf("GetSecret() = %v, want %v", secret, bundle.Secret)
	}

	// Try non-existent user
	_, err = manager.GetSecret("other@example.com")
	if err == nil {
		t.Error("GetSecret() for non-existent user should return error")
	}
}

func TestManagerDeleteUser(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")

	accountName := "user@example.com"

	// Enroll user first
	_, err := manager.EnrollUser(accountName)
	if err != nil {
		t.Fatalf("EnrollUser() error = %v", err)
	}

	// Verify user exists
	enrolled, err := manager.IsEnrolled(accountName)
	if err != nil {
		t.Fatalf("IsEnrolled() error = %v", err)
	}
	if !enrolled {
		t.Error("User should be enrolled")
	}

	// Delete user
	err = manager.DeleteUser(accountName)
	if err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	// Verify user is deleted
	enrolled, err = manager.IsEnrolled(accountName)
	if err != nil {
		t.Fatalf("IsEnrolled() error = %v", err)
	}
	if enrolled {
		t.Error("User should not be enrolled after deletion")
	}
}

func TestManagerIsEnrolled(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")

	accountName := "user@example.com"

	// Initially not enrolled
	enrolled, err := manager.IsEnrolled(accountName)
	if err != nil {
		t.Fatalf("IsEnrolled() error = %v", err)
	}
	if enrolled {
		t.Error("User should not be enrolled initially")
	}

	// Enroll user
	_, err = manager.EnrollUser(accountName)
	if err != nil {
		t.Fatalf("EnrollUser() error = %v", err)
	}

	// Now enrolled
	enrolled, err = manager.IsEnrolled(accountName)
	if err != nil {
		t.Fatalf("IsEnrolled() error = %v", err)
	}
	if !enrolled {
		t.Error("User should be enrolled after enrollment")
	}
}

func TestManagerListEnrolledUsers(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")

	// Initially empty
	users, err := manager.ListEnrolledUsers()
	if err != nil {
		t.Fatalf("ListEnrolledUsers() error = %v", err)
	}
	if len(users) != 0 {
		t.Errorf("ListEnrolledUsers() = %v, want empty", users)
	}

	// Enroll some users
	accounts := []string{"user1@example.com", "user2@example.com", "user3@example.com"}
	for _, account := range accounts {
		_, err := manager.EnrollUser(account)
		if err != nil {
			t.Fatalf("EnrollUser(%s) error = %v", account, err)
		}
	}

	// List users
	users, err = manager.ListEnrolledUsers()
	if err != nil {
		t.Fatalf("ListEnrolledUsers() error = %v", err)
	}

	if len(users) != len(accounts) {
		t.Errorf("ListEnrolledUsers() count = %v, want %v", len(users), len(accounts))
	}

	// Verify all users are present
	userMap := make(map[string]bool)
	for _, user := range users {
		userMap[user] = true
	}
	for _, account := range accounts {
		if !userMap[account] {
			t.Errorf("ListEnrolledUsers() missing user: %v", account)
		}
	}
}

func TestManagerRegenerateSecret(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")

	accountName := "user@example.com"

	// Enroll user first
	bundle1, err := manager.EnrollUser(accountName)
	if err != nil {
		t.Fatalf("EnrollUser() error = %v", err)
	}

	// Regenerate secret
	bundle2, err := manager.RegenerateSecret(accountName)
	if err != nil {
		t.Fatalf("RegenerateSecret() error = %v", err)
	}

	// Secrets should be different
	if bundle1.Secret == bundle2.Secret {
		t.Error("RegenerateSecret() should generate a new secret")
	}

	// New secret should be valid
	currentCode, err := manager.GenerateCurrentCode(accountName)
	if err != nil {
		t.Fatalf("GenerateCurrentCode() error = %v", err)
	}
	valid, err := manager.ValidateCode(accountName, currentCode, 1)
	if err != nil {
		t.Fatalf("ValidateCode() error = %v", err)
	}
	if !valid {
		t.Error("Regenerated secret should be valid")
	}

	// Try regenerating non-existent user
	_, err = manager.RegenerateSecret("other@example.com")
	if err == nil {
		t.Error("RegenerateSecret() for non-existent user should return error")
	}
}

func TestManagerGenerateCurrentCode(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")

	accountName := "user@example.com"

	// Enroll user first
	_, err := manager.EnrollUser(accountName)
	if err != nil {
		t.Fatalf("EnrollUser() error = %v", err)
	}

	// Generate current code
	code, err := manager.GenerateCurrentCode(accountName)
	if err != nil {
		t.Fatalf("GenerateCurrentCode() error = %v", err)
	}

	if len(code) != 6 {
		t.Errorf("GenerateCurrentCode() length = %v, want 6", len(code))
	}

	// Code should be valid
	valid, err := manager.ValidateCode(accountName, code, 1)
	if err != nil {
		t.Fatalf("ValidateCode() error = %v", err)
	}
	if !valid {
		t.Error("Generated code should be valid")
	}

	// Try non-existent user
	_, err = manager.GenerateCurrentCode("other@example.com")
	if err == nil {
		t.Error("GenerateCurrentCode() for non-existent user should return error")
	}
}

func TestManagerGetTimeRemaining(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")

	accountName := "user@example.com"

	// Enroll user first
	_, err := manager.EnrollUser(accountName)
	if err != nil {
		t.Fatalf("EnrollUser() error = %v", err)
	}

	// Get time remaining
	remaining, err := manager.GetTimeRemaining(accountName)
	if err != nil {
		t.Fatalf("GetTimeRemaining() error = %v", err)
	}

	if remaining < 0 || remaining > 30 {
		t.Errorf("GetTimeRemaining() = %v, want 0-30", remaining)
	}

	// Try non-existent user
	_, err = manager.GetTimeRemaining("other@example.com")
	if err == nil {
		t.Error("GetTimeRemaining() for non-existent user should return error")
	}
}

func TestManagerValidateWithTimeCheck(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")

	accountName := "user@example.com"

	// Enroll user first
	_, err := manager.EnrollUser(accountName)
	if err != nil {
		t.Fatalf("EnrollUser() error = %v", err)
	}

	// Generate current valid code
	validCode, err := manager.GenerateCurrentCode(accountName)
	if err != nil {
		t.Fatalf("GenerateCurrentCode() error = %v", err)
	}

	// Validate with time check
	valid, err := manager.ValidateWithTimeCheck(accountName, validCode, 1)
	if err != nil {
		t.Fatalf("ValidateWithTimeCheck() error = %v", err)
	}
	if !valid {
		t.Error("ValidateWithTimeCheck() should validate valid code")
	}
}
