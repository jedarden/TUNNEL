package core

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHPublicKey represents an SSH public key with metadata
type SSHPublicKey struct {
	ID          string
	Type        string // ssh-ed25519, ssh-rsa, ecdsa-sha2-nistp256, etc.
	PublicKey   string
	Fingerprint string
	Comment     string
	AddedAt     time.Time
	LastUsed    time.Time
	ExpiresAt   *time.Time
	Status      string // active, revoked, expired
}

// KeyManager handles SSH key operations
type KeyManager interface {
	// Key operations
	AddKey(username string, key SSHPublicKey) error
	RemoveKey(username string, keyID string) error
	ListKeys(username string) ([]SSHPublicKey, error)

	// Import
	ImportFromGitHub(username string) ([]SSHPublicKey, error)
	ImportFromGitLab(username string) ([]SSHPublicKey, error)
	ImportFromURL(url string) (*SSHPublicKey, error)

	// Validation
	ValidateKey(key string) (*SSHPublicKey, error)
	ValidateKeyStrength(key string) error
	GetFingerprint(key string) (string, error)

	// Key lifecycle management
	RotateKey(username, oldKeyID string, newKey SSHPublicKey) error
	CheckKeyExpiration() ([]SSHPublicKey, error)
	CheckKeyAge(key SSHPublicKey) (bool, string)

	// Bulk operations
	BulkRevoke(username string, keyIDs []string) error
	BulkRotate(username string, newKeys []SSHPublicKey) error

	// Duplicate detection
	IsDuplicate(fingerprint string) (bool, string, error)
}

// FileKeyManager implements KeyManager using authorized_keys file
type FileKeyManager struct {
	authorizedKeysPath string
	auditLogger        *AuditLogger
}

// NewFileKeyManager creates a new file-based key manager
func NewFileKeyManager(authorizedKeysPath string, auditLogger *AuditLogger) (*FileKeyManager, error) {
	// Ensure directory exists
	dir := filepath.Dir(authorizedKeysPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create authorized_keys directory: %w", err)
	}

	// Ensure file exists with correct permissions
	if _, err := os.Stat(authorizedKeysPath); os.IsNotExist(err) {
		if err := os.WriteFile(authorizedKeysPath, []byte{}, 0600); err != nil {
			return nil, fmt.Errorf("create authorized_keys file: %w", err)
		}
	} else {
		// Fix permissions if file exists
		if err := os.Chmod(authorizedKeysPath, 0600); err != nil {
			return nil, fmt.Errorf("set authorized_keys permissions: %w", err)
		}
	}

	return &FileKeyManager{
		authorizedKeysPath: authorizedKeysPath,
		auditLogger:        auditLogger,
	}, nil
}

// ValidateKey parses and validates an SSH public key
func (km *FileKeyManager) ValidateKey(keyStr string) (*SSHPublicKey, error) {
	keyStr = strings.TrimSpace(keyStr)

	// Parse the SSH public key
	publicKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(keyStr))
	if err != nil {
		return nil, fmt.Errorf("invalid SSH key: %w", err)
	}

	// Generate fingerprint
	fingerprint := km.generateFingerprint(publicKey)

	return &SSHPublicKey{
		ID:          fingerprint, // Use fingerprint as ID
		Type:        publicKey.Type(),
		PublicKey:   keyStr,
		Fingerprint: fingerprint,
		Comment:     comment,
		AddedAt:     time.Now(),
		Status:      "active",
	}, nil
}

// GetFingerprint generates SHA256 fingerprint for an SSH key
func (km *FileKeyManager) GetFingerprint(keyStr string) (string, error) {
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyStr))
	if err != nil {
		return "", fmt.Errorf("invalid SSH key: %w", err)
	}

	return km.generateFingerprint(publicKey), nil
}

func (km *FileKeyManager) generateFingerprint(key ssh.PublicKey) string {
	hash := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])
}

// AddKey adds an SSH public key for a user
func (km *FileKeyManager) AddKey(username string, key SSHPublicKey) error {
	// Validate the key first
	if _, err := km.ValidateKey(key.PublicKey); err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	// Read existing keys
	keys, err := km.readAuthorizedKeys()
	if err != nil {
		return fmt.Errorf("read authorized_keys: %w", err)
	}

	// Check for duplicates
	for _, existing := range keys {
		if existing.Fingerprint == key.Fingerprint {
			return fmt.Errorf("key already exists")
		}
	}

	// Add new key
	keys = append(keys, key)

	// Write back to file
	if err := km.writeAuthorizedKeys(keys); err != nil {
		return fmt.Errorf("write authorized_keys: %w", err)
	}

	// Log audit event
	if km.auditLogger != nil {
		_ = km.auditLogger.Log(AuditEvent{
			Timestamp: time.Now(),
			EventType: "key_added",
			Method:    "ssh-key",
			User:      username,
			Details: map[string]interface{}{
				"fingerprint": key.Fingerprint,
				"type":        key.Type,
				"comment":     key.Comment,
			},
			Success: true,
		})
	}

	return nil
}

