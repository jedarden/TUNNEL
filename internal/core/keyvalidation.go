package core

import (
	"crypto/dsa"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// WeakKeyError represents a custom error for weak SSH keys
type WeakKeyError struct {
	KeyType      string
	BitLength    int
	Issue        string
	Severity     string // "critical", "warning", "info"
	Recommendation string
}

func (e *WeakKeyError) Error() string {
	return fmt.Sprintf("%s key validation failed: %s (severity: %s) - %s",
		e.KeyType, e.Issue, e.Severity, e.Recommendation)
}

// KeySecurityReport contains a full security assessment of an SSH key
type KeySecurityReport struct {
	KeyType            string
	BitLength          int
	IsWeak             bool
	WeakReason         string
	AgeWarning         bool
	AgeMessage         string
	RecommendedAction  string
	FormatValid        bool
	FormatIssues       []string
}

// ValidateKeyStrength validates the cryptographic strength of an SSH public key
func ValidateKeyStrength(keyStr string) error {
	keyStr = strings.TrimSpace(keyStr)

	// Parse the SSH public key
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyStr))
	if err != nil {
		return fmt.Errorf("invalid SSH key: %w", err)
	}

	keyType := publicKey.Type()

	// Extract the underlying crypto key
	cryptoKey := publicKey.(ssh.CryptoPublicKey).CryptoPublicKey()

	switch keyType {
	case "ssh-rsa":
		rsaKey, ok := cryptoKey.(*rsa.PublicKey)
		if !ok {
			return &WeakKeyError{
				KeyType:        keyType,
				Issue:          "failed to extract RSA key",
				Severity:       "critical",
				Recommendation: "key format is corrupted, generate a new key",
			}
		}

		bitLength := rsaKey.N.BitLen()

		if bitLength < 2048 {
			return &WeakKeyError{
				KeyType:        keyType,
				BitLength:      bitLength,
				Issue:          fmt.Sprintf("RSA key too weak (%d bits, minimum 2048)", bitLength),
				Severity:       "critical",
				Recommendation: "generate a new RSA key with at least 4096 bits or switch to ED25519",
			}
		}

		if bitLength < 4096 {
			return &WeakKeyError{
				KeyType:        keyType,
				BitLength:      bitLength,
				Issue:          fmt.Sprintf("RSA key strength below recommended (%d bits, recommended 4096)", bitLength),
				Severity:       "warning",
				Recommendation: "consider generating a new RSA 4096-bit key or switching to ED25519",
			}
		}

	case "ssh-dss":
		return &WeakKeyError{
			KeyType:        keyType,
			Issue:          "DSA keys are deprecated and considered insecure",
			Severity:       "critical",
			Recommendation: "generate a new ED25519 or RSA 4096-bit key immediately",
		}

	case "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521":
		ecdsaKey, ok := cryptoKey.(*ecdsa.PublicKey)
		if !ok {
			return &WeakKeyError{
				KeyType:        keyType,
				Issue:          "failed to extract ECDSA key",
				Severity:       "critical",
				Recommendation: "key format is corrupted, generate a new key",
			}
		}

		bitLength := ecdsaKey.Curve.Params().BitSize

		// nistp256 (P-256) is the minimum acceptable
		if keyType == "ecdsa-sha2-nistp256" && bitLength >= 256 {
			// Valid but note that ED25519 is preferred
			return nil
		}

		if bitLength < 256 {
			return &WeakKeyError{
				KeyType:        keyType,
				BitLength:      bitLength,
				Issue:          fmt.Sprintf("ECDSA curve too weak (%d bits, minimum nistp256)", bitLength),
				Severity:       "critical",
				Recommendation: "generate a new ED25519 key or use ECDSA nistp384/nistp521",
			}
		}

	case "ssh-ed25519":
		// ED25519 keys are always 256 bits and considered secure
		_, ok := cryptoKey.(ed25519.PublicKey)
		if !ok {
			return &WeakKeyError{
				KeyType:        keyType,
				Issue:          "failed to extract ED25519 key",
				Severity:       "critical",
				Recommendation: "key format is corrupted, generate a new key",
			}
		}
		// ED25519 is always valid
		return nil

	default:
		return &WeakKeyError{
			KeyType:        keyType,
			Issue:          fmt.Sprintf("unsupported key type: %s", keyType),
			Severity:       "critical",
			Recommendation: "use ED25519, ECDSA nistp256+, or RSA 4096-bit",
		}
	}

	return nil
}

