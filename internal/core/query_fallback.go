package core

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// QueryStrategy represents a specific query approach
type QueryStrategy struct {
	Name        string   // Human-readable name
	Description string   // What this strategy tests
	Args        []string // Command-line arguments for bead list
	ExpectedUse string   // When this strategy is expected to return results
}

// QueryResult represents the result of executing a query strategy
type QueryResult struct {
	Strategy     QueryStrategy
	Beads        []Bead
	Count        int
	Error        error
	ExecutionTime time.Duration
	ExcludedBeads []Bead // Beads that this strategy excluded compared to workspace query
}

// FilterExclusionPattern represents a pattern of filter exclusion
type FilterExclusionPattern struct {
	FilterName   string    // Name of the filter causing exclusion
	ExcludedCount int      // Number of beads excluded by this filter
	SampleIDs    []string  // Sample bead IDs that were excluded
	Timestamp    time.Time // When this pattern was detected
}

// FallbackQueryExecutor executes multiple query strategies and analyzes results
type FallbackQueryExecutor struct {
	workspaceDir string
	binaryPath   string
	exclusionHistory []FilterExclusionPattern
}

// NewFallbackQueryExecutor creates a new fallback query executor
func NewFallbackQueryExecutor(workspaceDir, binaryPath string) *FallbackQueryExecutor {
	if binaryPath == "" {
		binaryPath = "bead"
	}
	return &FallbackQueryExecutor{
		workspaceDir: workspaceDir,
		binaryPath:   binaryPath,
		exclusionHistory: make([]FilterExclusionPattern, 0),
	}
}

// getQueryStrategies returns all available query strategies in order of preference
func (f *FallbackQueryExecutor) getQueryStrategies() []QueryStrategy {
	return []QueryStrategy{
		{
			Name:        "primary-ready-query",
			Description: "Primary query with all filters (--ready)",
			Args:        []string{"list", "--ready", "--json"},
			ExpectedUse: "Normal operation - returns unblocked open beads",
		},
		{
			Name:        "fallback-no-labels",
			Description: "Query without label filters (open status, minimal other filters)",
			Args:        []string{"list", "--status", "open", "--json"},
			ExpectedUse: "When labels are misapplied or missing",
		},
		{
			Name:        "fallback-no-deps",
			Description: "Query without dependency filters (open beads regardless of blocking state)",
			Args:        []string{"list", "--status", "open", "--json"},
			ExpectedUse: "When dependency graph is corrupted",
		},
		{
			Name:        "fallback-workspace-only",
			Description: "Minimal query - all beads in workspace",
			Args:        []string{"list", "--json"},
			ExpectedUse: "When multiple filters are corrupt or misconfigured",
		},
	}
}

// ExecuteFallbackQueries runs all query strategies and returns comprehensive analysis
func (f *FallbackQueryExecutor) ExecuteFallbackQueries() (*FallbackAnalysis, error) {
	strategies := f.getQueryStrategies()
	results := make([]QueryResult, 0, len(strategies))

	// Execute each strategy
	for _, strategy := range strategies {
		start := time.Now()
		beads, err := f.executeStrategy(strategy)
		executionTime := time.Since(start)

		result := QueryResult{
			Strategy:      strategy,
			Beads:         beads,
			Count:         len(beads),
			Error:         err,
			ExecutionTime: executionTime,
		}
		results = append(results, result)
	}

	// Analyze results to identify exclusion patterns
	analysis := f.analyzeResults(results)

	// Store exclusion patterns for historical tracking
	for _, pattern := range analysis.ExclusionPatterns {
		f.exclusionHistory = append(f.exclusionHistory, pattern)
	}

	return analysis, nil
}

// executeStrategy executes a single query strategy
func (f *FallbackQueryExecutor) executeStrategy(strategy QueryStrategy) ([]Bead, error) {
	args := strategy.Args
	cmd := exec.Command(f.binaryPath, args...)
	cmd.Dir = f.workspaceDir

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("strategy %s failed: %w", strategy.Name, err)
	}

	var beads []Bead
	if err := json.Unmarshal(output, &beads); err != nil {
		return nil, fmt.Errorf("failed to parse output from strategy %s: %w", strategy.Name, err)
	}

	return beads, nil
}

