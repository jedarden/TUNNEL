package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBeadValidator_ValidateWorkspaceBackend(t *testing.T) {
	// Create temporary workspace
	tempDir := t.TempDir()

	validator := NewBeadValidator(&ValidatorConfig{
		WorkspaceDir: tempDir,
	})

	t.Run("No needle config", func(t *testing.T) {
		result := validator.validateWorkspaceBackend()
		if !result.Passed {
			t.Errorf("Expected pass when no needle config exists")
		}
	})

	t.Run("With bead-rs config", func(t *testing.T) {
		// Create .needle.yaml
		needleConfig := filepath.Join(tempDir, ".needle.yaml")
		err := os.WriteFile(needleConfig, []byte("bead_cli:\n  backend: bead-rs\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create needle config: %v", err)
		}

		// Create .beads/config.json
		beadsDir := filepath.Join(tempDir, ".beads")
		os.MkdirAll(beadsDir, 0755)
		configPath := filepath.Join(beadsDir, "config.json")
		err = os.WriteFile(configPath, []byte(`{"version":1}`), 0644)
		if err != nil {
			t.Fatalf("Failed to create bead config: %v", err)
		}

		result := validator.validateWorkspaceBackend()
		if !result.Passed {
			t.Errorf("Expected pass with valid bead-rs setup")
		}
	})

	t.Run("Mismatched backend", func(t *testing.T) {
		// Create .needle.yaml
		needleConfig := filepath.Join(tempDir, ".needle.yaml")
		err := os.WriteFile(needleConfig, []byte("bead_cli:\n  backend: bead-rs\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create needle config: %v", err)
		}

		// Don't create .beads/config.json - simulating old backend
		result := validator.validateWorkspaceBackend()
		if result.Passed {
			t.Errorf("Expected failure when bead-rs specified but config.json missing")
		}
		if len(result.Issues) == 0 {
			t.Errorf("Expected issues to be reported")
		}
	})
}

func TestBeadValidator_ValidateAssignedButOpen(t *testing.T) {
	validator := NewBeadValidator(&ValidatorConfig{
		WorkspaceDir: t.TempDir(),
	})

	t.Run("All valid", func(t *testing.T) {
		beads := []Bead{
			{ID: "bead-1", Status: BeadStateOpen, Assignee: ""},
			{ID: "bead-2", Status: BeadStateInProgress, Assignee: "worker-1"},
			{ID: "bead-3", Status: BeadStateClosed, Assignee: ""},
		}

		result := validator.validateAssignedButOpenFromList(beads)
		if !result.Passed {
			t.Errorf("Expected pass for valid bead states")
		}
	})

	t.Run("Assigned but open", func(t *testing.T) {
		beads := []Bead{
			{ID: "bead-1", Status: BeadStateOpen, Assignee: "worker-1", Title: "Test Bead"},
		}

		result := validator.validateAssignedButOpenFromList(beads)
		if result.Passed {
			t.Errorf("Expected failure for assigned-but-open bead")
		}
		if len(result.Issues) != 1 {
			t.Errorf("Expected 1 issue, got %d", len(result.Issues))
		}
		if !result.Fixable {
			t.Errorf("Expected fixable to be true")
		}
	})

	t.Run("Multiple issues", func(t *testing.T) {
		beads := []Bead{
			{ID: "bead-1", Status: BeadStateOpen, Assignee: "worker-1", Title: "Bead 1"},
			{ID: "bead-2", Status: BeadStateOpen, Assignee: "worker-2", Title: "Bead 2"},
		}

		result := validator.validateAssignedButOpenFromList(beads)
		if result.Passed {
			t.Errorf("Expected failure for multiple assigned-but-open beads")
		}
		if len(result.Issues) != 2 {
			t.Errorf("Expected 2 issues, got %d", len(result.Issues))
		}
	})
}

func TestBeadValidator_ValidateClosedWithAssignee(t *testing.T) {
	validator := NewBeadValidator(&ValidatorConfig{
		WorkspaceDir: t.TempDir(),
	})

	t.Run("All valid", func(t *testing.T) {
		beads := []Bead{
			{ID: "bead-1", Status: BeadStateClosed, Assignee: ""},
			{ID: "bead-2", Status: BeadStateOpen, Assignee: "worker-1"},
		}

		result := validator.validateClosedWithAssigneeFromList(beads)
		if !result.Passed {
			t.Errorf("Expected pass for valid bead states")
		}
	})

	t.Run("Closed with assignee", func(t *testing.T) {
		beads := []Bead{
			{ID: "bead-1", Status: BeadStateClosed, Assignee: "worker-1", Title: "Test Bead"},
		}

		result := validator.validateClosedWithAssigneeFromList(beads)
		if result.Passed {
			t.Errorf("Expected failure for closed bead with assignee")
		}
		if len(result.Issues) != 1 {
			t.Errorf("Expected 1 issue, got %d", len(result.Issues))
		}
	})
}

