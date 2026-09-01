package totp

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/jedarden/tunnel/internal/core"
)

// Helper to create a test app
func setupTestApp(handler *Handler) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	api := app.Group("/api")
	handler.RegisterRoutes(api)

	return app
}

// Helper to make test requests
func makeRequest(app *fiber.App, method, path string, body interface{}) (int, []byte) {
	var bodyReader *bytes.Reader
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(jsonBody)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, _ := fiber.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := app.Test(req)
	if err != nil {
		return 0, []byte(err.Error())
	}

	defer resp.Body.Close()
	result := make([]byte, resp.ContentLength)
	resp.Body.Read(result)

	return resp.StatusCode, result
}

func TestHandleEnroll(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")
	handler := NewHandler(manager)
	app := setupTestApp(handler)

	tests := []struct {
		name        string
		requestBody interface{}
		wantStatus  int
		wantFields  []string
	}{
		{
			name: "successful enrollment",
			requestBody: map[string]string{
				"account_name": "user@example.com",
			},
			wantStatus: fiber.StatusCreated,
			wantFields: []string{"secret", "qr_code", "url", "issuer", "account", "period", "digits", "algorithm"},
		},
		{
			name:        "missing account name",
			requestBody: map[string]string{},
			wantStatus:  fiber.StatusBadRequest,
		},
		{
			name:        "invalid request body",
			requestBody: "invalid json",
			wantStatus:  fiber.StatusBadRequest,
		},
		{
			name: "duplicate enrollment",
			requestBody: map[string]string{
				"account_name": "user@example.com",
			},
			wantStatus: fiber.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For duplicate test, enroll first
			if tt.name == "duplicate enrollment" {
				_, err := manager.EnrollUser("user@example.com")
				if err != nil {
					t.Fatalf("Setup enrollment failed: %v", err)
				}
			}

			status, body := makeRequest(app, "POST", "/api/totp/enroll", tt.requestBody)

			if status != tt.wantStatus {
				t.Errorf("Status = %v, want %v, body: %s", status, tt.wantStatus, string(body))
			}

			if tt.wantFields != nil && len(tt.wantFields) > 0 {
				var response map[string]interface{}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				for _, field := range tt.wantFields {
					if _, ok := response[field]; !ok {
						t.Errorf("Response missing field: %v", field)
					}
				}
			}
		})
	}
}

func TestHandleValidate(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")
	handler := NewHandler(manager)
	app := setupTestApp(handler)

	// Setup: Enroll a user
	bundle, err := manager.EnrollUser("user@example.com")
	if err != nil {
		t.Fatalf("Setup enrollment failed: %v", err)
	}

	// Generate a valid code
	validCode, err := manager.GenerateCurrentCode("user@example.com")
	if err != nil {
		t.Fatalf("Generate code failed: %v", err)
	}

	tests := []struct {
		name        string
		setupFunc   func()
		requestBody interface{}
		wantStatus  int
		wantValid   bool
	}{
		{
			name: "valid code",
			requestBody: map[string]string{
				"account_name": "user@example.com",
				"code":        validCode,
				"window":      "1",
			},
			wantStatus: fiber.StatusOK,
			wantValid:  true,
		},
		{
			name: "invalid code",
			requestBody: map[string]string{
				"account_name": "user@example.com",
				"code":        "000000",
				"window":      "1",
			},
			wantStatus: fiber.StatusOK,
			wantValid:  false,
		},
		{
			name: "missing account name",
			requestBody: map[string]string{
				"code": validCode,
			},
			wantStatus: fiber.StatusBadRequest,
		},
		{
			name: "missing code",
			requestBody: map[string]string{
				"account_name": "user@example.com",
			},
			wantStatus: fiber.StatusBadRequest,
		},
		{
			name: "user not enrolled",
			requestBody: map[string]string{
				"account_name": "other@example.com",
				"code":        validCode,
				"window":      "1",
			},
			wantStatus: fiber.StatusInternalServerError,
		},
		{
			name: "default window",
			requestBody: map[string]string{
				"account_name": "user@example.com",
				"code":        validCode,
			},
			wantStatus: fiber.StatusOK,
			wantValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			status, body := makeRequest(app, "POST", "/api/totp/validate", tt.requestBody)

			if status != tt.wantStatus {
				t.Errorf("Status = %v, want %v, body: %s", status, tt.wantStatus, string(body))
			}

			if tt.wantValid || tt.wantStatus == fiber.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if tt.wantValid && response["valid"] != true {
					t.Errorf("Expected valid=true, got %v", response["valid"])
				}
			}
		})
	}

	// Cleanup
	_ = manager.DeleteUser("user@example.com")
}

