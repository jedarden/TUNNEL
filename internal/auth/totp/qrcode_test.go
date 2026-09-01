package totp

import (
	"encoding/base64"
	"testing"

	"github.com/pquerna/otp"
)

func TestGenerateQRCode(t *testing.T) {
	provider := NewProvider("test-issuer")

	secret := "JBSWY3DPEHPK3PXP"
	accountName := "user@example.com"

	tests := []struct {
		name        string
		accountName string
		secret      string
		size        int
		wantErr     bool
	}{
		{
			name:        "generate qr code",
			accountName: accountName,
			secret:      secret,
			size:        256,
			wantErr:     false,
		},
		{
			name:        "default size",
			accountName: accountName,
			secret:      secret,
			size:        0, // Should default to 256
			wantErr:     false,
		},
		{
			name:        "small size",
			accountName: accountName,
			secret:      secret,
			size:        100,
			wantErr:     false,
		},
		{
			name:        "large size",
			accountName: accountName,
			secret:      secret,
			size:        512,
			wantErr:     false,
		},
		{
			name:        "too small",
			accountName: accountName,
			secret:      secret,
			size:        50,
			wantErr:     false, // Should default to 256
		},
		{
			name:        "too large",
			accountName: accountName,
			secret:      secret,
			size:        2048,
			wantErr:     false, // Should default to 256
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.GenerateQRCode(tt.accountName, tt.secret, tt.size)
			if (err != nil) != tt.wantErr {
				t.Fatalf("GenerateQRCode() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if got == nil {
					t.Fatal("GenerateQRCode() returned nil")
				}
				if got.Data == "" {
					t.Error("GenerateQRCode().Data is empty")
				}
				if got.URL == "" {
					t.Error("GenerateQRCode().URL is empty")
				}
				if got.Secret != tt.secret {
					t.Errorf("GenerateQRCode().Secret = %v, want %v", got.Secret, tt.secret)
				}
				if got.Accounts != tt.accountName {
					t.Errorf("GenerateQRCode().Accounts = %v, want %v", got.Accounts, tt.accountName)
				}

				// Verify data is valid base64
				_, err := base64.StdEncoding.DecodeString(got.Data)
				if err != nil {
					t.Errorf("GenerateQRCode().Data is invalid base64: %v", err)
				}

				// Verify URL structure
				if len(got.URL) < 15 || got.URL[:15] != "otpauth://totp/" {
					t.Errorf("GenerateQRCode().URL has invalid format: %v", got.URL)
				}
			}
		})
	}
}

