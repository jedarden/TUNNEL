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
}

// StarvationDetector monitors for bead starvation conditions
type StarvationDetector struct {
	workspaceDir      string
	binaryPath        string
	beadValidator     *BeadValidator
	auditLogger       *AuditLogger
	eventPublisher    *EventPublisher

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

	// Detection history
	detectionHistory  []StarvationCondition
	maxHistorySize    int

	// Retry state with exponential backoff
	retryState        *RetryState
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

// Default retry backoff sequence: 5m → 15m → 30m → 1h
var defaultBackoffSequence = []time.Duration{
	5 * time.Minute,
	15 * time.Minute,
	30 * time.Minute,
	60 * time.Minute,
}

// DetectorConfig holds configuration for the starvation detector
type DetectorConfig struct {
	WorkspaceDir       string
	BinaryPath         string // Path to bead binary (default: "bead")
	CheckInterval      time.Duration
	AlertCooldown      time.Duration
	AutoRepairEnabled  bool
	DryRun            bool
	AuditLogger       *AuditLogger
	EventPublisher    *EventPublisher
	MaxHistorySize    int
	MaxRetries        int           // Maximum retry attempts before escalation (default: 4)
	BackoffSequence  []time.Duration // Custom backoff sequence (optional)
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

	return &StarvationDetector{
		workspaceDir:      config.WorkspaceDir,
		binaryPath:        config.BinaryPath,
		beadValidator:     NewBeadValidator(validatorConfig),
		auditLogger:       config.AuditLogger,
		eventPublisher:    config.EventPublisher,
		checkInterval:     config.CheckInterval,
		alertCooldown:     config.AlertCooldown,
		autoRepairEnabled: config.AutoRepairEnabled,
		dryRun:            config.DryRun,
		detectionHistory:  make([]StarvationCondition, 0, config.MaxHistorySize),
		maxHistorySize:    config.MaxHistorySize,
		retryState: &RetryState{
			MaxRetries:       config.MaxRetries,
			BackoffDurations: backoffSequence,
		},
	}
}

// detectStarvation checks for starvation conditions
func (d *StarvationDetector) detectStarvation() (*StarvationCondition, error) {
	condition := &StarvationCondition{
		Timestamp: time.Now(),
		Workspace: d.workspaceDir,
	}

	// Get ready candidates
	readyBeads, err := d.listBeads("--ready")
	if err != nil {
		return nil, fmt.Errorf("failed to list ready beads: %w", err)
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

	// Starvation detected! Now diagnose why
	excludedBeads := 0
	exclusionReasons := []string{}
	staleAssignees := 0
	closedWithAssignee := 0
	circularDeps := 0

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
	sb.WriteString("Review the exclusion reasons and manual intervention items above to resolve the underlying issue.\n\n")

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