package totp

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/url"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// Provider handles TOTP generation and validation
type Provider struct {
	issuer string
}

// NewProvider creates a new TOTP provider
func NewProvider(issuer string) *Provider {
	if issuer == "" {
		issuer = "TUNNEL"
	}
	return &Provider{
		issuer: issuer,
	}
}

// GenerateSecret creates a new TOTP secret
func (p *Provider) GenerateSecret() (string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return base32.StdEncoding.EncodeToString(secret), nil
}

// GenerateCode generates a TOTP code for a given secret
func (p *Provider) GenerateCode(secret string) (string, error) {
	return totp.GenerateCode(secret, time.Now())
}

// ValidateCode validates a TOTP code against a secret
func (p *Provider) ValidateCode(secret, code string) (bool, error) {
	return totp.Validate(code, secret)
}

// ValidateCodeWithWindow validates a TOTP code with a custom time window
func (p *Provider) ValidateCodeWithWindow(secret, code string, window int) (bool, error) {
	opts := totp.ValidateOpts{
		Period: uint(30), // 30 second period
		Skew:   uint(window),
	}
	return totp.ValidateCustom(code, secret, time.Now(), opts)
}

// GenerateURL creates an otpauth:// URL for QR code generation
func (p *Provider) GenerateURL(accountName, secret string) string {
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", p.issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")

	return fmt.Sprintf("otpauth://totp/%s:%s?%s", p.issuer, accountName, v.Encode())
}

// GetKey returns a TOTP key object for QR code generation
func (p *Provider) GetKey(accountName, secret string) (*otp.Key, error) {
	url := p.GenerateURL(accountName, secret)
	return otp.NewKeyFromURL(url)
}

// TimeSyncCheck checks if system time is within acceptable bounds
// Returns true if time is within 5 minutes of expected
func (p *Provider) TimeSyncCheck(referenceTime time.Time) bool {
	now := time.Now()
	diff := now.Sub(referenceTime)

	// Allow 5 minute drift
	maxDrift := 5 * time.Minute
	return diff >= -maxDrift && diff <= maxDrift
}

// GetCurrentWindow returns the current time window number for a TOTP
func (p *Provider) GetCurrentWindow(secret string) (int64, error) {
	key, err := otp.NewKeyFromURL(p.GenerateURL("user", secret))
	if err != nil {
		return 0, err
	}

	return totp.CodeCounter(key.Period(), time.Now()), nil
}
