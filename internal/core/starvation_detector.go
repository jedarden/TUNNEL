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

// StarvationCondition represents a detected starvation state
type StarvationCondition struct {
	Timestamp          time.Time   `json:"timestamp"`
	Workspace          string      `json:"workspace"`
	OpenBeads          int         `json:"open_beads"`
	ReadyCandidates    int         `json:"ready_candidates"`
	ExcludedBeads      int         `json:"excluded_beads"`
	ExclusionReasons   []string    `json:"exclusion_reasons"`
	StaleAssignees     int         `json:"stale_assignees"`
	ClosedWithAssignee int         `json:"closed_with_assignee"`
	CircularDeps       int         `json:"circular_deps"`
	DBInconsistent     bool        `json:"db_inconsistent"`
	FallbackAnalysis   *FallbackAnalysis `json:"fallback_analysis,omitempty"`
	CorruptionDetections []CorruptionDetection `json:"corruption_detections,omitempty"`
}

// StarvationDetector monitors for bead starvation conditions
type StarvationDetector struct {
	workspaceDir      string
	binaryPath        string
	beadValidator     *BeadValidator
	auditLogger       *AuditLogger
	eventPublisher    *EventPublisher
	fallbackExecutor  *FallbackQueryExecutor

	// Configuration
	checkInterval     time.Duration
	alertCooldown     time.Duration
	autoRepairEnabled bool
	dryRun            bool

	// State
	lastCheckTime     time.Time
	lastAlertTime     time.Time
	running           bool
	mu                sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc

	// Detection history
	detectionHistory  []StarvationCondition
	maxHistorySize    int

	// Retry state with exponential backoff
	retryState        *RetryState

	// Pluck-specific retry state with exponential backoff
	pluckRetryState   *PluckRetryState
}

// RetryState tracks retry attempts and backoff timing
type RetryState struct {
	CurrentAttempt    int
	MaxRetries        int
	FirstDetection    time.Time
	LastAttempt       time.Time
	Condition         *StarvationCondition
	BackoffDurations  []time.Duration
}

// PluckRetryState tracks Pluck-specific retry attempts with exponential backoff
type PluckRetryState struct {
	CurrentAttempt      int
	ConsecutiveFailures int
	MaxRetries          int
	FirstFailure        time.Time
	LastFailure         time.Time
	LastSuccess         time.Time
	InitialDelay        time.Duration
	ValidationThreshold int  // Attempts after which to trigger validation (e.g., 3)
	RemediationThreshold int  // Attempts after which to create remediation bead (e.g., 5)
}

// Default retry backoff sequence: 5m → 10m → 20m → 40m (2x exponential)
var defaultBackoffSequence = []time.Duration{
	5 * time.Minute,
	10 * time.Minute,
	20 * time.Minute,
	40 * time.Minute,
}

// Default Pluck retry backoff: 30s → 1m → 2m → 4m → 8m (2x exponential)
var defaultPluckBackoffSequence = []time.Duration{
	30 * time.Second,
	1 * time.Minute,
	2 * time.Minute,
	4 * time.Minute,
	8 * time.Minute,
}

// DetectorConfig holds configuration for the starvation detector
type DetectorConfig struct {
	WorkspaceDir         string
	BinaryPath         string // Path to bead binary (default: "bead")
	CheckInterval      time.Duration
	AlertCooldown      time.Duration
	AutoRepairEnabled  bool
	DryRun            bool
	AuditLogger       *AuditLogger
	EventPublisher    *EventPublisher
	MaxHistorySize    int
	MaxRetries        int           // Maximum retry attempts before escalation (default: 4)
	BackoffSequence   []time.Duration // Custom backoff sequence (optional)

	// Pluck-specific retry configuration
	PluckMaxRetries           int           // Maximum Pluck retry attempts (default: 5)
	PluckInitialDelay         time.Duration // Initial delay for Pluck retries (default: 30s)
	PluckValidationThreshold  int           // Attempts after which to trigger validation (default: 3)
	PluckRemediationThreshold int           // Attempts after which to create remediation bead (default: 5)
}

// NewStarvationDetector creates a new starvation detector
func NewStarvationDetector(config *DetectorConfig) *StarvationDetector {
	if config == nil {
		config = &DetectorConfig{}
	}

	// Set defaults
	if config.CheckInterval == 0 {
		config.CheckInterval = 5 * time.Minute
	}
	if config.AlertCooldown == 0 {
		config.AlertCooldown = 15 * time.Minute
	}
	if config.MaxHistorySize == 0 {
		config.MaxHistorySize = 100
	}
	if config.BinaryPath == "" {
		config.BinaryPath = "bead"
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 4 // Default: 4 retry attempts
	}
	if config.PluckMaxRetries == 0 {
		config.PluckMaxRetries = 5 // Default: 5 Pluck retry attempts
	}
	if config.PluckInitialDelay == 0 {
		config.PluckInitialDelay = 30 * time.Second // Default: 30s initial delay
	}
	if config.PluckValidationThreshold == 0 {
		config.PluckValidationThreshold = 3 // Default: trigger validation after 3 failures
	}
	if config.PluckRemediationThreshold == 0 {
		config.PluckRemediationThreshold = 5 // Default: create remediation bead after 5 failures
	}
	backoffSequence := config.BackoffSequence
	if len(backoffSequence) == 0 {
		backoffSequence = defaultBackoffSequence
	}

	// Create bead validator
	validatorConfig := &ValidatorConfig{
		WorkspaceDir: config.WorkspaceDir,
		BinaryPath:   config.BinaryPath,
		DryRun:       config.DryRun,
		AuditLogger:  config.AuditLogger,
	}

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	return &StarvationDetector{
		workspaceDir:      config.WorkspaceDir,
		binaryPath:        config.BinaryPath,
		beadValidator:     NewBeadValidator(validatorConfig),
		auditLogger:       config.AuditLogger,
		eventPublisher:    config.EventPublisher,
		fallbackExecutor:  NewFallbackQueryExecutor(config.WorkspaceDir, config.BinaryPath),
		checkInterval:     config.CheckInterval,
		alertCooldown:     config.AlertCooldown,
		autoRepairEnabled: config.AutoRepairEnabled,
		dryRun:            config.DryRun,
		detectionHistory:  make([]StarvationCondition, 0, config.MaxHistorySize),
		maxHistorySize:    config.MaxHistorySize,
		ctx:               ctx,
		cancel:            cancel,
		retryState: &RetryState{
			MaxRetries:       config.MaxRetries,
			BackoffDurations: backoffSequence,
		},
		pluckRetryState: &PluckRetryState{
			MaxRetries:           config.PluckMaxRetries,
			InitialDelay:         config.PluckInitialDelay,
			ValidationThreshold:  config.PluckValidationThreshold,
			RemediationThreshold: config.PluckRemediationThreshold,
		},
	}
}