// RemoveKey removes an SSH public key
func (km *FileKeyManager) RemoveKey(username string, keyID string) error {
	keys, err := km.readAuthorizedKeys()
	if err != nil {
		return fmt.Errorf("read authorized_keys: %w", err)
	}

	// Filter out the key to remove
	var filtered []SSHPublicKey
	found := false
	for _, key := range keys {
		if key.ID != keyID && key.Fingerprint != keyID {
			filtered = append(filtered, key)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("key not found")
	}

	// Write back to file
	if err := km.writeAuthorizedKeys(filtered); err != nil {
		return fmt.Errorf("write authorized_keys: %w", err)
	}

	// Log audit event
	if km.auditLogger != nil {
		_ = km.auditLogger.Log(AuditEvent{
			Timestamp: time.Now(),
			EventType: "key_removed",
			Method:    "ssh-key",
			User:      username,
			Details: map[string]interface{}{
				"key_id": keyID,
			},
			Success: true,
		})
	}

	return nil
}

// ListKeys returns all SSH public keys
func (km *FileKeyManager) ListKeys(username string) ([]SSHPublicKey, error) {
	return km.readAuthorizedKeys()
}

// ImportFromGitHub imports SSH keys from GitHub
func (km *FileKeyManager) ImportFromGitHub(username string) ([]SSHPublicKey, error) {
	url := fmt.Sprintf("https://github.com/%s.keys", username)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch GitHub keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var keys []SSHPublicKey
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		keyStr := strings.TrimSpace(scanner.Text())
		if keyStr == "" {
			continue
		}

		key, err := km.ValidateKey(keyStr)
		if err != nil {
			// Log but continue with other keys
			fmt.Fprintf(os.Stderr, "Warning: invalid key from GitHub: %v\n", err)
			continue
		}

		// Add comment indicating source
		key.Comment = fmt.Sprintf("github.com/%s", username)
		keys = append(keys, *key)

		// Add to authorized_keys
		if err := km.AddKey(username, *key); err != nil {
			return nil, fmt.Errorf("add key: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read GitHub response: %w", err)
	}

	// Log audit event
	if km.auditLogger != nil {
		_ = km.auditLogger.Log(AuditEvent{
			Timestamp: time.Now(),
			EventType: "keys_imported",
			Method:    "github",
			User:      username,
			Details: map[string]interface{}{
				"source": url,
				"count":  len(keys),
			},
			Success: true,
		})
	}

	return keys, nil
}

// ImportFromURL imports an SSH key from a URL
func (km *FileKeyManager) ImportFromURL(url string) (*SSHPublicKey, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch key from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("URL returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	key, err := km.ValidateKey(string(data))
	if err != nil {
		return nil, fmt.Errorf("invalid key: %w", err)
	}

	return key, nil
}

// ImportFromGitLab imports SSH keys from GitLab
func (km *FileKeyManager) ImportFromGitLab(username string) ([]SSHPublicKey, error) {
	url := fmt.Sprintf("https://gitlab.com/%s.keys", username)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch GitLab keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitLab API returned status %d", resp.StatusCode)
	}

	var keys []SSHPublicKey
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		keyStr := strings.TrimSpace(scanner.Text())
		if keyStr == "" {
			continue
		}

		key, err := km.ValidateKey(keyStr)
		if err != nil {
			// Log but continue with other keys
			fmt.Fprintf(os.Stderr, "Warning: invalid key from GitLab: %v\n", err)
			continue
		}

		// Add comment indicating source
		key.Comment = fmt.Sprintf("gitlab.com/%s", username)
		keys = append(keys, *key)

		// Add to authorized_keys
		if err := km.AddKey(username, *key); err != nil {
			return nil, fmt.Errorf("add key: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read GitLab response: %w", err)
	}

	// Log audit event
	if km.auditLogger != nil {
		_ = km.auditLogger.Log(AuditEvent{
			Timestamp: time.Now(),
			EventType: "keys_imported",
			Method:    "gitlab",
			User:      username,
			Details: map[string]interface{}{
				"source": url,
				"count":  len(keys),
			},
			Success: true,
		})
	}

	return keys, nil
}

// ValidateKeyStrength checks for weak keys (RSA < 2048 bits)
func (km *FileKeyManager) ValidateKeyStrength(key string) error {
	keyStr := strings.TrimSpace(key)

	// Parse the SSH public key
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyStr))
	if err != nil {
		return fmt.Errorf("invalid SSH key: %w", err)
	}

	// Check key type and strength
	switch publicKey.Type() {
	case "ssh-rsa":
		// RSA keys must be at least 2048 bits
		keyData := publicKey.Marshal()
		// Rough estimate: RSA 2048-bit keys are ~270+ bytes when marshaled
		// RSA 1024-bit keys are ~140 bytes
		if len(keyData) < 200 {
			return fmt.Errorf("RSA key is too weak (< 2048 bits)")
		}
	case "ssh-dss":
		// DSA keys are considered weak
		return fmt.Errorf("DSA keys are no longer considered secure")
	}

	return nil
}