// CheckKeyAge checks if a key is old and should be rotated
func CheckKeyAge(addedAt time.Time) (warning bool, message string) {
	age := time.Since(addedAt)
	years := age.Hours() / 24 / 365

	if years >= 2 {
		return true, fmt.Sprintf("CRITICAL: Key is %.1f years old and should be rotated immediately. SSH keys should be rotated at least every 2 years.", years)
	}

	if years >= 1 {
		return true, fmt.Sprintf("WARNING: Key is %.1f years old. Consider rotating keys older than 1 year for better security.", years)
	}

	return false, ""
}

// ValidateKeyFormat validates the format and structure of an SSH public key
func ValidateKeyFormat(keyStr string) error {
	originalKey := keyStr
	keyStr = strings.TrimSpace(keyStr)

	// Check for excessive whitespace
	if originalKey != keyStr {
		if strings.HasPrefix(originalKey, " ") || strings.HasPrefix(originalKey, "\t") {
			return fmt.Errorf("key has leading whitespace")
		}
		if strings.HasSuffix(originalKey, " ") || strings.HasSuffix(originalKey, "\t") {
			return fmt.Errorf("key has trailing whitespace")
		}
	}

	// Check for empty key
	if keyStr == "" {
		return fmt.Errorf("key is empty")
	}

	// Check for newlines within the key (except at the end)
	if strings.Count(keyStr, "\n") > 1 {
		return fmt.Errorf("key contains multiple newline characters")
	}

	// Parse to validate structure
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyStr))
	if err != nil {
		return fmt.Errorf("invalid key format: %w", err)
	}

	// Check key structure (should have at least 2 space-separated parts: type and key data)
	parts := strings.Fields(keyStr)
	if len(parts) < 2 {
		return fmt.Errorf("key missing required fields (expected: key-type key-data [comment])")
	}

	// Validate key type format
	keyType := parts[0]
	validKeyTypes := []string{
		"ssh-rsa", "ssh-dss", "ssh-ed25519",
		"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521",
		"sk-ecdsa-sha2-nistp256@openssh.com", "sk-ssh-ed25519@openssh.com",
	}

	validType := false
	for _, valid := range validKeyTypes {
		if keyType == valid {
			validType = true
			break
		}
	}

	if !validType {
		return fmt.Errorf("unknown or invalid key type: %s", keyType)
	}

	// Validate base64 encoding of key data
	keyData := parts[1]
	if len(keyData) < 20 {
		return fmt.Errorf("key data appears too short to be valid")
	}

	// Check for common encoding issues
	if strings.Contains(keyData, " ") {
		return fmt.Errorf("key data contains spaces (corrupted encoding)")
	}

	return nil
}

// GetKeyBitLength returns the bit length of an SSH public key
func GetKeyBitLength(keyStr string) (int, error) {
	keyStr = strings.TrimSpace(keyStr)

	// Parse the SSH public key
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyStr))
	if err != nil {
		return 0, fmt.Errorf("invalid SSH key: %w", err)
	}

	keyType := publicKey.Type()
	cryptoKey := publicKey.(ssh.CryptoPublicKey).CryptoPublicKey()

	switch keyType {
	case "ssh-rsa":
		rsaKey, ok := cryptoKey.(*rsa.PublicKey)
		if !ok {
			return 0, fmt.Errorf("failed to extract RSA key")
		}
		return rsaKey.N.BitLen(), nil

	case "ssh-dss":
		dsaKey, ok := cryptoKey.(*dsa.PublicKey)
		if !ok {
			return 0, fmt.Errorf("failed to extract DSA key")
		}
		return dsaKey.P.BitLen(), nil

	case "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521":
		ecdsaKey, ok := cryptoKey.(*ecdsa.PublicKey)
		if !ok {
			return 0, fmt.Errorf("failed to extract ECDSA key")
		}
		return ecdsaKey.Curve.Params().BitSize, nil

	case "ssh-ed25519":
		// ED25519 keys are always 256 bits
		return 256, nil

	default:
		return 0, fmt.Errorf("unsupported key type: %s", keyType)
	}
}