func TestHandleStatus(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")
	handler := NewHandler(manager)
	app := setupTestApp(handler)

	// Setup: Enroll a user
	_, err := manager.EnrollUser("user@example.com")
	if err != nil {
		t.Fatalf("Setup enrollment failed: %v", err)
	}

	tests := []struct {
		name       string
		account    string
		wantStatus int
		wantEnrolled bool
	}{
		{
			name:        "enrolled user",
			account:     "user@example.com",
			wantStatus:  fiber.StatusOK,
			wantEnrolled: true,
		},
		{
			name:        "not enrolled user",
			account:     "other@example.com",
			wantStatus:  fiber.StatusOK,
			wantEnrolled: false,
		},
		{
			name:        "empty account",
			account:     "",
			wantStatus:  fiber.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := makeRequest(app, "GET", "/api/totp/status/"+tt.account, nil)

			if status != tt.wantStatus {
				t.Errorf("Status = %v, want %v, body: %s", status, tt.wantStatus, string(body))
			}

			if tt.wantStatus == fiber.StatusOK {
				var response map[string]interface{}
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if response["enrolled"] != tt.wantEnrolled {
					t.Errorf("enrolled = %v, want %v", response["enrolled"], tt.wantEnrolled)
				}
			}
		})
	}

	// Cleanup
	_ = manager.DeleteUser("user@example.com")
}

func TestHandleDelete(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")
	handler := NewHandler(manager)
	app := setupTestApp(handler)

	// Setup: Enroll a user
	_, err := manager.EnrollUser("user@example.com")
	if err != nil {
		t.Fatalf("Setup enrollment failed: %v", err)
	}

	tests := []struct {
		name       string
		account    string
		wantStatus int
		checkDeleted bool
	}{
		{
			name:        "delete enrolled user",
			account:     "user@example.com",
			wantStatus:  fiber.StatusOK,
			checkDeleted: true,
		},
		{
			name:        "delete non-existent user",
			account:     "other@example.com",
			wantStatus:  fiber.StatusOK,
		},
		{
			name:        "empty account",
			account:     "",
			wantStatus:  fiber.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := makeRequest(app, "DELETE", "/api/totp/"+tt.account, nil)

			if status != tt.wantStatus {
				t.Errorf("Status = %v, want %v, body: %s", status, tt.wantStatus, string(body))
			}

			if tt.checkDeleted {
				enrolled, err := manager.IsEnrolled(tt.account)
				if err != nil {
					t.Errorf("Failed to check enrollment after delete: %v", err)
				}
				if enrolled {
					t.Error("User should not be enrolled after deletion")
				}
			}
		})
	}
}

