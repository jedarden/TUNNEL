package totp

import (
	"github.com/gofiber/fiber/v2"
)

// Handler provides HTTP handlers for TOTP operations
type Handler struct {
	manager *Manager
}

// NewHandler creates a new TOTP HTTP handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// EnrollmentRequest represents a TOTP enrollment request
type EnrollmentRequest struct {
	AccountName string `json:"account_name"`
}

// ValidationRequest represents a TOTP validation request
type ValidationRequest struct {
	AccountName string `json:"account_name"`
	Code        string `json:"code"`
	Window      int    `json:"window,omitempty"` // Optional, defaults to 1
}

// EnrollmentResponse represents a TOTP enrollment response
type EnrollmentResponse struct {
	Secret    string `json:"secret"`
	QRCode    string `json:"qr_code"` // Base64 encoded PNG
	URL       string `json:"url"`
	Issuer    string `json:"issuer"`
	Account   string `json:"account"`
	Period    int    `json:"period"`
	Digits    int    `json:"digits"`
	Algorithm string `json:"algorithm"`
}

// ValidationResponse represents a TOTP validation response
type ValidationResponse struct {
	Valid      bool   `json:"valid"`
	Message     string `json:"message,omitempty"`
	Remaining  int    `json:"remaining,omitempty"` // Seconds remaining in current window
}

// StatusResponse represents TOTP status response
type StatusResponse struct {
	Enrolled bool   `json:"enrolled"`
	Message  string `json:"message,omitempty"`
}

// RegisterRoutes registers TOTP routes with the Fiber app
func (h *Handler) RegisterRoutes(api fiber.Router) {
	totp := api.Group("/totp")
	{
		totp.Post("/enroll", h.handleEnroll)
		totp.Post("/validate", h.handleValidate)
		totp.Get("/status/:account", h.handleStatus)
		totp.Delete("/:account", h.handleDelete)
		totp.Post("/regenerate/:account", h.handleRegenerate)
		totp.Get("/code/:account", h.handleGetCurrentCode)
		totp.Get("/time/:account", h.handleGetTimeRemaining)
		totp.Get("/users", h.handleListUsers)
	}
}

// handleEnroll handles TOTP enrollment
func (h *Handler) handleEnroll(c *fiber.Ctx) error {
	var req EnrollmentRequest
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

	// Check if already enrolled
	enrolled, err := h.manager.IsEnrolled(req.AccountName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check enrollment status",
		})
	}

	if enrolled {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "User already enrolled",
		})
	}

	// Enroll user
	bundle, err := h.manager.EnrollUser(req.AccountName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to enroll user",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(EnrollmentResponse{
		Secret:    bundle.Secret,
		QRCode:    bundle.QRCode.Data,
		URL:       bundle.URL,
		Issuer:    bundle.Issuer,
		Account:   bundle.Account,
		Period:    bundle.Period,
		Digits:    bundle.Digits,
		Algorithm: bundle.Algorithm,
	})
}

// handleValidate handles TOTP code validation
func (h *Handler) handleValidate(c *fiber.Ctx) error {
	var req ValidationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.AccountName == "" || req.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account_name and code are required",
		})
	}

	// Default window to 1 if not provided
	window := req.Window
	if window == 0 {
		window = 1
	}

	// Validate code
	valid, err := h.manager.ValidateCode(req.AccountName, req.Code, window)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to validate code",
		})
	}

	// Get time remaining
	remaining, _ := h.manager.GetTimeRemaining(req.AccountName)

	message := "Invalid code"
	if valid {
		message = "Valid code"
	}

	return c.JSON(ValidationResponse{
		Valid:     valid,
		Message:   message,
		Remaining: remaining,
	})
}

// handleStatus checks if a user is enrolled in TOTP
func (h *Handler) handleStatus(c *fiber.Ctx) error {
	account := c.Params("account")
	if account == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account parameter is required",
		})
	}

	enrolled, err := h.manager.IsEnrolled(account)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to check enrollment status",
		})
	}

	message := "User is not enrolled in TOTP"
	if enrolled {
		message = "User is enrolled in TOTP"
	}

	return c.JSON(StatusResponse{
		Enrolled: enrolled,
		Message:  message,
	})
}

// handleDelete removes a user's TOTP enrollment
func (h *Handler) handleDelete(c *fiber.Ctx) error {
	account := c.Params("account")
	if account == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account parameter is required",
		})
	}

	if err := h.manager.DeleteUser(account); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete enrollment",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"message": "TOTP enrollment deleted successfully",
	})
}

// handleRegenerate regenerates a user's TOTP secret
func (h *Handler) handleRegenerate(c *fiber.Ctx) error {
	account := c.Params("account")
	if account == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account parameter is required",
		})
	}

	bundle, err := h.manager.RegenerateSecret(account)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to regenerate secret",
		})
	}

	return c.JSON(EnrollmentResponse{
		Secret:    bundle.Secret,
		QRCode:    bundle.QRCode.Data,
		URL:       bundle.URL,
		Issuer:    bundle.Issuer,
		Account:   bundle.Account,
		Period:    bundle.Period,
		Digits:    bundle.Digits,
		Algorithm: bundle.Algorithm,
	})
}

// handleGetCurrentCode returns the current valid TOTP code
func (h *Handler) handleGetCurrentCode(c *fiber.Ctx) error {
	account := c.Params("account")
	if account == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account parameter is required",
		})
	}

	code, err := h.manager.GenerateCurrentCode(account)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate current code",
		})
	}

	return c.JSON(fiber.Map{
		"code": code,
	})
}

// handleGetTimeRemaining returns the seconds remaining in the current TOTP window
func (h *Handler) handleGetTimeRemaining(c *fiber.Ctx) error {
	account := c.Params("account")
	if account == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "account parameter is required",
		})
	}

	remaining, err := h.manager.GetTimeRemaining(account)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get time remaining",
		})
	}

	return c.JSON(fiber.Map{
		"remaining": remaining,
	})
}

// handleListUsers returns all enrolled users
func (h *Handler) handleListUsers(c *fiber.Ctx) error {
	users, err := h.manager.ListEnrolledUsers()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list enrolled users",
		})
	}

	return c.JSON(fiber.Map{
		"users": users,
		"count": len(users),
	})
}
