package core

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type testCredentialStore struct {
	values   map[string][]byte
	getErr   error
	setErr   error
	setCalls int
}

func newTestCredentialStore() *testCredentialStore {
	return &testCredentialStore{values: make(map[string][]byte)}
}

func (s *testCredentialStore) Set(service, key string, value []byte) error {
	s.setCalls++
	if s.setErr != nil {
		return s.setErr
	}
	s.values[service+":"+key] = append([]byte(nil), value...)
	return nil
}

func (s *testCredentialStore) Get(service, key string) ([]byte, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	value, ok := s.values[service+":"+key]
	if !ok {
		return nil, ErrCredentialNotFound
	}
	return append([]byte(nil), value...), nil
}

func (s *testCredentialStore) Delete(service, key string) error {
	delete(s.values, service+":"+key)
	return nil
}

func (s *testCredentialStore) List(string) ([]string, error) {
	return nil, nil
}

func TestGetOrCreateControlToken(t *testing.T) {
	store := newTestCredentialStore()

	token, created, err := GetOrCreateControlToken(store)
	if err != nil {
		t.Fatalf("GetOrCreateControlToken() error = %v", err)
	}
	if !created {
		t.Fatal("GetOrCreateControlToken() created = false, want true")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("token is not unpadded URL-safe base64: %v", err)
	}
	if len(decoded) != controlTokenBytes {
		t.Fatalf("decoded token length = %d, want %d", len(decoded), controlTokenBytes)
	}

	storedToken, created, err := GetOrCreateControlToken(store)
	if err != nil {
		t.Fatalf("second GetOrCreateControlToken() error = %v", err)
	}
	if created {
		t.Fatal("second GetOrCreateControlToken() created = true, want false")
	}
	if storedToken != token {
		t.Fatal("second GetOrCreateControlToken() returned a different token")
	}
	if store.setCalls != 1 {
		t.Fatalf("credential store Set calls = %d, want 1", store.setCalls)
	}
}

func TestGetOrCreateControlTokenDoesNotRotateOnStoreFailure(t *testing.T) {
	backendErr := errors.New("backend unavailable")
	store := newTestCredentialStore()
	store.getErr = backendErr

	_, created, err := GetOrCreateControlToken(store)
	if !errors.Is(err, backendErr) {
		t.Fatalf("GetOrCreateControlToken() error = %v, want wrapped backend error", err)
	}
	if created {
		t.Fatal("GetOrCreateControlToken() created = true after backend failure")
	}
	if store.setCalls != 0 {
		t.Fatalf("credential store Set calls = %d, want 0", store.setCalls)
	}
}

func TestGetOrCreateControlTokenRejectsEmptyStoredToken(t *testing.T) {
	store := newTestCredentialStore()
	store.values[controlTokenService+":"+controlTokenKey] = []byte{}

	_, _, err := GetOrCreateControlToken(store)
	if !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("GetOrCreateControlToken() error = %v, want ErrInvalidCredential", err)
	}
	if store.setCalls != 0 {
		t.Fatalf("credential store Set calls = %d, want 0", store.setCalls)
	}
}

func TestFileStore(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Create file store
	store, err := NewFileStore(tmpDir, "test-passphrase-123")
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	// Test Set
	service := "test-service"
	key := "test-key"
	value := []byte("test-value-secret")

	if err := store.Set(service, key, value); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Test Get
	retrieved, err := store.Get(service, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(retrieved) != string(value) {
		t.Errorf("Expected %s, got %s", value, retrieved)
	}

	// Test List
	keys, err := store.List(service)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(keys) != 1 || keys[0] != key {
		t.Errorf("Expected [%s], got %v", key, keys)
	}

	// Test Delete
	if err := store.Delete(service, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = store.Get(service, key)
	if err != ErrCredentialNotFound {
		t.Errorf("Expected ErrCredentialNotFound, got %v", err)
	}
}

func TestEnvStore(t *testing.T) {
	store := NewEnvStore("TUNNEL_TEST")

	service := "test-service"
	key := "test-key"
	value := []byte("test-value")

	// Test Set
	if err := store.Set(service, key, value); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Test Get
	retrieved, err := store.Get(service, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(retrieved) != string(value) {
		t.Errorf("Expected %s, got %s", value, retrieved)
	}

	// Test Delete
	if err := store.Delete(service, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = store.Get(service, key)
	if err != ErrCredentialNotFound {
		t.Errorf("Expected ErrCredentialNotFound, got %v", err)
	}
}

func TestNewCredentialStore(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		storeType string
		expectErr bool
	}{
		{"file store", "file", false},
		{"env store", "env", false},
		{"invalid store", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCredentialStore(tt.storeType, "tunnel", tmpDir, "test-pass")
			if (err != nil) != tt.expectErr {
				t.Errorf("Expected error: %v, got: %v", tt.expectErr, err)
			}
		})
	}
}

func TestFileStoreEncryption(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir, "test-passphrase")
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}

	// Store a credential
	service := "test"
	key := "password"
	value := []byte("super-secret-password")

	if err := store.Set(service, key, value); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Read the file directly
	filePath := filepath.Join(tmpDir, "test.cred")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// Verify the file is encrypted (should not contain plaintext)
	if string(data) == string(value) {
		t.Error("Credential file is not encrypted!")
	}

	// Verify we can still retrieve it
	retrieved, err := store.Get(service, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if string(retrieved) != string(value) {
		t.Errorf("Expected %s, got %s", value, retrieved)
	}
}
