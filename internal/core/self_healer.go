package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SelfHealer performs proactive maintenance to prevent bead starvation
type SelfHealer struct {
	workspaceDir      string
	binaryPath        string
	beadValidator     *BeadValidator
	auditLogger       *AuditLogger
	eventPublisher    *EventPublisher

	// Configuration
	checkInterval     time.Duration
	autoRepairEnabled bool
	dryRun            bool

	// Thresholds
	staleAssigneeThreshold time.Duration // Default: 24 hours
	longRunningThreshold   time.Duration // Default: 7 days
	checkpointSyncInterval time.Duration // Default: 24 hours

	// State
	running           bool
	lastCheckTime     time.Time
	lastCheckpointSync time.Time
	mu                sync.RWMutex

	// Health tracking
	healingHistory    []HealingEvent
	maxHistorySize    int
}

// HealingEvent represents a single healing action
type HealingEvent struct {
	Timestamp    time.Time              `json:"timestamp"`
	EventType    string                 `json:"event_type"`
	Description  string                 `json:"description"`
	Details      map[string]interface{} `json:"details"`
	Success      bool                   `json:"success"`
	Error        string                 `json:"error,omitempty"`
}

// SelfHealerConfig holds configuration for the self-healer
type SelfHealerConfig struct {
	WorkspaceDir           string
	BinaryPath             string
	CheckInterval          time.Duration
	AutoRepairEnabled      bool
	DryRun                 bool
	AuditLogger            *AuditLogger
	EventPublisher         *EventPublisher
	MaxHistorySize         int
	StaleAssigneeThreshold time.Duration // Default: 24 hours
	LongRunningThreshold   time.Duration // Default: 7 days
	CheckpointSyncInterval time.Duration // Default: 24 hours
}

// NewSelfHealer creates a new self-healer
func NewSelfHealer(config *SelfHealerConfig) *SelfHealer {
	if config == nil {
		config = &SelfHealerConfig{}
	}

	// Set defaults
	if config.CheckInterval == 0 {
		config.CheckInterval = 5 * time.Minute
	}
	if config.MaxHistorySize == 0 {
		config.MaxHistorySize = 100
	}
	if config.BinaryPath == "" {
		config.BinaryPath = "bead"
	}
	if config.StaleAssigneeThreshold == 0 {
		config.StaleAssigneeThreshold = 24 * time.Hour
	}
	if config.LongRunningThreshold == 0 {
		config.LongRunningThreshold = 7 * 24 * time.Hour
	}
	if config.CheckpointSyncInterval == 0 {
		config.CheckpointSyncInterval = 24 * time.Hour
	}

	// Create bead validator
	validatorConfig := &ValidatorConfig{
		WorkspaceDir: config.WorkspaceDir,
		BinaryPath:   config.BinaryPath,
		DryRun:       config.DryRun,
		AuditLogger:  config.AuditLogger,
	}

	return &SelfHealer{
		workspaceDir:           config.WorkspaceDir,
		binaryPath:             config.BinaryPath,
		beadValidator:          NewBeadValidator(validatorConfig),
		auditLogger:            config.AuditLogger,
		eventPublisher:         config.EventPublisher,
		checkInterval:          config.CheckInterval,
		autoRepairEnabled:      config.AutoRepairEnabled,
		dryRun:                 config.DryRun,
		staleAssigneeThreshold: config.StaleAssigneeThreshold,
		longRunningThreshold:   config.LongRunningThreshold,
		checkpointSyncInterval: config.CheckpointSyncInterval,
		healingHistory:         make([]HealingEvent, 0, config.MaxHistorySize),
		maxHistorySize:         config.MaxHistorySize,
	}
}

// Start begins the self-healer loop
func (h *SelfHealer) Start(ctx context.Context) error {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return fmt.Errorf("self-healer already running")
	}
	h.running = true
	h.mu.Unlock()

	ticker := time.NewTicker(h.checkInterval)
	defer ticker.Stop()

	// Run initial check
	if err := h.runHealingCycle(); err != nil {
		h.logEvent("initial_check_failed", "Initial healing cycle failed", map[string]interface{}{
			"error": err.Error(),
		}, false)
	}

	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			h.running = false
			h.mu.Unlock()
			return nil
		case <-ticker.C:
			if err := h.runHealingCycle(); err != nil {
				h.logEvent("healing_cycle_failed", "Healing cycle failed", map[string]interface{}{
					"error": err.Error(),
				}, false)
			}
		}
	}
}

