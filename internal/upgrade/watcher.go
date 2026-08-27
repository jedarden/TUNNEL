package upgrade

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors the binary for changes and triggers graceful restarts
type Watcher struct {
	binaryPath    string
	watcher       *fsnotify.Watcher
	logger        *log.Logger
	onUpgrade     func()
	debounce      time.Duration
	lastMod       time.Time
	repoOwner     string // GitHub repo owner (e.g., "jedarden")
	repoName      string // GitHub repo name (e.g., "tunnel")
	currentVersion string // Current version (for finding the right release)
}

// NewWatcher creates a new binary upgrade watcher
func NewWatcher(logger *log.Logger) (*Watcher, error) {
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks to get the actual binary path
	binaryPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		binaryPath = execPath
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	w := &Watcher{
		binaryPath:    binaryPath,
		watcher:       watcher,
		logger:        logger,
		debounce:      2 * time.Second, // Wait 2 seconds after last change before restarting
		repoOwner:     "jedarden",
		repoName:      "tunnel",
		currentVersion: "", // Will be set if available
	}

	return w, nil
}

// SetVersion sets the current version for release checking
func (w *Watcher) SetVersion(version string) {
	w.currentVersion = version
}

// Start begins watching the binary for changes
func (w *Watcher) Start(onUpgrade func()) error {
	w.onUpgrade = onUpgrade

	// Get initial modification time
	info, err := os.Stat(w.binaryPath)
	if err != nil {
		return fmt.Errorf("failed to stat binary: %w", err)
	}
	w.lastMod = info.ModTime()

	// Watch the directory containing the binary (fsnotify doesn't track replaced files well)
	dir := filepath.Dir(w.binaryPath)
	if err := w.watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	go w.watch()

	if w.logger != nil {
		w.logger.Printf("Hot-swap watcher started for: %s", w.binaryPath)
	}

	return nil
}

func (w *Watcher) watch() {
	var debounceTimer *time.Timer

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Check if this is our binary
			eventPath, _ := filepath.Abs(event.Name)
			binaryPath, _ := filepath.Abs(w.binaryPath)

			if eventPath != binaryPath {
				continue
			}

			// Check for write or create events
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				// Debounce rapid changes (e.g., during copy operation)
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(w.debounce, func() {
					w.checkAndRestart()
				})
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			if w.logger != nil {
				w.logger.Printf("Watcher error: %v", err)
			}
		}
	}
}

func (w *Watcher) checkAndRestart() {
	// Verify the binary has actually changed
	info, err := os.Stat(w.binaryPath)
	if err != nil {
		if w.logger != nil {
			w.logger.Printf("Failed to stat binary for upgrade check: %v", err)
		}
		return
	}

	if !info.ModTime().After(w.lastMod) {
		return // No actual change
	}

	w.lastMod = info.ModTime()

	if w.logger != nil {
		w.logger.Printf("Binary update detected! Verifying integrity before restart...")
	}

	// Verify the new binary's integrity
	verified, err := w.verifyBinaryIntegrity()
	if err != nil {
		if w.logger != nil {
			w.logger.Printf("Integrity verification failed: %v", err)
			w.logger.Printf("NOT restarting into potentially corrupted binary")
		}
		return
	}

	if !verified {
		if w.logger != nil {
			w.logger.Printf("Binary integrity check failed - aborting restart")
		}
		return
	}

	if w.logger != nil {
		w.logger.Printf("Binary integrity verified - preparing to restart...")
	}

	// Call the upgrade callback (for cleanup)
	if w.onUpgrade != nil {
		w.onUpgrade()
	}

	// Perform the restart
	w.restart()
}

// verifyBinaryIntegrity checks the new binary's integrity using multiple methods
func (w *Watcher) verifyBinaryIntegrity() (bool, error) {
	// Method 1: Try to verify against GitHub release checksums
	checksumVerified, err := w.verifyWithReleaseChecksums()
	if err != nil {
		if w.logger != nil {
			w.logger.Printf("Release checksum verification unavailable: %v", err)
			w.logger.Printf("Falling back to basic sanity checks...")
		}
		// Fall through to basic checks
	} else if checksumVerified {
		if w.logger != nil {
			w.logger.Printf("Binary SHA256 matches release checksums")
		}
		return true, nil
	} else {
		return false, fmt.Errorf("binary SHA256 does not match any expected checksum")
	}

	// Method 2: Basic sanity checks (fallback when checksums unavailable)
	return w.basicSanityChecks()
}

// verifyWithReleaseChecksums downloads and verifies against the release checksums.txt
func (w *Watcher) verifyWithReleaseChecksums() (bool, error) {
	// Calculate the SHA256 of the binary on disk
	binaryHash, err := w.calculateBinarySHA256()
	if err != nil {
		return false, fmt.Errorf("failed to calculate binary SHA256: %w", err)
	}

	if w.logger != nil {
		w.logger.Printf("Binary SHA256: %s", binaryHash)
	}

	// Fetch checksums from GitHub releases
	checksums, err := w.fetchReleaseChecksums()
	if err != nil {
		return false, fmt.Errorf("failed to fetch release checksums: %w", err)
	}

	// Check if our binary's hash is in the expected checksums
	for _, expectedHash := range checksums {
		if strings.EqualFold(binaryHash, expectedHash) {
			return true, nil
		}
	}

	return false, nil
}

