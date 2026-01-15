package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/matias/regrada/config"
	"github.com/matias/regrada/eval"
	"github.com/spf13/cobra"
)

var (
	runTestsPath     string
	runBaselinePath  string
	runCIMode        bool
	runOutputFormat  string
	runConfigPath    string
	runVerboseOutput bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run evaluations and detect regressions",
	Args:  cobra.NoArgs,
	Run:   runEval,
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVarP(&runTestsPath, "tests", "t", "", "Path to test suite")
	runCmd.Flags().StringVarP(&runBaselinePath, "baseline", "b", "", "Path to baseline")
	runCmd.Flags().BoolVar(&runCIMode, "ci", false, "CI mode (exit 1 on regressions)")
	runCmd.Flags().StringVarP(&runOutputFormat, "output", "o", "text", "Output format: text, json, github")
	runCmd.Flags().StringVarP(&runConfigPath, "config", "c", ".regrada.yaml", "Path to config file")
	runCmd.Flags().BoolVarP(&runVerboseOutput, "verbose", "v", false, "Verbose output")
}

func runEval(cmd *cobra.Command, args []string) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	if runOutputFormat != "json" {
		fmt.Println()
		fmt.Println(titleStyle.Render("Regrada Eval Runner"))
		fmt.Println(dimStyle.Render("Running AI agent evaluations..."))
		fmt.Println()
	}

	cfg, err := config.Load(runConfigPath)
	if err != nil {
		if runOutputFormat != "json" {
			fmt.Printf("%s Config not found, using defaults\n", warnStyle.Render("Warning:"))
		}
		cfg = config.Defaults(".")
	}

	if runTestsPath == "" {
		runTestsPath = filepath.Join(cfg.Evals.Path, "tests.yaml")
	}

	suite, err := eval.LoadSuite(runTestsPath)
	if err != nil {
		if runOutputFormat == "json" {
			jsonErr, _ := json.Marshal(map[string]string{"error": err.Error()})
			fmt.Println(string(jsonErr))
		} else {
			fmt.Printf("%s Failed to load test suite: %v\n", failStyle.Render("✗"), err)
		}
		os.Exit(1)
	}

	if runOutputFormat != "json" {
		fmt.Printf("Test suite: %s\n", suite.Name)
		fmt.Printf("Tests: %d\n\n", len(suite.Tests))
	}

	session, err := eval.LoadLatestSession()
	if err != nil {
		if runOutputFormat == "json" {
			jsonErr, _ := json.Marshal(map[string]string{"error": err.Error()})
			fmt.Println(string(jsonErr))
		} else {
			fmt.Printf("%s Failed to load trace session: %v\n", failStyle.Render("✗"), err)
		}
		os.Exit(1)
	}

	if len(session.Traces) > len(suite.Tests) && runOutputFormat != "json" {
		unmatchedCount := len(session.Traces) - len(suite.Tests)
		fmt.Printf("%s Session has %d more traces than tests (%d traces, %d tests)\n",
			warnStyle.Render("Warning:"), unmatchedCount, len(session.Traces), len(suite.Tests))
		fmt.Printf("%s Run 'regrada trace --update-tests -- <command>' to add test stubs for new traces\n\n",
			dimStyle.Render("Tip:"))
	}

	result := &eval.EvalResult{
		Timestamp:   time.Now(),
		TestSuite:   suite.Name,
		TotalTests:  len(suite.Tests),
		TestResults: make([]eval.TestResult, 0, len(suite.Tests)),
	}

	for _, test := range suite.Tests {
		if runVerboseOutput {
			fmt.Printf("  Running: %s... ", test.Name)
		}

		tr, err := eval.GetTraceForTest(test, session)
		if err != nil {
			testResult := eval.TestResult{
				Name:   test.Name,
				Status: "error",
				Error:  err.Error(),
			}
			result.TestResults = append(result.TestResults, testResult)
			result.Failed++
			if runVerboseOutput {
				fmt.Println(failStyle.Render("✗ error: " + err.Error()))
			}
			continue
		}

		testResult := eval.RunTest(test, tr)
		result.TestResults = append(result.TestResults, testResult)

		if testResult.Status == "passed" {
			result.Passed++
			if runVerboseOutput {
				fmt.Println(successStyle.Render("✓ passed"))
			}
		} else {
			result.Failed++
			if runVerboseOutput {
				fmt.Println(failStyle.Render("✗ failed"))
				for _, cr := range testResult.CheckResults {
					if !cr.Passed {
						fmt.Printf("      %s: %s\n", cr.Check, cr.Message)
					}
				}
			}
		}
	}

	if runBaselinePath == "" {
		runBaselinePath = filepath.Join(".regrada", "baseline.json")
	}

	if comp, err := eval.CompareWithBaseline(result, runBaselinePath); err == nil {
		result.Comparison = comp
		result.Regressions = len(comp.NewFailures)

		for i := range result.TestResults {
			for _, name := range comp.NewFailures {
				if result.TestResults[i].Name == name {
					result.TestResults[i].Regression = true
				}
			}
		}
	}

	switch runOutputFormat {
	case "json":
		outputJSON(result)
	case "github":
		outputGitHub(result)
	default:
		outputText(result, successStyle, failStyle, warnStyle)
	}

	resultsPath := filepath.Join(".regrada", "results.json")
	eval.SaveResults(result, resultsPath)

	if runCIMode && result.Regressions > 0 {
		os.Exit(1)
	}
}

func outputText(result *eval.EvalResult, successStyle, failStyle, warnStyle lipgloss.Style) {
	fmt.Println()
	fmt.Println("Results:")
	fmt.Printf("  Total: %d\n", result.TotalTests)
	fmt.Printf("  %s: %d\n", successStyle.Render("Passed"), result.Passed)
	fmt.Printf("  %s: %d\n", failStyle.Render("Failed"), result.Failed)

	if result.Comparison != nil && len(result.Comparison.NewFailures) > 0 {
		fmt.Printf("  %s: %d\n", warnStyle.Render("Regressions"), result.Regressions)
		fmt.Println()
		fmt.Println(warnStyle.Render("New failures (regressions):"))
		for _, name := range result.Comparison.NewFailures {
			fmt.Printf("  - %s\n", name)
		}
	}

	fmt.Println()
}

func outputJSON(result *eval.EvalResult) {
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
}

func outputGitHub(result *eval.EvalResult) {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "## Regrada Evaluation Results\n\n")
	fmt.Fprintf(&buf, "**Total Tests:** %d  \n", result.TotalTests)
	fmt.Fprintf(&buf, "**Passed:** %d ✓  \n", result.Passed)
	fmt.Fprintf(&buf, "**Failed:** %d ✗  \n", result.Failed)

	if result.Regressions > 0 {
		fmt.Fprintf(&buf, "\n### ⚠️ Regressions Detected: %d\n\n", result.Regressions)
		fmt.Fprintf(&buf, "The following tests passed in the baseline but are now failing:\n\n")
		for _, name := range result.Comparison.NewFailures {
			fmt.Fprintf(&buf, "- %s\n", name)
		}
	}

	if len(result.Comparison.NewPasses) > 0 {
		fmt.Fprintf(&buf, "\n### ✓ Fixed Tests: %d\n\n", len(result.Comparison.NewPasses))
		for _, name := range result.Comparison.NewPasses {
			fmt.Fprintf(&buf, "- %s\n", name)
		}
	}

	fmt.Println(buf.String())
}
