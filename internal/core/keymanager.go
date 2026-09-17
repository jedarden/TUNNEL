package core

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	httpClient         *http.Client
	githubKeysURL      string
	mu                 sync.Mutex
}

// NewFileKeyManager creates a new file-based key manager
func NewFileKeyManager(authorizedKeysPath string, auditLogger *AuditLogger) (*FileKeyManager, error) {
	return newFileKeyManager(authorizedKeysPath, auditLogger, nil)
}

// NewFileKeyManagerWithHTTPClient creates a file key manager using client for
// remote key imports. Supplying a client makes imports testable and lets
// callers provide proxy, timeout, or transport policy without changing the
// authorized_keys workflow.
func NewFileKeyManagerWithHTTPClient(authorizedKeysPath string, auditLogger *AuditLogger, client *http.Client) (*FileKeyManager, error) {
	return newFileKeyManager(authorizedKeysPath, auditLogger, client)
}

func newFileKeyManager(authorizedKeysPath string, auditLogger *AuditLogger, client *http.Client) (*FileKeyManager, error) {
	if strings.TrimSpace(authorizedKeysPath) == "" {
		return nil, fmt.Errorf("authorized_keys path cannot be empty")
	}

	// Ensure directory exists
	dir := filepath.Dir(authorizedKeysPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create authorized_keys directory: %w", err)
	}

	// Ensure file exists with correct permissions. O_EXCL avoids truncating a
	// file created concurrently by another process.
	file, err := os.OpenFile(authorizedKeysPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("close authorized_keys file: %w", err)
		}
	} else if !os.IsExist(err) {
		return nil, fmt.Errorf("create authorized_keys file: %w", err)
	} else if err := os.Chmod(authorizedKeysPath, 0600); err != nil {
		return nil, fmt.Errorf("set authorized_keys permissions: %w", err)
	}

	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	return &FileKeyManager{
		authorizedKeysPath: authorizedKeysPath,
		auditLogger:        auditLogger,
		httpClient:         client,
		githubKeysURL:      "https://github.com/%s.keys",
	}, nil
}

// ValidateKey parses and validates an SSH public key
func (km *FileKeyManager) ValidateKey(keyStr string) (*SSHPublicKey, error) {
	publicKey, comment, options, err := parseAuthorizedKey(keyStr)
	if err != nil {
		return nil, err
	}

	// Generate fingerprint
	fingerprint := km.generateFingerprint(publicKey)

	return &SSHPublicKey{
		ID:          fingerprint, // Use fingerprint as ID
		Type:        publicKey.Type(),
		PublicKey:   formatAuthorizedKey(publicKey, options, comment),
		Fingerprint: fingerprint,
		Comment:     comment,
		AddedAt:     time.Now(),
		Status:      "active",
	}, nil
}

// GetFingerprint generates SHA256 fingerprint for an SSH key
func (km *FileKeyManager) GetFingerprint(keyStr string) (string, error) {
	publicKey, _, _, err := parseAuthorizedKey(keyStr)
	if err != nil {
		return "", err
	}

	return km.generateFingerprint(publicKey), nil
}

func (km *FileKeyManager) generateFingerprint(key ssh.PublicKey) string {
	hash := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(hash[:])
}

// parseAuthorizedKey validates that input contains exactly one public key.
// ssh.ParseAuthorizedKey intentionally returns any trailing input as rest;
// rejecting non-whitespace rest prevents a pasted multi-key value from being
// silently truncated during manual add or remote import.
func parseAuthorizedKey(keyStr string) (ssh.PublicKey, string, []string, error) {
	keyStr = strings.TrimSpace(keyStr)
	if keyStr == "" {
		return nil, "", nil, fmt.Errorf("invalid SSH key: key is empty")
	}
	if strings.ContainsAny(keyStr, "\r\n") {
		return nil, "", nil, fmt.Errorf("invalid SSH key: key must contain exactly one line")
	}

	publicKey, comment, options, rest, err := ssh.ParseAuthorizedKey([]byte(keyStr))
	if err != nil {
		return nil, "", nil, fmt.Errorf("invalid SSH key: %w", err)
	}
	if strings.TrimSpace(string(rest)) != "" {
		return nil, "", nil, fmt.Errorf("invalid SSH key: multiple keys provided")
	}

	return publicKey, comment, options, nil
}

func formatAuthorizedKey(publicKey ssh.PublicKey, options []string, comment string) string {
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey)))
	if len(options) > 0 {
		line = strings.Join(options, ",") + " " + line
	}
	if comment != "" {
		line += " " + comment
	}
	return line
}