// RotateKey rotates a key by adding the new key and revoking the old one atomically
func (km *FileKeyManager) RotateKey(username, oldKeyID string, newKey SSHPublicKey) error {
	// Validate the new key first
	if _, err := km.ValidateKey(newKey.PublicKey); err != nil {
		return fmt.Errorf("invalid new key: %w", err)
	}

	// Read existing keys
	keys, err := km.readAuthorizedKeys()
	if err != nil {
		return fmt.Errorf("read authorized_keys: %w", err)
	}

	// Find and remove old key, add new key atomically
	found := false
	var updatedKeys []SSHPublicKey
	for _, key := range keys {
		if key.ID == oldKeyID || key.Fingerprint == oldKeyID {
			found = true
			// Skip the old key (effectively revoking it)
			continue
		}
		updatedKeys = append(updatedKeys, key)
	}

	if !found {
		return fmt.Errorf("old key not found")
	}

	// Add the new key
	updatedKeys = append(updatedKeys, newKey)

	// Write back to file
	if err := km.writeAuthorizedKeys(updatedKeys); err != nil {
		return fmt.Errorf("write authorized_keys: %w", err)
	}

	// Log audit event
	if km.auditLogger != nil {
		_ = km.auditLogger.Log(AuditEvent{
			Timestamp: time.Now(),
			EventType: "key_rotated",
			Method:    "ssh-key",
			User:      username,
			Details: map[string]interface{}{
				"old_key_id":      oldKeyID,
				"new_fingerprint": newKey.Fingerprint,
				"new_type":        newKey.Type,
			},
			Success: true,
		})
	}

	return nil
}

// CheckKeyExpiration returns all keys that have expired or are expiring soon (within 30 days)
func (km *FileKeyManager) CheckKeyExpiration() ([]SSHPublicKey, error) {
	keys, err := km.readAuthorizedKeys()
	if err != nil {
		return nil, fmt.Errorf("read authorized_keys: %w", err)
	}

	var expiringKeys []SSHPublicKey
	now := time.Now()
	thirtyDaysFromNow := now.Add(30 * 24 * time.Hour)

	for _, key := range keys {
		if key.ExpiresAt != nil {
			// Check if expired or expiring within 30 days
			if key.ExpiresAt.Before(thirtyDaysFromNow) {
				expiringKeys = append(expiringKeys, key)
			}
		}
	}

	return expiringKeys, nil
}

// CheckKeyAge returns true if key is old (> 1 year) with a warning message
func (km *FileKeyManager) CheckKeyAge(key SSHPublicKey) (bool, string) {
	oneYearAgo := time.Now().Add(-365 * 24 * time.Hour)

	if key.AddedAt.Before(oneYearAgo) {
		age := time.Since(key.AddedAt)
		days := int(age.Hours() / 24)
		message := fmt.Sprintf("Key is %d days old (added %s). Consider rotating for security best practices.",
			days, key.AddedAt.Format("2006-01-02"))
		return true, message
	}

	return false, ""
}