// Stop stops the self-healer
func (h *SelfHealer) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.running = false
}

// IsRunning returns whether the self-healer is running
func (h *SelfHealer) IsRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}

// runHealingCycle runs a complete proactive maintenance cycle
func (h *SelfHealer) runHealingCycle() error {
	h.mu.Lock()
	h.lastCheckTime = time.Now()
	h.mu.Unlock()

	issuesDetected := 0
	issuesFixed := 0

	// 1. Check for stale assignees
	staleAssignees, err := h.detectStaleAssignees()
	if err != nil {
		return fmt.Errorf("failed to detect stale assignees: %w", err)
	}
	issuesDetected += len(staleAssignees)

	if len(staleAssignees) > 0 && h.autoRepairEnabled {
		fixed, err := h.clearStaleAssignees(staleAssignees)
		if err != nil {
			h.logEvent("clear_stale_assignees_failed", "Failed to clear stale assignees", map[string]interface{}{
				"count": len(staleAssignees),
				"error": err.Error(),
			}, false)
		} else {
			issuesFixed += fixed
			h.logEvent("cleared_stale_assignees", "Cleared stale assignees", map[string]interface{}{
				"count": fixed,
			}, true)
		}
	}

	// 2. Check for long-running open beads
	longRunning, err := h.detectLongRunningBeads()
	if err != nil {
		return fmt.Errorf("failed to detect long-running beads: %w", err)
	}
	issuesDetected += len(longRunning)

	if len(longRunning) > 0 {
		h.logEvent("detected_long_running_beads", "Detected long-running open beads", map[string]interface{}{
			"count": len(longRunning),
			"beads": h.getBeadIDs(longRunning),
		}, true)
	}

	// 3. Check for blocked beads with closed dependencies
	blockedWithClosedDeps, err := h.detectBlockedWithClosedDependencies()
	if err != nil {
		return fmt.Errorf("failed to detect blocked beads: %w", err)
	}
	issuesDetected += len(blockedWithClosedDeps)

	if len(blockedWithClosedDeps) > 0 && h.autoRepairEnabled {
		fixed, err := h.unblockClosedDependencies(blockedWithClosedDeps)
		if err != nil {
			h.logEvent("unblock_closed_deps_failed", "Failed to unblock closed dependencies", map[string]interface{}{
				"count": len(blockedWithClosedDeps),
				"error": err.Error(),
			}, false)
		} else {
			issuesFixed += fixed
			h.logEvent("unblocked_closed_dependencies", "Unblocked beads with closed dependencies", map[string]interface{}{
				"count": fixed,
			}, true)
		}
	}

	// 4. Check workspace configuration health
	validationResults, err := h.beadValidator.ValidateAll()
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	fixableIssues := 0
	for _, result := range validationResults {
		if !result.Passed && result.Fixable {
			fixableIssues++
		}
	}
	issuesDetected += fixableIssues

	if fixableIssues > 0 && h.autoRepairEnabled {
		if err := h.beadValidator.FixAll(validationResults); err != nil {
			h.logEvent("auto_fix_validation_failed", "Failed to auto-fix validation issues", map[string]interface{}{
				"error": err.Error(),
			}, false)
		} else {
			issuesFixed += h.beadValidator.GetFixesApplied()
			h.logEvent("auto_fixed_validation", "Auto-fixed validation issues", map[string]interface{}{
				"count": h.beadValidator.GetFixesApplied(),
			}, true)
		}
	}

	// 5. Periodic checkpoint sync (every 24 hours by default)
	if time.Since(h.lastCheckpointSync) > h.checkpointSyncInterval {
		if err := h.syncCheckpoint(); err != nil {
			h.logEvent("checkpoint_sync_failed", "Periodic checkpoint sync failed", map[string]interface{}{
				"error": err.Error(),
			}, false)
		} else {
			h.lastCheckpointSync = time.Now()
			h.logEvent("checkpoint_synced", "Periodic checkpoint sync completed", nil, true)
		}
	}

	// Publish summary event
	if h.eventPublisher != nil {
		event := map[string]interface{}{
			"type":            "healing_cycle_completed",
			"issues_detected": issuesDetected,
			"issues_fixed":    issuesFixed,
			"timestamp":       time.Now().Format(time.RFC3339),
		}
		_ = h.eventPublisher.Publish(event)
	}

	return nil
}

