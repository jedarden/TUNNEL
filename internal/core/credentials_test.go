package core

import (
	"os"
	"path/filepath"
	"testing"
)

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