// detectStarvation checks for starvation conditions
func (d *StarvationDetector) detectStarvation() (*StarvationCondition, error) {
	condition := &StarvationCondition{
		Timestamp: time.Now(),
		Workspace: d.workspaceDir,
	}

	// Get ready candidates using automated retry with exponential backoff
	readyBeads, err := d.listBeadsWithRetry("--ready")
	if err != nil {
		return nil, fmt.Errorf("failed to list ready beads with retry: %w", err)
	}
	condition.ReadyCandidates = len(readyBeads)

	// Get all open beads
	openBeads, err := d.listBeads("--status", "open")
	if err != nil {
		return nil, fmt.Errorf("failed to list open beads: %w", err)
	}
	condition.OpenBeads = len(openBeads)

	// Check if starvation condition exists
	if condition.OpenBeads == 0 {
		// No open beads, no starvation
		return nil, nil
	}

	if condition.ReadyCandidates > 0 {
		// Have ready candidates, no starvation
		return nil, nil
	}

	// Starvation detected! Run fallback queries to diagnose why
	fallbackAnalysis, err := d.fallbackExecutor.ExecuteFallbackQueries()
	if err != nil {
		// Log fallback error but continue with standard diagnosis
		if d.auditLogger != nil {
			event := map[string]interface{}{
				"action":    "fallback_query_failed",
				"error":     err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			}
			_ = d.auditLogger.Log(event)
		}
	}

	// Detect corruption patterns from fallback analysis
	var corruptionDetections []CorruptionDetection
	if fallbackAnalysis != nil {
		corruptionDetections = d.fallbackExecutor.DetectCorruptionPatterns(fallbackAnalysis)
	}

	// Auto-heal detected corruption patterns
	if d.autoRepairEnabled && len(corruptionDetections) > 0 {
		for _, detection := range corruptionDetections {
			if detection.AutoHealable {
				if err := d.attemptAutoHeal(detection, fallbackAnalysis); err != nil {
					// Log error but continue
					if d.auditLogger != nil {
						event := map[string]interface{}{
							"action":         "auto_heal_failed",
							"corruption_type": detection.CorruptionType,
							"error":          err.Error(),
							"timestamp":      time.Now().Format(time.RFC3339),
						}
						_ = d.auditLogger.Log(event)
					}
				}
			}
		}
	}

	// Now diagnose why with enhanced context from fallback queries
	excludedBeads := 0
	exclusionReasons := []string{}
	staleAssignees := 0
	closedWithAssignee := 0
	circularDeps := 0

	// Add fallback query insights to exclusion reasons
	if fallbackAnalysis != nil && len(fallbackAnalysis.ExclusionPatterns) > 0 {
		for _, pattern := range fallbackAnalysis.ExclusionPatterns {
			exclusionReasons = append(exclusionReasons,
				fmt.Sprintf("Fallback analysis: %s excluded %d beads",
					pattern.FilterName, pattern.ExcludedCount))
		}
	}

	// Check each open bead for exclusion reasons
	for _, bead := range openBeads {
		excluded := false

		// Check for stale assignee (assigned but open)
		if bead.Assignee != "" && bead.Status == BeadStateOpen {
			staleAssignees++
			excludedBeads++
			excluded = true
			exclusionReasons = append(exclusionReasons,
				fmt.Sprintf("Bead %s: assigned to %s but status is open", bead.ID, bead.Assignee))
		}

		// Check for blocked dependencies
		if len(bead.BlockedBy) > 0 {
			// Verify if blocking beads are actually closed
			for _, blockerID := range bead.BlockedBy {
				if d.isBeadClosed(blockerID) {
					// Blocked by a closed bead - this shouldn't happen
					excludedBeads++
					excluded = true
					exclusionReasons = append(exclusionReasons,
						fmt.Sprintf("Bead %s: blocked by closed bead %s", bead.ID, blockerID))
				}
			}
		}

		// Check for circular dependencies
		for _, blockedID := range bead.Blocks {
			if blockedID == bead.ID {
				circularDeps++
				excludedBeads++
				excluded = true
				exclusionReasons = append(exclusionReasons,
					fmt.Sprintf("Bead %s: circular self-dependency", bead.ID))
			}
		}
	}

	// Check for closed beads with assignees
	closedBeads, err := d.listBeads("--status", "closed")
	if err == nil {
		for _, bead := range closedBeads {
			if bead.Assignee != "" {
				closedWithAssignee++
				exclusionReasons = append(exclusionReasons,
					fmt.Sprintf("Bead %s: closed but still assigned to %s", bead.ID, bead.Assignee))
			}
		}
	}

	// Check database consistency
	validatorResults, err := d.beadValidator.ValidateAll()
	if err == nil {
		for _, result := range validatorResults {
			if result.Name == "database_consistency" && !result.Passed {
				condition.DBInconsistent = true
				exclusionReasons = append(exclusionReasons,
					"Database inconsistent with checkpoint")
			}
		}
	}

	condition.ExcludedBeads = excludedBeads
	condition.ExclusionReasons = exclusionReasons
	condition.StaleAssignees = staleAssignees
	condition.ClosedWithAssignee = closedWithAssignee
	condition.CircularDeps = circularDeps
	condition.FallbackAnalysis = fallbackAnalysis
	condition.CorruptionDetections = corruptionDetections

	return condition, nil
}

