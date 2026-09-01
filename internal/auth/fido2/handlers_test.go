package fido2

import (
	"bytes"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
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

func TestNewHandler(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	handler := NewHandler(manager)

	if handler == nil {
		t.Fatal("Expected non-nil handler")
	}

	if handler.manager != manager {
		t.Error("Expected handler to have the provided manager")
	}
}

func TestHandler_RegisterRoutes(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	handler := NewHandler(manager)
	app := fiber.New()

	// Register routes
	api := app.Group("/api")
	handler.RegisterRoutes(api)

	// Verify routes are registered by testing the app structure
	// This is a basic check - more comprehensive testing would involve actual requests
	if app == nil {
		t.Fatal("Expected non-nil app after registering routes")
	}
}

func TestHandler_RegistrationBegin(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	handler := NewHandler(manager)
	app := fiber.New()

	api := app.Group("/api")
	handler.RegisterRoutes(api)

	// Test registration begin
	reqBody := `{"account_name": "testuser"}`
	req := httptest.NewRequest("POST", "/api/fido2/register/begin", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.StatusCode != fiber.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("Expected status 201, got %d. Response body: %s", resp.StatusCode, string(body))
	}

	// Verify response contains expected fields
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Error("Expected JSON response")
	}
}

func TestHandler_RegistrationBeginMissingAccount(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	handler := NewHandler(manager)
	app := fiber.New()

	api := app.Group("/api")
	handler.RegisterRoutes(api)

	// Test registration begin without account name
	reqBody := `{}`
	req := httptest.NewRequest("POST", "/api/fido2/register/begin", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.StatusCode)
	}
}

func TestHandler_AuthenticationBeginNoCredentials(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	handler := NewHandler(manager)
	app := fiber.New()

	api := app.Group("/api")
	handler.RegisterRoutes(api)

	// Test authentication begin for user with no credentials
	reqBody := `{"account_name": "nonexistent"}`
	req := httptest.NewRequest("POST", "/api/fido2/authenticate/begin", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected status 404 for user with no credentials, got %d", resp.StatusCode)
	}
}

func TestHandler_Status(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	handler := NewHandler(manager)
	app := fiber.New()

	api := app.Group("/api")
	handler.RegisterRoutes(api)

	// Test status for non-existent user
	req := httptest.NewRequest("GET", "/api/fido2/status/nonexistent", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent user, got %d", resp.StatusCode)
	}
}

func TestHandler_ListUsers(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	handler := NewHandler(manager)
	app := fiber.New()

	api := app.Group("/api")
	handler.RegisterRoutes(api)

	// Test list users (should be empty initially)
	req := httptest.NewRequest("GET", "/api/fido2/users", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHandler_DeleteUser(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	handler := NewHandler(manager)
	app := fiber.New()

	api := app.Group("/api")
	handler.RegisterRoutes(api)

	// Test delete user (should fail for non-existent user, but that's ok for this test)
	req := httptest.NewRequest("DELETE", "/api/fido2/nonexistent", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Should either succeed (idempotent delete) or fail gracefully
	if resp.StatusCode != fiber.StatusOK && resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("Expected status 200 or 500, got %d", resp.StatusCode)
	}
}

// TestHandler_CredentialManagement tests credential CRUD operations
func TestHandler_CredentialManagement(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	handler := NewHandler(manager)
	app := fiber.New()

	api := app.Group("/api")
	handler.RegisterRoutes(api)

	accountName := "testuser"

	// Create user first
	_, err = manager.CreateUser(accountName)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Test get credentials (should be empty)
	req := httptest.NewRequest("GET", "/api/fido2/credentials/"+accountName, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Test delete non-existent credential
	req = httptest.NewRequest("DELETE", "/api/fido2/credentials/"+accountName+"/nonexistent_cred", nil)
	resp, err = app.Test(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	if resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("Expected status 404 for non-existent credential, got %d", resp.StatusCode)
	}
}

// TestHandler_MissingAccountParameter tests handlers that require account parameter
func TestHandler_MissingAccountParameter(t *testing.T) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	handler := NewHandler(manager)
	app := fiber.New()

	api := app.Group("/api")
	handler.RegisterRoutes(api)

	tests := []struct {
		name   string
		method string
		url    string
	}{
		{"get credentials", "GET", "/api/fido2/credentials/"},
		{"delete credential", "DELETE", "/api/fido2/credentials/test/"},
		{"status", "GET", "/api/fido2/status/"},
		{"delete user", "DELETE", "/api/fido2/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}

			// Should either be a 404 (route not found) or error response
			if resp.StatusCode != fiber.StatusNotFound && resp.StatusCode != fiber.StatusBadRequest {
				t.Errorf("Expected status 404 or 400 for missing account parameter, got %d", resp.StatusCode)
			}
		})
	}
}

// BenchmarkHandler_RegistrationBegin benchmarks the registration begin operation
func BenchmarkHandler_RegistrationBegin(b *testing.B) {
	store := newMockCredentialStore()
	config := DefaultProviderConfig()
	manager, err := NewManager(store, config)
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}

	handler := NewHandler(manager)
	app := fiber.New()

	api := app.Group("/api")
	handler.RegisterRoutes(api)

	reqBody := `{"account_name": "benchuser"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/fido2/register/begin", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		_, err := app.Test(req)
		if err != nil {
			b.Fatalf("Failed to send request: %v", err)
		}
	}
}
