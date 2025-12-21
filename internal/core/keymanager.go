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
	ImportFromURL(url string) (*SSHPublicKey, error)

	// Validation
	ValidateKey(key string) (*SSHPublicKey, error)
	GetFingerprint(key string) (string, error)
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
		km.auditLogger.Log(AuditEvent{
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
		km.auditLogger.Log(AuditEvent{
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
		km.auditLogger.Log(AuditEvent{
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