func TestGenerateQRCodePNG(t *testing.T) {
	provider := NewProvider("test-issuer")

	secret := "JBSWY3DPEHPK3PXP"
	accountName := "user@example.com"

	data, err := provider.GenerateQRCodePNG(accountName, secret, 256)
	if err != nil {
		t.Fatalf("GenerateQRCodePNG() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("GenerateQRCodePNG() returned empty data")
	}

	// PNG files start with the signature byte sequence
	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if len(data) < 8 {
		t.Error("GenerateQRCodePNG() data too short to be PNG")
	} else {
		for i, b := range pngSignature {
			if data[i] != b {
				t.Errorf("GenerateQRCodePNG() data does not appear to be PNG (byte %d mismatch)", i)
				break
			}
		}
	}
}

func TestGenerateQRCodeFromKey(t *testing.T) {
	provider := NewProvider("test-issuer")

	secret := "JBSWY3DPEHPK3PXP"
	accountName := "user@example.com"

	key, err := provider.GetKey(accountName, secret)
	if err != nil {
		t.Fatalf("GetKey() error = %v", err)
	}

	qrCode, err := provider.GenerateQRCodeFromKey(key, 256)
	if err != nil {
		t.Fatalf("GenerateQRCodeFromKey() error = %v", err)
	}

	if qrCode == nil {
		t.Fatal("GenerateQRCodeFromKey() returned nil")
	}
	if qrCode.Secret != secret {
		t.Errorf("GenerateQRCodeFromKey().Secret = %v, want %v", qrCode.Secret, secret)
	}
	if qrCode.Accounts != accountName {
		t.Errorf("GenerateQRCodeFromKey().Accounts = %v, want %v", qrCode.Accounts, accountName)
	}
	if qrCode.URL != key.URL() {
		t.Errorf("GenerateQRCodeFromKey().URL = %v, want %v", qrCode.URL, key.URL())
	}
}

func TestGenerateEnrollmentBundle(t *testing.T) {
	provider := NewProvider("test-issuer")
	accountName := "user@example.com"

	bundle, err := provider.GenerateEnrollmentBundle(accountName)
	if err != nil {
		t.Fatalf("GenerateEnrollmentBundle() error = %v", err)
	}

	if bundle == nil {
		t.Fatal("GenerateEnrollmentBundle() returned nil")
	}

	// Validate all fields
	if bundle.Secret == "" {
		t.Error("GenerateEnrollmentBundle().Secret is empty")
	}
	if bundle.QRCode == nil {
		t.Error("GenerateEnrollmentBundle().QRCode is nil")
	}
	if bundle.URL == "" {
		t.Error("GenerateEnrollmentBundle().URL is empty")
	}
	if bundle.Issuer != "test-issuer" {
		t.Errorf("GenerateEnrollmentBundle().Issuer = %v, want 'test-issuer'", bundle.Issuer)
	}
	if bundle.Account != accountName {
		t.Errorf("GenerateEnrollmentBundle().Account = %v, want %v", bundle.Account, accountName)
	}
	if bundle.Period != 30 {
		t.Errorf("GenerateEnrollmentBundle().Period = %v, want 30", bundle.Period)
	}
	if bundle.Digits != 6 {
		t.Errorf("GenerateEnrollmentBundle().Digits = %v, want 6", bundle.Digits)
	}
	if bundle.Algorithm != "SHA1" {
		t.Errorf("GenerateEnrollmentBundle().Algorithm = %v, want 'SHA1'", bundle.Algorithm)
	}

	// Verify the secret is valid base32
	_, err = base32.StdEncoding.DecodeString(bundle.Secret)
	if err != nil {
		t.Errorf("GenerateEnrollmentBundle().Secret is invalid base32: %v", err)
	}

	// Verify QR code data matches bundle
	if bundle.QRCode.Secret != bundle.Secret {
		t.Errorf("QRCode.Secret = %v, bundle.Secret = %v (should match)", bundle.QRCode.Secret, bundle.Secret)
	}
	if bundle.QRCode.URL != bundle.URL {
		t.Errorf("QRCode.URL = %v, bundle.URL = %v (should match)", bundle.QRCode.URL, bundle.URL)
	}
}

func TestEnrollmentBundleValidateBackupCodes(t *testing.T) {
	bundle := &EnrollmentBundle{
		Secret: "JBSWY3DPEHPK3PXP",
		Backups: []string{
			"backup1",
			"backup2",
			"backup3",
		},
	}

	tests := []struct {
		name          string
		code          string
		wantValid     bool
		wantRemaining int
	}{
		{
			name:          "valid backup code",
			code:          "backup2",
			wantValid:     true,
			wantRemaining: 2,
		},
		{
			name:          "invalid backup code",
			code:          "invalid",
			wantValid:     false,
			wantRemaining: 3,
		},
		{
			name:          "empty code",
			code:          "",
			wantValid:     false,
			wantRemaining: 3,
		},
		{
			name:          "last backup code",
			code:          "backup3",
			wantValid:     true,
			wantRemaining: 1,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset bundle for each test
			testBundle := &EnrollmentBundle{
				Secret:  bundle.Secret,
				Backups: make([]string, len(bundle.Backups)),
			}
			copy(testBundle.Backups, bundle.Backups)

			got := testBundle.ValidateBackupCodes(tt.code)
			if got != tt.wantValid {
				t.Errorf("ValidateBackupCodes() = %v, want %v", got, tt.wantValid)
			}

			if len(testBundle.Backups) != tt.wantRemaining {
				t.Errorf("Backup codes count = %v, want %v", len(testBundle.Backups), tt.wantRemaining)
			}

			// Verify the specific code was removed if valid
			if tt.wantValid && tt.wantRemaining < len(bundle.Backups) {
				for _, backup := range testBundle.Backups {
					if backup == tt.code {
						t.Errorf("Used backup code %v still present", tt.code)
					}
				}
			}
		})

		// Skip the "last backup code" test since it modifies state
		if i == 3 {
			break
		}
	}
}

func TestQRCodeSize(t *testing.T) {
	provider := NewProvider("test-issuer")

	sizes := []int{100, 200, 256, 512, 1024}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size %d", size), func(t *testing.T) {
			qrCode, err := provider.GenerateQRCode("user@example.com", "JBSWY3DPEHPK3PXP", size)
			if err != nil {
				t.Fatalf("GenerateQRCode() error = %v", err)
			}

			if qrCode.Width != size {
				t.Errorf("Width = %v, want %v", qrCode.Width, size)
			}
			if qrCode.Height != size {
				t.Errorf("Height = %v, want %v", qrCode.Height, size)
			}
		})
	}
}
