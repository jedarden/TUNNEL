package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BeadState represents the state of a bead
type BeadState string

const (
	BeadStateOpen       BeadState = "open"
	BeadStateInProgress BeadState = "in_progress"
	BeadStateClosed     BeadState = "closed"
	BeadStateDeferred   BeadState = "deferred"
)

// Bead represents a bead with its state
type Bead struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Status      BeadState  `json:"status"`
	Assignee    string     `json:"assignee,omitempty"`
	Priority    int        `json:"priority"`
	Revision    int        `json:"revision"`
	Labels      []string   `json:"labels,omitempty"`
	Blocks      []string   `json:"blocks,omitempty"`
	BlockedBy   []string   `json:"blocked_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ValidationResult represents the result of a validation check
type ValidationResult struct {
	Name        string   `json:"name"`
	Passed      bool     `json:"passed"`
	Description string   `json:"description"`
	Issues      []string `json:"issues,omitempty"`
	Fixable     bool     `json:"fixable"`
}

// BeadValidator handles bead state consistency validation
type BeadValidator struct {
	workspaceDir    string
	binaryPath      string
	dryRun          bool
	auditLogger     *AuditLogger
	fixesApplied    int
	mu              sync.Mutex
}

// ValidatorConfig holds configuration for the validator
type ValidatorConfig struct {
	WorkspaceDir    string
	BinaryPath      string // Path to bead binary (default: "bead")
	DryRun          bool   // If true, don't actually fix issues
	AuditLogger     *AuditLogger
}

// NewBeadValidator creates a new bead validator
func NewBeadValidator(config *ValidatorConfig) *BeadValidator {
	if config == nil {
		config = &ValidatorConfig{}
	}

	binaryPath := config.BinaryPath
	if binaryPath == "" {
		binaryPath = "bead"
	}

	return &BeadValidator{
		workspaceDir: config.WorkspaceDir,
		binaryPath:   binaryPath,
		dryRun:       config.DryRun,
		auditLogger:  config.AuditLogger,
	}
}

// ValidateAll runs all validation checks
func (v *BeadValidator) ValidateAll() ([]ValidationResult, error) {
	var results []ValidationResult
	var wg sync.WaitGroup
	resultsChan := make(chan ValidationResult, 10)

	// Run validation checks concurrently
	checks := []func() ValidationResult{
		v.validateWorkspaceBackend,
		v.validateAssignedButOpen,
		v.validateClosedWithAssignee,
		v.validateCircularDependencies,
		v.validateDatabaseConsistency,
		v.validateStaleDependencies,
		v.validateMissingLabels,
		v.validateWorkspaceConfiguration,
		v.validateBeadBackendHealth,
	}

	for _, check := range checks {
		wg.Add(1)
		go func(fn func() ValidationResult) {
			defer wg.Done()
			resultsChan <- fn()
		}(check)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	for result := range resultsChan {
		results = append(results, result)
	}

	return results, nil
}

// validateWorkspaceBackend checks that the bead backend matches configuration
func (v *BeadValidator) validateWorkspaceBackend() ValidationResult {
	result := ValidationResult{
		Name:        "workspace_backend",
		Description: "Verify workspace bead backend matches configuration",
		Fixable:     false,
	}

	configPath := filepath.Join(v.workspaceDir, ".needle.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		result.Passed = true // No needle config means no check needed
		return result
	}

	// Read needle config
	configData, err := os.ReadFile(configPath)
	if err != nil {
		result.Passed = false
		result.Issues = []string{fmt.Sprintf("Failed to read needle config: %v", err)}
		return result
	}

	// Check if it mentions bead-rs
	if strings.Contains(string(configData), "bead-rs") {
		// Verify we're using bead-rs
		beadConfigPath := filepath.Join(v.workspaceDir, ".beads", "config.json")
		if _, err := os.Stat(beadConfigPath); os.IsNotExist(err) {
			result.Passed = false
			result.Issues = []string{
				"Needle config specifies bead-rs but .beads/config.json not found",
				"Workspace may be using deprecated bead-forge backend",
			}
			return result
		}
	}

	result.Passed = true
	return result
}

// validateAssignedButOpen checks for beads that are assigned but in open status
func (v *BeadValidator) validateAssignedButOpen() ValidationResult {
	result := ValidationResult{
		Name:        "assigned_but_open",
		Description: "Check for assigned beads with open status (should be in_progress or unassigned)",
		Fixable:     true,
	}

	beads, err := v.listBeads("--status", "open")
	if err != nil {
		result.Passed = false
		result.Issues = []string{fmt.Sprintf("Failed to list open beads: %v", err)}
		return result
	}

	var issues []string
	for _, bead := range beads {
		if bead.Assignee != "" && bead.Status == BeadStateOpen {
			issues = append(issues, fmt.Sprintf(
				"Bead %s (%s) is assigned to %s but has status 'open'",
				bead.ID, bead.Title, bead.Assignee,
			))
		}
	}

	if len(issues) > 0 {
		result.Passed = false
		result.Issues = issues
	} else {
		result.Passed = true
	}

	return result
}

// validateClosedWithAssignee checks for closed beads that still have assignees
func (v *BeadValidator) validateClosedWithAssignee() ValidationResult {
	result := ValidationResult{
		Name:        "closed_with_assignee",
		Description: "Check for closed beads that still have assignees",
		Fixable:     true,
	}

	beads, err := v.listBeads("--status", "closed")
	if err != nil {
		result.Passed = false
		result.Issues = []string{fmt.Sprintf("Failed to list closed beads: %v", err)}
		return result
	}

	var issues []string
	for _, bead := range beads {
		if bead.Assignee != "" {
			issues = append(issues, fmt.Sprintf(
				"Bead %s (%s) is closed but still assigned to %s",
				bead.ID, bead.Title, bead.Assignee,
			))
		}
	}

	if len(issues) > 0 {
		result.Passed = false
		result.Issues = issues
	} else {
		result.Passed = true
	}

	return result
}

// validateCircularDependencies checks for beads blocking themselves
func (v *BeadValidator) validateCircularDependencies() ValidationResult {
	result := ValidationResult{
		Name:        "circular_dependencies",
		Description: "Check for beads with circular self-dependencies",
		Fixable:     true,
	}

	beads, err := v.listBeads("--status", "open", "--status", "in_progress")
	if err != nil {
		result.Passed = false
		result.Issues = []string{fmt.Sprintf("Failed to list beads: %v", err)}
		return result
	}

	var issues []string
	for _, bead := range beads {
		// Check if bead blocks itself
		for _, blockedID := range bead.Blocks {
			if blockedID == bead.ID {
				issues = append(issues, fmt.Sprintf(
					"Bead %s (%s) blocks itself",
					bead.ID, bead.Title,
				))
			}
		}

		// Check for simple 2-way cycles
		for _, blockedByID := range bead.BlockedBy {
			if blockedByID == bead.ID {
				issues = append(issues, fmt.Sprintf(
					"Bead %s (%s) is blocked by itself",
					bead.ID, bead.Title,
				))
			}
		}
	}

	if len(issues) > 0 {
		result.Passed = false
		result.Issues = issues
	} else {
		result.Passed = true
	}

	return result
}

// validateDatabaseConsistency checks database against checkpoint
func (v *BeadValidator) validateDatabaseConsistency() ValidationResult {
	result := ValidationResult{
		Name:        "database_consistency",
		Description: "Verify database consistency against checkpoint",
		Fixable:     true,
	}

	// Check if checkpoint exists
	checkpointPath := filepath.Join(v.workspaceDir, ".beads", "checkpoint", "current.json")
	if _, err := os.Stat(checkpointPath); os.IsNotExist(err) {
		result.Passed = false
		result.Issues = []string{
			"No checkpoint found at .beads/checkpoint/current.json",
			"Run 'bead sync flush-only' to create a checkpoint",
		}
		result.Fixable = false // User needs to create checkpoint first
		return result
	}

	// Compare bead counts
	dbBeads, err := v.listBeads()
	if err != nil {
		result.Passed = false
		result.Issues = []string{fmt.Sprintf("Failed to list beads from database: %v", err)}
		return result
	}

	// Read checkpoint
	checkpointData, err := os.ReadFile(checkpointPath)
	if err != nil {
		result.Passed = false
		result.Issues = []string{fmt.Sprintf("Failed to read checkpoint: %v", err)}
		return result
	}

	var checkpoint struct {
		Issues map[string]interface{} `json:"issues"`
	}
	if err := json.Unmarshal(checkpointData, &checkpoint); err != nil {
		result.Passed = false
		result.Issues = []string{fmt.Sprintf("Failed to parse checkpoint: %v", err)}
		return result
	}

	checkpointCount := len(checkpoint.Issues)
	dbCount := len(dbBeads)

	if checkpointCount != dbCount {
		result.Passed = false
		result.Issues = []string{
			fmt.Sprintf("Database has %d beads but checkpoint has %d beads", dbCount, checkpointCount),
			"Run 'bead sync flush-only' to sync database to checkpoint",
		}
		return result
	}

	result.Passed = true
	return result
}

// validateStaleDependencies checks for beads blocked by closed beads
func (v *BeadValidator) validateStaleDependencies() ValidationResult {
	result := ValidationResult{
		Name:        "stale_dependencies",
		Description: "Check for beads blocked by closed beads",
		Fixable:     true,
	}

	beads, err := v.listBeads("--status", "open", "--status", "in_progress")
	if err != nil {
		result.Passed = false
		result.Issues = []string{fmt.Sprintf("Failed to list beads: %v", err)}
		return result
	}

	var issues []string
	for _, bead := range beads {
		if len(bead.BlockedBy) == 0 {
			continue
		}

		// Check if any blocking beads are closed
		for _, blockerID := range bead.BlockedBy {
			if v.isBeadClosed(blockerID) {
				issues = append(issues, fmt.Sprintf(
					"Bead %s (%s) is blocked by closed bead %s",
					bead.ID, bead.Title, blockerID,
				))
			}
		}
	}

	if len(issues) > 0 {
		result.Passed = false
		result.Issues = issues
	} else {
		result.Passed = true
	}

	return result
}

// validateMissingLabels checks for beads missing expected labels
func (v *BeadValidator) validateMissingLabels() ValidationResult {
	result := ValidationResult{
		Name:        "missing_labels",
		Description: "Check for beads missing required labels based on their state",
		Fixable:     true,
	}

	beads, err := v.listBeads()
	if err != nil {
		result.Passed = false
		result.Issues = []string{fmt.Sprintf("Failed to list beads: %v", err)}
		return result
	}

	var issues []string
	for _, bead := range beads {
		// Check for high-priority beads without priority label
		if bead.Priority >= 3 {
			hasPriorityLabel := false
			for _, label := range bead.Labels {
				if strings.HasPrefix(label, "priority:") {
					hasPriorityLabel = true
					break
				}
			}
			if !hasPriorityLabel {
				issues = append(issues, fmt.Sprintf(
					"Bead %s (%s) has priority %d but no priority: label",
					bead.ID, bead.Title, bead.Priority,
				))
			}
		}

		// Check for in-progress beads without in-progress label
		if bead.Status == BeadStateInProgress {
			hasInProgressLabel := false
			for _, label := range bead.Labels {
				if label == "in-progress" || label == "wip" {
					hasInProgressLabel = true
					break
				}
			}
			if !hasInProgressLabel {
				issues = append(issues, fmt.Sprintf(
					"Bead %s (%s) is in_progress but missing in-progress label",
					bead.ID, bead.Title,
				))
			}
		}
	}

	if len(issues) > 0 {
		result.Passed = false
		result.Issues = issues
	} else {
		result.Passed = true
	}

	return result
}

// validateWorkspaceConfiguration checks for common workspace misconfigurations
func (v *BeadValidator) validateWorkspaceConfiguration() ValidationResult {
	result := ValidationResult{
		Name:        "workspace_configuration",
		Description: "Check for common workspace configuration issues",
		Fixable:     true,
	}

	var issues []string

	// Check for .needle.yaml without bead_cli.backend
	needleConfigPath := filepath.Join(v.workspaceDir, ".needle.yaml")
	if needleData, err := os.ReadFile(needleConfigPath); err == nil {
		if !strings.Contains(string(needleData), "bead_cli:") {
			issues = append(issues, ".needle.yaml exists but doesn't specify bead_cli.backend")
		}
	}

	// Check for mixed bead backends (bead-rs vs bead-forge indicators)
	beadRsConfig := filepath.Join(v.workspaceDir, ".beads", "config.json")
	bfConfig := filepath.Join(v.workspaceDir, ".beads", "config.yaml")

	beadRsExists, _ := os.Stat(beadRsConfig)
	bfExists, _ := os.Stat(bfConfig)

	if beadRsExists != nil && bfExists != nil {
		issues = append(issues, "No bead backend configuration found (.beads/config.json or .beads/config.yaml)")
	}

	if beadRsExists == nil && bfExists == nil {
		issues = append(issues, "Both bead-rs and bead-forge configurations exist (ambiguous backend)")
	}

	// Check for stale checkpoint (older than 24 hours)
	checkpointPath := filepath.Join(v.workspaceDir, ".beads", "checkpoint", "current.json")
	if info, err := os.Stat(checkpointPath); err == nil {
		if time.Since(info.ModTime()) > 24*time.Hour {
			issues = append(issues, "Checkpoint is older than 24 hours - may be stale")
		}
	}

	if len(issues) > 0 {
		result.Passed = false
		result.Issues = issues
	} else {
		result.Passed = true
	}

	return result
}

// validateBeadBackendHealth checks if bead backend service is healthy
func (v *BeadValidator) validateBeadBackendHealth() ValidationResult {
	result := ValidationResult{
		Name:        "bead_backend_health",
		Description: "Check if bead backend service is responsive",
		Fixable:     true,
	}

	// Try to run a simple bead command
	cmd := exec.Command(v.binaryPath, "--version")
	output, err := cmd.CombinedOutput()

	if err != nil {
		result.Passed = false
		result.Issues = []string{
			fmt.Sprintf("Bead backend not responsive: %v", err),
			"Output: " + string(output),
		}
		return result
	}

	// Check if we can list beads
	_, err = v.listBeads("--limit", "1")
	if err != nil {
		result.Passed = false
		result.Issues = []string{
			fmt.Sprintf("Bead backend unable to list beads: %v", err),
			"Backend may need to be restarted",
		}
		return result
	}

	result.Passed = true
	return result
}

// isBeadClosed checks if a bead is closed
func (v *BeadValidator) isBeadClosed(beadID string) bool {
	cmd := exec.Command(v.binaryPath, "show", beadID, "--json")
	cmd.Dir = v.workspaceDir

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

// listBeads lists all beads with optional filters
func (v *BeadValidator) listBeads(filters ...string) ([]Bead, error) {
	args := []string{"list", "--json"}
	args = append(args, filters...)

	cmd := exec.Command(v.binaryPath, args...)
	cmd.Dir = v.workspaceDir

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

// FixAll attempts to fix all validation issues
func (v *BeadValidator) FixAll(results []ValidationResult) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.fixesApplied = 0

	for _, result := range results {
		if result.Passed || !result.Fixable {
			continue
		}

		switch result.Name {
		case "assigned_but_open":
			if err := v.fixAssignedButOpen(); err != nil {
				return fmt.Errorf("failed to fix assigned-but-open beads: %w", err)
			}
		case "closed_with_assignee":
			if err := v.fixClosedWithAssignee(); err != nil {
				return fmt.Errorf("failed to fix closed-with-assignee beads: %w", err)
			}
		case "circular_dependencies":
			if err := v.fixCircularDependencies(); err != nil {
				return fmt.Errorf("failed to fix circular dependencies: %w", err)
			}
		case "database_consistency":
			if err := v.fixDatabaseConsistency(); err != nil {
				return fmt.Errorf("failed to fix database consistency: %w", err)
			}
		case "stale_dependencies":
			if err := v.fixStaleDependencies(); err != nil {
				return fmt.Errorf("failed to fix stale dependencies: %w", err)
			}
		case "missing_labels":
			if err := v.fixMissingLabels(); err != nil {
				return fmt.Errorf("failed to fix missing labels: %w", err)
			}
		case "workspace_configuration":
			if err := v.fixWorkspaceConfiguration(); err != nil {
				return fmt.Errorf("failed to fix workspace configuration: %w", err)
			}
		case "bead_backend_health":
			if err := v.fixBeadBackendHealth(); err != nil {
				return fmt.Errorf("failed to fix bead backend health: %w", err)
			}
		}
	}

	return nil
}

// fixAssignedButOpen releases assignees on assigned-but-open beads
func (v *BeadValidator) fixAssignedButOpen() error {
	beads, err := v.listBeads("--status", "open")
	if err != nil {
		return err
	}

	for _, bead := range beads {
		if bead.Assignee != "" && bead.Status == BeadStateOpen {
			if v.dryRun {
				fmt.Printf("[DRY RUN] Would release assignee from bead %s\n", bead.ID)
				v.fixesApplied++
				continue
			}

			if err := v.runBeadCommand("update", bead.ID, "--clear-assignee"); err != nil {
				v.logAudit("fix_assigned_but_open", bead.ID, false, err.Error())
				return err
			}

			v.logAudit("fix_assigned_but_open", bead.ID, true, "")
			v.fixesApplied++
		}
	}

	return nil
}

// fixClosedWithAssignee clears assignees from closed beads
func (v *BeadValidator) fixClosedWithAssignee() error {
	beads, err := v.listBeads("--status", "closed")
	if err != nil {
		return err
	}

	for _, bead := range beads {
		if bead.Assignee != "" {
			if v.dryRun {
				fmt.Printf("[DRY RUN] Would clear assignee from closed bead %s\n", bead.ID)
				v.fixesApplied++
				continue
			}

			if err := v.runBeadCommand("update", bead.ID, "--clear-assignee"); err != nil {
				v.logAudit("fix_closed_with_assignee", bead.ID, false, err.Error())
				return err
			}

			v.logAudit("fix_closed_with_assignee", bead.ID, true, "")
			v.fixesApplied++
		}
	}

	return nil
}

// fixCircularDependencies removes circular self-dependencies
func (v *BeadValidator) fixCircularDependencies() error {
	beads, err := v.listBeads("--status", "open", "--status", "in_progress")
	if err != nil {
		return err
	}

	for _, bead := range beads {
		needsFix := false

		// Check if bead blocks itself
		for _, blockedID := range bead.Blocks {
			if blockedID == bead.ID {
				needsFix = true
				break
			}
		}

		if !needsFix {
			// Check if bead is blocked by itself
			for _, blockedByID := range bead.BlockedBy {
				if blockedByID == bead.ID {
					needsFix = true
					break
				}
			}
		}

		if needsFix {
			if v.dryRun {
				fmt.Printf("[DRY RUN] Would fix circular dependencies for bead %s\n", bead.ID)
				v.fixesApplied++
				continue
			}

			// Use bead dep command to clear dependencies
			if err := v.runBeadCommand("dep", bead.ID, "--clear"); err != nil {
				v.logAudit("fix_circular_dependencies", bead.ID, false, err.Error())
				return err
			}

			v.logAudit("fix_circular_dependencies", bead.ID, true, "")
			v.fixesApplied++
		}
	}

	return nil
}

// fixDatabaseConsistency rebuilds database from checkpoint
func (v *BeadValidator) fixDatabaseConsistency() error {
	checkpointPath := filepath.Join(v.workspaceDir, ".beads", "checkpoint", "forensic.jsonl")

	if _, err := os.Stat(checkpointPath); os.IsNotExist(err) {
		return fmt.Errorf("no forensic checkpoint found at %s", checkpointPath)
	}

	if v.dryRun {
		fmt.Printf("[DRY RUN] Would rebuild database from checkpoint\n")
		v.fixesApplied++
		return nil
	}

	// Rebuild database from checkpoint
	if err := v.runBeadCommand("sync", "import-only", "--input", checkpointPath, "--restore-into-empty"); err != nil {
		v.logAudit("fix_database_consistency", "workspace", false, err.Error())
		return err
	}

	v.logAudit("fix_database_consistency", "workspace", true, "")
	v.fixesApplied++
	return nil
}

// fixStaleDependencies removes dependencies on closed beads
func (v *BeadValidator) fixStaleDependencies() error {
	beads, err := v.listBeads("--status", "open", "--status", "in_progress")
	if err != nil {
		return err
	}

	for _, bead := range beads {
		if len(bead.BlockedBy) == 0 {
			continue
		}

		// Find closed blocking beads
		var closedBlockers []string
		for _, blockerID := range bead.BlockedBy {
			if v.isBeadClosed(blockerID) {
				closedBlockers = append(closedBlockers, blockerID)
			}
		}

		if len(closedBlockers) == 0 {
			continue
		}

		if v.dryRun {
			fmt.Printf("[DRY RUN] Would unblock bead %s from closed blockers: %v\n", bead.ID, closedBlockers)
			v.fixesApplied++
			continue
		}

		// Use bead dep command to remove specific blockers
		for _, blockerID := range closedBlockers {
			if err := v.runBeadCommand("dep", bead.ID, "--remove", blockerID); err != nil {
				v.logAudit("fix_stale_dependencies", bead.ID, false, err.Error())
				return err
			}
		}

		v.logAudit("fix_stale_dependencies", bead.ID, true, fmt.Sprintf("unblocked from %v", closedBlockers))
		v.fixesApplied++
	}

	return nil
}

// fixMissingLabels adds missing labels based on bead state
func (v *BeadValidator) fixMissingLabels() error {
	beads, err := v.listBeads()
	if err != nil {
		return err
	}

	for _, bead := range beads {
		var labelsToAdd []string

		// Add priority label for high-priority beads
		if bead.Priority >= 3 {
			hasPriorityLabel := false
			for _, label := range bead.Labels {
				if strings.HasPrefix(label, "priority:") {
					hasPriorityLabel = true
					break
				}
			}
			if !hasPriorityLabel {
				labelsToAdd = append(labelsToAdd, fmt.Sprintf("priority:%d", bead.Priority))
			}
		}

		// Add in-progress label for in-progress beads
		if bead.Status == BeadStateInProgress {
			hasInProgressLabel := false
			for _, label := range bead.Labels {
				if label == "in-progress" || label == "wip" {
					hasInProgressLabel = true
					break
				}
			}
			if !hasInProgressLabel {
				labelsToAdd = append(labelsToAdd, "in-progress")
			}
		}

		if len(labelsToAdd) == 0 {
			continue
		}

		if v.dryRun {
			fmt.Printf("[DRY RUN] Would add labels to bead %s: %v\n", bead.ID, labelsToAdd)
			v.fixesApplied++
			continue
		}

		// Add labels using bead label command
		for _, label := range labelsToAdd {
			if err := v.runBeadCommand("label", bead.ID, "--add", label); err != nil {
				v.logAudit("fix_missing_labels", bead.ID, false, err.Error())
				return err
			}
		}

		v.logAudit("fix_missing_labels", bead.ID, true, fmt.Sprintf("added labels: %v", labelsToAdd))
		v.fixesApplied++
	}

	return nil
}

// fixWorkspaceConfiguration fixes common workspace configuration issues
func (v *BeadValidator) fixWorkspaceConfiguration() error {
	var fixesApplied int

	// Flush checkpoint to create/update it
	checkpointPath := filepath.Join(v.workspaceDir, ".beads", "checkpoint", "current.json")
	if info, err := os.Stat(checkpointPath); err == nil {
		if time.Since(info.ModTime()) > 24*time.Hour {
			if v.dryRun {
				fmt.Printf("[DRY RUN] Would flush stale checkpoint\n")
				fixesApplied++
			} else {
				if err := v.runBeadCommand("sync", "flush-only"); err != nil {
					v.logAudit("fix_workspace_configuration", "workspace", false, err.Error())
					return err
				}
				v.logAudit("fix_workspace_configuration", "workspace", true, "flushed checkpoint")
				fixesApplied++
			}
		}
	}

	// Note: We don't auto-fix .needle.yaml issues or mixed backend configs
	// as those require human decisions about which backend to use

	if fixesApplied > 0 {
		v.fixesApplied += fixesApplied
	}

	return nil
}

// fixBeadBackendHealth attempts to restart bead backend if needed
func (v *BeadValidator) fixBeadBackendHealth() error {
	// Try a simple bead command first to see if it's just a transient issue
	cmd := exec.Command(v.binaryPath, "--version")
	if err := cmd.Run(); err == nil {
		// Backend is actually fine, no fix needed
		return nil
	}

	// If bead backend is not responding, we can't fix it programmatically
	// This would require systemd/service management which is outside our scope
	// Log the issue for manual intervention
	if v.dryRun {
		fmt.Printf("[DRY RUN] Bead backend not responding - requires manual restart\n")
		return nil
	}

	v.logAudit("fix_bead_backend_health", "workspace", false, "backend not responding - requires manual restart")
	return fmt.Errorf("bead backend is not responding and cannot be auto-restarted - manual intervention required")
}

// runBeadCommand executes a bead command
func (v *BeadValidator) runBeadCommand(args ...string) error {
	cmd := exec.Command(v.binaryPath, args...)
	cmd.Dir = v.workspaceDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bead command failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// logAudit logs an action to the audit logger
func (v *BeadValidator) logAudit(action, target string, success bool, message string) {
	if v.auditLogger == nil {
		return
	}

	event := map[string]interface{}{
		"action":     action,
		"target":     target,
		"success":    success,
		"message":    message,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	_ = v.auditLogger.Log(event)
}

// GetFixesApplied returns the number of fixes applied
func (v *BeadValidator) GetFixesApplied() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.fixesApplied
}

// GenerateSummaryReport creates a summary report for unresolved issues
func (v *BeadValidator) GenerateSummaryReport(results []ValidationResult) string {
	var sb strings.Builder

	sb.WriteString("=== Bead State Validation Summary ===\n\n")

	passed := 0
	failed := 0
	fixableFailed := 0

	for _, result := range results {
		if result.Passed {
			passed++
		} else {
			failed++
			if result.Fixable {
				fixableFailed++
			}
		}
	}

	sb.WriteString(fmt.Sprintf("Total Checks: %d\n", len(results)))
	sb.WriteString(fmt.Sprintf("Passed: %d\n", passed))
	sb.WriteString(fmt.Sprintf("Failed: %d\n", failed))
	sb.WriteString(fmt.Sprintf("Fixable: %d\n", fixableFailed))
	sb.WriteString(fmt.Sprintf("Fixes Applied: %d\n", v.GetFixesApplied()))
	sb.WriteString("\n")

	// List failed checks
	if failed > 0 {
		sb.WriteString("Failed Checks:\n")
		for _, result := range results {
			if !result.Passed {
				sb.WriteString(fmt.Sprintf("  - %s: %s\n", result.Name, result.Description))
				for _, issue := range result.Issues {
					sb.WriteString(fmt.Sprintf("    * %s\n", issue))
				}
				if result.Fixable {
					sb.WriteString("    [Fixable]\n")
				} else {
					sb.WriteString("    [Manual intervention required]\n")
				}
			}
		}
	}

	return sb.String()
}

// CreateSummaryBead creates a bead for the validation summary if there are unresolved issues
func (v *BeadValidator) CreateSummaryBead(results []ValidationResult) error {
	// Check if there are any unresolved issues
	hasUnresolved := false
	for _, result := range results {
		if !result.Passed && !result.Fixable {
			hasUnresolved = true
			break
		}
	}

	if !hasUnresolved && v.GetFixesApplied() == 0 {
		return nil // No issues, no bead needed
	}

	// Generate summary
	summary := v.GenerateSummaryReport(results)

	// Generate a unique ID for the summary bead
	timestamp := time.Now().Format("20060102-150405")
	beadID := fmt.Sprintf("bead-validator-%s", timestamp)

	// Create bead command
	if v.dryRun {
		fmt.Printf("[DRY RUN] Would create summary bead: %s\n", beadID)
		fmt.Printf("Summary content:\n%s\n", summary)
		return nil
	}

	// Note: This would require the bead CLI to support creating beads from stdin/args
	// For now, we'll just log the summary
	fmt.Printf("Summary Report:\n%s\n", summary)

	return nil
}

// ValidateAndFix performs validation and auto-fix in one operation
func (v *BeadValidator) ValidateAndFix() ([]ValidationResult, error) {
	// Run validation
	results, err := v.ValidateAll()
	if err != nil {
		return nil, err
	}

	// Auto-fix if not in dry-run mode
	if !v.dryRun {
		if err := v.FixAll(results); err != nil {
			return results, fmt.Errorf("validation completed but fixes failed: %w", err)
		}
	}

	return results, nil
}

// RunScheduledValidation runs validation and generates report for scheduled execution
func (v *BeadValidator) RunScheduledValidation() (string, error) {
	results, err := v.ValidateAndFix()
	if err != nil {
		return "", err
	}

	summary := v.GenerateSummaryReport(results)

	// Create summary bead if there are unresolved issues
	if err := v.CreateSummaryBead(results); err != nil {
		return summary, fmt.Errorf("validation completed but failed to create summary bead: %w", err)
	}

	return summary, nil
}

// InteractiveReport presents validation results in a human-readable format
func (v *BeadValidator) InteractiveReport(results []ValidationResult) error {
	fmt.Println("\n=== Bead State Validation Results ===\n")

	for _, result := range results {
		status := "✓ PASS"
		color := "\033[32m" // Green
		if !result.Passed {
			status = "✗ FAIL"
			color = "\033[31m" // Red
		}

		fmt.Printf("%s%s\033[0m: %s\n", color, status, result.Description)
		fmt.Printf("  Check: %s\n", result.Name)

		if !result.Passed {
			for _, issue := range result.Issues {
				fmt.Printf("  - %s\n", issue)
			}
			if result.Fixable {
				fmt.Printf("  Status: Auto-fixable\n")
			} else {
				fmt.Printf("  Status: Manual intervention required\n")
			}
		}
		fmt.Println()
	}

	// Print summary
	fmt.Printf("Summary: %d passed, %d failed\n",
		countPassed(results), countFailed(results))

	if v.GetFixesApplied() > 0 {
		fmt.Printf("Fixes applied: %d\n", v.GetFixesApplied())
	}

	return nil
}

func countPassed(results []ValidationResult) int {
	count := 0
	for _, r := range results {
		if r.Passed {
			count++
		}
	}
	return count
}

func countFailed(results []ValidationResult) int {
	count := 0
	for _, r := range results {
		if !r.Passed {
			count++
		}
	}
	return count
}

// Validate reads beads from stdin in JSON format and validates
func (v *BeadValidator) Validate(input *bufio.Reader) error {
	var beads []Bead
	decoder := json.NewDecoder(input)
	if err := decoder.Decode(&beads); err != nil {
		return fmt.Errorf("failed to parse beads from stdin: %w", err)
	}

	// Run validation checks
	results := []ValidationResult{
		v.validateAssignedButOpenFromList(beads),
		v.validateClosedWithAssigneeFromList(beads),
		v.validateCircularDependenciesFromList(beads),
	}

	// Print results
	return v.InteractiveReport(results)
}

// validateAssignedButOpenFromList validates assigned-but-open from provided bead list
func (v *BeadValidator) validateAssignedButOpenFromList(beads []Bead) ValidationResult {
	result := ValidationResult{
		Name:        "assigned_but_open",
		Description: "Check for assigned beads with open status",
		Fixable:     true,
	}

	var issues []string
	for _, bead := range beads {
		if bead.Assignee != "" && bead.Status == BeadStateOpen {
			issues = append(issues, fmt.Sprintf(
				"Bead %s (%s) is assigned to %s but has status 'open'",
				bead.ID, bead.Title, bead.Assignee,
			))
		}
	}

	if len(issues) > 0 {
		result.Passed = false
		result.Issues = issues
	} else {
		result.Passed = true
	}

	return result
}

// validateClosedWithAssigneeFromList validates closed-with-assignee from provided bead list
func (v *BeadValidator) validateClosedWithAssigneeFromList(beads []Bead) ValidationResult {
	result := ValidationResult{
		Name:        "closed_with_assignee",
		Description: "Check for closed beads with assignees",
		Fixable:     true,
	}

	var issues []string
	for _, bead := range beads {
		if bead.Status == BeadStateClosed && bead.Assignee != "" {
			issues = append(issues, fmt.Sprintf(
				"Bead %s (%s) is closed but still assigned to %s",
				bead.ID, bead.Title, bead.Assignee,
			))
		}
	}

	if len(issues) > 0 {
		result.Passed = false
		result.Issues = issues
	} else {
		result.Passed = true
	}

	return result
}

// validateCircularDependenciesFromList validates circular dependencies from provided bead list
func (v *BeadValidator) validateCircularDependenciesFromList(beads []Bead) ValidationResult {
	result := ValidationResult{
		Name:        "circular_dependencies",
		Description: "Check for circular self-dependencies",
		Fixable:     true,
	}

	var issues []string
	for _, bead := range beads {
		// Check if bead blocks itself
		for _, blockedID := range bead.Blocks {
			if blockedID == bead.ID {
				issues = append(issues, fmt.Sprintf(
					"Bead %s (%s) blocks itself",
					bead.ID, bead.Title,
				))
			}
		}

		// Check if bead is blocked by itself
		for _, blockedByID := range bead.BlockedBy {
			if blockedByID == bead.ID {
				issues = append(issues, fmt.Sprintf(
					"Bead %s (%s) is blocked by itself",
					bead.ID, bead.Title,
				))
			}
		}
	}

	if len(issues) > 0 {
		result.Passed = false
		result.Issues = issues
	} else {
		result.Passed = true
	}

	return result
}