// AnalyzeKey generates a comprehensive security report for an SSH public key
func AnalyzeKey(key SSHPublicKey) (*KeySecurityReport, error) {
	report := &KeySecurityReport{
		KeyType:      key.Type,
		FormatValid:  true,
		FormatIssues: []string{},
	}

	// Validate format
	if err := ValidateKeyFormat(key.PublicKey); err != nil {
		report.FormatValid = false
		report.FormatIssues = append(report.FormatIssues, err.Error())
	}

	// Get bit length
	bitLength, err := GetKeyBitLength(key.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to determine key bit length: %w", err)
	}
	report.BitLength = bitLength

	// Check key strength
	if err := ValidateKeyStrength(key.PublicKey); err != nil {
		report.IsWeak = true
		if weakErr, ok := err.(*WeakKeyError); ok {
			report.WeakReason = weakErr.Issue
			report.RecommendedAction = weakErr.Recommendation
		} else {
			report.WeakReason = err.Error()
			report.RecommendedAction = "Generate a new ED25519 or RSA 4096-bit key"
		}
	}

	// Check key age
	warning, message := CheckKeyAge(key.AddedAt)
	if warning {
		report.AgeWarning = true
		report.AgeMessage = message

		// If no existing recommendation, add age-based recommendation
		if report.RecommendedAction == "" {
			if strings.Contains(message, "CRITICAL") {
				report.RecommendedAction = "Rotate this key immediately - it has exceeded the maximum recommended age"
			} else {
				report.RecommendedAction = "Consider rotating this key soon to maintain good security hygiene"
			}
		}
	}

	// If everything is good, provide positive feedback
	if !report.IsWeak && !report.AgeWarning && report.FormatValid {
		report.RecommendedAction = "Key meets security requirements. Continue to monitor age and rotate periodically."
	}

	return report, nil
}

// GetSecuritySummary returns a human-readable security summary for a key
func (r *KeySecurityReport) GetSecuritySummary() string {
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("Key Type: %s (%d bits)\n", r.KeyType, r.BitLength))

	if !r.FormatValid {
		summary.WriteString("\nFORMAT ISSUES:\n")
		for _, issue := range r.FormatIssues {
			summary.WriteString(fmt.Sprintf("  - %s\n", issue))
		}
	}

	if r.IsWeak {
		summary.WriteString("\nSECURITY: WEAK\n")
		summary.WriteString("  Reason: " + r.WeakReason + "\n")
	} else {
		summary.WriteString("\nSECURITY: STRONG\n")
	}

	if r.AgeWarning {
		summary.WriteString(fmt.Sprintf("\n%s\n", r.AgeMessage))
	}

	if r.RecommendedAction != "" {
		summary.WriteString(fmt.Sprintf("\nRECOMMENDATION: %s\n", r.RecommendedAction))
	}

	return summary.String()
}

// IsSecure returns true if the key meets all security requirements
func (r *KeySecurityReport) IsSecure() bool {
	return r.FormatValid && !r.IsWeak && !r.AgeWarning
}

// GetSecurityScore returns a security score from 0-100
func (r *KeySecurityReport) GetSecurityScore() int {
	score := 100

	// Format issues
	if !r.FormatValid {
		score -= 30
	}

	// Weak key issues
	if r.IsWeak {
		if strings.Contains(strings.ToLower(r.WeakReason), "deprecated") ||
			strings.Contains(strings.ToLower(r.WeakReason), "too weak") {
			score -= 50 // Critical weakness
		} else {
			score -= 25 // Warning level weakness
		}
	}

	// Age issues
	if r.AgeWarning {
		if strings.Contains(r.AgeMessage, "CRITICAL") {
			score -= 30
		} else {
			score -= 15
		}
	}

	if score < 0 {
		score = 0
	}

	return score
}
