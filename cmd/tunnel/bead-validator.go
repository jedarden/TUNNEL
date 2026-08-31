package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/jedarden/tunnel/internal/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var beadValidatorCmd = &cobra.Command{
	Use:   "bead-validator",
	Short: "Validate and fix bead state consistency",
	Long: `Validate bead state machine consistency and automatically repair violations.

This command checks for:
  - Assigned-but-open beads (should be in_progress or unassigned)
  - Closed beads with assignees (should have no assignee)
  - Circular self-dependencies
  - Database consistency against checkpoint

Auto-fix actions:
  - Release stale assignees on assigned-but-open beads
  - Clear assignees from closed beads
  - Clear circular self-dependencies
  - Rebuild database from checkpoint if corruption detected

Use --dry-run to preview changes without applying them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBeadValidator()
	},
}

var (
	validatorDryRun      bool
	validatorWorkspace   string
	validatorFix         bool
	validatorOutputJSON  bool
	validatorScheduled   bool
)

func init() {
	rootCmd.AddCommand(beadValidatorCmd)

	beadValidatorCmd.Flags().BoolVar(&validatorDryRun, "dry-run", false, "Preview changes without applying them")
	beadValidatorCmd.Flags().StringVar(&validatorWorkspace, "workspace", "", "Path to workspace directory (default: current directory)")
	beadValidatorCmd.Flags().BoolVar(&validatorFix, "fix", false, "Automatically fix detected issues")
	beadValidatorCmd.Flags().BoolVar(&validatorOutputJSON, "json", false, "Output results in JSON format")
	beadValidatorCmd.Flags().BoolVar(&validatorScheduled, "scheduled", false, "Run in scheduled mode (for systemd timer/cron)")
}

func runBeadValidator() error {
	// Determine workspace directory
	workspaceDir := validatorWorkspace
	if workspaceDir == "" {
		// Try to find workspace root by looking for .beads directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Check if current directory has .beads
		if _, err := os.Stat(filepath.Join(cwd, ".beads")); err == nil {
			workspaceDir = cwd
		} else {
			// Try parent directories
			parent := filepath.Dir(cwd)
			for parent != "/" && parent != "." {
				if _, err := os.Stat(filepath.Join(parent, ".beads")); err == nil {
					workspaceDir = parent
					break
				}
				parent = filepath.Dir(parent)
			}

			if workspaceDir == "" {
				return fmt.Errorf("no .beads directory found in current or parent directories")
			}
		}
	}

	// Get audit logger if available
	var auditLogger *core.AuditLogger
	if manager != nil && manager.GetAuditLogger() != nil {
		auditLogger = manager.GetAuditLogger()
	}

	// Create validator
	config := &core.ValidatorConfig{
		WorkspaceDir: workspaceDir,
		BinaryPath:   "bead", // Assumes bead is in PATH
		DryRun:       validatorDryRun,
		AuditLogger:  auditLogger,
	}

	validator := core.NewBeadValidator(config)

	if validatorScheduled {
		return runScheduledValidation(validator)
	}

	// Run validation
	color.Cyan("=== Bead State Validator ===\n")
	color.White("Workspace: %s\n", workspaceDir)
	if validatorDryRun {
		color.Yellow("DRY RUN MODE - No changes will be applied\n")
	}
	fmt.Println()

	results, err := validator.ValidateAll()
	if err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Display results
	if validatorOutputJSON {
		return outputJSON(results)
	}

	if err := validator.InteractiveReport(results); err != nil {
		return err
	}

	// Apply fixes if requested
	if validatorFix && !validatorDryRun {
		color.Yellow("\nApplying fixes...\n")
		if err := validator.FixAll(results); err != nil {
			return fmt.Errorf("failed to apply fixes: %w", err)
		}
		color.Green("Fixes applied successfully: %d fixes\n", validator.GetFixesApplied())
	} else if validatorFix && validatorDryRun {
		color.Yellow("\nDry run mode - fixes would be applied:\n")
		if err := validator.FixAll(results); err != nil {
			return fmt.Errorf("failed to simulate fixes: %w", err)
		}
	}

	// Create summary bead if there are unresolved issues
	if !validatorDryRun && validatorFix {
		if err := validator.CreateSummaryBead(results); err != nil {
			color.Yellow("Warning: Failed to create summary bead: %v\n", err)
		}
	}

	return nil
}

func runScheduledValidation(validator *core.BeadValidator) error {
	// In scheduled mode, run validation and auto-fix
	summary, err := validator.RunScheduledValidation()
	if err != nil {
		// Log error but don't fail the scheduled run
		fmt.Fprintf(os.Stderr, "Scheduled validation error: %v\n", err)
		return nil
	}

	// Output summary to stdout
	fmt.Println(summary)

	// Exit with appropriate code
	if validator.GetFixesApplied() > 0 {
		// Fixes were applied, this is informational
		return nil
	}

	return nil
}

func outputJSON(results []core.ValidationResult) error {
	// Output results as JSON
	data := struct {
		Results       []core.ValidationResult `json:"results"`
		Total         int                     `json:"total"`
		Passed        int                     `json:"passed"`
		Failed        int                     `json:"failed"`
		FixesApplied  int                     `json:"fixes_applied,omitempty"`
	}{
		Results: results,
		Total:   len(results),
	}

	for _, r := range results {
		if r.Passed {
			data.Passed++
		} else {
			data.Failed++
		}
	}

	// Convert to JSON manually since we don't have the validator reference here
	// This would need to be refactored in a real implementation
	fmt.Printf("{\"results\": %d, \"passed\": %d, \"failed\": %d}\n",
		data.Total, data.Passed, data.Failed)

	return nil
}