// normalizedKey makes the parsed key, rather than caller-provided metadata,
// authoritative for the key type, fingerprint, and ID. This prevents a
// malformed or stale fingerprint from making revocation and duplicate checks
// unreliable.
func (km *FileKeyManager) normalizedKey(key SSHPublicKey) (SSHPublicKey, error) {
	validated, err := km.ValidateKey(key.PublicKey)
	if err != nil {
		return SSHPublicKey{}, err
	}

	if key.Comment != "" {
		if strings.ContainsAny(key.Comment, "\r\n") {
			return SSHPublicKey{}, fmt.Errorf("key comment must be a single line")
		}
		publicKey, _, options, err := parseAuthorizedKey(validated.PublicKey)
		if err != nil {
			return SSHPublicKey{}, err
		}
		validated.Comment = key.Comment
		validated.PublicKey = formatAuthorizedKey(publicKey, options, key.Comment)
	}
	if !key.AddedAt.IsZero() {
		validated.AddedAt = key.AddedAt
	}
	if !key.LastUsed.IsZero() {
		validated.LastUsed = key.LastUsed
	}
	if key.ExpiresAt != nil {
		validated.ExpiresAt = key.ExpiresAt
	}
	if key.Status != "" {
		validated.Status = key.Status
	}

	return *validated, nil
}

func (km *FileKeyManager) logKeyAdded(username string, key SSHPublicKey) {
	if km.auditLogger == nil {
		return
	}
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

func (km *FileKeyManager) get(rawURL string) (*http.Response, error) {
	request, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create key request: %w", err)
	}
	request.Header.Set("Accept", "text/plain")
	request.Header.Set("User-Agent", "tunnel-ssh-key-manager")

	client := km.httpClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return client.Do(request)
}

