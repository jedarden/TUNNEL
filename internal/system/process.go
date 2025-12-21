package system

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// ProcessManager manages background processes
type ProcessManager struct {
	processes map[string]*Process
	mu        sync.RWMutex
}

// Process represents a managed process
type Process struct {
	Name      string
	PID       int
	Cmd       *exec.Cmd
	StartTime time.Time
	Status    ProcessStatus
	mu        sync.RWMutex
}

// ProcessStatus represents the status of a process
type ProcessStatus string

const (
	StatusRunning ProcessStatus = "running"
	StatusStopped ProcessStatus = "stopped"
	StatusFailed  ProcessStatus = "failed"
	StatusUnknown ProcessStatus = "unknown"
)

// NewProcessManager creates a new process manager
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		processes: make(map[string]*Process),
	}
}

// Start starts a new process
func (pm *ProcessManager) Start(name string, command string, args ...string) (*Process, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Check if process already exists
	if proc, exists := pm.processes[name]; exists {
		if proc.IsRunning() {
			return nil, fmt.Errorf("process %s is already running (PID: %d)", name, proc.PID)
		}
	}

	// Create command
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	// Create process object
	proc := &Process{
		Name:      name,
		PID:       cmd.Process.Pid,
		Cmd:       cmd,
		StartTime: time.Now(),
		Status:    StatusRunning,
	}

	// Store process
	pm.processes[name] = proc

	// Monitor process in background
	go pm.monitorProcess(proc)

	return proc, nil
}

// Stop stops a process gracefully
func (pm *ProcessManager) Stop(name string) error {
	pm.mu.RLock()
	proc, exists := pm.processes[name]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process %s not found", name)
	}

	return proc.Stop()
}

// Kill forcefully kills a process
func (pm *ProcessManager) Kill(name string) error {
	pm.mu.RLock()
	proc, exists := pm.processes[name]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("process %s not found", name)
	}

	return proc.Kill()
}

// Get retrieves a process by name
func (pm *ProcessManager) Get(name string) (*Process, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	proc, exists := pm.processes[name]
	return proc, exists
}

// List returns all managed processes
func (pm *ProcessManager) List() []*Process {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	procs := make([]*Process, 0, len(pm.processes))
	for _, proc := range pm.processes {
		procs = append(procs, proc)
	}
	return procs
}

// StopAll stops all managed processes
func (pm *ProcessManager) StopAll() error {
	pm.mu.RLock()
	names := make([]string, 0, len(pm.processes))
	for name := range pm.processes {
		names = append(names, name)
	}
	pm.mu.RUnlock()

	var lastErr error
	for _, name := range names {
		if err := pm.Stop(name); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// monitorProcess monitors a process and updates its status
func (pm *ProcessManager) monitorProcess(proc *Process) {
	err := proc.Cmd.Wait()

	proc.mu.Lock()
	defer proc.mu.Unlock()

	if err != nil {
		proc.Status = StatusFailed
	} else {
		proc.Status = StatusStopped
	}
}

// IsRunning checks if the process is currently running
func (p *Process) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.Cmd == nil || p.Cmd.Process == nil {
		return false
	}

	// Check if process exists
	if err := p.Cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return false
	}

	return p.Status == StatusRunning
}

// Stop stops the process gracefully (SIGTERM)
func (p *Process) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Cmd == nil || p.Cmd.Process == nil {
		return fmt.Errorf("process not started")
	}

	// Send SIGTERM
	if err := p.Cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM: %w", err)
	}

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		done <- p.Cmd.Wait()
	}()

	select {
	case <-done:
		p.Status = StatusStopped
		return nil
	case <-time.After(5 * time.Second):
		// Timeout, force kill
		return p.Kill()
	}
}

// Kill forcefully kills the process (SIGKILL)
func (p *Process) Kill() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Cmd == nil || p.Cmd.Process == nil {
		return fmt.Errorf("process not started")
	}

	if err := p.Cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	p.Status = StatusStopped
	return nil
}

// GetStatus returns the current status of the process
func (p *Process) GetStatus() ProcessStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Status
}

// GetPID returns the process ID
func (p *Process) GetPID() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.PID
}

// GetUptime returns how long the process has been running
func (p *Process) GetUptime() time.Duration {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return time.Since(p.StartTime)
}

// FindOrphanedProcesses finds processes that may have been started by tunnel but are not managed
func FindOrphanedProcesses(processNames []string) ([]int, error) {
	var orphanedPIDs []int

	for _, name := range processNames {
		// Use pgrep to find processes
		cmd := exec.Command("pgrep", "-x", name)
		output, err := cmd.Output()
		if err != nil {
			// pgrep returns exit code 1 if no processes found
			continue
		}

		// Parse PIDs (simple approach)
		// In production, you'd want more robust parsing
		_ = output // PIDs would be parsed from output
		// orphanedPIDs = append(orphanedPIDs, ...)
	}

	return orphanedPIDs, nil
}

// KillProcessByPID kills a process by PID
func KillProcessByPID(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	return nil
}

// IsProcessRunning checks if a process with given PID is running
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
