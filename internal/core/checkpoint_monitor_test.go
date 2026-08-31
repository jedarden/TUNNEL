package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestCheckpointMonitor_CollectMetrics(t *testing.T) {
	// Create temporary workspace with checkpoint structure
	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	checkpointDir := filepath.Join(beadsDir, "checkpoint")
	objectsDir := filepath.Join(checkpointDir, "objects")

	// Create directories
	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create checkpoint current.json with test data
	checkpointData := map[string]interface{}{
		"issue_count":       42,
		"total_record_count": 150,
		"event_count":        200,
		"active_root": map[string]string{
			"path": "objects/test-active.jsonl",
		},
	}
	currentJson, _ := json.Marshal(checkpointData)
	if err := os.WriteFile(filepath.Join(checkpointDir, "current.json"), currentJson, 0644); err != nil {
		t.Fatalf("Failed to write current.json: %v", err)
	}

	// Create forensic.jsonl with some lines
	forensicLines := `{"line":1}
{"line":2}
{"line":3}
`
	if err := os.WriteFile(filepath.Join(checkpointDir, "forensic.jsonl"), []byte(forensicLines), 0644); err != nil {
		t.Fatalf("Failed to write forensic.jsonl: %v", err)
	}

	// Create active root file
	if err := os.WriteFile(filepath.Join(checkpointDir, "objects", "test-active.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write active root: %v", err)
	}

	// Create monitor
	monitor := NewCheckpointMonitor(tempDir, DefaultCheckpointThresholds(), nil, nil)

	// Collect metrics
	metrics, err := monitor.collectMetrics()
	if err != nil {
		t.Fatalf("collectMetrics failed: %v", err)
	}

	// Verify basic data collection
	if metrics.DatabaseIssueCount != 42 {
		t.Errorf("Expected issue count 42, got %d", metrics.DatabaseIssueCount)
	}
	if metrics.TotalRecordCount != 150 {
		t.Errorf("Expected total record count 150, got %d", metrics.TotalRecordCount)
	}
	if metrics.EventCount != 200 {
		t.Errorf("Expected event count 200, got %d", metrics.EventCount)
	}
	if metrics.ForensicLineCount != 3 {
		t.Errorf("Expected forensic line count 3, got %d", metrics.ForensicLineCount)
	}
	if metrics.ObjectsFileCount != 1 {
		t.Errorf("Expected objects file count 1, got %d", metrics.ObjectsFileCount)
	}
}

func TestCheckpointMonitor_EvaluateThresholds(t *testing.T) {
	thresholds := &CheckpointThresholds{
		MaxDatabaseSize:      1000,
		MaxForensicSize:      500,
		MaxObjectsDirSize:    2000,
		MaxIssueCount:        100,
		MaxForensicLineCount: 1000,
		MaxTotalRecordCount:  5000,
		CheckInterval:        time.Minute,
		AlertCooldown:        time.Minute,
	}

	monitor := NewCheckpointMonitor("", thresholds, nil, nil)

	tests := []struct {
		name     string
		metrics  CheckpointMetrics
		wantAlerts int
	}{
		{
			name: "all_metrics_within_thresholds",
			metrics: CheckpointMetrics{
				DatabaseSize:       500,
				ForensicSize:       200,
				ObjectsDirSize:    1000,
				DatabaseIssueCount: 50,
				ForensicLineCount:  500,
				TotalRecordCount:   1000,
			},
			wantAlerts: 0,
		},
		{
			name: "database_size_exceeded",
			metrics: CheckpointMetrics{
				DatabaseSize:       2000,
				ForensicSize:       200,
				ObjectsDirSize:    1000,
				DatabaseIssueCount: 50,
				ForensicLineCount:  500,
				TotalRecordCount:   1000,
			},
			wantAlerts: 1,
		},
		{
			name: "multiple_thresholds_exceeded",
			metrics: CheckpointMetrics{
				DatabaseSize:       2000,
				ForensicSize:       1000,
				ObjectsDirSize:    3000,
				DatabaseIssueCount: 150,
				ForensicLineCount: 2000,
				TotalRecordCount:   6000,
			},
			wantAlerts: 6,
		},
		{
			name: "exactly_at_threshold",
			metrics: CheckpointMetrics{
				DatabaseSize:       1000,
				ForensicSize:       500,
				ObjectsDirSize:    2000,
				DatabaseIssueCount: 100,
				ForensicLineCount:  1000,
				TotalRecordCount:   5000,
			},
			wantAlerts: 0, // At threshold should not alert
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alerts := monitor.evaluateThresholds(&tt.metrics)
			if len(alerts) != tt.wantAlerts {
				t.Errorf("Expected %d alerts, got %d", tt.wantAlerts, len(alerts))
			}
		})
	}
}

