# TOTP Authentication Provider

This package provides a comprehensive TOTP (Time-based One-Time Password) authentication provider for TUNNEL.

## Features

- **TOTP Generation & Validation**: Full RFC 6238 compliant TOTP implementation
- **QR Code Generation**: Automatic QR code generation for easy enrollment
- **Credential Storage Integration**: Seamless integration with TUNNEL's credential store
- **Time Synchronization**: Built-in time drift detection and handling
- **HTTP API Handlers**: Ready-to-use Fiber web handlers
- **Authentication Middleware**: Middleware for protecting routes with TOTP
- **Enrollment Management**: Complete enrollment lifecycle management

## Architecture

### Core Components

1. **Provider** (`totp.go`): Core TOTP operations
   - Secret generation
   - Code generation and validation
   - Time synchronization checks

2. **QR Code Generator** (`qrcode.go`): QR code generation for enrollment
   - PNG QR code generation
   - Base64 encoding for JSON responses
   - Enrollment bundle creation

3. **Manager** (`manager.go`): Business logic layer
   - Credential storage integration
   - User enrollment management
   - Code validation with storage

4. **Handlers** (`handlers.go`): HTTP API endpoints
   - Enrollment endpoint
   - Validation endpoint
   - Status and management endpoints

5. **Middleware** (`middleware.go`): Authentication middleware
   - Required TOTP authentication
   - Optional TOTP authentication
   - WebSocket support

## Usage

### Basic Setup

```go
import (
    "github.com/jedarden/tunnel/internal/auth/totp"
    "github.com/jedarden/tunnel/internal/core"
)

// Create credential store
store, err := core.NewCredentialStore("keyring", "tunnel", "", "")
if err != nil {
    log.Fatal(err)
}

// Create TOTP manager
manager := totp.NewManager(store, "TUNNEL")

// Create HTTP handler
handler := totp.NewHandler(manager)
```

### HTTP API Integration

```go
import "github.com/gofiber/fiber/v2"

app := fiber.New()

// Register TOTP routes
api := app.Group("/api")
handler.RegisterRoutes(api)

// Optional: Protect routes with TOTP middleware
accountExtractor := (&totp.AccountExtractor{}).FromHeader
protected := app.Group("/protected", handler.AuthMiddleware(accountExtractor))
```

### API Endpoints

#### POST `/api/totp/enroll`
Enroll a user in TOTP authentication.

**Request:**
```json
{
  "account_name": "user@example.com"
}
```

**Response:**
```json
{
  "secret": "JBSWY3DPEHPK3PXP",
  "qr_code": "data:image/png;base64,...",
  "url": "otpauth://totp/TUNNEL:user@example.com?...",
  "issuer": "TUNNEL",
  "account": "user@example.com",
  "period": 30,
  "digits": 6,
  "algorithm": "SHA1"
}
```

#### POST `/api/totp/validate`
Validate a TOTP code.

**Request:**
```json
{
  "account_name": "user@example.com",
  "code": "123456",
  "window": 1
}
```

**Response:**
```json
{
  "valid": true,
  "message": "Valid code",
  "remaining": 25
}
```

#### GET `/api/totp/status/:account`
Check if a user is enrolled in TOTP.

**Response:**
```json
{
  "enrolled": true,
  "message": "User is enrolled in TOTP"
}
```

#### DELETE `/api/totp/:account`
Remove a user's TOTP enrollment.

#### POST `/api/totp/regenerate/:account`
Regenerate a user's TOTP secret (useful if authenticator is lost).

#### GET `/api/totp/code/:account`
Get the current valid TOTP code (for testing/debugging).

#### GET `/api/totp/time/:account`
Get seconds remaining in the current TOTP window.

#### GET `/api/totp/users`
List all enrolled users.

**Response:**
```json
{
  "users": ["user1@example.com", "user2@example.com"],
  "count": 2
}
```

### Middleware Usage

#### Required TOTP Authentication

```go
// All routes in this group require valid TOTP
accountExtractor := (&totp.AccountExtractor{}).FromHeader
protected := app.Group("/api", handler.AuthMiddleware(accountExtractor))
```

#### Optional TOTP Authentication

```go
// Routes work with or without TOTP
accountExtractor := (&totp.AccountExtractor{}).FromQuery
app.Use(handler.OptionalAuthMiddleware(accountExtractor))

// In your handler, check if TOTP was validated
if c.Locals("totp_authenticated") == true {
    // User provided valid TOTP
}
```

#### Account Extraction Strategies

```go
extractor := &totp.AccountExtractor{}

// From X-Account-Name header
app.Use(handler.AuthMiddleware(extractor.FromHeader))

// From query parameter
app.Use(handler.AuthMiddleware(extractor.FromQuery))

// From Basic Auth username
app.Use(handler.AuthMiddleware(extractor.FromBasicAuth))

// From Bearer token (simplified)
app.Use(handler.AuthMiddleware(extractor.FromBearerToken))

// Custom extractor
app.Use(handler.AuthMiddleware(extractor.Custom(func(c *fiber.Ctx) string {
    return c.Locals("user_id")
})))
```

## Configuration

### Time Window

The default time window is 1 (allows 1 period before and after). Configure in validation:

```go
// More lenient - allows 2 periods before/after
valid, err := manager.ValidateCode(account, code, 2)

// Strict - only current period
valid, err := manager.ValidateCode(account, code, 0)
```

### Time Synchronization

Built-in time sync checks prevent issues with clock drift:

```go
// Automatically checks time sync
valid, err := manager.ValidateWithTimeCheck(account, code, 1)
```

## Testing

Run the comprehensive test suite:

```bash
cd internal/auth/totp
go test -v ./...
```

Tests cover:
- Secret generation and validation
- QR code generation
- Enrollment lifecycle
- Credential storage integration
- Time synchronization
- API handlers
- Middleware authentication

## Security Considerations

1. **Secret Storage**: TOTP secrets are stored using TUNNEL's credential store (keyring, encrypted file, or env)
2. **QR Code Transmission**: QR codes are base64-encoded in JSON responses
3. **Rate Limiting**: Consider implementing rate limiting on validation endpoints
4. **Time Drift**: The default window of 1 allows for minor clock skew (±30 seconds)
5. **Backup Codes**: Consider implementing backup codes for account recovery

## Integration with TUNNEL Config

The TOTP provider integrates with TUNNEL's configuration system:

```yaml
methods:
  totp:
    enabled: true
    priority: 70
    auth_key_ref: "tunnel:totp-secret"
    settings:
      window: "1"
      period: "30"
```

## Dependencies

- `github.com/pquerna/otp`: TOTP generation and validation
- `github.com/skip2/go-qrcode`: QR code generation
- `github.com/gofiber/fiber/v2`: HTTP framework

## Future Enhancements

- Backup codes generation
- Hardware token support (YubiKey)
- Rate limiting on validation
- Audit logging for TOTP events
- Per-user time window configuration
- TOTP statistics and monitoring
