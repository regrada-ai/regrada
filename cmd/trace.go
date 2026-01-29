// SPDX-License-Identifier: LicenseRef-Regrada-Proprietary

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/matias/regrada/config"
	"github.com/matias/regrada/eval"
	"github.com/matias/regrada/proxy"
	"github.com/matias/regrada/trace"
	"github.com/spf13/cobra"
)

var (
	traceSaveBaseline bool
	traceOutputFile   string
	traceConfigPath   string
	traceNoProxy      bool
	traceVerbose      bool
	traceUpdateTests  bool
	traceOnConflict   string
)

var traceCmd = &cobra.Command{
	Use:   "trace -- <command>",
	Short: "Trace LLM API calls from a command",
	Long:  "Start a proxy, run your command, and capture LLM API calls for regression testing.",
	Args:  cobra.ArbitraryArgs,
	Run:   runTrace,
}

func init() {
	rootCmd.AddCommand(traceCmd)

	traceCmd.Flags().BoolVarP(&traceSaveBaseline, "save-baseline", "b", false, "Save traces as baseline")
	traceCmd.Flags().StringVarP(&traceOutputFile, "output", "o", "", "Output file for traces")
	traceCmd.Flags().StringVarP(&traceConfigPath, "config", "c", ".regrada.yaml", "Path to config file")
	traceCmd.Flags().BoolVar(&traceNoProxy, "no-proxy", false, "Run without proxy")
	traceCmd.Flags().BoolVarP(&traceVerbose, "verbose", "v", false, "Verbose output")
	traceCmd.Flags().BoolVar(&traceUpdateTests, "update-tests", false, "Auto-generate test stubs for new traces")
	traceCmd.Flags().StringVar(&traceOnConflict, "on-conflict", "merge", "Handle existing tests: merge, replace, append")

	traceCmd.Flags().SetInterspersed(false)
}

func runTrace(cmd *cobra.Command, args []string) {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no command specified after --\n")
		os.Exit(1)
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	fmt.Println()
	fmt.Println(titleStyle.Render("Regrada Trace"))
	fmt.Println(dimStyle.Render("Capturing LLM API calls..."))
	fmt.Println()

	cfg, err := config.Load(traceConfigPath)
	if err != nil {
		fmt.Printf("%s Config not found, using defaults\n", warnStyle.Render("Warning:"))
		cfg = config.Defaults(".")
	}

	traceDir := filepath.Join(".regrada", "traces")
	if err := os.MkdirAll(traceDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create trace directory %s: %v\n", traceDir, err)
		os.Exit(1)
	}

	var session *trace.TraceSession

	if traceNoProxy {
		session = &trace.TraceSession{
			ID:        generateTraceID(),
			StartTime: time.Now(),
			Command:   strings.Join(args, " "),
			Traces:    []trace.LLMTrace{},
		}

		exitCode := executeCommand(args, nil)
		session.EndTime = time.Now()

		if exitCode != 0 {
			os.Exit(exitCode)
		}
	} else {
		prox, err := proxy.New(cfg)
		if err != nil {
			fmt.Printf("%s Failed to start proxy: %v\n", warnStyle.Render("Error:"), err)
			os.Exit(1)
		}

		proxyAddr := prox.Address()
		if traceVerbose {
			fmt.Printf("%s Proxy running on %s\n", dimStyle.Render("→"), proxyAddr)
		}

		env := buildProxyEnv(proxyAddr, cfg)

		session = &trace.TraceSession{
			ID:        generateTraceID(),
			StartTime: time.Now(),
			Command:   strings.Join(args, " "),
		}

		exitCode := executeCommand(args, env)
		session.EndTime = time.Now()

		session.Traces = prox.Traces()
		session.Summary = trace.CalculateSummary(session.Traces)

		prox.Shutdown()

		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}

	outputPath := traceOutputFile
	if outputPath == "" {
		outputPath = filepath.Join(traceDir, fmt.Sprintf("%s.json", session.ID))
	}

	if err := trace.Save(session, outputPath); err != nil {
		fmt.Printf("%s Failed to save traces: %v\n", warnStyle.Render("Error:"), err)
		os.Exit(1)
	}

	if traceSaveBaseline {
		baselinePath := filepath.Join(".regrada", "baseline.json")
		if err := trace.Save(session, baselinePath); err != nil {
			fmt.Printf("%s Failed to save baseline: %v\n", warnStyle.Render("Error:"), err)
		} else {
			fmt.Printf("%s Saved as baseline\n", successStyle.Render("✓"))
		}

		if len(session.Traces) > 0 {
			stubs := eval.GenerateTestStubs(session)
			testsPath := filepath.Join(cfg.Evals.Path, "tests.yaml")

			if err := handleTestGeneration(stubs, testsPath, traceOnConflict); err != nil {
				fmt.Printf("%s Failed to generate test stubs: %v\n", warnStyle.Render("Warning:"), err)
			} else {
				fmt.Printf("%s Generated %d test stubs in %s\n", successStyle.Render("✓"), len(stubs.Tests), testsPath)
			}
		}
	}

	if traceUpdateTests && !traceSaveBaseline && len(session.Traces) > 0 {
		testsPath := filepath.Join(cfg.Evals.Path, "tests.yaml")

		existingSuite, err := eval.LoadSuite(testsPath)
		if err != nil {
			fmt.Printf("%s No existing tests found, skipping test update\n", warnStyle.Render("Warning:"))
		} else if len(session.Traces) > len(existingSuite.Tests) {
			newTraceCount := len(session.Traces) - len(existingSuite.Tests)
			allStubs := eval.GenerateTestStubs(session)
			newTests := allStubs.Tests[len(existingSuite.Tests):]

			existingSuite.Tests = append(existingSuite.Tests, newTests...)
			if err := eval.SaveSuite(existingSuite, testsPath); err != nil {
				fmt.Printf("%s Failed to update tests: %v\n", warnStyle.Render("Warning:"), err)
			} else {
				fmt.Printf("%s Added %d new test stubs for new traces\n", successStyle.Render("✓"), newTraceCount)
			}
		}
	}

	trace.PrintSummary(session)

	baselinePath := filepath.Join(".regrada", "baseline.json")
	if comp, err := trace.Compare(session, baselinePath); err == nil {
		trace.PrintComparison(comp)
	}

	fmt.Println()
	fmt.Printf("%s Traces saved to %s\n", successStyle.Render("✓"), outputPath)
}