// analyzeResults analyzes query results to identify exclusion patterns
func (f *FallbackQueryExecutor) analyzeResults(results []QueryResult) *FallbackAnalysis {
	analysis := &FallbackAnalysis{
		Results:          results,
		ExclusionPatterns: make([]FilterExclusionPattern, 0),
		RecommendedActions: make([]string, 0),
	}

	// Find the workspace-only result (most permissive)
	var workspaceResult *QueryResult
	for i := range results {
		if results[i].Strategy.Name == "fallback-workspace-only" {
			workspaceResult = &results[i]
			break
		}
	}

	if workspaceResult == nil {
		analysis.Error = "workspace-only query result not found"
		return analysis
	}

	// Compare each result against workspace result
	workspaceBeadsMap := make(map[string]Bead)
	for _, bead := range workspaceResult.Beads {
		workspaceBeadsMap[bead.ID] = bead
	}

	for i := range results {
		if results[i].Strategy.Name == "fallback-workspace-only" {
			continue
		}

		// Find beads in workspace but not in this result
		excludedBeads := make([]Bead, 0)
		excludedIDs := make([]string, 0)

		for _, bead := range workspaceResult.Beads {
			found := false
			for _, resultBead := range results[i].Beads {
				if resultBead.ID == bead.ID {
					found = true
					break
				}
			}
			if !found {
				excludedBeads = append(excludedBeads, bead)
				excludedIDs = append(excludedIDs, bead.ID)
			}
		}

		results[i].ExcludedBeads = excludedBeads

		// If significant exclusion, create pattern
		if len(excludedBeads) > 0 {
			pattern := FilterExclusionPattern{
				FilterName:    f.inferFilterFromStrategy(results[i].Strategy.Name),
				ExcludedCount: len(excludedBeads),
				SampleIDs:     getSampleIDs(excludedIDs, 5),
				Timestamp:     time.Now(),
			}
			analysis.ExclusionPatterns = append(analysis.ExclusionPatterns, pattern)
		}
	}

	// Generate recommended actions based on patterns
	analysis.RecommendedActions = f.generateRecommendedActions(analysis)

	return analysis
}

// inferFilterFromStrategy infers which filter is being tested based on strategy name
func (f *FallbackQueryExecutor) inferFilterFromStrategy(strategyName string) string {
	switch strategyName {
	case "primary-ready-query":
		return "ready-filter (combines status, dependency, and label filters)"
	case "fallback-no-labels":
		return "label-filters"
	case "fallback-no-deps":
		return "dependency-filters"
	default:
		return "unknown-filter"
	}
}

// generateRecommendedActions generates recommended actions based on exclusion patterns
func (f *FallbackQueryExecutor) generateRecommendedActions(analysis *FallbackAnalysis) []string {
	actions := make([]string, 0)

	for _, pattern := range analysis.ExclusionPatterns {
		switch pattern.FilterName {
		case "ready-filter (combines status, dependency, and label filters)":
			if pattern.ExcludedCount > 0 {
				actions = append(actions, fmt.Sprintf(
					"Ready filter excluded %d beads - check dependency graph and assignees",
					pattern.ExcludedCount))
			}
		case "label-filters":
			if pattern.ExcludedCount > 0 {
				actions = append(actions, fmt.Sprintf(
					"Label filters may be misapplied - %d beads excluded by labels",
					pattern.ExcludedCount))
			}
		case "dependency-filters":
			if pattern.ExcludedCount > 0 {
				actions = append(actions, fmt.Sprintf(
					"Dependency graph may be corrupted - %d beads blocked by invalid dependencies",
					pattern.ExcludedCount))
			}
		}
	}

	// If no clear patterns but primary query failed
	if len(analysis.ExclusionPatterns) == 0 {
		primaryResult := getPrimaryResult(analysis.Results)
		if primaryResult != nil && primaryResult.Count == 0 {
			actions = append(actions, "All query strategies returned zero results - workspace may be empty or database may be inaccessible")
		}
	}

	return actions
}

// getSampleIDs returns a sample of bead IDs
func getSampleIDs(ids []string, maxSample int) []string {
	if len(ids) <= maxSample {
		return ids
	}
	return ids[:maxSample]
}

// getPrimaryResult returns the primary query result
func getPrimaryResult(results []QueryResult) *QueryResult {
	for i := range results {
		if results[i].Strategy.Name == "primary-ready-query" {
			return &results[i]
		}
	}
	return nil
}