// isBeadClosed checks if a bead is closed
func (d *StarvationDetector) isBeadClosed(beadID string) bool {
	cmd := exec.Command(d.binaryPath, "show", beadID, "--json")
	cmd.Dir = d.workspaceDir

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

// listBeads lists beads with optional filters
func (d *StarvationDetector) listBeads(filters ...string) ([]Bead, error) {
	args := []string{"list", "--json"}
	args = append(args, filters...)

	cmd := exec.Command(d.binaryPath, args...)
	cmd.Dir = d.workspaceDir

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

// listBeadsWithRetry executes bead list with automated retry and exponential backoff
// This specifically handles the case where Pluck (bead list --ready) returns zero candidates
// but open beads exist, indicating a transient query failure
func (d *StarvationDetector) listBeadsWithRetry(filters ...string) ([]Bead, error) {
	// Check if this is a --ready query (Pluck operation)
	isReadyQuery := false
	for _, filter := range filters {
		if filter == "--ready" {
			isReadyQuery = true
			break
		}
	}

	if !isReadyQuery {
		// Not a Pluck operation, use normal listBeads
		return d.listBeads(filters...)
	}

	// This is a Pluck operation, use retry logic
	d.mu.Lock()
	defer d.mu.Unlock()

	var lastErr error
	var beads []Bead

	for attempt := 0; attempt <= d.pluckRetryState.MaxRetries; attempt++ {
		// Execute the query
		beads, lastErr = d.listBeads(filters...)
		if lastErr == nil && len(beads) > 0 {
			// Success! Reset consecutive failure counter
			d.pluckRetryState.ConsecutiveFailures = 0
			d.pluckRetryState.LastSuccess = time.Now()
			return beads, nil
		}

		// Query returned zero candidates or failed
		// Verify if open beads actually exist (mismatch detection)
		openBeads, err := d.listBeads("--status", "open")
		if err != nil {
			// Can't verify open beads count, treat as hard failure
			lastErr = fmt.Errorf("failed to verify open beads count: %w", err)
			break
		}

		if len(openBeads) == 0 {
			// No open beads, zero candidates is expected
			d.pluckRetryState.ConsecutiveFailures = 0
			return beads, nil // Return empty list, not an error
		}

		// Mismatch detected: zero ready candidates but open beads exist
		d.pluckRetryState.ConsecutiveFailures++
		d.pluckRetryState.CurrentAttempt = attempt + 1
		d.pluckRetryState.LastFailure = time.Now()

		if attempt == 0 {
			d.pluckRetryState.FirstFailure = time.Now()
		}

		// Log the failure
		if d.auditLogger != nil {
			event := map[string]interface{}{
				"action":             "pluck_retry_attempt",
				"attempt_number":      attempt + 1,
				"consecutive_failures": d.pluckRetryState.ConsecutiveFailures,
				"ready_candidates":    len(beads),
				"open_beads":          len(openBeads),
				"timestamp":           time.Now().Format(time.RFC3339),
			}
			_ = d.auditLogger.Log(event)
		}

		// Check escalation thresholds
		if d.pluckRetryState.ConsecutiveFailures >= d.pluckRetryState.ValidationThreshold {
			// Trigger bead state validation after 3 consecutive failures
			go d.triggerBeadStateValidation(attempt + 1, len(openBeads))
		}

		if d.pluckRetryState.ConsecutiveFailures >= d.pluckRetryState.RemediationThreshold {
			// Create remediation bead after 5 consecutive failures
			go d.createRemediationBead(d.pluckRetryState.ConsecutiveFailures, len(openBeads))
			// Reset counter after creating remediation bead to avoid duplicate beads
			d.pluckRetryState.ConsecutiveFailures = 0
			// Return error to caller
			return nil, fmt.Errorf("Pluck failed after %d consecutive attempts despite %d open beads - remediation bead created",
				d.pluckRetryState.RemediationThreshold, len(openBeads))
		}

		// Calculate backoff delay with 2x exponential growth
		backoffDelay := d.calculatePluckBackoff(attempt)

		// Publish retry event
		if d.eventPublisher != nil {
			event := map[string]interface{}{
				"type":                "pluck_retry",
				"attempt_number":      attempt + 1,
				"max_retries":         d.pluckRetryState.MaxRetries,
				"backoff_duration":    backoffDelay.String(),
				"ready_candidates":    len(beads),
				"open_beads":          len(openBeads),
				"consecutive_failures": d.pluckRetryState.ConsecutiveFailures,
				"timestamp":           time.Now().Format(time.RFC3339),
			}
			_ = d.eventPublisher.Publish(event)
		}

		// Wait before retry (unless this is the last attempt)
		if attempt < d.pluckRetryState.MaxRetries {
			select {
			case <-time.After(backoffDelay):
				// Continue to next attempt
			case <-d.ctx.Done():
				return nil, fmt.Errorf("pluck retry cancelled: %w", d.ctx.Err())
			}
		}
	}

	// All retries exhausted
	if lastErr != nil {
		return nil, fmt.Errorf("Pluck retry exhausted after %d attempts: %w", d.pluckRetryState.MaxRetries, lastErr)
	}

	// Return the last result (even if empty) after retries
	return beads, nil
}

// calculatePluckBackoff calculates the backoff delay with 2x exponential growth
func (d *StarvationDetector) calculatePluckBackoff(attempt int) time.Duration {
	if attempt == 0 {
		return d.pluckRetryState.InitialDelay
	}

	// 2x exponential growth: delay = initial_delay * (2 ^ attempt)
	delay := d.pluckRetryState.InitialDelay * time.Duration(1<<uint(attempt))

	// Cap at maximum reasonable delay (8 minutes)
	maxDelay := 8 * time.Minute
	if delay > maxDelay {
		return maxDelay
	}

	return delay
}

// triggerBeadStateValidation triggers bead state validation after consecutive Pluck failures
func (d *StarvationDetector) triggerBeadStateValidation(attempt int, openBeads int) {
	if d.auditLogger != nil {
		event := map[string]interface{}{
			"action":            "bead_state_validation_triggered",
			"trigger_reason":    "consecutive_pluck_failures",
			"attempt_number":    attempt,
			"open_beads":        openBeads,
			"timestamp":         time.Now().Format(time.RFC3339),
		}
		_ = d.auditLogger.Log(event)
	}

	// Run bead validator
	report, err := d.runDiagnostics(&StarvationCondition{
		Timestamp:       time.Now(),
		Workspace:       d.workspaceDir,
		OpenBeads:       openBeads,
		ReadyCandidates: 0,
	})

	if err != nil {
		if d.auditLogger != nil {
			event := map[string]interface{}{
				"action":    "bead_state_validation_failed",
				"error":     err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			}
			_ = d.auditLogger.Log(event)
		}
		return
	}

	// Publish validation results
	if d.eventPublisher != nil {
		event := map[string]interface{}{
			"type":      "bead_state_validation",
			"report":    report,
			"triggered_by": "pluck_retry",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		_ = d.eventPublisher.Publish(event)
	}
}

// createRemediationBead creates a remediation bead for the workspace after repeated Pluck failures
func (d *StarvationDetector) createRemediationBead(failureCount int, openBeads int) error {
	if d.dryRun {
		if d.auditLogger != nil {
			event := map[string]interface{}{
				"action":        "remediation_bead_skipped",
				"reason":        "dry_run",
				"failure_count": failureCount,
				"open_beads":    openBeads,
				"timestamp":     time.Now().Format(time.RFC3339),
			}
			_ = d.auditLogger.Log(event)
		}
		return nil
	}

	// Create remediation bead using bead CLI
	beadTitle := fmt.Sprintf("Pluck starvation remediation - %d consecutive failures detected", failureCount)
	beadDescription := fmt.Sprintf(
		`## Pluck Starvation Detected

The starvation detector detected %d consecutive Pluck failures despite %d open beads existing in the workspace.

### Detection Details
- **Consecutive Failures**: %d
- **Open Beads Affected**: %d
- **First Failure**: %s
- **Last Failure**: %s

### Automated Actions Taken
1. Attempted automated retry with exponential backoff (2x delay multiplier)
2. Triggered bead state validation after %d consecutive failures
3. This remediation bead created after %d consecutive failures (threshold reached)

### Recommended Actions
1. Review bead database for corruption: \`bead doctor\`
2. Check checkpoint consistency: \`bead sync flush-only\`
3. Verify dependency graph integrity
4. Review recent bead operations for data loss
5. Consider restoring from checkpoint if corruption is confirmed

### Next Steps
- If \`bead doctor\` reports issues, follow recommended repair steps
- If validation shows corruption, consider: \`bead sync import-only --input .beads/checkpoint/forensic.jsonl --restore-into-empty\`
- Monitor future Pluck operations for recurrence

---
This is an automated remediation bead created by the starvation detector after exhausting automated retry attempts.`,
		failureCount, openBeads, failureCount, openBeads,
		d.pluckRetryState.FirstFailure.Format(time.RFC3339),
		d.pluckRetryState.LastFailure.Format(time.RFC3339),
		d.pluckRetryState.ValidationThreshold,
		d.pluckRetryState.RemediationThreshold,
	)

	args := []string{
		"create",
		"--title", beadTitle,
		"--issue-type", "task",
		"--priority", "2", // High priority
		"--label", "remediation",
		"--label", "starvation-detector",
	}

	cmd := exec.Command(d.binaryPath, args...)
	cmd.Dir = d.workspaceDir

	// Set description via stdin (bead create reads from stdin for multi-line descriptions)
	descriptionInput := beadDescription
	cmd.Stdin = strings.NewReader(descriptionInput)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if d.auditLogger != nil {
			event := map[string]interface{}{
				"action":        "remediation_bead_creation_failed",
				"error":         err.Error(),
				"output":        string(output),
				"failure_count": failureCount,
				"timestamp":     time.Now().Format(time.RFC3339),
			}
			_ = d.auditLogger.Log(event)
		}
		return fmt.Errorf("failed to create remediation bead: %w (output: %s)", err, string(output))
	}

	// Log successful remediation bead creation
	if d.auditLogger != nil {
		event := map[string]interface{}{
			"action":        "remediation_bead_created",
			"failure_count": failureCount,
			"open_beads":    openBeads,
			"timestamp":     time.Now().Format(time.RFC3339),
		}
		_ = d.auditLogger.Log(event)
	}

	// Publish creation event
	if d.eventPublisher != nil {
		event := map[string]interface{}{
			"type":          "remediation_bead_created",
			"failure_count": failureCount,
			"open_beads":    openBeads,
			"timestamp":     time.Now().Format(time.RFC3339),
		}
		_ = d.eventPublisher.Publish(event)
	}

	return nil
}

// runDiagnostics performs full diagnostic check
func (d *StarvationDetector) runDiagnostics(condition *StarvationCondition) (*DiagnosticReport, error) {
	report := &DiagnosticReport{
		Timestamp:      condition.Timestamp,
		Workspace:      condition.Workspace,
		StarvationDetected: true,
	}

	// Run bead validator
	validatorResults, err := d.beadValidator.ValidateAll()
	if err != nil {
		report.ValidationErrors = append(report.ValidationErrors,
			fmt.Sprintf("Validator error: %v", err))
	} else {
		report.ValidationResults = validatorResults
	}

	// Check each validation result
	for _, result := range validatorResults {
		if !result.Passed {
			if result.Fixable {
				report.FixableIssues = append(report.FixableIssues, result)
			} else {
				report.ManualInterventionRequired = append(report.ManualInterventionRequired, result)
			}
		}
	}

	// Add condition details
	report.Condition = condition

	return report, nil
}

// DiagnosticReport represents the results of diagnostic checks
type DiagnosticReport struct {
	Timestamp              time.Time             `json:"timestamp"`
	Workspace              string                `json:"workspace"`
	StarvationDetected     bool                  `json:"starvation_detected"`
	Condition              *StarvationCondition  `json:"condition,omitempty"`
	ValidationResults      []ValidationResult    `json:"validation_results,omitempty"`
	FixableIssues          []ValidationResult    `json:"fixable_issues,omitempty"`
	ManualInterventionRequired []ValidationResult `json:"manual_intervention_required,omitempty"`
	ValidationErrors       []string              `json:"validation_errors,omitempty"`
	AutoRepairsApplied     int                   `json:"auto_repairs_applied"`
}

// autoRepair performs automatic repairs for all fixable issues
func (d *StarvationDetector) autoRepair(report *DiagnosticReport) error {
	if !d.autoRepairEnabled {
		return fmt.Errorf("auto-repair is disabled")
	}

	// Auto-repair ALL fixable issues (not just "safe, common issues")
	fixableCount := len(report.FixableIssues)
	if fixableCount == 0 {
		return nil
	}

	// Run fixes through the validator - FixAll handles all validation checks
	if err := d.beadValidator.FixAll(report.ValidationResults); err != nil {
		return fmt.Errorf("auto-repair failed: %w", err)
	}

	report.AutoRepairsApplied = d.beadValidator.GetFixesApplied()

	// Log audit event
	if d.auditLogger != nil {
		event := map[string]interface{}{
			"action":     "auto_repair",
			"workspace":  d.workspaceDir,
			"fixes_applied": report.AutoRepairsApplied,
			"timestamp":  time.Now().Format(time.RFC3339),
		}
		_ = d.auditLogger.Log(event)
	}

	return nil
}

// createRemediationBead creates an agent-workable bead with exact remediation steps
func (d *StarvationDetector) createRemediationBead(report *DiagnosticReport) error {
	if len(report.ManualInterventionRequired) == 0 && report.AutoRepairsApplied == 0 {
		// No issues requiring attention
		return nil
	}

	// Generate bead description with actionable steps
	description := d.generateRemediationDescription(report)

	// Create bead using bead CLI with agent-friendly labels (no 'human' blocking)
	args := []string{
		"create",
		"--title", d.generateRemediationTitle(report),
		"--priority", "2", // P2 for remediation issues
		"--issue-type", "task",
		"--label", "alert:starvation-remediation",
		"--label", "agent-workable",
	}

	cmd := exec.Command(d.binaryPath, args...)
	cmd.Dir = d.workspaceDir

	// Set description via stdin
	cmd.Stdin = strings.NewReader(description)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create remediation bead: %w\nOutput: %s", err, string(output))
	}

	// Log bead creation
	if d.auditLogger != nil {
		event := map[string]interface{}{
			"action":     "create_remediation_bead",
			"workspace":  d.workspaceDir,
			"timestamp":  time.Now().Format(time.RFC3339),
		}
		_ = d.auditLogger.Log(event)
	}

	return nil
}

// createEscalationBead creates a system-level escalation bead after retries are exhausted
func (d *StarvationDetector) createEscalationBead(report *DiagnosticReport) error {
	// Generate bead description
	description := d.generateEscalationDescription(report)

	// Create bead using bead CLI with system-level labels (not human-blocked)
	args := []string{
		"create",
		"--title", d.generateEscalationTitle(report),
		"--priority", "1", // P1 for escalation (higher priority than diagnostic)
		"--issue-type", "task",
		"--label", "alert:starvation-escalation",
		"--label", "system-level",
		"--label", "automated-escalation",
	}

	cmd := exec.Command(d.binaryPath, args...)
	cmd.Dir = d.workspaceDir

	// Set description via stdin
	cmd.Stdin = strings.NewReader(description)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create escalation bead: %w\nOutput: %s", err, string(output))
	}

	// Log escalation
	if d.auditLogger != nil {
		event := map[string]interface{}{
			"action":         "create_escalation_bead",
			"workspace":      d.workspaceDir,
			"attempts_made":  d.retryState.CurrentAttempt,
			"timestamp":      time.Now().Format(time.RFC3339),
		}
		_ = d.auditLogger.Log(event)
	}

	return nil
}