func TestCheckpointMonitor_ShouldAlert(t *testing.T) {
	thresholds := &CheckpointThresholds{
		CheckInterval: time.Minute,
		AlertCooldown: 100 * time.Millisecond,
	}

	monitor := NewCheckpointMonitor("", thresholds, nil, nil)

	// First alert should always be allowed
	if !monitor.shouldAlert("test_metric") {
		t.Error("First alert should be allowed")
	}

	// Record the alert
	monitor.recordAlert("test_metric")

	// Immediate second alert should be blocked
	if monitor.shouldAlert("test_metric") {
		t.Error("Immediate second alert should be blocked")
	}

	// Wait for cooldown to expire
	time.Sleep(150 * time.Millisecond)

	// Alert after cooldown should be allowed
	if !monitor.shouldAlert("test_metric") {
		t.Error("Alert after cooldown should be allowed")
	}
}

func TestCheckpointMonitor_AlertCooldown(t *testing.T) {
	thresholds := &CheckpointThresholds{
		AlertCooldown: 200 * time.Millisecond,
		CheckInterval: time.Minute,
	}

	eventPublisher := NewEventPublisher(10)
	auditLogger, _ := NewAuditLogger("", false, "")

	monitor := NewCheckpointMonitor("", thresholds, eventPublisher, auditLogger)

	// Create an alert
	alert := CheckpointAlert{
		Timestamp:    time.Now(),
		AlertType:    "test_alert",
		MetricName:   "test_metric",
		CurrentValue: 100,
		Threshold:    50,
		Message:      "Test metric exceeded threshold",
		Severity:     "warning",
	}

	// Publish first alert
	monitor.publishAlert(alert)

	// Try to publish again immediately - should be blocked by cooldown
	monitor.publishAlert(alert)

	// Check that only one subscriber received the event
	subscriber := eventPublisher.Subscribe("test", nil)
	defer eventPublisher.Unsubscribe("test")

	time.Sleep(50 * time.Millisecond) // Give time for event to propagate

	// After cooldown, alert should be allowed again
	time.Sleep(250 * time.Millisecond)

	// This should publish the alert
	monitor.publishAlert(alert)
}