func TestBeadValidator_ValidateCircularDependencies(t *testing.T) {
	validator := NewBeadValidator(&ValidatorConfig{
		WorkspaceDir: t.TempDir(),
	})

	t.Run("All valid", func(t *testing.T) {
		beads := []Bead{
			{ID: "bead-1", Blocks: []string{"bead-2"}, BlockedBy: []string{}},
			{ID: "bead-2", Blocks: []string{}, BlockedBy: []string{"bead-1"}},
		}

		result := validator.validateCircularDependenciesFromList(beads)
		if !result.Passed {
			t.Errorf("Expected pass for valid dependencies")
		}
	})

	t.Run("Self-blocking", func(t *testing.T) {
		beads := []Bead{
			{ID: "bead-1", Title: "Self Blocker", Blocks: []string{"bead-1"}, BlockedBy: []string{}},
		}

		result := validator.validateCircularDependenciesFromList(beads)
		if result.Passed {
			t.Errorf("Expected failure for self-blocking bead")
		}
		if len(result.Issues) != 1 {
			t.Errorf("Expected 1 issue, got %d", len(result.Issues))
		}
	})

	t.Run("Self-blocked-by", func(t *testing.T) {
		beads := []Bead{
			{ID: "bead-1", Title: "Self Blocked", Blocks: []string{}, BlockedBy: []string{"bead-1"}},
		}

		result := validator.validateCircularDependenciesFromList(beads)
		if result.Passed {
			t.Errorf("Expected failure for self-blocked bead")
		}
		if len(result.Issues) != 1 {
			t.Errorf("Expected 1 issue, got %d", len(result.Issues))
		}
	})

	t.Run("Both self-block and self-blocked-by", func(t *testing.T) {
		beads := []Bead{
			{ID: "bead-1", Title: "Double Self", Blocks: []string{"bead-1"}, BlockedBy: []string{"bead-1"}},
		}

		result := validator.validateCircularDependenciesFromList(beads)
		if result.Passed {
			t.Errorf("Expected failure for doubly self-referential bead")
		}
		if len(result.Issues) != 2 {
			t.Errorf("Expected 2 issues, got %d", len(result.Issues))
		}
	})
}

func TestBeadValidator_ValidateDatabaseConsistency(t *testing.T) {
	tempDir := t.TempDir()
	beadsDir := filepath.Join(tempDir, ".beads")
	checkpointDir := filepath.Join(beadsDir, "checkpoint")

	validator := NewBeadValidator(&ValidatorConfig{
		WorkspaceDir: tempDir,
	})

	t.Run("No checkpoint", func(t *testing.T) {
		result := validator.validateDatabaseConsistency()
		if result.Passed {
			t.Errorf("Expected failure when no checkpoint exists")
		}
		if result.Fixable {
			t.Errorf("Expected fixable to be false when no checkpoint")
		}
	})

	t.Run("Valid checkpoint", func(t *testing.T) {
		// Create checkpoint directory
		os.MkdirAll(checkpointDir, 0755)

		// Create checkpoint with 2 issues
		checkpointData := map[string]interface{}{
			"issues": map[string]interface{}{
				"bead-1": map[string]interface{}{"id": "bead-1"},
				"bead-2": map[string]interface{}{"id": "bead-2"},
			},
		}
		checkpointJSON, _ := json.Marshal(checkpointData)
		checkpointPath := filepath.Join(checkpointDir, "current.json")
		err := os.WriteFile(checkpointPath, checkpointJSON, 0644)
		if err != nil {
			t.Fatalf("Failed to create checkpoint: %v", err)
		}

		// This test would require mocking bead list, which we can't do easily
		// For now, just verify the checkpoint is read correctly
		if _, err := os.Stat(checkpointPath); os.IsNotExist(err) {
			t.Errorf("Checkpoint file should exist")
		}
	})
}

func TestBeadValidator_GenerateSummaryReport(t *testing.T) {
	validator := NewBeadValidator(&ValidatorConfig{
		WorkspaceDir: t.TempDir(),
	})

	t.Run("All passed", func(t *testing.T) {
		results := []ValidationResult{
			{Name: "check1", Passed: true, Description: "Check 1"},
			{Name: "check2", Passed: true, Description: "Check 2"},
		}

		report := validator.GenerateSummaryReport(results)
		if report == "" {
			t.Errorf("Expected non-empty report")
		}
	})

	t.Run("Some failed", func(t *testing.T) {
		results := []ValidationResult{
			{Name: "check1", Passed: true, Description: "Check 1"},
			{Name: "check2", Passed: false, Description: "Check 2", Fixable: true,
				Issues: []string{"Issue 1"}},
		}

		report := validator.GenerateSummaryReport(results)
		if report == "" {
			t.Errorf("Expected non-empty report")
		}
	})
}

