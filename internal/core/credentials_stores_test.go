package core

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// binarySecret contains NUL bytes and invalid UTF-8 to prove stores never
// treat credentials as text.
var binarySecret = []byte{0x00, 0x01, 0xff, 0xfe, 'a', 0x00, 0xb0, 0x5f}

// --- KeyringStore ---

func TestKeyringStoreCRUD(t *testing.T) {
	keyring.MockInit()

	store := NewKeyringStore("tunnel-unit-test")

	if err := store.Set("svc", "token", []byte("keyring-secret")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := store.Get("svc", "token")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got) != "keyring-secret" {
		t.Errorf("Get() = %q, want %q", got, "keyring-secret")
	}

	// Overwrite
	if err := store.Set("svc", "token", []byte("rotated")); err != nil {
		t.Fatalf("Set() overwrite error = %v", err)
	}
	got, err = store.Get("svc", "token")
	if err != nil {
		t.Fatalf("Get() after overwrite error = %v", err)
	}
	if string(got) != "rotated" {
		t.Errorf("Get() after overwrite = %q, want %q", got, "rotated")
	}

	if err := store.Delete("svc", "token"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get("svc", "token"); !errors.Is(err, ErrCredentialNotFound) {
		t.Errorf("Get() after Delete() error = %v, want ErrCredentialNotFound", err)
	}
}

func TestKeyringStoreBinaryValues(t *testing.T) {
	keyring.MockInit()

	store := NewKeyringStore("tunnel-unit-test")

	if err := store.Set("svc", "binary", binarySecret); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, err := store.Get("svc", "binary")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !bytes.Equal(got, binarySecret) {
		t.Errorf("Get() = %v, want %v", got, binarySecret)
	}
}

func TestKeyringStoreGetMissingMapsToNotFound(t *testing.T) {
	keyring.MockInit()

	store := NewKeyringStore("tunnel-unit-test")

	if _, err := store.Get("missing-svc", "missing-key"); !errors.Is(err, ErrCredentialNotFound) {
		t.Errorf("Get() missing error = %v, want ErrCredentialNotFound", err)
	}
}

func TestKeyringStoreServiceNamespacing(t *testing.T) {
	keyring.MockInit()

	store := NewKeyringStore("tunnel-unit-test")

	if err := store.Set("svc-a", "token", []byte("value-a")); err != nil {
		t.Fatalf("Set() svc-a error = %v", err)
	}
	if err := store.Set("svc-b", "token", []byte("value-b")); err != nil {
		t.Fatalf("Set() svc-b error = %v", err)
	}

	got, err := store.Get("svc-a", "token")
	if err != nil || string(got) != "value-a" {
		t.Errorf("Get(svc-a) = %q, %v; want %q, nil", got, err, "value-a")
	}
	got, err = store.Get("svc-b", "token")
	if err != nil || string(got) != "value-b" {
		t.Errorf("Get(svc-b) = %q, %v; want %q, nil", got, err, "value-b")
	}

	// Deleting under one service must not affect the other.
	if err := store.Delete("svc-a", "token"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get("svc-b", "token"); err != nil {
		t.Errorf("Get(svc-b) after deleting svc-a error = %v, want nil", err)
	}
}

func TestKeyringStoreBackendErrorsAreWrapped(t *testing.T) {
	keyring.MockInitWithError(errors.New("dbus unavailable"))
	t.Cleanup(keyring.MockInit)

	store := NewKeyringStore("tunnel-unit-test")

	// A backend failure must not be misreported as a missing credential.
	_, err := store.Get("svc", "key")
	if err == nil || errors.Is(err, ErrCredentialNotFound) {
		t.Errorf("Get() error = %v, want wrapped backend error (not ErrCredentialNotFound)", err)
	}
	if !strings.Contains(err.Error(), "keyring get") {
		t.Errorf("Get() error = %v, want it wrapped with context", err)
	}

	if err := store.Set("svc", "key", []byte("v")); err == nil {
		t.Error("Set() = nil error, want backend error")
	}
	if err := store.Delete("svc", "key"); err == nil {
		t.Error("Delete() = nil error, want backend error")
	}
}

func TestKeyringStoreListUnsupported(t *testing.T) {
	keyring.MockInit()

	store := NewKeyringStore("tunnel-unit-test")

	if _, err := store.List("svc"); err == nil {
		t.Error("List() = nil error, want not-supported error")
	}
}

// --- FileStore ---

func TestFileStoreMultiKeyCRUD(t *testing.T) {
	store, err := NewFileStore(t.TempDir(), "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	if err := store.Set("svc", "a", []byte("value-a")); err != nil {
		t.Fatalf("Set(a) error = %v", err)
	}
	if err := store.Set("svc", "b", []byte("value-b")); err != nil {
		t.Fatalf("Set(b) error = %v", err)
	}

	// Updating one key must preserve the other.
	if err := store.Set("svc", "a", []byte("value-a2")); err != nil {
		t.Fatalf("Set(a) update error = %v", err)
	}
	for key, want := range map[string]string{"a": "value-a2", "b": "value-b"} {
		got, err := store.Get("svc", key)
		if err != nil {
			t.Fatalf("Get(%s) error = %v", key, err)
		}
		if string(got) != want {
			t.Errorf("Get(%s) = %q, want %q", key, got, want)
		}
	}

	keys, err := store.List("svc")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	sort.Strings(keys)
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "b" {
		t.Errorf("List() = %v, want [a b]", keys)
	}

	// Deleting one key must keep the file and the other key.
	if err := store.Delete("svc", "a"); err != nil {
		t.Fatalf("Delete(a) error = %v", err)
	}
	if _, err := store.Get("svc", "a"); !errors.Is(err, ErrCredentialNotFound) {
		t.Errorf("Get(a) after delete error = %v, want ErrCredentialNotFound", err)
	}
	if _, err := store.Get("svc", "b"); err != nil {
		t.Errorf("Get(b) after deleting a error = %v, want nil", err)
	}

	// Deleting the last key removes the service file entirely.
	if err := store.Delete("svc", "b"); err != nil {
		t.Fatalf("Delete(b) error = %v", err)
	}
	if _, err := os.Stat(store.getFilePath("svc")); !os.IsNotExist(err) {
		t.Errorf("credential file still present after deleting last key (stat err = %v), want removed", err)
	}
	if keys, err := store.List("svc"); err != nil || len(keys) != 0 {
		t.Errorf("List() after deleting all keys = %v, %v; want empty, nil", keys, err)
	}
}

func TestFileStoreBinaryValues(t *testing.T) {
	store, err := NewFileStore(t.TempDir(), "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	if err := store.Set("svc", "key", binarySecret); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, err := store.Get("svc", "key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !bytes.Equal(got, binarySecret) {
		t.Errorf("Get() = %v, want %v", got, binarySecret)
	}
}

func TestFileStoreDeleteMissingIsNoop(t *testing.T) {
	store, err := NewFileStore(t.TempDir(), "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	if err := store.Delete("never-existed", "key"); err != nil {
		t.Errorf("Delete() on missing service error = %v, want nil", err)
	}
}

func TestFileStoreListMissingServiceIsEmpty(t *testing.T) {
	store, err := NewFileStore(t.TempDir(), "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	keys, err := store.List("never-existed")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("List() = %v, want empty", keys)
	}
}

func TestFileStoreServiceIsolation(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir, "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	if err := store.Set("svc-a", "key", []byte("a")); err != nil {
		t.Fatalf("Set(svc-a) error = %v", err)
	}
	if err := store.Set("svc-b", "key", []byte("b")); err != nil {
		t.Fatalf("Set(svc-b) error = %v", err)
	}

	for _, svc := range []string{"svc-a", "svc-b"} {
		if _, err := os.Stat(filepath.Join(dir, svc+".cred")); err != nil {
			t.Errorf("expected separate file %s.cred: %v", svc, err)
		}
	}

	if err := store.Delete("svc-a", "key"); err != nil {
		t.Fatalf("Delete(svc-a) error = %v", err)
	}
	if _, err := store.Get("svc-b", "key"); err != nil {
		t.Errorf("Get(svc-b) after deleting svc-a error = %v, want nil", err)
	}
}

func TestFileStoreEncryptDecryptRoundtrip(t *testing.T) {
	store, err := NewFileStore(t.TempDir(), "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	for name, plain := range map[string][]byte{
		"text":    []byte("super-secret"),
		"empty":   {},
		"binary":  binarySecret,
		"long":    bytes.Repeat([]byte("x"), 4096),
		"unicode": []byte("pässwörd-キー"),
	} {
		t.Run(name, func(t *testing.T) {
			encrypted, err := store.encrypt(plain)
			if err != nil {
				t.Fatalf("encrypt() error = %v", err)
			}
			decrypted, err := store.decrypt(encrypted)
			if err != nil {
				t.Fatalf("decrypt() error = %v", err)
			}
			if !bytes.Equal(decrypted, plain) {
				t.Errorf("roundtrip = %v, want %v", decrypted, plain)
			}
		})
	}
}

func TestFileStoreDecryptRejectsTamperedCiphertext(t *testing.T) {
	store, err := NewFileStore(t.TempDir(), "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	encrypted, err := store.encrypt([]byte("super-secret"))
	if err != nil {
		t.Fatalf("encrypt() error = %v", err)
	}

	// Flip the last ciphertext byte; GCM authentication must fail.
	tampered := append([]byte(nil), encrypted...)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := store.decrypt(tampered); err == nil {
		t.Error("decrypt(tampered) = nil error, want authentication failure")
	}

	// Truncating the ciphertext must also fail.
	if _, err := store.decrypt(encrypted[:len(encrypted)-1]); err == nil {
		t.Error("decrypt(truncated) = nil error, want authentication failure")
	}
}

func TestFileStoreDecryptRejectsShortInput(t *testing.T) {
	store, err := NewFileStore(t.TempDir(), "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	for name, data := range map[string][]byte{
		"empty":         {},
		"partial-salt":  bytes.Repeat([]byte{0x00}, 31),
		"salt-only":     bytes.Repeat([]byte{0x00}, 32),
		"salt-plus-one": append(bytes.Repeat([]byte{0x00}, 32), 0x01),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := store.decrypt(data); !errors.Is(err, ErrInvalidCredential) {
				t.Errorf("decrypt() error = %v, want ErrInvalidCredential", err)
			}
		})
	}
}

func TestFileStoreWrongPassphraseCannotDecrypt(t *testing.T) {
	dir := t.TempDir()

	writer, err := NewFileStore(dir, "correct-passphrase")
	if err != nil {
		t.Fatalf("NewFileStore(writer) error = %v", err)
	}
	if err := writer.Set("svc", "key", []byte("super-secret")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	reader, err := NewFileStore(dir, "wrong-passphrase")
	if err != nil {
		t.Fatalf("NewFileStore(reader) error = %v", err)
	}

	if _, err := reader.Get("svc", "key"); err == nil || errors.Is(err, ErrCredentialNotFound) {
		t.Errorf("Get() with wrong passphrase error = %v, want decrypt failure (not ErrCredentialNotFound)", err)
	}

	// Writing with the wrong passphrase must fail too: the existing file
	// cannot be decrypted before merging in the new key.
	if err := reader.Set("svc", "other", []byte("v")); err == nil {
		t.Error("Set() with wrong passphrase = nil error, want decrypt failure")
	}
	if _, err := reader.List("svc"); err == nil {
		t.Error("List() with wrong passphrase = nil error, want decrypt failure")
	}
	if err := reader.Delete("svc", "key"); err == nil {
		t.Error("Delete() with wrong passphrase = nil error, want decrypt failure")
	}
	got, err := writer.Get("svc", "key")
	if err != nil || string(got) != "super-secret" {
		t.Errorf("original credential after wrong-passphrase operations = %q, %v; want unchanged", got, err)
	}
	if _, err := writer.Get("svc", "other"); !errors.Is(err, ErrCredentialNotFound) {
		t.Errorf("new key after wrong-passphrase Set() error = %v, want ErrCredentialNotFound", err)
	}
}

func TestFileStoreCorruptFileErrors(t *testing.T) {
	dir := t.TempDir()

	store, err := NewFileStore(dir, "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	// Random bytes long enough to pass the salt/nonce size checks: the GCM
	// open must fail rather than return garbage.
	if err := os.WriteFile(filepath.Join(dir, "svc.cred"), bytes.Repeat([]byte{0xAA}, 64), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := store.Get("svc", "key"); err == nil || errors.Is(err, ErrCredentialNotFound) {
		t.Errorf("Get() corrupt file error = %v, want decrypt failure (not ErrCredentialNotFound)", err)
	}

	// Truncated garbage shorter than a salt maps to ErrInvalidCredential.
	if err := os.WriteFile(filepath.Join(dir, "svc.cred"), bytes.Repeat([]byte{0xAA}, 10), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	_, err = store.Get("svc", "key")
	if !errors.Is(err, ErrInvalidCredential) {
		t.Errorf("Get() truncated file error = %v, want ErrInvalidCredential", err)
	}
}

func TestFileStoreCiphertextIsUniquePerWrite(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir, "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	secret := []byte("super-secret-password")
	if err := store.Set("svc", "key", secret); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	first, err := os.ReadFile(filepath.Join(dir, "svc.cred"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if err := store.Set("svc", "key", secret); err != nil {
		t.Fatalf("Set() second error = %v", err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "svc.cred"))
	if err != nil {
		t.Fatalf("ReadFile() second error = %v", err)
	}

	if bytes.Equal(first, second) {
		t.Error("two writes of the same value produced identical ciphertext; salt/nonce reuse")
	}
}

func TestFileStoreFileDoesNotLeakPlaintext(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileStore(dir, "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	secret := []byte("super-secret-password")
	if err := store.Set("svc", "key", secret); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "svc.cred"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Neither the raw secret nor its base64 form (which is what the JSON
	// envelope embeds) may appear in the on-disk file.
	if bytes.Contains(data, secret) {
		t.Error("credential file contains the raw secret")
	}
	if b64 := []byte(base64.StdEncoding.EncodeToString(secret)); bytes.Contains(data, b64) {
		t.Error("credential file contains the base64-encoded secret")
	}
}

func TestFileStoreSecurePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not enforced on Windows")
	}

	dir := t.TempDir()
	base := filepath.Join(dir, "creds")

	store, err := NewFileStore(base, "passphrase")
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	info, err := os.Stat(base)
	if err != nil {
		t.Fatalf("Stat(base dir) error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("base dir mode = %o, want 700", perm)
	}

	if err := store.Set("svc", "key", []byte("v")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	credPath := filepath.Join(base, "svc.cred")
	info, err = os.Stat(credPath)
	if err != nil {
		t.Fatalf("Stat(cred file) error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("credential file mode = %o, want 600", perm)
	}

	// Re-writing an existing file must not loosen the permissions.
	if err := store.Set("svc", "key2", []byte("v2")); err != nil {
		t.Fatalf("Set() second error = %v", err)
	}
	info, err = os.Stat(credPath)
	if err != nil {
		t.Fatalf("Stat(cred file) second error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("credential file mode after rewrite = %o, want 600", perm)
	}
}

// --- EnvStore ---

func TestEnvStoreListAndCRUD(t *testing.T) {
	store := NewEnvStore("TUNNEL_UNIT_TEST")

	t.Cleanup(func() {
		_ = store.Delete("svc", "one")
		_ = store.Delete("svc", "two")
	})

	if err := store.Set("svc", "one", []byte("value-one")); err != nil {
		t.Fatalf("Set(one) error = %v", err)
	}
	if err := store.Set("svc", "two", []byte("value-two")); err != nil {
		t.Fatalf("Set(two) error = %v", err)
	}

	keys, err := store.List("svc")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	sort.Strings(keys)
	// Env var names are uppercased by the store, so listed keys come back
	// in their normalized form.
	if len(keys) != 2 || keys[0] != "ONE" || keys[1] != "TWO" {
		t.Errorf("List() = %v, want [ONE TWO]", keys)
	}

	// Overwrite
	if err := store.Set("svc", "one", []byte("rotated")); err != nil {
		t.Fatalf("Set(one) overwrite error = %v", err)
	}
	got, err := store.Get("svc", "one")
	if err != nil || string(got) != "rotated" {
		t.Errorf("Get(one) = %q, %v; want %q, nil", got, err, "rotated")
	}

	if err := store.Delete("svc", "one"); err != nil {
		t.Fatalf("Delete(one) error = %v", err)
	}
	if _, err := store.Get("svc", "one"); !errors.Is(err, ErrCredentialNotFound) {
		t.Errorf("Get(one) after delete error = %v, want ErrCredentialNotFound", err)
	}
	keys, err = store.List("svc")
	if err != nil {
		t.Fatalf("List() after delete error = %v", err)
	}
	if len(keys) != 1 || keys[0] != "TWO" {
		t.Errorf("List() after delete = %v, want [TWO]", keys)
	}
}

func TestEnvStoreKeySanitization(t *testing.T) {
	store := NewEnvStore("TUNNEL_UNIT_TEST")

	t.Cleanup(func() { _ = store.Delete("my-svc", "api-token") })

	if err := store.Set("my-svc", "api-token", []byte("v")); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if os.Getenv("TUNNEL_UNIT_TEST_MY_SVC_API_TOKEN") == "" {
		t.Error("hyphens were not normalized to underscores in the env var name")
	}

	got, err := store.Get("my-svc", "api-token")
	if err != nil || string(got) != "v" {
		t.Errorf("Get() = %q, %v; want %q, nil", got, err, "v")
	}
}

func TestEnvStoreBinaryValues(t *testing.T) {
	store := NewEnvStore("TUNNEL_UNIT_TEST")

	t.Cleanup(func() { _ = store.Delete("svc", "bin") })

	if err := store.Set("svc", "bin", binarySecret); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, err := store.Get("svc", "bin")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !bytes.Equal(got, binarySecret) {
		t.Errorf("Get() = %v, want %v", got, binarySecret)
	}
}

func TestEnvStoreEmptyCredential(t *testing.T) {
	store := NewEnvStore("TUNNEL_EMPTY_CREDENTIAL_TEST")
	t.Cleanup(func() { _ = store.Delete("svc", "empty") })

	if _, err := store.Get("svc", "empty"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("Get() before Set() error = %v, want ErrCredentialNotFound", err)
	}
	if err := store.Set("svc", "empty", []byte{}); err != nil {
		t.Fatalf("Set(empty) error = %v", err)
	}
	got, err := store.Get("svc", "empty")
	if err != nil || len(got) != 0 {
		t.Errorf("Get(empty) = %v, %v; want empty value, nil", got, err)
	}
	keys, err := store.List("svc")
	if err != nil || len(keys) != 1 || keys[0] != "EMPTY" {
		t.Errorf("List() = %v, %v; want [EMPTY], nil", keys, err)
	}
	if err := store.Delete("svc", "empty"); err != nil {
		t.Fatalf("Delete(empty) error = %v", err)
	}
	if _, err := store.Get("svc", "empty"); !errors.Is(err, ErrCredentialNotFound) {
		t.Errorf("Get() after Delete() error = %v, want ErrCredentialNotFound", err)
	}
}

func TestEnvStoreRejectsMalformedCredential(t *testing.T) {
	store := NewEnvStore("TUNNEL_MALFORMED_CREDENTIAL_TEST")
	t.Setenv(store.makeEnvKey("svc", "key"), "not-base64!")

	if _, err := store.Get("svc", "key"); err == nil || errors.Is(err, ErrCredentialNotFound) {
		t.Errorf("Get() malformed value error = %v, want decoding error", err)
	}
}

// --- Fallback selection ---

func TestNewCredentialStoreKeyringFallsBackToFile(t *testing.T) {
	keyring.MockInitWithError(errors.New("no secret service available"))
	t.Cleanup(keyring.MockInit)

	dir := t.TempDir()

	store, err := NewCredentialStore("keyring", "tunnel-unit-test", dir, "fallback-pass")
	if err != nil {
		t.Fatalf("NewCredentialStore(keyring) error = %v", err)
	}
	if _, ok := store.(*FileStore); !ok {
		t.Fatalf("NewCredentialStore(keyring) with unavailable backend = %T, want *FileStore", store)
	}

	// The fallback store must be immediately usable.
	if err := store.Set("svc", "key", []byte("v")); err != nil {
		t.Fatalf("fallback Set() error = %v", err)
	}
	got, err := store.Get("svc", "key")
	if err != nil || string(got) != "v" {
		t.Errorf("fallback Get() = %q, %v; want %q, nil", got, err, "v")
	}
}

func TestNewCredentialStoreKeyringAvailable(t *testing.T) {
	keyring.MockInit()

	store, err := NewCredentialStore("keyring", "tunnel-unit-test", t.TempDir(), "pass")
	if err != nil {
		t.Fatalf("NewCredentialStore(keyring) error = %v", err)
	}
	if _, ok := store.(*KeyringStore); !ok {
		t.Fatalf("NewCredentialStore(keyring) with available backend = %T, want *KeyringStore", store)
	}

	// The availability probe must not leave a residue credential behind.
	if _, err := store.Get("test", "test"); !errors.Is(err, ErrCredentialNotFound) {
		t.Errorf("probe residue check error = %v, want ErrCredentialNotFound", err)
	}
}

func TestNewCredentialStoreExplicitTypes(t *testing.T) {
	dir := t.TempDir()

	fileStore, err := NewCredentialStore("file", "tunnel", dir, "pass")
	if err != nil {
		t.Fatalf("NewCredentialStore(file) error = %v", err)
	}
	if _, ok := fileStore.(*FileStore); !ok {
		t.Errorf("NewCredentialStore(file) = %T, want *FileStore", fileStore)
	}

	envStore, err := NewCredentialStore("env", "tunnel", dir, "")
	if err != nil {
		t.Fatalf("NewCredentialStore(env) error = %v", err)
	}
	if _, ok := envStore.(*EnvStore); !ok {
		t.Errorf("NewCredentialStore(env) = %T, want *EnvStore", envStore)
	}
}
