package upgrade

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors the binary for changes and triggers graceful restarts
type Watcher struct {
	binaryPath string
	watcher    *fsnotify.Watcher
	logger     *log.Logger
	onUpgrade  func()
	debounce   time.Duration
	lastMod    time.Time
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
		binaryPath: binaryPath,
		watcher:    watcher,
		logger:     logger,
		debounce:   2 * time.Second, // Wait 2 seconds after last change before restarting
	}

	return w, nil
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
		w.logger.Printf("Binary update detected! Preparing to restart...")
	}

	// Call the upgrade callback (for cleanup)
	if w.onUpgrade != nil {
		w.onUpgrade()
	}

	// Perform the restart
	w.restart()
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