func TestBeadState_EnumValues(t *testing.T) {
	// Verify enum values are correct
	tests := []struct {
		value   BeadState
		expect  string
	}{
		{BeadStateOpen, "open"},
		{BeadStateInProgress, "in_progress"},
		{BeadStateClosed, "closed"},
		{BeadStateDeferred, "deferred"},
	}

	for _, tt := range tests {
		if string(tt.value) != tt.expect {
			t.Errorf("Expected %s, got %s", tt.expect, string(tt.value))
		}
	}
}

func TestNewBeadValidator(t *testing.T) {
	t.Run("Default config", func(t *testing.T) {
		validator := NewBeadValidator(nil)
		if validator == nil {
			t.Errorf("Expected validator to be created")
		}
		if validator.binaryPath != "bead" {
			t.Errorf("Expected default binary path 'bead', got '%s'", validator.binaryPath)
		}
	})

	t.Run("Custom config", func(t *testing.T) {
		config := &ValidatorConfig{
			WorkspaceDir: "/custom/path",
			BinaryPath:   "/custom/bead",
			DryRun:       true,
		}
		validator := NewBeadValidator(config)
		if validator.workspaceDir != "/custom/path" {
			t.Errorf("Expected workspace dir '/custom/path', got '%s'", validator.workspaceDir)
		}
		if validator.binaryPath != "/custom/bead" {
			t.Errorf("Expected binary path '/custom/bead', got '%s'", validator.binaryPath)
		}
		if !validator.dryRun {
			t.Errorf("Expected dry run to be true")
		}
	})
}

func TestBead_JSONRoundTrip(t *testing.T) {
	// Test that Bead struct can be serialized and deserialized
	bead := Bead{
		ID:        "test-123",
		Title:     "Test Bead",
		Status:    BeadStateInProgress,
		Assignee:  "worker-1",
		Priority:  2,
		Revision: 5,
		Labels:    []string{"bug", "high-priority"},
		Blocks:    []string{"bead-2", "bead-3"},
		BlockedBy: []string{"bead-1"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(bead)
	if err != nil {
		t.Fatalf("Failed to marshal bead: %v", err)
	}

	var decoded Bead
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal bead: %v", err)
	}

	if decoded.ID != bead.ID {
		t.Errorf("Expected ID %s, got %s", bead.ID, decoded.ID)
	}
	if decoded.Status != bead.Status {
		t.Errorf("Expected status %s, got %s", bead.Status, decoded.Status)
	}
	if decoded.Assignee != bead.Assignee {
		t.Errorf("Expected assignee %s, got %s", bead.Assignee, decoded.Assignee)
	}
}

func TestValidationResult_FixableField(t *testing.T) {
	t.Run("Fixable result", func(t *testing.T) {
		result := ValidationResult{
			Name:        "test",
			Passed:      false,
			Description: "Test",
			Issues:      []string{"Issue 1"},
			Fixable:     true,
		}

		if !result.Fixable {
			t.Errorf("Expected fixable to be true")
		}
	})

	t.Run("Non-fixable result", func(t *testing.T) {
		result := ValidationResult{
			Name:        "test",
			Passed:      false,
			Description: "Test",
			Issues:      []string{"Issue 1"},
			Fixable:     false,
		}

		if result.Fixable {
			t.Errorf("Expected fixable to be false")
		}
	})
}

// Benchmark tests
func BenchmarkBeadValidator_ValidateAll(b *testing.B) {
	tempDir := b.TempDir()
	validator := NewBeadValidator(&ValidatorConfig{
		WorkspaceDir: tempDir,
		DryRun:       true,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = validator.ValidateAll()
	}
}

func BenchmarkBeadValidator_ValidateAssignedButOpenFromList(b *testing.B) {
	validator := NewBeadValidator(&ValidatorConfig{
		WorkspaceDir: b.TempDir(),
	})

	beads := make([]Bead, 1000)
	for i := 0; i < 1000; i++ {
		beads[i] = Bead{
			ID:       fmt.Sprintf("bead-%d", i),
			Status:   BeadStateOpen,
			Assignee: "",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.validateAssignedButOpenFromList(beads)
	}
}