func generateTraceID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func executeCommand(args []string, env []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no command specified after --\n")
		return 1
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if env != nil {
		cmd.Env = env
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case <-sigChan:
		cmd.Process.Signal(syscall.SIGTERM)
		return 130
	case err := <-done:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode()
			}
			return 1
		}
		return 0
	}
}

func handleTestGeneration(newSuite *eval.TestSuite, path string, onConflict string) error {
	existing, err := eval.LoadSuite(path)
	if err != nil {
		return eval.SaveSuite(newSuite, path)
	}

	switch onConflict {
	case "replace":
		return eval.SaveSuite(newSuite, path)
	case "append":
		existing.Tests = append(existing.Tests, newSuite.Tests...)
		return eval.SaveSuite(existing, path)
	case "merge":
		return mergeTestSuites(existing, newSuite, path)
	default:
		return fmt.Errorf("unknown conflict resolution: %s", onConflict)
	}
}

func mergeTestSuites(existing, newSuite *eval.TestSuite, path string) error {
	filteredTests := make([]eval.TestCase, 0)
	for _, test := range existing.Tests {
		if test.Name != "example_trace" {
			filteredTests = append(filteredTests, test)
		}
	}
	existing.Tests = filteredTests

	existingNames := make(map[string]bool)
	for _, test := range existing.Tests {
		existingNames[test.Name] = true
	}

	for _, test := range newSuite.Tests {
		if !existingNames[test.Name] {
			existing.Tests = append(existing.Tests, test)
		}
	}

	return eval.SaveSuite(existing, path)
}

func buildProxyEnv(proxyAddr string, cfg *config.RegradaConfig) []string {
	env := os.Environ()
	proxyURL := fmt.Sprintf("http://%s", proxyAddr)

	env = append(env, fmt.Sprintf("HTTP_PROXY=%s", proxyURL))
	env = append(env, fmt.Sprintf("HTTPS_PROXY=%s", proxyURL))
	env = append(env, fmt.Sprintf("http_proxy=%s", proxyURL))
	env = append(env, fmt.Sprintf("https_proxy=%s", proxyURL))

	switch cfg.Provider.Type {
	case "openai":
		env = append(env, "OPENAI_BASE_URL=http://"+proxyAddr)
	case "anthropic":
		env = append(env, "ANTHROPIC_BASE_URL=http://"+proxyAddr)
	case "azure-openai":
		if cfg.Provider.BaseURL != "" {
			env = append(env, "AZURE_OPENAI_ENDPOINT=http://"+proxyAddr)
		}
	case "custom":
		env = append(env, "BASE_URL=http://"+proxyAddr)
		env = append(env, "API_BASE_URL=http://"+proxyAddr)
		env = append(env, "OLLAMA_HOST=http://"+proxyAddr)
	}

	env = append(env, "REGRADA_TRACING=1")

	return env
}
