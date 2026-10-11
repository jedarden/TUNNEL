// Package procman manages long-running provider subprocesses: process
// lifecycle (start/stop with graceful shutdown), liveness tracking, and a
// bounded in-memory ring of captured log output. It exists so providers that
// work by wrapping a daemon-style CLI (ssh, code tunnel, ...) share one
// correct implementation instead of each hand-rolling process handling.
package procman

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// defaultMaxLogs is the number of retained log entries per process manager.
const defaultMaxLogs = 500

// Manager owns at most one subprocess at a time. A Manager can be reused
// after Stop: logs accumulate across runs, state resets on the next Start.
type Manager struct {
	mu         sync.Mutex
	source     string
	bin        string
	args       []string
	cmd        *exec.Cmd
	started    time.Time
	exited     bool
	exitErr    error
	done       chan struct{}
	generation uint64
	logs       []providers.LogEntry
	maxLogs    int
}

// New creates a Manager whose captured log entries carry source as their
// LogEntry.Source field.
func New(source string) *Manager {
	return &Manager{
		source:  source,
		maxLogs: defaultMaxLogs,
	}
}

// Start launches bin with args. It returns an error if the manager already
// has a running process, if bin cannot be found in PATH, or if the process
// fails to spawn. stdout and stderr are captured into the log ring.
func (m *Manager) Start(bin string, args []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && !m.exited {
		return providers.ErrAlreadyConnected
	}

	path, err := exec.LookPath(bin)
	if err != nil {
		return fmt.Errorf("%w: %v", providers.ErrNotInstalled, err)
	}

	cmd := exec.Command(path, args...)
	stdout := m.newLineWriter()
	stderr := m.newLineWriter()
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %v", providers.ErrConnectionFailed, err)
	}

	m.bin = path
	m.args = append([]string(nil), args...)
	m.cmd = cmd
	m.started = time.Now()
	m.exited = false
	m.exitErr = nil
	m.generation++
	generation := m.generation
	m.done = make(chan struct{})

	m.appendLog("Info", fmt.Sprintf("started %s %s", path, strings.Join(args, " ")))

	go func() {
		err := cmd.Wait()
		stdout.close()
		stderr.close()
		m.recordExit(generation, err)
	}()

	return nil
}

// Running reports whether the managed process is alive.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cmd != nil && !m.exited
}

// PID returns the operating-system process id, or 0 when not running.
func (m *Manager) PID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd == nil || m.exited || m.cmd.Process == nil {
		return 0
	}
	return m.cmd.Process.Pid
}

// StartedAt returns the time the current (or most recent) process started.
func (m *Manager) StartedAt() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

// Stop terminates the managed process: an interrupt signal first, then a
// kill after grace elapses. Stopping an already-stopped manager is a no-op.
func (m *Manager) Stop(grace time.Duration) error {
	m.mu.Lock()
	cmd := m.cmd
	running := m.cmd != nil && !m.exited
	done := m.done
	m.mu.Unlock()

	if !running || cmd == nil || cmd.Process == nil {
		return nil
	}

	// Interrupt first so well-behaved children (ssh) tear down cleanly.
	_ = cmd.Process.Signal(os.Interrupt)

	if done != nil {
		select {
		case <-done:
			return nil
		case <-time.After(grace):
		}
	}

	_ = cmd.Process.Kill()
	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			return fmt.Errorf("%w: process %d did not exit after kill", providers.ErrCommandFailed, cmd.Process.Pid)
		}
	}
	return nil
}

// Logs returns captured log entries no older than since, oldest first.
func (m *Manager) Logs(since time.Time) []providers.LogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	logs := make([]providers.LogEntry, 0, len(m.logs))
	for _, entry := range m.logs {
		if !entry.Timestamp.Before(since) {
			logs = append(logs, entry)
		}
	}
	return logs
}

// RecentErrors returns up to n error-level messages, most recent last.
// Providers use it to give Connect failures actionable text.
func (m *Manager) RecentErrors(n int) []string {
	if n <= 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []string
	for _, entry := range m.logs {
		if entry.Level == "Error" {
			errs = append(errs, entry.Message)
		}
	}
	if len(errs) > n {
		errs = errs[len(errs)-n:]
	}
	return append([]string(nil), errs...)
}

// Command returns a printable form of the managed command line.
func (m *Manager) Command() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bin == "" {
		return ""
	}
	return strings.Join(append([]string{m.bin}, m.args...), " ")
}

func (m *Manager) recordExit(generation uint64, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ignore exits recorded after a subsequent Start reset our channel.
	if m.exited || generation != m.generation {
		return
	}
	m.exited = true
	m.exitErr = err
	if err != nil {
		m.appendLog("Error", fmt.Sprintf("process exited: %v", err))
	} else {
		m.appendLog("Info", "process exited cleanly")
	}
	if m.done != nil {
		close(m.done)
	}
}

func (m *Manager) appendLog(level, message string) {
	entry := providers.LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    m.source,
	}
	m.logs = append(m.logs, entry)
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
}

// classifyLevel guesses a log level from message content the same way the
// tailscale provider's journalctl parser does.
func classifyLevel(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "error"), strings.Contains(lower, "fatal"), strings.Contains(lower, "denied"), strings.Contains(lower, "failed"):
		return "Error"
	case strings.Contains(lower, "warn"):
		return "Warning"
	default:
		return "Info"
	}
}

// lineWriter converts a subprocess output stream into timestamped log
// entries. It buffers partial lines until a newline arrives. It shares the
// manager's mutex so log appends serialize with all other Manager calls.
type lineWriter struct {
	mu     *sync.Mutex
	owner  *Manager
	buf    []byte
	closed bool
}

func (m *Manager) newLineWriter() *lineWriter {
	return &lineWriter{mu: &m.mu, owner: m}
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return len(p), nil
	}
	w.buf = append(w.buf, p...)
	for {
		idx := indexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := strings.TrimSpace(string(w.buf[:idx]))
		w.buf = w.buf[idx+1:]
		if line != "" {
			w.owner.appendLogLocked(classifyLevel(line), line)
		}
	}
	return len(p), nil
}

// close flushes any trailing partial line. Called by the manager's Wait
// goroutine after the process exits.
func (w *lineWriter) close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return
	}
	w.closed = true
	if line := strings.TrimSpace(string(w.buf)); line != "" {
		w.owner.appendLogLocked(classifyLevel(line), line)
	}
	w.buf = nil
}

// appendLogLocked appends without re-acquiring the manager mutex; callers
// must already hold it. (Write/close hold the same mutex the manager uses,
// so this is race-free.)
func (m *Manager) appendLogLocked(level, message string) {
	m.logs = append(m.logs, providers.LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Source:    m.source,
	})
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}