// FallbackAnalysis represents the comprehensive analysis of fallback queries
type FallbackAnalysis struct {
	Results             []QueryResult         `json:"results"`
	ExclusionPatterns   []FilterExclusionPattern `json:"exclusion_patterns"`
	RecommendedActions  []string              `json:"recommended_actions"`
	Error               string                `json:"error,omitempty"`
	PrimaryQueryEmpty   bool                  `json:"primary_query_empty"`
	AnyQuerySucceeded   bool                  `json:"any_query_succeeded"`
	TotalBeadsInWorkspace int                  `json:"total_beads_in_workspace"`
}

// GetExclusionSummary returns a human-readable summary of exclusion patterns
func (a *FallbackAnalysis) GetExclusionSummary() string {
	var sb strings.Builder

	sb.WriteString("=== Query Fallback Analysis ===\n\n")

	// Primary query status
	primaryResult := getPrimaryResult(a.Results)
	if primaryResult != nil {
		if primaryResult.Count == 0 {
			sb.WriteString("⚠️  PRIMARY QUERY RETURNED ZERO CANDIDATES\n")
		} else {
			sb.WriteString(fmt.Sprintf("✓ Primary query returned %d candidates\n", primaryResult.Count))
		}
	}
	sb.WriteString("\n")

	// Query strategy comparison
	sb.WriteString("Query Strategy Comparison:\n")
	for _, result := range a.Results {
		status := "✓"
		if result.Error != nil {
			status = "✗"
		}
		sb.WriteString(fmt.Sprintf("  %s %s: %d beads (%s)\n",
			status, result.Strategy.Name, result.Count,
			result.ExecutionTime.String()))
		if len(result.ExcludedBeads) > 0 {
			sb.WriteString(fmt.Sprintf("      (excluded %d beads compared to workspace)\n",
				len(result.ExcludedBeads)))
		}
	}
	sb.WriteString("\n")

	// Exclusion patterns
	if len(a.ExclusionPatterns) > 0 {
		sb.WriteString("Detected Exclusion Patterns:\n")
		for _, pattern := range a.ExclusionPatterns {
			sb.WriteString(fmt.Sprintf("  • %s: %d beads excluded\n",
				pattern.FilterName, pattern.ExcludedCount))
			if len(pattern.SampleIDs) > 0 {
				sb.WriteString(fmt.Sprintf("    Sample IDs: %s\n",
					strings.Join(pattern.SampleIDs, ", ")))
			}
		}
		sb.WriteString("\n")
	}

	// Recommended actions
	if len(a.RecommendedActions) > 0 {
		sb.WriteString("Recommended Actions:\n")
		for i, action := range a.RecommendedActions {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, action))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// DetectCorruptionPatterns analyzes exclusion patterns to detect specific corruption types
func (f *FallbackQueryExecutor) DetectCorruptionPatterns(analysis *FallbackAnalysis) []CorruptionDetection {
	detections := make([]CorruptionDetection, 0)

	for _, pattern := range analysis.ExclusionPatterns {
		detection := CorruptionDetection{
			FilterName: pattern.FilterName,
			Timestamp:  pattern.Timestamp,
			BeadIDs:    pattern.SampleIDs,
		}

		switch pattern.FilterName {
		case "dependency-filters":
			detection.CorruptionType = "dependency-graph-corruption"
			detection.Severity = "high"
			detection.AutoHealable = true
			detection.HealMethod = "validate-dependencies"
			detection.Description = fmt.Sprintf("Dependency filters excluded %d beads, indicating potential corruption in dependency graph",
				pattern.ExcludedCount)

		case "label-filters":
			detection.CorruptionType = "label-misapplication"
			detection.Severity = "medium"
			detection.AutoHealable = true
			detection.HealMethod = "validate-labels"
			detection.Description = fmt.Sprintf("Label filters excluded %d beads, indicating labels may be missing or incorrect",
				pattern.ExcludedCount)

		case "ready-filter (combines status, dependency, and label filters)":
			detection.CorruptionType = "ready-filter-combined-corruption"
			detection.Severity = "medium"
			detection.AutoHealable = false
			detection.Description = fmt.Sprintf("Ready filter excluded %d beads - may indicate issues with status, dependencies, or labels",
				pattern.ExcludedCount)
		}

		detections = append(detections, detection)
	}

	return detections
}

// CorruptionDetection represents a detected corruption pattern
type CorruptionDetection struct {
	CorruptionType string   `json:"corruption_type"`
	Severity       string   `json:"severity"`
	FilterName     string   `json:"filter_name"`
	Description    string   `json:"description"`
	BeadIDs        []string `json:"bead_ids,omitempty"`
	AutoHealable   bool     `json:"auto_healable"`
	HealMethod     string   `json:"heal_method,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}
