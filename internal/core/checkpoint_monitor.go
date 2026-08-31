package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CheckpointMetrics holds metrics about the bead checkpoint
type CheckpointMetrics struct {
	Timestamp        time.Time `json:"timestamp"`
	DatabasePath      string    `json:"database_path"`
	CheckpointDir     string    `json:"checkpoint_dir"`
	DatabaseSize      int64     `json:"database_size_bytes"`
	DatabaseIssueCount int      `json:"database_issue_count"`
	ForensicSize      int64     `json:"forensic_size_bytes"`
	ForensicLineCount int       `json:"forensic_line_count"`
	ObjectsDirSize    int64     `json:"objects_dir_size_bytes"`
	ObjectsFileCount  int       `json:"objects_file_count"`
	TotalRecordCount  int       `json:"total_record_count"`
	IsActiveRootSize  int64     `json:"is_active_root_size"`
	EventCount        int       `json:"event_count"`
}

// CheckpointThresholds defines alert thresholds for checkpoint metrics
type CheckpointThresholds struct {
	MaxDatabaseSize      int64         `json:"max_database_size_bytes"`       // Default: 100MB
	MaxForensicSize      int64         `json:"max_forensic_size_bytes"`       // Default: 50MB
	MaxObjectsDirSize    int64         `json:"max_objects_dir_size_bytes"`    // Default: 200MB
	MaxIssueCount        int           `json:"max_issue_count"`               // Default: 1000
	MaxForensicLineCount int           `json:"max_forensic_line_count"`       // Default: 10000
	MaxTotalRecordCount  int           `json:"max_total_record_count"`        // Default: 5000
	CheckInterval       time.Duration `json:"check_interval"`                 // Default: 5 minutes
	AlertCooldown        time.Duration `json:"alert_cooldown"`                // Default: 1 hour
}

// DefaultCheckpointThresholds returns sensible default thresholds
func DefaultCheckpointThresholds() *CheckpointThresholds {
	return &CheckpointThresholds{
		MaxDatabaseSize:      100 * 1024 * 1024, // 100MB
		MaxForensicSize:      50 * 1024 * 1024,  // 50MB
		MaxObjectsDirSize:    200 * 1024 * 1024, // 200MB
		MaxIssueCount:        1000,
		MaxForensicLineCount: 10000,
		MaxTotalRecordCount:  5000,
		CheckInterval:        5 * time.Minute,
		AlertCooldown:        1 * time.Hour,
	}
}

// CheckpointAlert represents an alert triggered by threshold violation
type CheckpointAlert struct {
	Timestamp   time.Time `json:"timestamp"`
	AlertType   string    `json:"alert_type"`
	MetricName  string    `json:"metric_name"`
	CurrentValue float64  `json:"current_value"`
	Threshold   float64   `json:"threshold"`
	Message     string    `json:"message"`
	Severity    string    `json:"severity"` // "warning", "critical"
}

// CheckpointMonitor monitors bead checkpoint metrics and generates alerts
type CheckpointMonitor struct {
	mu            sync.RWMutex
	workspaceDir  string
	thresholds    *CheckpointThresholds
	eventPublisher *EventPublisher
	auditLogger   *AuditLogger

	// Alert tracking
	lastAlertTime map[string]time.Time // metric_name -> last alert time

	// Control
	ctx           context.Context
	cancel        context.CancelFunc
	running       bool
	wg            sync.WaitGroup
}