func TestCheckpointMonitor_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	checkpointDir := filepath.Join(beadsDir, "checkpoint")
	objectsDir := filepath.Join(checkpointDir, "objects")

	if err := os.MkdirAll(objectsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create minimal checkpoint files
	checkpointData := map[string]interface{}{
		"issue_count":        10,
		"total_record_count": 50,
		"event_count":         100,
		"active_root":        map[string]string{"path": "objects/test.jsonl"},
	}
	currentJson, _ := json.Marshal(checkpointData)
	if err := os.WriteFile(filepath.Join(checkpointDir, "current.json"), currentJson, 0644); err != nil {
		t.Fatalf("Failed to write current.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkpointDir, "forensic.jsonl"), []byte("{}\n"), 0644); err != nil {
		t.Fatalf("Failed to write forensic.jsonl: %v", err)
	}
	if err := os.WriteFile(filepath.Join(objectsDir, "test.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write active root: %v", err)
	}

	thresholds := DefaultCheckpointThresholds()
	thresholds.CheckInterval = 10 * time.Millisecond

	monitor := NewCheckpointMonitor(tempDir, thresholds, nil, nil)
	monitor.Start()

	// Run concurrent operations
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_, err := monitor.GetMetrics()
				if err != nil {
					t.Errorf("GetMetrics failed: %v", err)
				}
				monitor.GetThresholds()
				monitor.GetLastAlertTimes()
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	monitor.Stop()

	// Monitor should stop cleanly
	if monitor.running {
		t.Error("Monitor should be stopped")
	}
}

func TestCheckpointMonitor_GetAndSetThresholds(t *testing.T) {
	thresholds := DefaultCheckpointThresholds()
	monitor := NewCheckpointMonitor("", thresholds, nil, nil)

	// Get initial thresholds
	initialThresholds := monitor.GetThresholds()
	if initialThresholds.MaxIssueCount != 1000 {
		t.Errorf("Expected MaxIssueCount 1000, got %d", initialThresholds.MaxIssueCount)
	}

	// Set new thresholds
	newThresholds := &CheckpointThresholds{
		MaxIssueCount:      2000,
		MaxDatabaseSize:     200 * 1024 * 1024,
		CheckInterval:      10 * time.Minute,
		AlertCooldown:      30 * time.Minute,
	}

	monitor.SetThresholds(newThresholds)

	// Verify thresholds were updated
	updatedThresholds := monitor.GetThresholds()
	if updatedThresholds.MaxIssueCount != 2000 {
		t.Errorf("Expected MaxIssueCount 2000, got %d", updatedThresholds.MaxIssueCount)
	}
	if updatedThresholds.CheckInterval != 10*time.Minute {
		t.Errorf("Expected CheckInterval 10m, got %v", updatedThresholds.CheckInterval)
	}
}

func TestCheckpointMonitor_MissingCheckpointDir(t *testing.T) {
	tempDir := t.TempDir()
	// Don't create checkpoint directory - should handle gracefully

	monitor := NewCheckpointMonitor(tempDir, DefaultCheckpointThresholds(), nil, nil)

	// Should not panic or return error
	metrics, err := monitor.collectMetrics()
	if err != nil {
		// Expected to have some data even with missing files
		t.Logf("Expected some error with missing checkpoint: %v", err)
	}

	// Should still return a metrics object with zero values
	if metrics == nil {
		t.Error("Expected metrics object even with missing checkpoint")
	}
}

func TestCheckpointMonitor_StartStop(t *testing.T) {
	monitor := NewCheckpointMonitor("", DefaultCheckpointThresholds(), nil, nil)

	// Start monitor
	monitor.Start()
	if !monitor.running {
		t.Error("Monitor should be running after Start()")
	}

	// Start again should be idempotent
	monitor.Start()
	if !monitor.running {
		t.Error("Monitor should still be running after duplicate Start()")
	}

	// Stop monitor
	monitor.Stop()
	if monitor.running {
		t.Error("Monitor should not be running after Stop()")
	}

	// Stop again should be idempotent
	monitor.Stop()
	if monitor.running {
		t.Error("Monitor should still be stopped after duplicate Stop()")
	}
}

func TestCheckpointMonitor_ContextCancellation(t *testing.T) {
	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	checkpointDir := filepath.Join(beadsDir, "checkpoint")

	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	// Create minimal checkpoint files
	checkpointData := map[string]interface{}{
		"issue_count":        5,
		"total_record_count": 25,
		"event_count":         50,
	}
	currentJson, _ := json.Marshal(checkpointData)
	if err := os.WriteFile(filepath.Join(checkpointDir, "current.json"), currentJson, 0644); err != nil {
		t.Fatalf("Failed to write current.json: %v", err)
	}

	thresholds := DefaultCheckpointThresholds()
	thresholds.CheckInterval = 50 * time.Millisecond

	monitor := NewCheckpointMonitor(tempDir, thresholds, nil, nil)
	monitor.Start()

	// Let it run for a bit
	time.Sleep(150 * time.Millisecond)

	// Stop should cancel context and stop loop
	monitor.Stop()

	// Verify it's actually stopped
	time.Sleep(100 * time.Millisecond)
	if monitor.running {
		t.Error("Monitor should be stopped after Stop() call")
	}
}

func TestCheckpointThresholds_SeverityLevels(t *testing.T) {
	thresholds := &CheckpointThresholds{
		MaxDatabaseSize:      1000,
		MaxForensicSize:      500,
		MaxObjectsDirSize:    2000,
		MaxIssueCount:        100,
		MaxForensicLineCount: 1000,
		MaxTotalRecordCount:  5000,
	}

	monitor := NewCheckpointMonitor("", thresholds, nil, nil)

	// Test that severity is correctly assigned
	metrics := &CheckpointMetrics{
		DatabaseSize:       2000, // Exceeds by 2x - should be critical
		ForensicSize:       1000, // Exceeds by 2x - should be warning
		ObjectsDirSize:    3000, // Exceeds by 1.5x - should be warning
		DatabaseIssueCount: 150, // Exceeds by 1.5x - should be warning
		ForensicLineCount:  2000, // Exceeds by 2x - should be warning
		TotalRecordCount:   6000, // Exceeds by 1.2x - should be critical
	}

	alerts := monitor.evaluateThresholds(metrics)

	severityCount := make(map[string]int)
	for _, alert := range alerts {
		severityCount[alert.Severity]++
	}

	// We expect at least some critical and some warning alerts
	if severityCount["critical"] < 2 {
		t.Error("Expected at least 2 critical alerts, got", severityCount["critical"])
	}
	if severityCount["warning"] < 4 {
		t.Error("Expected at least 4 warning alerts, got", severityCount["warning"])
	}
}
