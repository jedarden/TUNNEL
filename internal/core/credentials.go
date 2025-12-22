package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/pbkdf2"
)

var (
	ErrCredentialNotFound = errors.New("credential not found")
	ErrInvalidCredential  = errors.New("invalid credential")
	ErrStoreUnavailable   = errors.New("credential store unavailable")
)

// CredentialStore defines the interface for storing and retrieving credentials
type CredentialStore interface {
	Set(service, key string, value []byte) error
	Get(service, key string) ([]byte, error)
	Delete(service, key string) error
	List(service string) ([]string, error)
}

// KeyringStore implements CredentialStore using the system keyring
type KeyringStore struct {
	serviceName string
}

// NewKeyringStore creates a new keyring-based credential store
func NewKeyringStore(serviceName string) *KeyringStore {
	return &KeyringStore{
		serviceName: serviceName,
	}
}

func (k *KeyringStore) Set(service, key string, value []byte) error {
	fullKey := fmt.Sprintf("%s:%s", service, key)
	encoded := base64.StdEncoding.EncodeToString(value)
	return keyring.Set(k.serviceName, fullKey, encoded)
}

func (k *KeyringStore) Get(service, key string) ([]byte, error) {
	fullKey := fmt.Sprintf("%s:%s", service, key)
	encoded, err := keyring.Get(k.serviceName, fullKey)
	if err != nil {
		if err == keyring.ErrNotFound {
			return nil, ErrCredentialNotFound
		}
		return nil, fmt.Errorf("keyring get: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode credential: %w", err)
	}
	return decoded, nil
}

func (k *KeyringStore) Delete(service, key string) error {
	fullKey := fmt.Sprintf("%s:%s", service, key)
	return keyring.Delete(k.serviceName, fullKey)
}

func (k *KeyringStore) List(service string) ([]string, error) {
	// Keyring doesn't support listing, return not implemented
	return nil, errors.New("list not supported by keyring store")
}

// FileStore implements CredentialStore using encrypted file storage
type FileStore struct {
	baseDir    string
	passphrase string
}

// NewFileStore creates a new file-based credential store
func NewFileStore(baseDir, passphrase string) (*FileStore, error) {
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("create credential directory: %w", err)
	}

	return &FileStore{
		baseDir:    baseDir,
		passphrase: passphrase,
	}, nil
}

func (f *FileStore) getFilePath(service string) string {
	filename := fmt.Sprintf("%s.cred", service)
	return filepath.Join(f.baseDir, filename)
}

func (f *FileStore) encrypt(data []byte) ([]byte, error) {
	// Derive key from passphrase using PBKDF2
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	key := pbkdf2.Key([]byte(f.passphrase), salt, 100000, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// Prepend salt to ciphertext
	result := append(salt, ciphertext...)
	return result, nil
}

func (f *FileStore) decrypt(data []byte) ([]byte, error) {
	if len(data) < 32 {
		return nil, ErrInvalidCredential
	}

	// Extract salt
	salt := data[:32]
	ciphertext := data[32:]

	// Derive key from passphrase
	key := pbkdf2.Key([]byte(f.passphrase), salt, 100000, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, ErrInvalidCredential
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plaintext, nil
}

func (f *FileStore) Set(service, key string, value []byte) error {
	filePath := f.getFilePath(service)

	// Load existing credentials
	credentials := make(map[string][]byte)
	if data, err := os.ReadFile(filePath); err == nil {
		decrypted, err := f.decrypt(data)
		if err != nil {
			return fmt.Errorf("decrypt existing credentials: %w", err)
		}
		if err := json.Unmarshal(decrypted, &credentials); err != nil {
			return fmt.Errorf("unmarshal credentials: %w", err)
		}
	}

	// Add/update credential
	credentials[key] = value

	// Marshal and encrypt
	marshaled, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	encrypted, err := f.encrypt(marshaled)
	if err != nil {
		return fmt.Errorf("encrypt credentials: %w", err)
	}

	// Write to file with secure permissions
	if err := os.WriteFile(filePath, encrypted, 0600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}

	return nil
}

func (f *FileStore) Get(service, key string) ([]byte, error) {
	filePath := f.getFilePath(service)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCredentialNotFound
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	decrypted, err := f.decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("decrypt credentials: %w", err)
	}

	credentials := make(map[string][]byte)
	if err := json.Unmarshal(decrypted, &credentials); err != nil {
		return nil, fmt.Errorf("unmarshal credentials: %w", err)
	}

	value, ok := credentials[key]
	if !ok {
		return nil, ErrCredentialNotFound
	}

	return value, nil
}

func (f *FileStore) Delete(service, key string) error {
	filePath := f.getFilePath(service)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("read credentials: %w", err)
	}

	decrypted, err := f.decrypt(data)
	if err != nil {
		return fmt.Errorf("decrypt credentials: %w", err)
	}

	credentials := make(map[string][]byte)
	if err := json.Unmarshal(decrypted, &credentials); err != nil {
		return fmt.Errorf("unmarshal credentials: %w", err)
	}

	delete(credentials, key)

	// If no credentials left, delete file
	if len(credentials) == 0 {
		return os.Remove(filePath)
	}

	// Otherwise, re-encrypt and save
	marshaled, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	encrypted, err := f.encrypt(marshaled)
	if err != nil {
		return fmt.Errorf("encrypt credentials: %w", err)
	}

	if err := os.WriteFile(filePath, encrypted, 0600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}

	return nil
}