// NewCheckpointMonitor creates a new checkpoint monitor
func NewCheckpointMonitor(workspaceDir string, thresholds *CheckpointThresholds, eventPublisher *EventPublisher, auditLogger *AuditLogger) *CheckpointMonitor {
	if thresholds == nil {
		thresholds = DefaultCheckpointThresholds()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &CheckpointMonitor{
		workspaceDir:   workspaceDir,
		thresholds:     thresholds,
		eventPublisher: eventPublisher,
		auditLogger:    auditLogger,
		lastAlertTime:  make(map[string]time.Time),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins the monitoring loop
func (cm *CheckpointMonitor) Start() {
	cm.mu.Lock()
	if cm.running {
		cm.mu.Unlock()
		return
	}
	cm.running = true
	cm.mu.Unlock()

	cm.wg.Add(1)
	go cm.monitorLoop()
}

// Stop halts the monitoring loop
func (cm *CheckpointMonitor) Stop() {
	cm.mu.Lock()
	if !cm.running {
		cm.mu.Unlock()
		return
	}
	cm.running = false
	cm.cancel()
	cm.mu.Unlock()

	cm.wg.Wait()
}

// monitorLoop runs periodic checks
func (cm *CheckpointMonitor) monitorLoop() {
	defer cm.wg.Done()

	ticker := time.NewTicker(cm.thresholds.CheckInterval)
	defer ticker.Stop()

	// Run initial check
	cm.checkAndAlert()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.checkAndAlert()
		}
	}
}

// checkAndAlert performs a single check and generates alerts if needed
func (cm *CheckpointMonitor) checkAndAlert() {
	metrics, err := cm.collectMetrics()
	if err != nil {
		cm.logError("Failed to collect checkpoint metrics", err)
		return
	}

	alerts := cm.evaluateThresholds(metrics)

	for _, alert := range alerts {
		if cm.shouldAlert(alert.MetricName) {
			cm.publishAlert(alert)
			cm.recordAlert(alert.MetricName)
		}
	}
}

// collectMetrics gathers current checkpoint metrics
func (cm *CheckpointMonitor) collectMetrics() (*CheckpointMetrics, error) {
	beadsDir := filepath.Join(cm.workspaceDir, ".beads")
	checkpointDir := filepath.Join(beadsDir, "checkpoint")

	metrics := &CheckpointMetrics{
		Timestamp:    time.Now(),
		DatabasePath: filepath.Join(beadsDir, "beads.db"),
		CheckpointDir: checkpointDir,
	}

	// Get database size and count
	if info, err := os.Stat(metrics.DatabasePath); err == nil {
		metrics.DatabaseSize = info.Size()
	}

	// Parse checkpoint current.json for metadata
	currentJsonPath := filepath.Join(checkpointDir, "current.json")
	if data, err := os.ReadFile(currentJsonPath); err == nil {
		var checkpointData struct {
			IssueCount       int    `json:"issue_count"`
			TotalRecordCount int    `json:"total_record_count"`
			EventCount       int    `json:"event_count"`
			ActiveRoot       struct {
				Path string `json:"path"`
			} `json:"active_root"`
		}
		if err := json.Unmarshal(data, &checkpointData); err == nil {
			metrics.DatabaseIssueCount = checkpointData.IssueCount
			metrics.TotalRecordCount = checkpointData.TotalRecordCount
			metrics.EventCount = checkpointData.EventCount

			// Get active root size
			if checkpointData.ActiveRoot.Path != "" {
				activeRootPath := filepath.Join(checkpointDir, checkpointData.ActiveRoot.Path)
				if info, err := os.Stat(activeRootPath); err == nil {
					metrics.IsActiveRootSize = info.Size()
				}
			}
		}
	}

	// Get forensic.jsonl size and line count
	forensicPath := filepath.Join(checkpointDir, "forensic.jsonl")
	if info, err := os.Stat(forensicPath); err == nil {
		metrics.ForensicSize = info.Size()

		// Count lines efficiently
		if content, err := os.ReadFile(forensicPath); err == nil {
			lineCount := 0
			for _, b := range content {
				if b == '\n' {
					lineCount++
				}
			}
			metrics.ForensicLineCount = lineCount
		}
	}

	// Get objects directory stats
	objectsDir := filepath.Join(checkpointDir, "objects")
	if info, err := os.Stat(objectsDir); err == nil {
		metrics.ObjectsDirSize = info.Size()
	}

	// Count objects files
	if entries, err := os.ReadDir(objectsDir); err == nil {
		metrics.ObjectsFileCount = len(entries)
	}

	return metrics, nil
}

// evaluateThresholds checks metrics against thresholds and generates alerts
func (cm *CheckpointMonitor) evaluateThresholds(metrics *CheckpointMetrics) []CheckpointAlert {
	var alerts []CheckpointAlert
	now := time.Now()

	// Check database size
	if metrics.DatabaseSize > cm.thresholds.MaxDatabaseSize {
		alerts = append(alerts, CheckpointAlert{
			Timestamp:     now,
			AlertType:     "size_threshold_exceeded",
			MetricName:    "database_size",
			CurrentValue:  float64(metrics.DatabaseSize),
			Threshold:     float64(cm.thresholds.MaxDatabaseSize),
			Message:       fmt.Sprintf("Database size %.2f MB exceeds threshold %.2f MB",
				float64(metrics.DatabaseSize)/(1024*1024), float64(cm.thresholds.MaxDatabaseSize)/(1024*1024)),
			Severity: "critical",
		})
	}

	// Check forensic file size
	if metrics.ForensicSize > cm.thresholds.MaxForensicSize {
		alerts = append(alerts, CheckpointAlert{
			Timestamp:     now,
			AlertType:     "size_threshold_exceeded",
			MetricName:    "forensic_size",
			CurrentValue:  float64(metrics.ForensicSize),
			Threshold:     float64(cm.thresholds.MaxForensicSize),
			Message:       fmt.Sprintf("Forensic file size %.2f MB exceeds threshold %.2f MB",
				float64(metrics.ForensicSize)/(1024*1024), float64(cm.thresholds.MaxForensicSize)/(1024*1024)),
			Severity: "warning",
		})
	}

	// Check objects directory size
	if metrics.ObjectsDirSize > cm.thresholds.MaxObjectsDirSize {
		alerts = append(alerts, CheckpointAlert{
			Timestamp:     now,
			AlertType:     "size_threshold_exceeded",
			MetricName:    "objects_dir_size",
			CurrentValue:  float64(metrics.ObjectsDirSize),
			Threshold:     float64(cm.thresholds.MaxObjectsDirSize),
			Message:       fmt.Sprintf("Objects directory size %.2f MB exceeds threshold %.2f MB",
				float64(metrics.ObjectsDirSize)/(1024*1024), float64(cm.thresholds.MaxObjectsDirSize)/(1024*1024)),
			Severity: "warning",
		})
	}

	// Check issue count
	if metrics.DatabaseIssueCount > cm.thresholds.MaxIssueCount {
		alerts = append(alerts, CheckpointAlert{
			Timestamp:     now,
			AlertType:     "count_threshold_exceeded",
			MetricName:    "issue_count",
			CurrentValue:  float64(metrics.DatabaseIssueCount),
			Threshold:     float64(cm.thresholds.MaxIssueCount),
			Message:       fmt.Sprintf("Issue count %d exceeds threshold %d",
				metrics.DatabaseIssueCount, cm.thresholds.MaxIssueCount),
			Severity: "warning",
		})
	}

	// Check forensic line count
	if metrics.ForensicLineCount > cm.thresholds.MaxForensicLineCount {
		alerts = append(alerts, CheckpointAlert{
			Timestamp:     now,
			AlertType:     "count_threshold_exceeded",
			MetricName:    "forensic_line_count",
			CurrentValue:  float64(metrics.ForensicLineCount),
			Threshold:     float64(cm.thresholds.MaxForensicLineCount),
			Message:       fmt.Sprintf("Forensic line count %d exceeds threshold %d",
				metrics.ForensicLineCount, cm.thresholds.MaxForensicLineCount),
			Severity: "warning",
		})
	}

	// Check total record count
	if metrics.TotalRecordCount > cm.thresholds.MaxTotalRecordCount {
		alerts = append(alerts, CheckpointAlert{
			Timestamp:     now,
			AlertType:     "count_threshold_exceeded",
			MetricName:    "total_record_count",
			CurrentValue:  float64(metrics.TotalRecordCount),
			Threshold:     float64(cm.thresholds.MaxTotalRecordCount),
			Message:       fmt.Sprintf("Total record count %d exceeds threshold %d",
				metrics.TotalRecordCount, cm.thresholds.MaxTotalRecordCount),
			Severity: "critical",
		})
	}

	return alerts
}

// shouldAlert checks if enough time has passed since the last alert for this metric
func (cm *CheckpointMonitor) shouldAlert(metricName string) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	lastAlert, exists := cm.lastAlertTime[metricName]
	if !exists {
		return true
	}

	return time.Since(lastAlert) >= cm.thresholds.AlertCooldown
}