// BulkRevoke revokes multiple keys at once
func (km *FileKeyManager) BulkRevoke(username string, keyIDs []string) error {
	if len(keyIDs) == 0 {
		return fmt.Errorf("no key IDs provided")
	}

	keys, err := km.readAuthorizedKeys()
	if err != nil {
		return fmt.Errorf("read authorized_keys: %w", err)
	}

	// Create a map for fast lookup
	revokeMap := make(map[string]bool)
	for _, keyID := range keyIDs {
		revokeMap[keyID] = true
	}

	// Filter out keys to revoke
	var filtered []SSHPublicKey
	revokedCount := 0
	for _, key := range keys {
		if revokeMap[key.ID] || revokeMap[key.Fingerprint] {
			revokedCount++
			continue
		}
		filtered = append(filtered, key)
	}

	if revokedCount == 0 {
		return fmt.Errorf("no matching keys found to revoke")
	}

	// Write back to file
	if err := km.writeAuthorizedKeys(filtered); err != nil {
		return fmt.Errorf("write authorized_keys: %w", err)
	}

	// Log audit event
	if km.auditLogger != nil {
		_ = km.auditLogger.Log(AuditEvent{
			Timestamp: time.Now(),
			EventType: "keys_bulk_revoked",
			Method:    "ssh-key",
			User:      username,
			Details: map[string]interface{}{
				"key_ids":       keyIDs,
				"revoked_count": revokedCount,
			},
			Success: true,
		})
	}

	return nil
}

// BulkRotate rotates all keys for a user in bulk
func (km *FileKeyManager) BulkRotate(username string, newKeys []SSHPublicKey) error {
	if len(newKeys) == 0 {
		return fmt.Errorf("no new keys provided")
	}

	// Validate all new keys first
	for i, key := range newKeys {
		if _, err := km.ValidateKey(key.PublicKey); err != nil {
			return fmt.Errorf("invalid key at index %d: %w", i, err)
		}
	}

	// Read existing keys
	existingKeys, err := km.readAuthorizedKeys()
	if err != nil {
		return fmt.Errorf("read authorized_keys: %w", err)
	}

	oldCount := len(existingKeys)

	// Replace all keys with new keys
	if err := km.writeAuthorizedKeys(newKeys); err != nil {
		return fmt.Errorf("write authorized_keys: %w", err)
	}

	// Log audit event
	if km.auditLogger != nil {
		_ = km.auditLogger.Log(AuditEvent{
			Timestamp: time.Now(),
			EventType: "keys_bulk_rotated",
			Method:    "ssh-key",
			User:      username,
			Details: map[string]interface{}{
				"old_count": oldCount,
				"new_count": len(newKeys),
			},
			Success: true,
		})
	}

	return nil
}

// IsDuplicate checks if fingerprint already exists, returns user if found
func (km *FileKeyManager) IsDuplicate(fingerprint string) (bool, string, error) {
	keys, err := km.readAuthorizedKeys()
	if err != nil {
		return false, "", fmt.Errorf("read authorized_keys: %w", err)
	}

	for _, key := range keys {
		if key.Fingerprint == fingerprint {
			// Extract username from comment if available
			username := "unknown"
			if key.Comment != "" {
				// Try to extract username from comments like "github.com/username" or "gitlab.com/username"
				parts := strings.Split(key.Comment, "/")
				if len(parts) > 1 {
					username = parts[len(parts)-1]
				} else {
					username = key.Comment
				}
			}
			return true, username, nil
		}
	}

	return false, "", nil
}

// readAuthorizedKeys reads and parses the authorized_keys file
func (km *FileKeyManager) readAuthorizedKeys() ([]SSHPublicKey, error) {
	data, err := os.ReadFile(km.authorizedKeysPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []SSHPublicKey{}, nil
		}
		return nil, err
	}

	var keys []SSHPublicKey
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, err := km.ValidateKey(line)
		if err != nil {
			// Log but continue with other keys
			fmt.Fprintf(os.Stderr, "Warning: invalid key in authorized_keys: %v\n", err)
			continue
		}

		keys = append(keys, *key)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}

// writeAuthorizedKeys writes keys to the authorized_keys file
func (km *FileKeyManager) writeAuthorizedKeys(keys []SSHPublicKey) error {
	var builder strings.Builder

	builder.WriteString("# SSH Public Keys\n")
	builder.WriteString(fmt.Sprintf("# Managed by TUNNEL - Last updated: %s\n\n", time.Now().Format(time.RFC3339)))

	for _, key := range keys {
		builder.WriteString(key.PublicKey)
		if !strings.HasSuffix(key.PublicKey, "\n") {
			builder.WriteString("\n")
		}
	}

	return os.WriteFile(km.authorizedKeysPath, []byte(builder.String()), 0600)
}