func (f *FileStore) List(service string) ([]string, error) {
	filePath := f.getFilePath(service)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	decrypted, err := f.decrypt(data)
	if err != nil {
		return nil, fmt.Errorf("decrypt credentials: %w", err)
	}

	credentials := make(map[string][]byte)
	if err := json.Unmarshal(decrypted, &credentials); err != nil {
		return nil, fmt.Errorf("unmarshal credentials: %w", err)
	}

	keys := make([]string, 0, len(credentials))
	for k := range credentials {
		keys = append(keys, k)
	}

	return keys, nil
}

// EnvStore implements CredentialStore using environment variables (fallback)
type EnvStore struct {
	prefix string
}

// NewEnvStore creates a new environment variable-based credential store
func NewEnvStore(prefix string) *EnvStore {
	return &EnvStore{
		prefix: prefix,
	}
}

func (e *EnvStore) makeEnvKey(service, key string) string {
	// Convert to uppercase and replace special chars with underscores
	envKey := fmt.Sprintf("%s_%s_%s", e.prefix, service, key)
	envKey = strings.ToUpper(envKey)
	envKey = strings.Map(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, envKey)
	return envKey
}

func (e *EnvStore) Set(service, key string, value []byte) error {
	envKey := e.makeEnvKey(service, key)
	encoded := base64.StdEncoding.EncodeToString(value)
	return os.Setenv(envKey, encoded)
}

func (e *EnvStore) Get(service, key string) ([]byte, error) {
	envKey := e.makeEnvKey(service, key)
	encoded := os.Getenv(envKey)
	if encoded == "" {
		return nil, ErrCredentialNotFound
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode credential: %w", err)
	}
	return decoded, nil
}

func (e *EnvStore) Delete(service, key string) error {
	envKey := e.makeEnvKey(service, key)
	return os.Unsetenv(envKey)
}

func (e *EnvStore) List(service string) ([]string, error) {
	prefix := e.makeEnvKey(service, "")
	var keys []string

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, prefix) {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				// Extract the key part after service
				key := strings.TrimPrefix(parts[0], prefix)
				if key != "" {
					keys = append(keys, key)
				}
			}
		}
	}

	return keys, nil
}

// NewCredentialStore creates the appropriate credential store based on configuration
func NewCredentialStore(storeType, serviceName, baseDir, passphrase string) (CredentialStore, error) {
	switch storeType {
	case "keyring":
		// Try keyring, fallback to file if unavailable
		store := NewKeyringStore(serviceName)
		// Test if keyring is available
		testKey := "test"
		if err := store.Set("test", testKey, []byte("test")); err != nil {
			// Keyring unavailable, fallback to file
			if baseDir == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return nil, fmt.Errorf("get home directory: %w", err)
				}
				baseDir = filepath.Join(homeDir, ".config", "tunnel", "credentials")
			}
			return NewFileStore(baseDir, passphrase)
		}
		_ = store.Delete("test", testKey)
		return store, nil

	case "file":
		if baseDir == "" {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("get home directory: %w", err)
			}
			baseDir = filepath.Join(homeDir, ".config", "tunnel", "credentials")
		}
		return NewFileStore(baseDir, passphrase)

	case "env":
		return NewEnvStore(serviceName), nil

	default:
		return nil, fmt.Errorf("unsupported credential store type: %s", storeType)
	}
}