// recordAlert records the timestamp of an alert for a metric
func (cm *CheckpointMonitor) recordAlert(metricName string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.lastAlertTime[metricName] = time.Now()
}

// publishAlert publishes an alert event
func (cm *CheckpointMonitor) publishAlert(alert CheckpointAlert) {
	// Publish to event system
	if cm.eventPublisher != nil {
		event := NewEvent(EventError, "checkpoint-monitor", alert, alert.Message)
		cm.eventPublisher.Publish(event)
	}

	// Log to audit
	if cm.auditLogger != nil {
		details := map[string]interface{}{
			"alert_type":     alert.AlertType,
			"metric_name":    alert.MetricName,
			"current_value":  alert.CurrentValue,
			"threshold":      alert.Threshold,
			"severity":       alert.Severity,
		}
		_ = cm.auditLogger.LogError("checkpoint_alert", "checkpoint-monitor", "system",
			fmt.Errorf(alert.Message), details)
	}
}

// logError logs an error
func (cm *CheckpointMonitor) logError(message string, err error) {
	if cm.auditLogger != nil {
		_ = cm.auditLogger.LogError("checkpoint_monitor_error", "checkpoint", "system", err,
			map[string]interface{}{"message": message})
	}
}

// GetMetrics returns the current checkpoint metrics
func (cm *CheckpointMonitor) GetMetrics() (*CheckpointMetrics, error) {
	return cm.collectMetrics()
}

// GetThresholds returns the current thresholds
func (cm *CheckpointMonitor) GetThresholds() *CheckpointThresholds {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.thresholds
}

// SetThresholds updates the monitoring thresholds
func (cm *CheckpointMonitor) SetThresholds(thresholds *CheckpointThresholds) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.thresholds = thresholds
}

// GetLastAlertTimes returns the last alert times for all metrics
func (cm *CheckpointMonitor) GetLastAlertTimes() map[string]time.Time {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]time.Time, len(cm.lastAlertTime))
	for k, v := range cm.lastAlertTime {
		result[k] = v
	}
	return result
}
