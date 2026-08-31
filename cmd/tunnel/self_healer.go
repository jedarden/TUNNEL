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
)

var selfHealerCmd = &cobra.Command{
	Use:   "self-healer",
	Short: "Proactive self-healing to prevent bead starvation",
	Long: `Continuously monitor and automatically fix bead state issues before they cause starvation.

This service performs proactive maintenance every 5 minutes:
  - Clears stale assignees (beads open > 24 hours with assignee)
  - Unblocks beads with closed dependencies
  - Auto-fixes validation issues
  - Syncs checkpoint daily
  - Detects long-running open beads (> 7 days)

Run in daemon mode for continuous monitoring, or use --scheduled for single-shot
execution suitable for cron/systemd timers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSelfHealer()
	},
}

var (
	healerCheckInterval     time.Duration
	healerAutoRepair        bool
	healerDryRun           bool
	healerOutputJSON       bool
	healerScheduled        bool
	healerDaemonMode       bool
	healerWorkspace        string
	healerStaleThreshold   time.Duration
	healerLongRunningThreshold time.Duration
	healerCheckpointSyncInterval time.Duration
)

func init() {
	rootCmd.AddCommand(selfHealerCmd)

	selfHealerCmd.Flags().DurationVar(&healerCheckInterval, "check-interval", 5*time.Minute, "How often to run healing checks")
	selfHealerCmd.Flags().BoolVar(&healerAutoRepair, "auto-repair", true, "Automatically repair detected issues")
	selfHealerCmd.Flags().BoolVar(&healerDryRun, "dry-run", false, "Preview changes without applying them")
	selfHealerCmd.Flags().BoolVar(&healerOutputJSON, "json", false, "Output results in JSON format")
	selfHealerCmd.Flags().BoolVar(&healerScheduled, "scheduled", false, "Run in scheduled mode (single check for cron/systemd)")
	selfHealerCmd.Flags().BoolVar(&healerDaemonMode, "daemon", false, "Run as continuous daemon")
	selfHealerCmd.Flags().StringVar(&healerWorkspace, "workspace", "", "Path to workspace directory (default: current directory)")
	selfHealerCmd.Flags().DurationVar(&healerStaleThreshold, "stale-threshold", 24*time.Hour, "Threshold for stale assignees (default: 24h)")
	selfHealerCmd.Flags().DurationVar(&healerLongRunningThreshold, "long-running-threshold", 7*24*time.Hour, "Threshold for long-running beads (default: 7 days)")
	selfHealerCmd.Flags().DurationVar(&healerCheckpointSyncInterval, "checkpoint-sync-interval", 24*time.Hour, "Checkpoint sync interval (default: 24h)")
}

func runSelfHealer() error {
	// Determine workspace directory
	workspaceDir := healerWorkspace
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

	// Create self-healer
	config := &core.SelfHealerConfig{
		WorkspaceDir:           workspaceDir,
		BinaryPath:             "bead",
		CheckInterval:          healerCheckInterval,
		AutoRepairEnabled:      healerAutoRepair,
		DryRun:                healerDryRun,
		AuditLogger:           auditLogger,
		EventPublisher:        eventPublisher,
		StaleAssigneeThreshold: healerStaleThreshold,
		LongRunningThreshold:   healerLongRunningThreshold,
		CheckpointSyncInterval: healerCheckpointSyncInterval,
	}

	healer := core.NewSelfHealer(config)

	if healerScheduled {
		return runScheduledHealing(healer)
	}

	if healerDaemonMode {
		return runHealerDaemonMode(healer)
	}

	// Default: single interactive check
	return runInteractiveHealing(healer)
}

func runScheduledHealing(healer *core.SelfHealer) error {
	summary, err := healer.RunScheduledHealing()
	if err != nil {
		// Log error but don't fail the scheduled run
		fmt.Fprintf(os.Stderr, "Scheduled healing error: %v\n", err)
		return nil
	}

	// Output summary to stdout
	fmt.Println(summary)

	return nil
}

func runHealerDaemonMode(healer *core.SelfHealer) error {
	color.Cyan("=== Self-Healer Daemon ===\n")
	color.White("Workspace: %s\n", healer.GetWorkspace())
	color.White("Check interval: %s\n", healer.GetCheckInterval())
	color.White("Stale assignee threshold: %s\n", healerStaleThreshold)
	color.White("Long-running threshold: %s\n", healerLongRunningThreshold)
	color.White("Checkpoint sync interval: %s\n", healerCheckpointSyncInterval)

	if healerDryRun {
		color.Yellow("DRY RUN MODE - No changes will be applied\n")
	}

	if !healerAutoRepair {
		color.Yellow("AUTO-REPAIR DISABLED - Issues will be detected but not fixed\n")
	}

	fmt.Println()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start healer in background
	errChan := make(chan error, 1)
	go func() {
		errChan <- healer.Start(ctx)
	}()

	color.Green("Self-healer started\n")
	color.White("Press Ctrl+C to stop\n")
	fmt.Println()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		color.Yellow("\nReceived shutdown signal, stopping healer...\n")
		healer.Stop()
		cancel()
		<-errChan // Wait for healer to stop
		color.Green("Healer stopped cleanly\n")
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			return fmt.Errorf("healer error: %w", err)
		}
	}

	return nil
}

func runInteractiveHealing(healer *core.SelfHealer) error {
	color.Cyan("=== Self-Healing Check ===\n")
	color.White("Workspace: %s\n", healer.GetWorkspace())

	if healerDryRun {
		color.Yellow("DRY RUN MODE - No changes will be applied\n")
	}

	fmt.Println()

	// Run single healing cycle
	summary, err := healer.RunScheduledHealing()
	if err != nil {
		return fmt.Errorf("healing cycle failed: %w", err)
	}

	fmt.Println(summary)
	fmt.Println()

	// Show history
	history := healer.GetHistory()
	if len(history) > 0 {
		color.Yellow("Recent Healing Events:\n")
		shownCount := 0
		for i := len(history) - 1; i >= 0 && shownCount < 10; i++ {
			event := history[i]
			status := "✓"
			if !event.Success {
				status = "✗"
			}
			color.White("  [%s] %s: %s\n", status, event.EventType, event.Description)
			shownCount++
		}
	}

	return nil
}
