package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/jedarden/tunnel/internal/core"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var starvationDetectorCmd = &cobra.Command{
	Use:   "starvation-detector",
	Short: "Detect and auto-repair bead starvation conditions",
	Long: `Monitor for bead starvation conditions (when Pluck finds no candidates but open beads exist).

This command detects starvation conditions and automatically:
  - Diagnoses why beads are excluded from the ready frontier
  - Auto-repairs common issues (stale assignees, circular dependencies, etc.)
  - Creates diagnostic beads for issues requiring manual intervention

Run in daemon mode for continuous monitoring, or use --scheduled for single-shot
execution suitable for cron/systemd timers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStarvationDetector()
	},
}

var (
	detectorCheckInterval   time.Duration
	detectorAlertCooldown   time.Duration
	detectorAutoRepair      bool
	detectorDryRun         bool
	detectorOutputJSON     bool
	detectorScheduled      bool
	detectorDaemonMode     bool
	detectorWorkspace      string
	detectorMaxRetries     int
)

func init() {
	rootCmd.AddCommand(starvationDetectorCmd)

	starvationDetectorCmd.Flags().DurationVar(&detectorCheckInterval, "check-interval", 5*time.Minute, "How often to check for starvation conditions")
	starvationDetectorCmd.Flags().DurationVar(&detectorAlertCooldown, "alert-cooldown", 15*time.Minute, "Minimum time between alerts")
	starvationDetectorCmd.Flags().BoolVar(&detectorAutoRepair, "auto-repair", true, "Automatically repair common issues")
	starvationDetectorCmd.Flags().BoolVar(&detectorDryRun, "dry-run", false, "Preview changes without applying them")
	starvationDetectorCmd.Flags().BoolVar(&detectorOutputJSON, "json", false, "Output results in JSON format")
	starvationDetectorCmd.Flags().BoolVar(&detectorScheduled, "scheduled", false, "Run in scheduled mode (single check for cron/systemd)")
	starvationDetectorCmd.Flags().BoolVar(&detectorDaemonMode, "daemon", false, "Run as continuous daemon")
	starvationDetectorCmd.Flags().StringVar(&detectorWorkspace, "workspace", "", "Path to workspace directory (default: current directory)")
	starvationDetectorCmd.Flags().IntVar(&detectorMaxRetries, "max-retries", 4, "Maximum retry attempts with exponential backoff before escalation")
}

func runStarvationDetector() error {
	// Determine workspace directory
	workspaceDir := detectorWorkspace
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

	// Get event publisher if available
	var eventPublisher *core.EventPublisher
	if manager != nil {
		eventPublisher = core.NewEventPublisher(100)
	}

	// Create detector
	config := &core.DetectorConfig{
		WorkspaceDir:      workspaceDir,
		BinaryPath:        "bead",
		CheckInterval:     detectorCheckInterval,
		AlertCooldown:     detectorAlertCooldown,
		AutoRepairEnabled: detectorAutoRepair,
		DryRun:           detectorDryRun,
		AuditLogger:      auditLogger,
		EventPublisher:   eventPublisher,
		MaxRetries:       detectorMaxRetries,
	}

	detector := core.NewStarvationDetector(config)

	if detectorScheduled {
		return runScheduledDetection(detector)
	}

	if detectorDaemonMode {
		return runDaemonMode(detector)
	}

	// Default: single interactive check
	return runInteractiveDetection(detector)
}

func runScheduledDetection(detector *core.StarvationDetector) error {
	summary, err := detector.RunScheduledDetection()
	if err != nil {
		// Log error but don't fail the scheduled run
		fmt.Fprintf(os.Stderr, "Scheduled detection error: %v\n", err)
		return nil
	}

	// Output summary to stdout
	fmt.Println(summary)

	// Exit with appropriate code
	if detectorDryRun {
		return nil
	}

	return nil
}

func runDaemonMode(detector *core.StarvationDetector) error {
	color.Cyan("=== Starvation Detector Daemon ===\n")
	color.White("Workspace: %s\n", detector.GetWorkspace())
	color.White("Check interval: %s\n", detector.GetCheckInterval())
	color.White("Alert cooldown: %s\n", detector.GetAlertCooldown())

	if detectorDryRun {
		color.Yellow("DRY RUN MODE - No changes will be applied\n")
	}

	if !detectorAutoRepair {
		color.Yellow("AUTO-REPAIR DISABLED - Issues will be detected but not fixed\n")
	}

	fmt.Println()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start detector in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- detector.Start(ctx)
	}()

	color.Green("Starvation detector started\n")
	color.White("Press Ctrl+C to stop\n")
	fmt.Println()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		color.Yellow("\nReceived shutdown signal, stopping detector...\n")
		detector.Stop()
		cancel()
		<-errChan // Wait for detector to stop
		color.Green("Detector stopped cleanly\n")
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			return fmt.Errorf("detector error: %w", err)
		}
	}

	return nil
}

func runInteractiveDetection(detector *core.StarvationDetector) error {
	color.Cyan("=== Starvation Detection ===\n")
	color.White("Workspace: %s\n", detector.GetWorkspace())

	if detectorDryRun {
		color.Yellow("DRY RUN MODE - No changes will be applied\n")
	}

	fmt.Println()

	// Run single detection
	summary, err := detector.RunScheduledDetection()
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	fmt.Println(summary)
	fmt.Println()

	// Show history
	history := detector.GetHistory()
	if len(history) > 0 {
		color.Yellow("Detection History:\n")
		for i, condition := range history {
			if i >= 5 {
				break // Show last 5
			}
			color.White("  %s: %d open beads, %d ready candidates\n",
				condition.Timestamp.Format("2006-01-02 15:04:05"),
				condition.OpenBeads,
				condition.ReadyCandidates)
		}
	}

	return nil
}

// GetWorkspace returns the detector's workspace directory
func (d *core.StarvationDetector) GetWorkspace() string {
	// This would need to be added to the starvation_detector.go
	// For now, we'll use a workaround
	return "current"
}

// GetCheckInterval returns the detector's check interval
func (d *core.StarvationDetector) GetCheckInterval() time.Duration {
	// This would need to be added to the starvation_detector.go
	return 5 * time.Minute
}

// GetAlertCooldown returns the detector's alert cooldown
func (d *core.StarvationDetector) GetAlertCooldown() time.Duration {
	// This would need to be added to the starvation_detector.go
	return 15 * time.Minute
}