// calculateBinarySHA256 computes the SHA256 hash of the binary
func (w *Watcher) calculateBinarySHA256() (string, error) {
	file, err := os.Open(w.binaryPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// fetchReleaseChecksums downloads checksums.txt from GitHub releases
func (w *Watcher) fetchReleaseChecksums() ([]string, error) {
	// Try multiple common tags for the release
	tags := []string{"latest"}

	// If we have a version, try that too (strip 'v' prefix if present)
	if w.currentVersion != "" && w.currentVersion != "dev" && w.currentVersion != "unknown" {
		version := strings.TrimPrefix(w.currentVersion, "v")
		tags = append([]string{version}, tags...)
	}

	var lastErr error
	for _, tag := range tags {
		url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/checksums.txt",
			w.repoOwner, w.repoName, tag)

		checksums, err := w.downloadChecksums(url)
		if err == nil {
			return checksums, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed to fetch checksums from all release tags: %w", lastErr)
}

// downloadChecksums downloads and parses a checksums.txt file
func (w *Watcher) downloadChecksums(checksumsURL string) ([]string, error) {
	// Validate URL
	parsedURL, err := url.Parse(checksumsURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "https" || parsedURL.Host != "github.com" {
		return nil, fmt.Errorf("only https://github.com URLs are allowed")
	}

	// Download with timeout
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(checksumsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Parse checksums
	var checksums []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Checksums.txt format: "hash  filename"
		// We extract just the hash part
		parts := strings.Fields(line)
		if len(parts) >= 1 {
			checksums = append(checksums, parts[0])
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read checksums: %w", err)
	}

	if len(checksums) == 0 {
		return nil, fmt.Errorf("no checksums found in file")
	}

	if w.logger != nil {
		w.logger.Printf("Downloaded %d checksums from release", len(checksums))
	}

	return checksums, nil
}

// basicSanityChecks performs minimal integrity checks when checksums unavailable
func (w *Watcher) basicSanityChecks() (bool, error) {
	info, err := os.Stat(w.binaryPath)
	if err != nil {
		return false, fmt.Errorf("failed to stat binary: %w", err)
	}

	// Check 1: File must be non-zero
	if info.Size() == 0 {
		return false, fmt.Errorf("binary is zero bytes")
	}

	// Check 2: Executable bit must be set
	mode := info.Mode()
	if mode.Perm()&0111 == 0 {
		return false, fmt.Errorf("binary is not executable (missing executable permission)")
	}

	// Check 3: Verify magic bytes for the platform
	if err := w.verifyMagicBytes(); err != nil {
		return false, fmt.Errorf("magic bytes verification failed: %w", err)
	}

	if w.logger != nil {
		w.logger.Printf("Basic sanity checks passed (size=%d, executable, valid magic bytes)", info.Size())
	}

	return true, nil
}

// verifyMagicBytes checks the binary has correct magic bytes for the platform
func (w *Watcher) verifyMagicBytes() error {
	file, err := os.Open(w.binaryPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read first 8 bytes for magic number check
	magic := make([]byte, 8)
	if _, err := io.ReadFull(file, magic); err != nil {
		return err
	}

	// Platform-specific magic bytes
	switch runtime.GOOS {
	case "linux", "freebsd", "openbsd", "netbsd":
		// ELF magic: 0x7F 'E' 'L' 'F'
		if magic[0] != 0x7F || magic[1] != 'E' || magic[2] != 'L' || magic[3] != 'F' {
			return fmt.Errorf("invalid ELF magic bytes")
		}
	case "darwin":
		// Mach-O magic: 0xFEEDFACE or 0xFEEDFACF (64-bit)
		magic32 := []byte{0xFE, 0xED, 0xFA, 0xCE}
		magic64 := []byte{0xFE, 0xED, 0xFA, 0xCF}
		if !bytesStartsWith(magic, magic32) && !bytesStartsWith(magic, magic64) {
			return fmt.Errorf("invalid Mach-O magic bytes")
		}
	case "windows":
		// PE/COFF magic: 'MZ' header
		if magic[0] != 'M' || magic[1] != 'Z' {
			return fmt.Errorf("invalid PE/COFF magic bytes (MZ header missing)")
		}
	default:
		// Unknown platform - skip magic check
		if w.logger != nil {
			w.logger.Printf("Unknown platform %s - skipping magic byte check", runtime.GOOS)
		}
	}

	return nil
}

// bytesStartsWith checks if a byte slice starts with a prefix
func bytesStartsWith(b, prefix []byte) bool {
	if len(b) < len(prefix) {
		return false
	}
	for i := range prefix {
		if b[i] != prefix[i] {
			return false
		}
	}
	return true
}

func (w *Watcher) restart() {
	if w.logger != nil {
		w.logger.Printf("Restarting with new binary...")
	}

	// Get the current executable path (may have changed if it was a symlink)
	execPath, err := os.Executable()
	if err != nil {
		if w.logger != nil {
			w.logger.Printf("Failed to get executable path: %v", err)
		}
		return
	}

	// Use syscall.Exec to replace the current process with the new binary
	// This preserves the PID and is the cleanest way to do a hot restart
	err = syscall.Exec(execPath, os.Args, os.Environ())
	if err != nil {
		if w.logger != nil {
			w.logger.Printf("Failed to exec new binary: %v", err)
		}
	}
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	return w.watcher.Close()
}

// GetBinaryPath returns the path being watched
func (w *Watcher) GetBinaryPath() string {
	return w.binaryPath
}