func TestHandleListUsers(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")
	handler := NewHandler(manager)
	app := setupTestApp(handler)

	// Setup: Enroll multiple users
	users := []string{"user1@example.com", "user2@example.com", "user3@example.com"}
	for _, user := range users {
		_, err := manager.EnrollUser(user)
		if err != nil {
			t.Fatalf("Setup enrollment failed for %s: %v", user, err)
		}
	}

	status, body := makeRequest(app, "GET", "/api/totp/users", nil)

	if status != fiber.StatusOK {
		t.Errorf("Status = %v, want %v, body: %s", status, fiber.StatusOK, string(body))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	usersList, ok := response["users"].([]interface{})
	if !ok {
		t.Fatal("Response missing users list")
	}

	if len(usersList) != len(users) {
		t.Errorf("Users count = %v, want %v", len(usersList), len(users))
	}

	// Cleanup
	for _, user := range users {
		_ = manager.DeleteUser(user)
	}
}

func TestHandleRegenerate(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")
	handler := NewHandler(manager)
	app := setupTestApp(handler)

	// Setup: Enroll a user
	bundle1, err := manager.EnrollUser("user@example.com")
	if err != nil {
		t.Fatalf("Setup enrollment failed: %v", err)
	}

	// Regenerate
	status, body := makeRequest(app, "POST", "/api/totp/regenerate/user@example.com", nil)

	if status != fiber.StatusOK {
		t.Errorf("Status = %v, want %v, body: %s", status, fiber.StatusOK, string(body))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	newSecret := response["secret"].(string)
	if newSecret == bundle1.Secret {
		t.Error("Regenerated secret should be different from original")
	}

	// Try non-existent user
	status, _ = makeRequest(app, "POST", "/api/totp/regenerate/other@example.com", nil)
	if status != fiber.StatusInternalServerError {
		t.Errorf("Status for non-existent user = %v, want %v", status, fiber.StatusInternalServerError)
	}

	// Cleanup
	_ = manager.DeleteUser("user@example.com")
}

func TestHandleGetCurrentCode(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")
	handler := NewHandler(manager)
	app := setupTestApp(handler)

	// Setup: Enroll a user
	_, err := manager.EnrollUser("user@example.com")
	if err != nil {
		t.Fatalf("Setup enrollment failed: %v", err)
	}

	status, body := makeRequest(app, "GET", "/api/totp/code/user@example.com", nil)

	if status != fiber.StatusOK {
		t.Errorf("Status = %v, want %v, body: %s", status, fiber.StatusOK, string(body))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	code, ok := response["code"].(string)
	if !ok || len(code) != 6 {
		t.Errorf("Code should be 6 digits, got: %v", code)
	}

	// Try non-existent user
	status, _ = makeRequest(app, "GET", "/api/totp/code/other@example.com", nil)
	if status != fiber.StatusInternalServerError {
		t.Errorf("Status for non-existent user = %v, want %v", status, fiber.StatusInternalServerError)
	}

	// Cleanup
	_ = manager.DeleteUser("user@example.com")
}

func TestHandleGetTimeRemaining(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")
	handler := NewHandler(manager)
	app := setupTestApp(handler)

	// Setup: Enroll a user
	_, err := manager.EnrollUser("user@example.com")
	if err != nil {
		t.Fatalf("Setup enrollment failed: %v", err)
	}

	status, body := makeRequest(app, "GET", "/api/totp/time/user@example.com", nil)

	if status != fiber.StatusOK {
		t.Errorf("Status = %v, want %v, body: %s", status, fiber.StatusOK, string(body))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	remaining, ok := response["remaining"].(float64)
	if !ok || remaining < 0 || remaining > 30 {
		t.Errorf("Remaining time should be 0-30, got: %v", remaining)
	}

	// Try non-existent user
	status, _ = makeRequest(app, "GET", "/api/totp/time/other@example.com", nil)
	if status != fiber.StatusInternalServerError {
		t.Errorf("Status for non-existent user = %v, want %v", status, fiber.StatusInternalServerError)
	}

	// Cleanup
	_ = manager.DeleteUser("user@example.com")
}

func TestMiddlewareAuth(t *testing.T) {
	store := newMockCredentialStore()
	manager := NewManager(store, "test-issuer")
	handler := NewHandler(manager)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Setup: Enroll a user and get valid code
	_, err := manager.EnrollUser("user@example.com")
	if err != nil {
		t.Fatalf("Setup enrollment failed: %v", err)
	}

	validCode, err := manager.GenerateCurrentCode("user@example.com")
	if err != nil {
		t.Fatalf("Generate code failed: %v", err)
	}

	// Create test route with middleware
	extractor := (&AccountExtractor{}).FromHeader
	app.Get("/protected", handler.AuthMiddleware(extractor), func(c *fiber.Ctx) error {
		return c.SendString("protected content")
	})

	tests := []struct {
		name       string
		headerKey  string
		headerVal  string
		wantStatus int
	}{
		{
			name:       "valid code",
			headerKey:  "X-Account-Name",
			headerVal:  "user@example.com",
			wantStatus: fiber.StatusUnauthorized, // Missing X-TOTP-Code header
		},
		{
			name:       "missing account",
			headerKey:  "X-TOTP-Code",
			headerVal:  validCode,
			wantStatus: fiber.StatusUnauthorized,
		},
		{
			name:       "invalid code",
			headerKey:  "X-TOTP-Code",
			headerVal:  "000000",
			wantStatus: fiber.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := fiber.NewRequest("GET", "/protected", nil)
			if tt.headerKey != "" {
				req.Header.Set(tt.headerKey, tt.headerVal)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("Test request failed: %v", err)
			}

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Status = %v, want %v", resp.StatusCode, tt.wantStatus)
			}
		})
	}

	// Cleanup
	_ = manager.DeleteUser("user@example.com")
}