func validateRemoteUsername(username string) error {
	if username == "" {
		return fmt.Errorf("remote username cannot be empty")
	}
	if len(username) > 39 {
		return fmt.Errorf("remote username is too long")
	}
	for _, char := range username {
		if (char < 'a' || char > 'z') &&
			(char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '-' {
			return fmt.Errorf("invalid remote username %q", username)
		}
	}
	return nil
}

func (km *FileKeyManager) importFromRemote(username, source, commentHost, sourceURL string) ([]SSHPublicKey, error) {
	resp, err := km.get(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("fetch %s keys: %w", source, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s API returned status %d", source, resp.StatusCode)
	}

	var keys []SSHPublicKey
	invalidCount := 0
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	for scanner.Scan() {
		keyStr := strings.TrimSpace(scanner.Text())
		if keyStr == "" {
			continue
		}

		key, err := km.ValidateKey(keyStr)
		if err != nil {
			invalidCount++
			continue
		}

		key.Comment = fmt.Sprintf("%s/%s", commentHost, username)
		publicKey, _, options, err := parseAuthorizedKey(key.PublicKey)
		if err != nil {
			invalidCount++
			continue
		}
		key.PublicKey = formatAuthorizedKey(publicKey, options, key.Comment)
		keys = append(keys, *key)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s response: %w", source, err)
	}
	if len(keys) == 0 && invalidCount > 0 {
		return nil, fmt.Errorf("%s response contained no valid SSH keys", source)
	}

	return km.mergeImportedKeys(username, source, sourceURL, keys)
}

func (km *FileKeyManager) mergeImportedKeys(username, source, sourceURL string, imported []SSHPublicKey) ([]SSHPublicKey, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	existing, err := km.readAuthorizedKeys()
	if err != nil {
		return nil, fmt.Errorf("read authorized_keys: %w", err)
	}

	existingFingerprints := make(map[string]struct{}, len(existing))
	for _, key := range existing {
		existingFingerprints[key.Fingerprint] = struct{}{}
	}

	newKeys := make([]SSHPublicKey, 0, len(imported))
	for _, key := range imported {
		if _, exists := existingFingerprints[key.Fingerprint]; exists {
			continue
		}
		existingFingerprints[key.Fingerprint] = struct{}{}
		newKeys = append(newKeys, key)
	}

	if len(newKeys) > 0 {
		updated := append(existing, newKeys...)
		if err := km.writeAuthorizedKeys(updated); err != nil {
			return nil, fmt.Errorf("write authorized_keys: %w", err)
		}
		for _, key := range newKeys {
			km.logKeyAdded(username, key)
		}
	}

	if km.auditLogger != nil {
		_ = km.auditLogger.Log(AuditEvent{
			Timestamp: time.Now(),
			EventType: "keys_imported",
			Method:    strings.ToLower(source),
			User:      username,
			Details: map[string]interface{}{
				"source": sourceURL,
				"count":  len(newKeys),
			},
			Success: true,
		})
	}

	return newKeys, nil
}

// AddKey adds an SSH public key for a user
func (km *FileKeyManager) AddKey(username string, key SSHPublicKey) error {
	key, err := km.normalizedKey(key)
	if err != nil {
		return fmt.Errorf("invalid key: %w", err)
	}

	km.mu.Lock()
	defer km.mu.Unlock()

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
	km.logKeyAdded(username, key)

	return nil
}

// RemoveKey removes an SSH public key
func (km *FileKeyManager) RemoveKey(username string, keyID string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	keys, err := km.readAuthorizedKeys()
	if err != nil {
		return fmt.Errorf("read authorized_keys: %w", err)
	}

	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return fmt.Errorf("key ID cannot be empty")
	}

	// Accept a one-based list index as a convenience for the CLI, in addition
	// to the stable fingerprint/ID accepted by the API.
	keyIndex := -1
	if index, parseErr := strconv.Atoi(keyID); parseErr == nil {
		keyIndex = index - 1
	}

	// Filter out the key to remove
	var filtered []SSHPublicKey
	found := false
	for index, key := range keys {
		if index != keyIndex && key.ID != keyID && key.Fingerprint != keyID {
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
	km.mu.Lock()
	defer km.mu.Unlock()

	return km.readAuthorizedKeys()
}

// ImportFromGitHub imports SSH keys from GitHub
func (km *FileKeyManager) ImportFromGitHub(username string) ([]SSHPublicKey, error) {
	if err := validateRemoteUsername(username); err != nil {
		return nil, err
	}

	keysURL := km.githubKeysURL
	if keysURL == "" {
		keysURL = "https://github.com/%s.keys"
	}
	return km.importFromRemote(
		username,
		"GitHub",
		"github.com",
		fmt.Sprintf(keysURL, url.PathEscape(username)),
	)
}

// ImportFromURL imports an SSH key from a URL
func (km *FileKeyManager) ImportFromURL(url string) (*SSHPublicKey, error) {
	resp, err := km.get(url)
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
	if err := validateRemoteUsername(username); err != nil {
		return nil, err
	}

	return km.importFromRemote(
		username,
		"GitLab",
		"gitlab.com",
		fmt.Sprintf("https://gitlab.com/%s.keys", url.PathEscape(username)),
	)
}

// ValidateKeyStrength checks for weak keys (RSA < 2048 bits)
func (km *FileKeyManager) ValidateKeyStrength(key string) error {
	// Parse exactly one SSH public key before checking its strength.
	publicKey, _, _, err := parseAuthorizedKey(key)
	if err != nil {
		return err
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
	newKey, err := km.normalizedKey(newKey)
	if err != nil {
		return fmt.Errorf("invalid new key: %w", err)
	}

	km.mu.Lock()
	defer km.mu.Unlock()

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
	km.mu.Lock()
	defer km.mu.Unlock()

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

	km.mu.Lock()
	defer km.mu.Unlock()

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

	// Validate and normalize all new keys before changing the file.
	normalizedKeys := make([]SSHPublicKey, 0, len(newKeys))
	for i, key := range newKeys {
		normalized, err := km.normalizedKey(key)
		if err != nil {
			return fmt.Errorf("invalid key at index %d: %w", i, err)
		}
		normalizedKeys = append(normalizedKeys, normalized)
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	// Read existing keys
	existingKeys, err := km.readAuthorizedKeys()
	if err != nil {
		return fmt.Errorf("read authorized_keys: %w", err)
	}

	oldCount := len(existingKeys)

	// Replace all keys with new keys
	if err := km.writeAuthorizedKeys(normalizedKeys); err != nil {
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
				"new_count": len(normalizedKeys),
			},
			Success: true,
		})
	}

	return nil
}

// IsDuplicate checks if fingerprint already exists, returns user if found
func (km *FileKeyManager) IsDuplicate(fingerprint string) (bool, string, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

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

	file, err := os.CreateTemp(filepath.Dir(km.authorizedKeysPath), ".authorized_keys.tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary authorized_keys file: %w", err)
	}
	temporaryPath := file.Name()
	defer os.Remove(temporaryPath)

	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return fmt.Errorf("set temporary authorized_keys permissions: %w", err)
	}
	if _, err := io.WriteString(file, builder.String()); err != nil {
		_ = file.Close()
		return fmt.Errorf("write temporary authorized_keys file: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync temporary authorized_keys file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary authorized_keys file: %w", err)
	}
	if err := os.Rename(temporaryPath, km.authorizedKeysPath); err != nil {
		return fmt.Errorf("replace authorized_keys file: %w", err)
	}
	if err := os.Chmod(km.authorizedKeysPath, 0600); err != nil {
		return fmt.Errorf("set authorized_keys permissions: %w", err)
	}

	return nil
}
