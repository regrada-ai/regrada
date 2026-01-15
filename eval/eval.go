package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/matias/regrada/trace"
	"gopkg.in/yaml.v3"
)

// TestSuite represents a collection of tests.
type TestSuite struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Tests       []TestCase `yaml:"tests"`
}

// TestCase represents a single test.
type TestCase struct {
	Name   string   `yaml:"name"`
	Prompt string   `yaml:"prompt"`
	Checks []string `yaml:"checks"`
}

// EvalResult represents the result of running evaluations.
type EvalResult struct {
	Timestamp   time.Time           `json:"timestamp"`
	TestSuite   string              `json:"test_suite"`
	TotalTests  int                 `json:"total_tests"`
	Passed      int                 `json:"passed"`
	Failed      int                 `json:"failed"`
	Regressions int                 `json:"regressions"`
	TestResults []TestResult        `json:"test_results"`
	Comparison  *BaselineComparison `json:"comparison,omitempty"`
}

// TestResult represents a single test result.
type TestResult struct {
	Name         string        `json:"name"`
	Status       string        `json:"status"` // passed, failed, error
	Duration     time.Duration `json:"duration_ms"`
	CheckResults []CheckResult `json:"checks"`
	Error        string        `json:"error,omitempty"`
	Regression   bool          `json:"regression,omitempty"`
}

// CheckResult represents a single check result.
type CheckResult struct {
	Check   string `json:"check"`
	Passed  bool   `json:"passed"`
	Message string `json:"message,omitempty"`
}

// BaselineComparison represents comparison with baseline.
type BaselineComparison struct {
	BaselineDate    time.Time `json:"baseline_date"`
	NewFailures     []string  `json:"new_failures,omitempty"`
	NewPasses       []string  `json:"new_passes,omitempty"`
	RemovedTests    []string  `json:"removed_tests,omitempty"`
	AddedTests      []string  `json:"added_tests,omitempty"`
	BehaviorChanges []string  `json:"behavior_changes,omitempty"`
}

// LoadSuite loads a test suite from a YAML file.
func LoadSuite(path string) (*TestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read test suite: %w", err)
	}

	var suite TestSuite
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("could not parse test suite: %w", err)
	}

	return &suite, nil
}

// RunTest executes a single test case against a trace.
func RunTest(test TestCase, tr *trace.LLMTrace) TestResult {
	startTime := time.Now()

	result := TestResult{
		Name:         test.Name,
		Status:       "passed",
		CheckResults: make([]CheckResult, 0, len(test.Checks)),
	}

	// Run each check against the trace
	for _, check := range test.Checks {
		checkResult := RunCheck(check, tr)
		result.CheckResults = append(result.CheckResults, checkResult)

		if !checkResult.Passed {
			result.Status = "failed"
		}
	}

	result.Duration = time.Since(startTime) / time.Millisecond

	return result
}

// LoadLatestTrace loads the most recent trace from the traces directory.
func LoadLatestTrace() (*trace.LLMTrace, error) {
	traceDir := filepath.Join(".regrada", "traces")

	// Find the most recent trace file
	files, err := filepath.Glob(filepath.Join(traceDir, "*.json"))
	if err != nil || len(files) == 0 {
		return nil, fmt.Errorf("no trace files found in %s", traceDir)
	}

	// Sort by modification time to get the latest
	var latestFile string
	var latestTime time.Time
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if info.ModTime().After(latestTime) {
			latestTime = info.ModTime()
			latestFile = file
		}
	}

	if latestFile == "" {
		return nil, fmt.Errorf("no valid trace files found")
	}

	// Load the trace session
	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read trace file: %w", err)
	}

	var session trace.TraceSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse trace file: %w", err)
	}

	// For now, return the first trace in the session
	// TODO: Match traces to tests based on test metadata
	if len(session.Traces) == 0 {
		return nil, fmt.Errorf("no traces found in session")
	}

	return &session.Traces[0], nil
}

// CompareWithBaseline compares current results with a baseline file.
func CompareWithBaseline(current *EvalResult, baselinePath string) (*BaselineComparison, error) {
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		return nil, err
	}

	var baseline EvalResult
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, err
	}

	comparison := &BaselineComparison{
		BaselineDate: baseline.Timestamp,
		NewFailures:  []string{},
		NewPasses:    []string{},
		RemovedTests: []string{},
		AddedTests:   []string{},
	}

	// Build maps for easy lookup
	baselineTests := make(map[string]TestResult)
	for _, tr := range baseline.TestResults {
		baselineTests[tr.Name] = tr
	}

	currentTests := make(map[string]TestResult)
	for _, tr := range current.TestResults {
		currentTests[tr.Name] = tr
	}

	// Find new failures and new passes
	for name, currentTest := range currentTests {
		baselineTest, existsInBaseline := baselineTests[name]

		if !existsInBaseline {
			comparison.AddedTests = append(comparison.AddedTests, name)
			continue
		}

		// Check for regressions (new failures)
		if baselineTest.Status == "passed" && currentTest.Status == "failed" {
			comparison.NewFailures = append(comparison.NewFailures, name)
		}

		// Check for fixes (new passes)
		if baselineTest.Status == "failed" && currentTest.Status == "passed" {
			comparison.NewPasses = append(comparison.NewPasses, name)
		}
	}

	// Find removed tests
	for name := range baselineTests {
		if _, exists := currentTests[name]; !exists {
			comparison.RemovedTests = append(comparison.RemovedTests, name)
		}
	}

	return comparison, nil
}

// SaveResults saves evaluation results to a file.
func SaveResults(result *EvalResult, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