// detectStaleAssignees finds beads that have been assigned but open for too long
func (h *SelfHealer) detectStaleAssignees() ([]Bead, error) {
	beads, err := h.listBeads("--status", "open")
	if err != nil {
		return nil, err
	}

	var staleAssignees []Bead
	threshold := time.Now().Add(-h.staleAssigneeThreshold)

	for _, bead := range beads {
		if bead.Assignee != "" && bead.Status == BeadStateOpen {
			// Check when the bead was last updated
			if bead.UpdatedAt.Before(threshold) || bead.UpdatedAt.IsZero() {
				staleAssignees = append(staleAssignees, bead)
			}
		}
	}

	return staleAssignees, nil
}

// clearStaleAssignees clears stale assignees from beads
func (h *SelfHealer) clearStaleAssignees(beads []Bead) (int, error) {
	fixed := 0

	for _, bead := range beads {
		if h.dryRun {
			fixed++
			continue
		}

		args := []string{"update", bead.ID, "--clear-assignee"}
		cmd := exec.Command(h.binaryPath, args...)
		cmd.Dir = h.workspaceDir

		if output, err := cmd.CombinedOutput(); err != nil {
			return fixed, fmt.Errorf("failed to clear assignee for bead %s: %w\nOutput: %s", bead.ID, err, string(output))
		}

		fixed++
	}

	return fixed, nil
}

// detectLongRunningBeads finds beads that have been open for a very long time
func (h *SelfHealer) detectLongRunningBeads() ([]Bead, error) {
	beads, err := h.listBeads("--status", "open")
	if err != nil {
		return nil, err
	}

	var longRunning []Bead
	threshold := time.Now().Add(-h.longRunningThreshold)

	for _, bead := range beads {
		if bead.CreatedAt.Before(threshold) && !bead.CreatedAt.IsZero() {
			longRunning = append(longRunning, bead)
		}
	}

	return longRunning, nil
}

// detectBlockedWithClosedDependencies finds beads blocked by closed dependencies
func (h *SelfHealer) detectBlockedWithClosedDependencies() ([]Bead, error) {
	beads, err := h.listBeads("--status", "open")
	if err != nil {
		return nil, err
	}

	var blocked []Bead

	for _, bead := range beads {
		if len(bead.BlockedBy) > 0 {
			// Check if any blocking beads are closed
			for _, blockerID := range bead.BlockedBy {
				if h.isBeadClosed(blockerID) {
					blocked = append(blocked, bead)
					break
				}
			}
		}
	}

	return blocked, nil
}

// unblockClosedDependencies removes closed dependencies from blockedBy list
func (h *SelfHealer) unblockClosedDependencies(beads []Bead) (int, error) {
	fixed := 0

	for _, bead := range beads {
		// Find which dependencies are closed
		var closedDependencies []string
		for _, blockerID := range bead.BlockedBy {
			if h.isBeadClosed(blockerID) {
				closedDependencies = append(closedDependencies, blockerID)
			}
		}

		if len(closedDependencies) == 0 {
			continue
		}

		if h.dryRun {
			fixed++
			continue
		}

		// Remove closed dependencies from blockedBy
		args := []string{"ref", bead.ID, "remove-blockedBy"}
		args = append(args, closedDependencies...)

		cmd := exec.Command(h.binaryPath, args...)
		cmd.Dir = h.workspaceDir

		if output, err := cmd.CombinedOutput(); err != nil {
			return fixed, fmt.Errorf("failed to unblock bead %s: %w\nOutput: %s", bead.ID, err, string(output))
		}

		fixed++
	}

	return fixed, nil
}

// isBeadClosed checks if a bead is closed
func (h *SelfHealer) isBeadClosed(beadID string) bool {
	cmd := exec.Command(h.binaryPath, "show", beadID, "--json")
	cmd.Dir = h.workspaceDir

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	var bead Bead
	if err := json.Unmarshal(output, &bead); err != nil {
		return false
	}

	return bead.Status == BeadStateClosed
}

