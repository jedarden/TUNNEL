package fido2

import (
	"github.com/gofiber/fiber/v2"
)

// Handler provides HTTP handlers for FIDO2 operations
type Handler struct {
	manager *Manager
}

// NewHandler creates a new FIDO2 HTTP handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegistrationBeginRequest represents a FIDO2 registration begin request
type RegistrationBeginRequest struct {
	AccountName string `json:"account_name"`
}

// AuthenticationBeginRequest represents a FIDO2 authentication begin request
type AuthenticationBeginRequest struct {
	AccountName string `json:"account_name"`
}

// RegistrationBeginResponse represents a FIDO2 registration begin response
type RegistrationBeginResponse struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Challenge   string `json:"challenge"`
	Options     string `json:"options"`
}

// AuthenticationBeginResponse represents a FIDO2 authentication begin response
type AuthenticationBeginResponse struct {
	Username  string `json:"username"`
	Challenge string `json:"challenge"`
	Options   string `json:"options"`
}

// CredentialResponse represents a stored credential response
type CredentialResponse struct {
	ID            string              `json:"id"`
	Type          string              `json:"type"`
	Attestation   string              `json:"attestation_type"`
	AAGUID        string              `json:"aaguid"`
	SignCount     uint32              `json:"sign_count"`
	PublicKey     string              `json:"public_key"`
	Transport     []string            `json:"transport,omitempty"`
	Flags         []string            `json:"flags,omitempty"`
	RegisteredAt  string              `json:"registered_at"`
	LastUsedAt    string              `json:"last_used_at"`
	Metadata      map[string]string   `json:"metadata,omitempty"`
}

// UserCredentialsResponse represents user credentials response
type UserCredentialsResponse struct {
	Username    string              `json:"username"`
	DisplayName string              `json:"display_name"`
	UserID      string              `json:"user_id"`
	CreatedAt   string              `json:"created_at"`
	Credentials []CredentialResponse `json:"credentials"`
}

// AssertionResponse represents a completed authentication assertion
type AssertionResponse struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	SignCount  uint32 `json:"sign_count"`
	UserHandle string `json:"user_handle"`
	Verified   bool   `json:"verified"`
}

// StatusResponse represents FIDO2 status response
type StatusResponse struct {
	Enabled     bool   `json:"enabled"`
	Credentials int    `json:"credentials"`
	Message     string `json:"message,omitempty"`
}

// RegisterRoutes registers FIDO2 routes with the Fiber app
func (h *Handler) RegisterRoutes(api fiber.Router) {
	fido2 := api.Group("/fido2")
	{
		// Registration flow
		fido2.Post("/register/begin", h.handleRegistrationBegin)
		fido2.Post("/register/finish", h.handleRegistrationFinish)

		// Authentication flow
		fido2.Post("/authenticate/begin", h.handleAuthenticationBegin)
		fido2.Post("/authenticate/finish", h.handleAuthenticationFinish)

		// Credential management
		fido2.Get("/credentials/:account", h.handleGetCredentials)
		fido2.Delete("/credentials/:account/:credential_id", h.handleDeleteCredential)
		fido2.Get("/status/:account", h.handleStatus)

		// User management
		fido2.Get("/users", h.handleListUsers)
		fido2.Delete("/:account", h.handleDeleteUser)
	}
}

// handleRegistrationBegin handles FIDO2 registration begin
func (h *Handler) handleRegistrationBegin(c *fiber.Ctx) error {
	var req RegistrationBeginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.AccountName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account_name is required",
		})
	}

	// Begin registration ceremony
	regReq, err := h.manager.BeginRegistration(req.AccountName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to begin registration",
		})
	}

	return c.JSON(RegistrationBeginResponse{
		UserID:      string(regReq.User.WebAuthnID()),
		Username:    regReq.User.WebAuthnName(),
		DisplayName: regReq.User.WebAuthnDisplayName(),
		Challenge:   regReq.Challenge,
		Options:     regReq.JSON,
	})
}

// handleRegistrationFinish handles FIDO2 registration finish
func (h *Handler) handleRegistrationFinish(c *fiber.Ctx) error {
	accountName := c.Query("account")
	if accountName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account query parameter is required",
		})
	}

	// Finish registration ceremony
	cred, err := h.manager.FinishRegistration(accountName, c.Context().Request)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to finish registration",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(CredentialResponse{
		ID:           cred.ID,
		Type:         cred.Type,
		Attestation:  cred.Attestation,
		AAGUID:       cred.AAGUID,
		SignCount:    cred.SignCount,
		PublicKey:    cred.PublicKey,
		Transport:    cred.Transport,
		Flags:        cred.Flags,
		RegisteredAt: cred.RegisteredAt.Format("2006-01-02T15:04:05Z"),
		LastUsedAt:   cred.LastUsedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// handleAuthenticationBegin handles FIDO2 authentication begin
