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

// createDiagnosticBead creates a bead for manual intervention issues
func (d *StarvationDetector) createDiagnosticBead(report *DiagnosticReport) error {
	if len(report.ManualInterventionRequired) == 0 && report.AutoRepairsApplied == 0 {
		// No issues requiring attention
		return nil
	}

	// Generate bead description
	description := d.generateDiagnosticDescription(report)

	// Create bead using bead CLI
	args := []string{
		"create",
		"--title", d.generateDiagnosticTitle(report),
		"--priority", "2", // P2 for diagnostic issues
		"--issue-type", "task",
		"--label", "alert:starvation-diagnostic",
	}

	cmd := exec.Command(d.binaryPath, args...)
	cmd.Dir = d.workspaceDir

	// Set description via stdin
	cmd.Stdin = strings.NewReader(description)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create diagnostic bead: %w\nOutput: %s", err, string(output))
	}

	// Log bead creation
	if d.auditLogger != nil {
		event := map[string]interface{}{
			"action":     "create_diagnostic_bead",
			"workspace":  d.workspaceDir,
			"timestamp":  time.Now().Format(time.RFC3339),
		}
		_ = d.auditLogger.Log(event)
	}

	return nil
}

// generateDiagnosticTitle creates a title for the diagnostic bead
func (d *StarvationDetector) generateDiagnosticTitle(report *DiagnosticReport) string {
	if len(report.ManualInterventionRequired) > 0 {
		return fmt.Sprintf("Starvation diagnostic: %d issues requiring manual intervention",
			len(report.ManualInterventionRequired))
	}
	return fmt.Sprintf("Starvation diagnostic: %d auto-repairs applied",
		report.AutoRepairsApplied)
}

// generateDiagnosticDescription creates a detailed description for the diagnostic bead
func (d *StarvationDetector) generateDiagnosticDescription(report *DiagnosticReport) string {
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
		sb.WriteString("## Manual Intervention Required\n\n")
		sb.WriteString("The following issues require manual review:\n\n")
		for _, issue := range report.ManualInterventionRequired {
			sb.WriteString(fmt.Sprintf("### %s\n", issue.Name))
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

	if len(report.ValidationErrors) > 0 {
		sb.WriteString("## Validation Errors\n\n")
		for _, err := range report.ValidationErrors {
			sb.WriteString(fmt.Sprintf("- %s\n", err))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// checkAndAlert runs a single check and alerts if starvation is detected
func (d *StarvationDetector) checkAndAlert() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if we're in cooldown
	if time.Since(d.lastAlertTime) < d.alertCooldown {
		return nil
	}

	// Detect starvation
	condition, err := d.detectStarvation()
	if err != nil {
		return fmt.Errorf("starvation detection failed: %w", err)
	}

	// Update last check time
	d.lastCheckTime = time.Now()

	// No starvation detected
	if condition == nil {
		return nil
	}

	// Add to history
	d.detectionHistory = append(d.detectionHistory, *condition)
	if len(d.detectionHistory) > d.maxHistorySize {
		d.detectionHistory = d.detectionHistory[1:]
	}

	// Run diagnostics
	report, err := d.runDiagnostics(condition)
	if err != nil {
		return fmt.Errorf("diagnostics failed: %w", err)
	}

	// Publish event
	if d.eventPublisher != nil {
		event := map[string]interface{}{
			"type":      "starvation_detected",
			"condition": condition,
			"report":    report,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		_ = d.eventPublisher.Publish(event)
	}

	// Auto-repair if enabled
	if d.autoRepairEnabled {
		if err := d.autoRepair(report); err != nil {
			// Log error but continue
			if d.auditLogger != nil {
				event := map[string]interface{}{
					"action":   "auto_repair_failed",
					"error":    err.Error(),
					"timestamp": time.Now().Format(time.RFC3339),
				}
				_ = d.auditLogger.Log(event)
			}
		}
	}

	// Create diagnostic bead for manual intervention issues
	if err := d.createDiagnosticBead(report); err != nil {
		return fmt.Errorf("failed to create diagnostic bead: %w", err)
	}

	// Update last alert time
	d.lastAlertTime = time.Now()

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
	if err := d.createDiagnosticBead(report); err != nil {
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