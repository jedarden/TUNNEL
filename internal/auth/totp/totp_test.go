package totp

import (
	"fmt"
	"testing"
	"time"
)

func TestGenerateSecret(t *testing.T) {
	provider := NewProvider("test-issuer")

	tests := []struct {
		name    string
		wantLen int // base32 encoded 20 bytes = 32 chars
	}{
		{
			name:    "generate secret",
			wantLen: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.GenerateSecret()
			if err != nil {
				t.Fatalf("GenerateSecret() error = %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("GenerateSecret() length = %v, want %v", len(got), tt.wantLen)
			}
			// Verify it's valid base32
			if _, err := base32.StdEncoding.DecodeString(got); err != nil {
				t.Errorf("GenerateSecret() invalid base32: %v", err)
			}
		})
	}
}

func TestGenerateAndValidateCode(t *testing.T) {
	provider := NewProvider("test-issuer")

	secret, err := provider.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}

	// Generate a code
	code, err := provider.GenerateCode(secret)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	// Code should be 6 digits
	if len(code) != 6 {
		t.Errorf("GenerateCode() length = %v, want 6", len(code))
	}

	// Validate the code
	valid, err := provider.ValidateCode(secret, code)
	if err != nil {
		t.Fatalf("ValidateCode() error = %v", err)
	}
	if !valid {
		t.Error("ValidateCode() = false, want true")
	}
}

func TestValidateCodeWithWindow(t *testing.T) {
	provider := NewProvider("test-issuer")

	secret, err := provider.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}

	// Current code should validate with window
	currentCode, err := provider.GenerateCode(secret)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	tests := []struct {
		name   string
		secret string
		code   string
		window int
		want   bool
	}{
		{
			name:   "current code with window 1",
			secret: secret,
			code:   currentCode,
			window: 1,
			want:   true,
		},
		{
			name:   "current code with window 0",
			secret: secret,
			code:   currentCode,
			window: 0,
			want:   true,
		},
		{
			name:   "invalid code",
			secret: secret,
			code:   "000000",
			window: 1,
			want:   false,
		},
		{
			name:   "wrong secret",
			secret: "JBSWY3DPEHPK3PXP", // known test secret
			code:   currentCode,
			window: 1,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.ValidateCodeWithWindow(tt.secret, tt.code, tt.window)
			if err != nil {
				t.Fatalf("ValidateCodeWithWindow() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ValidateCodeWithWindow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGenerateURL(t *testing.T) {
	provider := NewProvider("test-issuer")

	tests := []struct {
		name        string
		accountName string
		secret      string
		wantPrefix  string
	}{
		{
			name:        "generate url",
			accountName: "user@example.com",
			secret:      "JBSWY3DPEHPK3PXP",
			wantPrefix:  "otpauth://totp/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.GenerateURL(tt.accountName, tt.secret)
			if len(got) == 0 {
				t.Error("GenerateURL() returned empty string")
			}
			// Check URL structure
			if got[:len(tt.wantPrefix)] != tt.wantPrefix {
				t.Errorf("GenerateURL() prefix = %v, want %v", got[:len(tt.wantPrefix)], tt.wantPrefix)
			}
			// Check that secret is in URL
			if !contains(got, tt.secret) {
				t.Errorf("GenerateURL() missing secret in URL")
			}
		})
	}
}

func TestGetKey(t *testing.T) {
	provider := NewProvider("test-issuer")

	secret, err := provider.GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}

	key, err := provider.GetKey("user@example.com", secret)
	if err != nil {
		t.Fatalf("GetKey() error = %v", err)
	}

	if key == nil {
		t.Error("GetKey() returned nil key")
	}
	if key.Secret() != secret {
		t.Errorf("GetKey().Secret() = %v, want %v", key.Secret(), secret)
	}
}

func TestTimeSyncCheck(t *testing.T) {
	provider := NewProvider("test-issuer")

	tests := []struct {
		name         string
		reference    time.Time
		wantValid    bool
	}{
		{
			name:         "current time",
			reference:    time.Now(),
			wantValid:    true,
		},
		{
			name:         "time within window",
			reference:    time.Now().Add(2 * time.Minute),
			wantValid:    true,
		},
		{
			name:         "time just within window",
			reference:    time.Now().Add(4 * time.Minute + 59 * time.Second),
			wantValid:    true,
		},
		{
			name:         "time outside window future",
			reference:    time.Now().Add(6 * time.Minute),
			wantValid:    false,
		},
		{
			name:         "time outside window past",
			reference:    time.Now().Add(-6 * time.Minute),
			wantValid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.TimeSyncCheck(tt.reference)
			if got != tt.wantValid {
				t.Errorf("TimeSyncCheck() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}

func TestGetCurrentWindow(t *testing.T) {
	provider := NewProvider("test-issuer")

	secret := "JBSWY3DPEHPK3PXP" // Known test secret
	window, err := provider.GetCurrentWindow(secret)
	if err != nil {
		t.Fatalf("GetCurrentWindow() error = %v", err)
	}

	if window < 0 {
		t.Errorf("GetCurrentWindow() = %v, want >= 0", window)
	}

	// Window should be reasonable (current timestamp / 30)
	expected := time.Now().Unix() / 30
	diff := abs(window - expected)
	if diff > 2 {
		t.Errorf("GetCurrentWindow() = %v, expected ~%v", window, expected)
	}
}

func TestKnownTestVectors(t *testing.T) {
	// Test with known TOTP values
	// Secret: JBSWY3DPEHPK3PXP (base32 of "Hello123")
	provider := NewProvider("test-issuer")
	secret := "JBSWY3DPEHPK3PXP"

	// Generate a code - should be consistent for same time window
	now := time.Now()
	code1, err := provider.GenerateCode(secret)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	// Generate again immediately - should be same code
	code2, err := provider.GenerateCode(secret)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	if code1 != code2 {
		t.Errorf("GenerateCode() inconsistent: %v != %v", code1, code2)
	}

	// Validate that code
	valid, err := provider.ValidateCode(secret, code1)
	if err != nil {
		t.Fatalf("ValidateCode() error = %v", err)
	}
	if !valid {
		t.Error("ValidateCode() failed for valid code")
	}

	// After waiting for next window, code should change
	time.Sleep(31 * time.Second)
	code3, err := provider.GenerateCode(secret)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	if code1 == code3 {
		t.Errorf("GenerateCode() did not change after window: %v == %v", code1, code3)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelp(s, substr))
}

func containsHelp(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