// syncCheckpoint syncs the checkpoint
func (h *SelfHealer) syncCheckpoint() error {
	if h.dryRun {
		return nil
	}

	cmd := exec.Command(h.binaryPath, "sync", "flush-only")
	cmd.Dir = h.workspaceDir

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkpoint sync failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// listBeads lists beads with optional filters
func (h *SelfHealer) listBeads(filters ...string) ([]Bead, error) {
	args := []string{"list", "--json"}
	args = append(args, filters...)

	cmd := exec.Command(h.binaryPath, args...)
	cmd.Dir = h.workspaceDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run bead list: %w", err)
	}

	var beads []Bead
	if err := json.Unmarshal(output, &beads); err != nil {
		return nil, fmt.Errorf("failed to parse bead list output: %w", err)
	}

	return beads, nil
}

// getBeadIDs extracts IDs from a list of beads
func (h *SelfHealer) getBeadIDs(beads []Bead) []string {
	ids := make([]string, len(beads))
	for i, bead := range beads {
		ids[i] = bead.ID
	}
	return ids
}

// logEvent logs a healing event
func (h *SelfHealer) logEvent(eventType, description string, details map[string]interface{}, success bool) {
	if details == nil {
		details = make(map[string]interface{})
	}

	event := HealingEvent{
		Timestamp:   time.Now(),
		EventType:   eventType,
		Description: description,
		Details:     details,
		Success:     success,
	}

	if !success {
		event.Error = fmt.Sprintf("%v", details["error"])
	}

	h.mu.Lock()
	h.healingHistory = append(h.healingHistory, event)
	if len(h.healingHistory) > h.maxHistorySize {
		h.healingHistory = h.healingHistory[1:]
	}
	h.mu.Unlock()

	// Log to audit logger
	if h.auditLogger != nil {
		auditEvent := map[string]interface{}{
			"action":      eventType,
			"description": description,
			"success":     success,
			"timestamp":   event.Timestamp.Format(time.RFC3339),
		}
		for k, v := range details {
			auditEvent[k] = v
		}
		_ = h.auditLogger.Log(auditEvent)
	}

	// Publish event
	if h.eventPublisher != nil {
		pubEvent := map[string]interface{}{
			"type":        "self_healer_event",
			"event_type":  eventType,
			"description": description,
			"success":     success,
			"timestamp":   event.Timestamp.Format(time.RFC3339),
		}
		for k, v := range details {
			pubEvent[k] = v
		}
		_ = h.eventPublisher.Publish(pubEvent)
	}
}

// GetHistory returns the healing history
func (h *SelfHealer) GetHistory() []HealingEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	history := make([]HealingEvent, len(h.healingHistory))
	copy(history, h.healingHistory)
	return history
}

// GetWorkspace returns the self-healer's workspace directory
func (h *SelfHealer) GetWorkspace() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.workspaceDir
}

// GetCheckInterval returns the check interval
func (h *SelfHealer) GetCheckInterval() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.checkInterval
}

// RunScheduledHealing runs a single healing cycle for scheduled execution
func (h *SelfHealer) RunScheduledHealing() (string, error) {
	if err := h.runHealingCycle(); err != nil {
		return "", err
	}

	// Generate summary
	summary := h.generateSummary()
	return summary, nil
}

// generateSummary creates a human-readable summary
func (h *SelfHealer) generateSummary() string {
	var sb strings.Builder

	sb.WriteString("=== Self-Healer Summary ===\n\n")
	sb.WriteString(fmt.Sprintf("Workspace: %s\n", h.workspaceDir))
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n\n", time.Now().Format(time.RFC3339)))

	// Get recent history
	history := h.GetHistory()
	if len(history) > 0 {
		sb.WriteString("Recent Healing Events:\n")
		shownCount := 0
		for i := len(history) - 1; i >= 0 && shownCount < 10; i-- {
			event := history[i]
			status := "✓"
			if !event.Success {
				status = "✗"
			}
			sb.WriteString(fmt.Sprintf("  [%s] %s: %s\n", status, event.EventType, event.Description))
			shownCount++
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("No healing events recorded.\n\n")
	}

	return sb.String()
}