func (h *Handler) handleAuthenticationBegin(c *fiber.Ctx) error {
	var req AuthenticationBeginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.AccountName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account_name is required",
		})
	}

	// Begin authentication ceremony
	authReq, err := h.manager.BeginAuthentication(req.AccountName)
	if err != nil {
		if err.Error() == "user has no registered credentials" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "No credentials registered for this user",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to begin authentication",
		})
	}

	return c.JSON(AuthenticationBeginResponse{
		Username:  authReq.User.WebAuthnName(),
		Challenge: authReq.Challenge,
		Options:   authReq.JSON,
	})
}

// handleAuthenticationFinish handles FIDO2 authentication finish
func (h *Handler) handleAuthenticationFinish(c *fiber.Ctx) error {
	accountName := c.Query("account")
	if accountName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account query parameter is required",
		})
	}

	// Finish authentication ceremony
	assertion, err := h.manager.FinishAuthentication(accountName, c.Context().Request)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Authentication failed",
		})
	}

	return c.JSON(AssertionResponse{
		ID:         assertion.ID,
		Type:       assertion.Type,
		SignCount:  assertion.SignCount,
		UserHandle: assertion.UserHandle,
		Verified:   assertion.Verified,
	})
}

// handleGetCredentials retrieves all credentials for a user
func (h *Handler) handleGetCredentials(c *fiber.Ctx) error {
	account := c.Params("account")
	if account == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account parameter is required",
		})
	}

	userCreds, err := h.manager.GetUserCredentials(account)
	if err != nil {
		if err.Error() == "user not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve credentials",
		})
	}

	// Convert credentials to response format
	credResponses := make([]CredentialResponse, len(userCreds.Credentials))
	for i, cred := range userCreds.Credentials {
		credResponses[i] = CredentialResponse{
			ID:           cred.ID,
			Type:         cred.Type,
			Attestation:  cred.Attestation,
			AAGUID:       cred.AAGUID,
			SignCount:    cred.SignCount,
			PublicKey:    cred.PublicKey,
			Transport:    cred.Transport,
			Flags:        cred.Flags,
			RegisteredAt: cred.RegisteredAt.Format("2006-01-02T15:04:05Z"),
			LastUsedAt:   cred.LastUsedAt.Format("2006-01-02T15:04:05Z"),
			Metadata:     cred.Metadata,
		}
	}

	return c.JSON(UserCredentialsResponse{
		Username:    userCreds.Metadata.Username,
		DisplayName: userCreds.Metadata.DisplayName,
		UserID:      userCreds.Metadata.UserID,
		CreatedAt:   userCreds.Metadata.CreatedAt.Format("2006-01-02T15:04:05Z"),
		Credentials: credResponses,
	})
}

// handleDeleteCredential removes a specific credential for a user
func (h *Handler) handleDeleteCredential(c *fiber.Ctx) error {
	account := c.Params("account")
	if account == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account parameter is required",
		})
	}

	credentialID := c.Params("credential_id")
	if credentialID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "credential_id parameter is required",
		})
	}

	if err := h.manager.DeleteCredential(account, credentialID); err != nil {
		if err.Error() == "credential not found" {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Credential not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete credential",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "Credential deleted successfully",
	})
}

// handleStatus checks FIDO2 status for a user
func (h *Handler) handleStatus(c *fiber.Ctx) error {
	account := c.Params("account")
	if account == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account parameter is required",
		})
	}

	exists, err := h.manager.UserExists(account)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check user status",
		})
	}

	if !exists {
		return c.Status(fiber.StatusNotFound).JSON(StatusResponse{
			Enabled:     false,
			Credentials: 0,
			Message:     "User not found",
		})
	}

	userCreds, err := h.manager.GetUserCredentials(account)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve user credentials",
		})
	}

	message := "User has registered credentials"
	if len(userCreds.Credentials) == 0 {
		message = "User exists but has no registered credentials"
	}

	return c.JSON(StatusResponse{
		Enabled:     true,
		Credentials: len(userCreds.Credentials),
		Message:     message,
	})
}

// handleListUsers returns all users with FIDO2 enabled
func (h *Handler) handleListUsers(c *fiber.Ctx) error {
	users, err := h.manager.ListUsers()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list users",
		})
	}

	return c.JSON(fiber.Map{
		"users": users,
		"count": len(users),
	})
}

// handleDeleteUser removes a user and all their credentials
func (h *Handler) handleDeleteUser(c *fiber.Ctx) error {
	account := c.Params("account")
	if account == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account parameter is required",
		})
	}

	if err := h.manager.DeleteUser(account); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete user",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "User deleted successfully",
	})
}
