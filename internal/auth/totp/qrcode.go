package totp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	"github.com/pquerna/otp"
	"github.com/skip2/go-qrcode"
)

// QRCode represents a generated QR code with metadata
type QRCode struct {
	Data     string `json:"data"`     // Base64 encoded PNG image
	URL      string `json:"url"`      // otpauth:// URL
	Width    int    `json:"width"`    // QR code width in pixels
	Height   int    `json:"height"`   // QR code height in pixels
	Secret   string `json:"secret"`   // TOTP secret
	Accounts string `json:"accounts"` // Account name
}

// GenerateQRCode generates a QR code for TOTP enrollment
func (p *Provider) GenerateQRCode(accountName, secret string, size int) (*QRCode, error) {
	if size < 100 || size > 1024 {
		size = 256 // Default size
	}

	// Generate otpauth URL
	authURL := p.GenerateURL(accountName, secret)

	// Generate QR code
	qrCode, err := qrcode.New(authURL, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("create qrcode: %w", err)
	}

	// Convert to PNG
	var buf bytes.Buffer
	if err := qrCode.Write(size, &buf); err != nil {
		return nil, fmt.Errorf("write qrcode: %w", err)
	}

	// Encode to base64
	data := base64.StdEncoding.EncodeToString(buf.Bytes())

	return &QRCode{
		Data:     data,
		URL:      authURL,
		Width:    size,
		Height:   size,
		Secret:   secret,
		Accounts: accountName,
	}, nil
}

// GenerateQRCodePNG generates a QR code and returns the PNG bytes
func (p *Provider) GenerateQRCodePNG(accountName, secret string, size int) ([]byte, error) {
	if size < 100 || size > 1024 {
		size = 256 // Default size
	}

	// Generate otpauth URL
	authURL := p.GenerateURL(accountName, secret)

	// Generate QR code
	qrCode, err := qrcode.New(authURL, qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("create qrcode: %w", err)
	}

	// Convert to PNG
	var buf bytes.Buffer
	if err := qrCode.Write(size, &buf); err != nil {
		return nil, fmt.Errorf("write qrcode: %w", err)
	}

	return buf.Bytes(), nil
}

// GenerateQRCodeFromKey generates a QR code from an OTP key
func (p *Provider) GenerateQRCodeFromKey(key *otp.Key, size int) (*QRCode, error) {
	if size < 100 || size > 1024 {
		size = 256 // Default size
	}

	// Generate QR code from key's URL
	qrCode, err := qrcode.New(key.URL(), qrcode.Medium)
	if err != nil {
		return nil, fmt.Errorf("create qrcode: %w", err)
	}

	// Convert to PNG
	var buf bytes.Buffer
	if err := qrCode.Write(size, &buf); err != nil {
		return nil, fmt.Errorf("write qrcode: %w", err)
	}

	// Encode to base64
	data := base64.StdEncoding.EncodeToString(buf.Bytes())

	return &QRCode{
		Data:     data,
		URL:      key.URL(),
		Width:    size,
		Height:   size,
		Secret:   key.Secret(),
		Accounts: key.AccountName(),
	}, nil
}

// GenerateEnrollmentBundle creates a complete enrollment bundle
// This includes the secret, QR code, and setup instructions
func (p *Provider) GenerateEnrollmentBundle(accountName string) (*EnrollmentBundle, error) {
	secret, err := p.GenerateSecret()
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	qrCode, err := p.GenerateQRCode(accountName, secret, 256)
	if err != nil {
		return nil, fmt.Errorf("generate qr code: %w", err)
	}

	return &EnrollmentBundle{
		Secret:   secret,
		QRCode:   qrCode,
		URL:      qrCode.URL,
		Issuer:   p.issuer,
		Account:  accountName,
		Period:   30,     // 30 seconds
		Digits:   6,      // 6 digits
		Algorithm: "SHA1", // SHA1
	}, nil
}

// EnrollmentBundle contains everything needed for TOTP enrollment
type EnrollmentBundle struct {
	Secret    string   `json:"secret"`             // Base32 encoded secret
	QRCode    *QRCode  `json:"qr_code"`            // QR code data
	URL       string   `json:"url"`                // otpauth:// URL
	Issuer    string   `json:"issuer"`             // Issuer name
	Account   string   `json:"account"`            // Account name
	Period    int      `json:"period"`             // Time period in seconds
	Digits    int      `json:"digits"`             // Number of digits
	Algorithm string   `json:"algorithm"`          // Algorithm (SHA1, SHA256, etc.)
	Backups   []string `json:"backup_codes,omitempty"` // Backup codes (optional)
}

// ValidateBackupCodes validates backup codes
// Returns true if the code matches and removes it from the list
func (b *EnrollmentBundle) ValidateBackupCodes(code string) bool {
	for i, backup := range b.Backups {
		if backup == code {
			// Remove used backup code
			b.Backups = append(b.Backups[:i], b.Backups[i+1:]...)
			return true
		}
	}
	return false
}