// generateEscalationTitle creates a title for the escalation bead
func (d *StarvationDetector) generateEscalationTitle(report *DiagnosticReport) string {
	return fmt.Sprintf("[ESCALATION] Starvation persisted after %d automated retry attempts",
		d.retryState.CurrentAttempt)
}

// generateEscalationDescription creates a detailed description for the escalation bead
func (d *StarvationDetector) generateEscalationDescription(report *DiagnosticReport) string {
	var sb strings.Builder

	sb.WriteString("## Starvation Escalation - Automated Recovery Exhausted\n\n")
	sb.WriteString("**This bead was automatically created after exhausting automated retry attempts.**\n\n")

	sb.WriteString(fmt.Sprintf("**Workspace:** %s\n", report.Workspace))
	sb.WriteString(fmt.Sprintf("**First detection:** %s\n", d.retryState.FirstDetection.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Escalation time:** %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Total retry attempts:** %d\n\n", d.retryState.CurrentAttempt))

	sb.WriteString("### Retry History\n\n")
	sb.WriteString("The following automated retry attempts were made with exponential backoff:\n\n")

	for i := 1; i <= d.retryState.CurrentAttempt; i++ {
		backoff := d.getBackoffDuration(i)
		sb.WriteString(fmt.Sprintf("**Attempt %d:** Backoff duration %s\n", i, backoff))
	}
	sb.WriteString("\n")

	if report.Condition != nil {
		sb.WriteString("### Current Condition\n\n")
		sb.WriteString(fmt.Sprintf("**Open beads:** %d\n", report.Condition.OpenBeads))
		sb.WriteString(fmt.Sprintf("**Ready candidates:** %d\n", report.Condition.ReadyCandidates))
		sb.WriteString(fmt.Sprintf("**Excluded beads:** %d\n\n", report.Condition.ExcludedBeads))

		if len(report.Condition.ExclusionReasons) > 0 {
			sb.WriteString("**Exclusion reasons:**\n")
			for _, reason := range report.Condition.ExclusionReasons {
				sb.WriteString(fmt.Sprintf("- %s\n", reason))
			}
			sb.WriteString("\n")
		}

		// Add fallback analysis if available
		if report.Condition.FallbackAnalysis != nil {
			sb.WriteString("### Fallback Query Analysis\n\n")
			sb.WriteString(report.Condition.FallbackAnalysis.GetExclusionSummary())
			sb.WriteString("\n")
		}

		// Add corruption detections if available
		if len(report.Condition.CorruptionDetections) > 0 {
			sb.WriteString("### Detected Corruption Patterns\n\n")
			for _, detection := range report.Condition.CorruptionDetections {
				sb.WriteString(fmt.Sprintf("#### %s\n", detection.CorruptionType))
				sb.WriteString(fmt.Sprintf("**Severity:** %s\n", detection.Severity))
				sb.WriteString(fmt.Sprintf("**Description:** %s\n", detection.Description))
				if detection.AutoHealable {
					sb.WriteString(fmt.Sprintf("**Auto-heal method:** %s\n", detection.HealMethod))
				}
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("### Automated Repairs Attempted\n\n")
	if report.AutoRepairsApplied > 0 {
		sb.WriteString(fmt.Sprintf("Applied %d automatic fixes across retry attempts:\n", report.AutoRepairsApplied))
		for _, issue := range report.FixableIssues {
			if !issue.Passed && issue.Fixable {
				sb.WriteString(fmt.Sprintf("- ✓ %s: %s\n", issue.Name, issue.Description))
			}
		}
		sb.WriteString("\n")

		// List aggressive repairs attempted
		sb.WriteString("**Aggressive repairs attempted on retries 2+:**\n")
		sb.WriteString("- Checkpoint sync\n")
		sb.WriteString("- Cache/stale lock cleanup\n")
		sb.WriteString("- Fallback query analysis\n")
		sb.WriteString("- Dependency corruption healing\n")
		sb.WriteString("- Label misapplication healing\n")
		sb.WriteString("\n")
	} else {
		sb.WriteString("No auto-repairs were successfully applied.\n\n")
	}

	if len(report.ManualInterventionRequired) > 0 {
		sb.WriteString("### Remaining Issues Requiring Attention\n\n")
		sb.WriteString("The following issues require manual review:\n\n")
		for _, issue := range report.ManualInterventionRequired {
			sb.WriteString(fmt.Sprintf("#### %s\n", issue.Name))
			sb.WriteString(fmt.Sprintf("**Description:** %s\n", issue.Description))
			if len(issue.Issues) > 0 {
				sb.WriteString("**Issues:**\n")
				for _, i := range issue.Issues {
					sb.WriteString(fmt.Sprintf("- %s\n", i))
				}
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("### Recommended Actions\n\n")
	sb.WriteString("This starvation condition has persisted through all automated recovery attempts. ")
	sb.WriteString("Review the exclusion reasons, fallback analysis, and manual intervention items above to resolve the underlying issue.\n\n")

	sb.WriteString("**Do NOT delete this bead** until the root cause is resolved and starvation is confirmed cleared.\n")

	return sb.String()
}

// generateRemediationTitle creates a title for the remediation bead
func (d *StarvationDetector) generateRemediationTitle(report *DiagnosticReport) string {
	if len(report.ManualInterventionRequired) > 0 {
		return fmt.Sprintf("Starvation remediation: %d issues requiring attention",
			len(report.ManualInterventionRequired))
	}
	return fmt.Sprintf("Starvation remediation: %d auto-repairs applied",
		report.AutoRepairsApplied)
}

// generateRemediationDescription creates a detailed description with exact remediation steps
func (d *StarvationDetector) generateRemediationDescription(report *DiagnosticReport) string {
	var sb strings.Builder

	sb.WriteString("## Starvation Condition Detected\n\n")
	sb.WriteString(fmt.Sprintf("**Workspace:** %s\n", report.Workspace))
	sb.WriteString(fmt.Sprintf("**Timestamp:** %s\n\n", report.Timestamp.Format(time.RFC3339)))

	if report.Condition != nil {
		sb.WriteString(fmt.Sprintf("**Open beads:** %d\n", report.Condition.OpenBeads))
		sb.WriteString(fmt.Sprintf("**Ready candidates:** %d\n", report.Condition.ReadyCandidates))
		sb.WriteString(fmt.Sprintf("**Excluded beads:** %d\n\n", report.Condition.ExcludedBeads))

		if len(report.Condition.ExclusionReasons) > 0 {
			sb.WriteString("**Exclusion reasons:**\n")
			for _, reason := range report.Condition.ExclusionReasons {
				sb.WriteString(fmt.Sprintf("- %s\n", reason))
			}
			sb.WriteString("\n")
		}

		// Add fallback analysis if available
		if report.Condition.FallbackAnalysis != nil {
			sb.WriteString("## Fallback Query Analysis\n\n")
			sb.WriteString(report.Condition.FallbackAnalysis.GetExclusionSummary())
			sb.WriteString("\n")
		}

		// Add corruption detections if available
		if len(report.Condition.CorruptionDetections) > 0 {
			sb.WriteString("## Detected Corruption Patterns\n\n")
			for _, detection := range report.Condition.CorruptionDetections {
				sb.WriteString(fmt.Sprintf("### %s\n", detection.CorruptionType))
				sb.WriteString(fmt.Sprintf("**Severity:** %s\n", detection.Severity))
				sb.WriteString(fmt.Sprintf("**Filter:** %s\n", detection.FilterName))
				sb.WriteString(fmt.Sprintf("**Description:** %s\n", detection.Description))
				if detection.AutoHealable {
					sb.WriteString("**Auto-healable:** Yes\n")
					sb.WriteString(fmt.Sprintf("**Heal method:** %s\n", detection.HealMethod))
				} else {
					sb.WriteString("**Auto-healable:** No (requires manual intervention)\n")
				}
				if len(detection.BeadIDs) > 0 {
					sb.WriteString("**Affected beads:**\n")
					for _, beadID := range detection.BeadIDs {
						sb.WriteString(fmt.Sprintf("- %s\n", beadID))
					}
				}
				sb.WriteString("\n")
			}
		}
	}

	sb.WriteString("## Auto-Repairs Applied\n\n")
	if report.AutoRepairsApplied > 0 {
		sb.WriteString(fmt.Sprintf("Successfully applied %d automatic fixes:\n", report.AutoRepairsApplied))
		for _, issue := range report.FixableIssues {
			if !issue.Passed && issue.Fixable {
				sb.WriteString(fmt.Sprintf("- ✓ %s: %s\n", issue.Name, issue.Description))
			}
		}
	} else {
		sb.WriteString("No auto-repairs were applied (either disabled or no fixable issues found)\n")
	}
	sb.WriteString("\n")

	if len(report.ManualInterventionRequired) > 0 {
		sb.WriteString("## Remediation Steps\n\n")
		sb.WriteString("The following issues require specific remediation actions:\n\n")
		for _, issue := range report.ManualInterventionRequired {
			sb.WriteString(fmt.Sprintf("### %s\n", issue.Name))
			sb.WriteString(fmt.Sprintf("**Description:** %s\n", issue.Description))
			if len(issue.Issues) > 0 {
				sb.WriteString("**Issues:**\n")
				for _, i := range issue.Issues {
					sb.WriteString(fmt.Sprintf("- %s\n", i))
				}
			}
			sb.WriteString("**Recommended action:**\n")
			sb.WriteString(fmt.Sprintf("%s\n\n", d.getRemediationSteps(issue.Name)))
		}
	}

	if len(report.ValidationErrors) > 0 {
		sb.WriteString("## Validation Errors\n\n")
		for _, err := range report.ValidationErrors {
			sb.WriteString(fmt.Sprintf("- %s\n", err))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// getRemediationSteps provides exact remediation steps for each issue type
func (d *StarvationDetector) getRemediationSteps(issueName string) string {
	switch issueName {
	case "workspace_backend":
		return "1. Run `bead doctor` to verify workspace health\n" +
			"2. Check .needle.yaml for `bead_cli.backend` setting\n" +
			"3. If using bead-rs, ensure .beads/config.json exists\n" +
			"4. If migrating from bead-forge, run: `bead init && bead sync import-only --input .beads/checkpoint/forensic.jsonl --restore-into-empty --actor <you>`"
	case "database_consistency":
		return "1. Verify checkpoint exists at .beads/checkpoint/current.json\n" +
			"2. Run `bead sync flush-only` to create/update checkpoint\n" +
			"3. If checkpoint is missing or stale, run: `bead sync import-only --input .beads/checkpoint/forensic.jsonl --restore-into-empty --actor <you>`"
	case "bead_backend_health":
		return "1. Verify bead binary is accessible: `bead --version`\n" +
			"2. Check for database lock: `lsof .beads/beads.db`\n" +
			"3. If locked, identify and terminate the locking process\n" +
			"4. Run `bead doctor --repair` to fix any database issues"
	case "workspace_configuration":
		return "1. Verify .needle.yaml contains `bead_cli.backend` setting\n" +
			"2. Ensure only one backend configuration exists (bead-rs or bead-forge, not both)\n" +
			"3. Run `bead sync flush-only` to update stale checkpoint"
	default:
		return "Run `bead doctor` for full diagnostic report\n" +
			"Review the validation output above for specific issue details"
	}
}

// checkAndAlert runs a single check and implements retry logic with exponential backoff
func (d *StarvationDetector) checkAndAlert() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Update last check time
	d.lastCheckTime = time.Now()

	// Detect starvation
	condition, err := d.detectStarvation()
	if err != nil {
		// Check if this is a transient failure
		if d.isTransientFailure(err) {
			// Don't reset retry state for transient failures
			if d.auditLogger != nil {
				event := map[string]interface{}{
					"action":    "transient_failure",
					"error":     err.Error(),
					"timestamp": time.Now().Format(time.RFC3339),
				}
				_ = d.auditLogger.Log(event)
			}
			return nil // Will retry on next check interval
		}
		return fmt.Errorf("starvation detection failed: %w", err)
	}

	// No starvation detected - reset retry state
	if condition == nil {
		d.resetRetryState()
		return nil
	}

	// Check if we need to wait for backoff
	if d.retryState.CurrentAttempt > 0 && !d.shouldRetryNow() {
		// Not time to retry yet
		return nil
	}

	// Starvation detected - handle with retry logic
	return d.handleStarvationWithRetry(condition)
}

// handleStarvationWithRetry implements the retry logic with exponential backoff
func (d *StarvationDetector) handleStarvationWithRetry(condition *StarvationCondition) error {
	// First detection or new condition
	if d.retryState.CurrentAttempt == 0 || !d.isSameCondition(condition) {
		d.retryState.CurrentAttempt = 0
		d.retryState.FirstDetection = time.Now()
		d.retryState.Condition = condition
	}

	// Increment attempt counter
	d.retryState.CurrentAttempt++
	d.retryState.LastAttempt = time.Now()

	// Add to detection history
	d.detectionHistory = append(d.detectionHistory, *condition)
	if len(d.detectionHistory) > d.maxHistorySize {
		d.detectionHistory = d.detectionHistory[1:]
	}

	// Run diagnostics
	report, err := d.runDiagnostics(condition)
	if err != nil {
		return fmt.Errorf("diagnostics failed: %w", err)
	}

	// Get backoff duration for this attempt
	backoffDuration := d.getBackoffDuration(d.retryState.CurrentAttempt)

	// Log attempt
	if d.auditLogger != nil {
		event := map[string]interface{}{
			"action":            "starvation_retry_attempt",
			"attempt_number":    d.retryState.CurrentAttempt,
			"max_retries":       d.retryState.MaxRetries,
			"backoff_duration":  backoffDuration.String(),
			"open_beads":        condition.OpenBeads,
			"ready_candidates":  condition.ReadyCandidates,
			"timestamp":         time.Now().Format(time.RFC3339),
		}
		_ = d.auditLogger.Log(event)
	}

	// Publish event
	if d.eventPublisher != nil {
		event := map[string]interface{}{
			"type":              "starvation_detected",
			"condition":         condition,
			"report":            report,
			"retry_attempt":     d.retryState.CurrentAttempt,
			"max_retries":       d.retryState.MaxRetries,
			"backoff_duration":  backoffDuration.String(),
			"timestamp":         time.Now().Format(time.RFC3339),
		}
		_ = d.eventPublisher.Publish(event)
	}

	// Attempt repair with increasing aggressiveness based on attempt number
	if d.autoRepairEnabled {
		repairErr := d.attemptRepair(report, d.retryState.CurrentAttempt)
		if repairErr != nil {
			// Log error but continue to retry logic
			if d.auditLogger != nil {
				event := map[string]interface{}{
					"action":         "auto_repair_failed",
					"attempt_number": d.retryState.CurrentAttempt,
					"error":          repairErr.Error(),
					"timestamp":      time.Now().Format(time.RFC3339),
				}
				_ = d.auditLogger.Log(event)
			}
		}
	}

	// Check if we've exhausted retries
	if d.retryState.CurrentAttempt >= d.retryState.MaxRetries {
		// Max retries exhausted - create escalation bead
		if err := d.createEscalationBead(report); err != nil {
			return fmt.Errorf("failed to create escalation bead: %w", err)
		}

		// Update last alert time and reset retry state
		d.lastAlertTime = time.Now()
		d.resetRetryState()
	} else {
		// Not exhausted yet - wait for backoff before next attempt
		// The retry will happen on the next checkAndAlert call after backoff
	}

	return nil
}

// shouldRetryNow checks if enough time has passed for the next retry
func (d *StarvationDetector) shouldRetryNow() bool {
	if d.retryState.CurrentAttempt == 0 {
		return true
	}

	backoffDuration := d.getBackoffDuration(d.retryState.CurrentAttempt + 1)
	return time.Since(d.retryState.LastAttempt) >= backoffDuration
}

// getBackoffDuration returns the backoff duration for a given attempt number
func (d *StarvationDetector) getBackoffDuration(attemptNumber int) time.Duration {
	if attemptNumber <= 0 || attemptNumber > len(d.retryState.BackoffDurations) {
		return d.retryState.BackoffDurations[len(d.retryState.BackoffDurations)-1]
	}
	return d.retryState.BackoffDurations[attemptNumber-1]
}

// isSameCondition checks if the new condition is essentially the same as the stored one
func (d *StarvationDetector) isSameCondition(newCondition *StarvationCondition) bool {
	if d.retryState.Condition == nil {
		return false
	}

	// Consider it the same condition if workspace and key metrics match
	return d.retryState.Condition.Workspace == newCondition.Workspace &&
		d.retryState.Condition.OpenBeads == newCondition.OpenBeads &&
		d.retryState.Condition.ReadyCandidates == newCondition.ReadyCandidates
}

// resetRetryState resets the retry state when starvation is resolved
func (d *StarvationDetector) resetRetryState() {
	if d.retryState != nil && d.retryState.CurrentAttempt > 0 {
		// Log resolution
		if d.auditLogger != nil {
			event := map[string]interface{}{
				"action":         "starvation_resolved",
				"attempts_made":  d.retryState.CurrentAttempt,
				"timestamp":       time.Now().Format(time.RFC3339),
			}
			_ = d.auditLogger.Log(event)
		}
	}

	d.retryState = &RetryState{
		MaxRetries:       d.retryState.MaxRetries,
		BackoffDurations: d.retryState.BackoffDurations,
	}
}

// isTransientFailure checks if an error is transient (network issues, temporary locks, etc.)
func (d *StarvationDetector) isTransientFailure(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()

	// Network-related errors
	transientPatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"resource temporarily unavailable",
		"database is locked",
		"database file is locked",
		"no such file or directory", // May be temporary file system issues
		"i/o timeout",
		"dial tcp",
		"network is unreachable",
		"temporary", // generic catch-all
	}

	for _, pattern := range transientPatterns {
		if strings.Contains(strings.ToLower(errMsg), pattern) {
			return true
		}
	}

	return false
}

// attemptRepair performs repairs with increasing aggressiveness based on attempt number
func (d *StarvationDetector) attemptRepair(report *DiagnosticReport, attemptNumber int) error {
	if !d.autoRepairEnabled {
		return fmt.Errorf("auto-repair is disabled")
	}

	// Always attempt basic auto-repair
	if err := d.autoRepair(report); err != nil {
		return err
	}

	// On attempt 2+, try more aggressive repairs
	if attemptNumber >= 2 {
		// Attempt checkpoint sync
		if err := d.syncCheckpoint(); err != nil && d.auditLogger != nil {
			event := map[string]interface{}{
				"action":         "checkpoint_sync_failed",
				"attempt_number": attemptNumber,
				"error":          err.Error(),
				"timestamp":      time.Now().Format(time.RFC3339),
			}
			_ = d.auditLogger.Log(event)
		}
	}

	// On attempt 3+, try service restart (if applicable)
	if attemptNumber >= 3 {
		// Attempt to clear any caches or temporary state
		if err := d.clearCaches(); err != nil && d.auditLogger != nil {
			event := map[string]interface{}{
				"action":         "cache_clear_failed",
				"attempt_number": attemptNumber,
				"error":          err.Error(),
				"timestamp":      time.Now().Format(time.RFC3339),
			}
			_ = d.auditLogger.Log(event)
		}
	}

	return nil
}

// syncCheckpoint attempts to sync the checkpoint
func (d *StarvationDetector) syncCheckpoint() error {
	if d.dryRun {
		return nil
	}

	cmd := exec.Command(d.binaryPath, "sync", "flush-only")
	cmd.Dir = d.workspaceDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("checkpoint sync failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// clearCaches attempts to clear any caches or temporary state
func (d *StarvationDetector) clearCaches() error {
	if d.dryRun {
		return nil
	}

	// Check for and clear any lock files in the .beads directory
	beadsDir := filepath.Join(d.workspaceDir, ".beads")
	lockFiles, err := filepath.Glob(filepath.Join(beadsDir, "*.lock"))
	if err == nil {
		for _, lockFile := range lockFiles {
			// Try to remove stale lock files
			if info, statErr := os.Stat(lockFile); statErr == nil {
				// Only remove if older than 10 minutes
				if time.Since(info.ModTime()) > 10*time.Minute {
					os.Remove(lockFile)
				}
			}
		}
	}

	return nil
}

// attemptAutoHeal attempts to automatically heal detected corruption patterns
func (d *StarvationDetector) attemptAutoHeal(detection CorruptionDetection, fallbackAnalysis *FallbackAnalysis) error {
	if !d.autoRepairEnabled {
		return fmt.Errorf("auto-repair is disabled")
	}

	if d.dryRun {
		return nil
	}

	switch detection.HealMethod {
	case "validate-dependencies":
		return d.healDependencies(detection, fallbackAnalysis)
	case "validate-labels":
		return d.healLabels(detection, fallbackAnalysis)
	default:
		return fmt.Errorf("unknown heal method: %s", detection.HealMethod)
	}
}

// healDependencies attempts to heal dependency corruption
func (d *StarvationDetector) healDependencies(detection CorruptionDetection, fallbackAnalysis *FallbackAnalysis) error {
	// Get beads that were excluded by dependency filters
	for _, pattern := range fallbackAnalysis.ExclusionPatterns {
		if pattern.FilterName == "dependency-filters" {
			for _, beadID := range pattern.SampleIDs {
				// Show the bead to check its dependencies
				cmd := exec.Command(d.binaryPath, "show", beadID, "--json")
				cmd.Dir = d.workspaceDir

				output, err := cmd.Output()
				if err != nil {
					continue
				}

				var bead Bead
				if err := json.Unmarshal(output, &bead); err != nil {
					continue
				}

				// Check for invalid dependencies (blocked by closed beads)
				for _, blockerID := range bead.BlockedBy {
					if d.isBeadClosed(blockerID) {
						// This bead is blocked by a closed bead - clear the dependency
						updateArgs := []string{"update", bead.ID, "--remove-blocked-by", blockerID}
						updateCmd := exec.Command(d.binaryPath, updateArgs...)
						updateCmd.Dir = d.workspaceDir

						if output, err := updateCmd.CombinedOutput(); err != nil {
							if d.auditLogger != nil {
								event := map[string]interface{}{
									"action":         "dependency_heal_failed",
									"bead_id":        beadID,
									"blocked_by":     blockerID,
									"error":          err.Error(),
									"output":         string(output),
									"timestamp":      time.Now().Format(time.RFC3339),
								}
								_ = d.auditLogger.Log(event)
							}
						} else {
							if d.auditLogger != nil {
								event := map[string]interface{}{
									"action":         "dependency_healed",
									"bead_id":        beadID,
									"blocked_by":     blockerID,
									"timestamp":      time.Now().Format(time.RFC3339),
								}
								_ = d.auditLogger.Log(event)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

// healLabels attempts to heal label corruption
func (d *StarvationDetector) healLabels(detection CorruptionDetection, fallbackAnalysis *FallbackAnalysis) error {
	// Get beads that were excluded by label filters
	for _, pattern := range fallbackAnalysis.ExclusionPatterns {
		if pattern.FilterName == "label-filters" {
			for _, beadID := range pattern.SampleIDs {
				// Show the bead to check its labels
				cmd := exec.Command(d.binaryPath, "show", beadID, "--json")
				cmd.Dir = d.workspaceDir

				output, err := cmd.Output()
				if err != nil {
					continue
				}

				var bead Bead
				if err := json.Unmarshal(output, &bead); err != nil {
					continue
				}

				// Check for problematic labels
				problematicLabels := []string{}
				for _, label := range bead.Labels {
					// Check for labels that might be causing issues
					if strings.HasPrefix(label, "human:") || strings.HasPrefix(label, "blocked:") {
						problematicLabels = append(problematicLabels, label)
					}
				}

				// Remove problematic labels
				for _, label := range problematicLabels {
					updateArgs := []string{"label", "remove", beadID, label}
					updateCmd := exec.Command(d.binaryPath, updateArgs...)
					updateCmd.Dir = d.workspaceDir

					if output, err := updateCmd.CombinedOutput(); err != nil {
						if d.auditLogger != nil {
							event := map[string]interface{}{
								"action":         "label_heal_failed",
								"bead_id":        beadID,
								"label":          label,
								"error":          err.Error(),
								"output":         string(output),
								"timestamp":      time.Now().Format(time.RFC3339),
							}
							_ = d.auditLogger.Log(event)
						}
					} else {
						if d.auditLogger != nil {
							event := map[string]interface{}{
								"action":         "label_healed",
								"bead_id":        beadID,
								"label":          label,
								"timestamp":      time.Now().Format(time.RFC3339),
							}
							_ = d.auditLogger.Log(event)
						}
					}
				}
			}
		}
	}

	return nil
}
				// Only remove if older than 10 minutes
				if time.Since(info.ModTime()) > 10*time.Minute {
					os.Remove(lockFile)
				}
			}
		}
	}

	return nil
}

// Start begins the starvation detection loop
func (d *StarvationDetector) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("detector already running")
	}
	d.running = true
	d.mu.Unlock()

	ticker := time.NewTicker(d.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.mu.Lock()
			d.running = false
			d.mu.Unlock()
			return nil
		case <-ticker.C:
			if err := d.checkAndAlert(); err != nil {
				// Log error but continue running
				if d.auditLogger != nil {
					event := map[string]interface{}{
						"action":    "starvation_check_failed",
						"error":     err.Error(),
						"timestamp": time.Now().Format(time.RFC3339),
					}
					_ = d.auditLogger.Log(event)
				}
			}
		}
	}
}

// Stop stops the starvation detector
func (d *StarvationDetector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.running = false
}

// IsRunning returns whether the detector is running
func (d *StarvationDetector) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// GetHistory returns the detection history
func (d *StarvationDetector) GetHistory() []StarvationCondition {
	d.mu.RLock()
	defer d.mu.RUnlock()

	history := make([]StarvationCondition, len(d.detectionHistory))
	copy(history, d.detectionHistory)
	return history
}

// RunScheduledDetection runs a single detection check for scheduled execution
func (d *StarvationDetector) RunScheduledDetection() (string, error) {
	// Detect starvation
	condition, err := d.detectStarvation()
	if err != nil {
		return "", fmt.Errorf("starvation detection failed: %w", err)
	}

	// No starvation detected
	if condition == nil {
		return "No starvation detected", nil
	}

	// Run diagnostics
	report, err := d.runDiagnostics(condition)
	if err != nil {
		return "", fmt.Errorf("diagnostics failed: %w", err)
	}

	// Auto-repair if enabled
	if d.autoRepairEnabled {
		if err := d.autoRepair(report); err != nil {
			return "", fmt.Errorf("auto-repair failed: %w", err)
		}
	}

	// Create diagnostic bead
	if err := d.createRemediationBead(report); err != nil {
		return "", fmt.Errorf("failed to create diagnostic bead: %w", err)
	}

	// Generate summary
	summary := d.generateSummary(report)

	return summary, nil
}

// generateSummary creates a human-readable summary
func (d *StarvationDetector) generateSummary(report *DiagnosticReport) string {
	var sb strings.Builder

	sb.WriteString("=== Starvation Detection Summary ===\n\n")
	sb.WriteString(fmt.Sprintf("Workspace: %s\n", report.Workspace))
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n\n", report.Timestamp.Format(time.RFC3339)))

	if report.Condition != nil {
		sb.WriteString(fmt.Sprintf("Open beads: %d\n", report.Condition.OpenBeads))
		sb.WriteString(fmt.Sprintf("Ready candidates: %d\n", report.Condition.ReadyCandidates))
		sb.WriteString(fmt.Sprintf("Excluded beads: %d\n\n", report.Condition.ExcludedBeads))
	}

	sb.WriteString(fmt.Sprintf("Validation checks passed: %d\n", countValidationPassed(report.ValidationResults)))
	sb.WriteString(fmt.Sprintf("Validation checks failed: %d\n", countValidationFailed(report.ValidationResults)))
	sb.WriteString(fmt.Sprintf("Fixable issues: %d\n", len(report.FixableIssues)))
	sb.WriteString(fmt.Sprintf("Manual intervention required: %d\n", len(report.ManualInterventionRequired)))
	sb.WriteString(fmt.Sprintf("Auto-repairs applied: %d\n", report.AutoRepairsApplied))

	return sb.String()
}

func countValidationPassed(results []ValidationResult) int {
	count := 0
	for _, r := range results {
		if r.Passed {
			count++
		}
	}
	return count
}

func countValidationFailed(results []ValidationResult) int {
	count := 0
	for _, r := range results {
		if !r.Passed {
			count++
		}
	}
	return count
}

// GetWorkspace returns the detector's workspace directory
func (d *StarvationDetector) GetWorkspace() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.workspaceDir
}

// GetCheckInterval returns the detector's check interval
func (d *StarvationDetector) GetCheckInterval() time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.checkInterval
}

// GetAlertCooldown returns the detector's alert cooldown period
func (d *StarvationDetector) GetAlertCooldown() time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.alertCooldown
}