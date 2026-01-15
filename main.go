package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/matias/regrada/config"
	"github.com/matias/regrada/eval"
	"github.com/matias/regrada/proxy"
	"github.com/matias/regrada/trace"
	"gopkg.in/yaml.v3"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		runInit()
	case "trace":
		runTrace()
	case "run":
		runEval()
	case "version":
		fmt.Printf("regrada version %s\n", version)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Regrada - CI/CD for AI applications

Usage:
  regrada init                   Initialize new project with interactive setup
  regrada trace -- <command>     Trace LLM API calls from command
  regrada run [options]          Run evaluations and detect regressions
  regrada version                Show version information
  regrada help                   Show this help message

Global options:
  -v, --verbose                  Enable verbose output

Init options:
  -f, --force                    Force initialization even if project exists
  -y, --yes                      Use default values without interactive prompts

Trace options:
  -b, --save-baseline           Save traces as baseline
  -o, --output <file>           Output file for traces
  -c, --config <path>           Path to config file (default: .regrada.yaml)
  --no-proxy                    Run without proxy

Run options:
  -t, --tests <path>            Path to test suite (default: evals/tests.yaml)
  -b, --baseline <path>         Path to baseline (default: .regrada/baseline.json)
  --ci                          CI mode (exit 1 on regressions)
  -o, --output <format>         Output format: text, json, github (default: text)
  -c, --config <path>           Path to config file (default: .regrada.yaml)

Examples:
  regrada init
  regrada trace -- python app.py
  regrada trace --save-baseline -- python app.py
  regrada run
  regrada run --ci --output github

Learn more: https://regrada.com/docs`)
}

//
// INIT COMMAND
//

func runInit() {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	force := fs.Bool("force", false, "Force initialization")
	forceShort := fs.Bool("f", false, "Force initialization (short)")
	useDefaults := fs.Bool("yes", false, "Use default values")
	useDefaultsShort := fs.Bool("y", false, "Use default values (short)")

	fs.Parse(os.Args[2:])

	// Merge short and long flags
	if *forceShort {
		*force = true
	}
	if *useDefaultsShort {
		*useDefaults = true
	}

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	fmt.Println()
	fmt.Println(titleStyle.Render("Regrada Initialize"))
	fmt.Println(dimStyle.Render("Setting up your AI testing environment..."))
	fmt.Println()

	// Check if already initialized
	if _, err := os.Stat(".regrada.yaml"); err == nil && !*force {
		fmt.Printf("%s Project already initialized. Use --force to reinitialize.\n", warnStyle.Render("Warning:"))
		os.Exit(1)
	}

	var cfg *config.RegradaConfig

	if *useDefaults {
		// Use defaults
		cfg = config.Defaults(".")
	} else {
		// Interactive setup
		cfg = runInteractiveSetup()
	}

	// Write config
	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Printf("%s Failed to serialize config: %v\n", warnStyle.Render("Error:"), err)
		os.Exit(1)
	}

	if err := os.WriteFile(".regrada.yaml", data, 0644); err != nil {
		fmt.Printf("%s Failed to write config: %v\n", warnStyle.Render("Error:"), err)
		os.Exit(1)
	}

	// Create directories
	os.MkdirAll(".regrada/traces", 0755)
	os.MkdirAll("evals/prompts", 0755)

	// Create example test file
	createExampleEval()

	fmt.Println()
	fmt.Println(successStyle.Render("✓ Project initialized successfully!"))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit evals/tests.yaml to define your tests")
	fmt.Println("  2. Run:", dimStyle.Render("regrada trace -- <your-command>"))
	fmt.Println("  3. Evaluate:", dimStyle.Render("regrada run"))
	fmt.Println()
}

func runInteractiveSetup() *config.RegradaConfig {
	cfg := config.Defaults(".")

	// Get current directory name for default project name
	cwd, _ := os.Getwd()
	defaultProject := filepath.Base(cwd)

	var projectName string
	var env string
	var providerType string
	var model string
	var baseURL string
	var captureOptions []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project Name").
				Value(&projectName).
				Placeholder(defaultProject),

			huh.NewSelect[string]().
				Title("Environment").
				Options(
					huh.NewOption("Local Development", "local"),
					huh.NewOption("Staging", "staging"),
					huh.NewOption("Production", "production"),
				).
				Value(&env),
		),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("LLM Provider").
				Options(
					huh.NewOption("OpenAI", "openai"),
					huh.NewOption("Anthropic (Claude)", "anthropic"),
					huh.NewOption("Azure OpenAI", "azure-openai"),
					huh.NewOption("Custom/Ollama", "custom"),
				).
				Value(&providerType),
		),

		huh.NewGroup(
			huh.NewInput().
				Title("Model").
				Description("e.g., gpt-4o, claude-3-5-sonnet-20241022, llama2").
				Value(&model).
				Placeholder("gpt-4o"),
		),

		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Data to Capture").
				Description("Select what data to capture during tracing").
				Options(
					huh.NewOption("Requests", "requests"),
					huh.NewOption("Responses", "responses"),
					huh.NewOption("Traces", "traces"),
					huh.NewOption("Latency", "latency"),
				).
				Value(&captureOptions).
				Limit(4),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Ask for base URL if needed
	if providerType == "azure-openai" || providerType == "custom" {
		baseURLForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Base URL").
					Description("API endpoint URL").
					Value(&baseURL).
					Placeholder("https://your-resource.openai.azure.com"),
			),
		).WithTheme(huh.ThemeCharm())

		if err := baseURLForm.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	}

	// Set values
	if projectName == "" {
		projectName = defaultProject
	}
	if model == "" {
		model = "gpt-5.1"
	}

	cfg.Project = projectName
	cfg.Env = env
	cfg.Provider.Type = providerType
	cfg.Provider.Model = model
	cfg.Provider.BaseURL = baseURL

	// Set capture options based on user selection
	cfg.Capture.Requests = contains(captureOptions, "requests")
	cfg.Capture.Responses = contains(captureOptions, "responses")
	cfg.Capture.Traces = contains(captureOptions, "traces")
	cfg.Capture.Latency = contains(captureOptions, "latency")

	// If no options selected, default to capturing everything
	if len(captureOptions) == 0 {
		cfg.Capture.Requests = true
		cfg.Capture.Responses = true
		cfg.Capture.Traces = true
		cfg.Capture.Latency = true
	}

	return cfg
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func createExampleEval() {
	exampleTests := `name: Example Test Suite
description: Sample tests for your AI application

tests:
  - name: test_basic_response
    prompt: prompts/basic.txt
    checks:
      - schema_valid: evals/schemas/response.json
      - tool_called: get_weather

  - name: test_no_hallucination
    prompt: prompts/factual.txt
    checks:
      - no_tool_called
`

	examplePrompt := `What is the weather in San Francisco?`

	exampleSchema := `{
  "type": "object",
  "required": ["response"],
  "properties": {
    "response": {
      "type": "string"
    }
  }
}`

	os.WriteFile("evals/tests.yaml", []byte(exampleTests), 0644)
	os.WriteFile("evals/prompts/basic.txt", []byte(examplePrompt), 0644)
	os.WriteFile("evals/schemas/response.json", []byte(exampleSchema), 0644)
}

//
// TRACE COMMAND
//

func runTrace() {
	fs := flag.NewFlagSet("trace", flag.ExitOnError)
	saveBaseline := fs.Bool("save-baseline", false, "Save as baseline")
	saveBaselineShort := fs.Bool("b", false, "Save as baseline (short)")
	outputFile := fs.String("output", "", "Output file")
	outputFileShort := fs.String("o", "", "Output file (short)")
	configPath := fs.String("config", ".regrada.yaml", "Config file path")
	configPathShort := fs.String("c", ".regrada.yaml", "Config file path (short)")
	noProxy := fs.Bool("no-proxy", false, "Run without proxy")
	verbose := fs.Bool("verbose", false, "Verbose output")
	verboseShort := fs.Bool("v", false, "Verbose output (short)")

	fs.Parse(os.Args[2:])
	args := fs.Args()

	// Merge short and long flags
	if *saveBaselineShort {
		*saveBaseline = true
	}
	if *outputFileShort != "" {
		*outputFile = *outputFileShort
	}
	if *configPathShort != ".regrada.yaml" {
		*configPath = *configPathShort
	}
	if *verboseShort {
		*verbose = true
	}

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	fmt.Println()
	fmt.Println(titleStyle.Render("Regrada Trace"))
	fmt.Println(dimStyle.Render("Capturing LLM API calls..."))
	fmt.Println()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("%s Config not found, using defaults\n", warnStyle.Render("Warning:"))
		cfg = config.Defaults(".")
	}

	// Ensure trace directory exists
	traceDir := filepath.Join(".regrada", "traces")
	os.MkdirAll(traceDir, 0755)

	var session *trace.TraceSession

	if *noProxy {
		// Run without proxy
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
		// Run with proxy
		prox, err := proxy.New(cfg)
		if err != nil {
			fmt.Printf("%s Failed to start proxy: %v\n", warnStyle.Render("Error:"), err)
			os.Exit(1)
		}

		proxyAddr := prox.Address()
		if *verbose {
			fmt.Printf("%s Proxy running on %s\n", dimStyle.Render("→"), proxyAddr)
		}

		// Build environment with proxy settings
		env := buildProxyEnv(proxyAddr, cfg)

		// Execute command
		session = &trace.TraceSession{
			ID:        generateTraceID(),
			StartTime: time.Now(),
			Command:   strings.Join(args, " "),
		}

		exitCode := executeCommand(args, env)
		session.EndTime = time.Now()

		// Get traces from proxy
		session.Traces = prox.Traces()
		session.Summary = trace.CalculateSummary(session.Traces)

		prox.Shutdown()

		if exitCode != 0 {
			os.Exit(exitCode)
		}
	}

	// Save trace session
	outputPath := *outputFile
	if outputPath == "" {
		outputPath = filepath.Join(traceDir, fmt.Sprintf("%s.json", session.ID))
	}

	if err := trace.Save(session, outputPath); err != nil {
		fmt.Printf("%s Failed to save traces: %v\n", warnStyle.Render("Error:"), err)
		os.Exit(1)
	}

	// Save as baseline if requested
	if *saveBaseline {
		baselinePath := filepath.Join(".regrada", "baseline.json")
		if err := trace.Save(session, baselinePath); err != nil {
			fmt.Printf("%s Failed to save baseline: %v\n", warnStyle.Render("Error:"), err)
		} else {
			fmt.Printf("%s Saved as baseline\n", successStyle.Render("✓"))
		}
	}

	// Print summary
	trace.PrintSummary(session)

	// Compare with baseline if exists
	baselinePath := filepath.Join(".regrada", "baseline.json")
	if comp, err := trace.Compare(session, baselinePath); err == nil {
		trace.PrintComparison(comp)
	}

	fmt.Println()
	fmt.Printf("%s Traces saved to %s\n", successStyle.Render("✓"), outputPath)
}

//
// RUN COMMAND (EVALUATIONS)
//

func runEval() {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	testsPath := fs.String("tests", "", "Path to test suite")
	testsPathShort := fs.String("t", "", "Path to test suite (short)")
	baselinePath := fs.String("baseline", "", "Path to baseline")
	baselinePathShort := fs.String("b", "", "Path to baseline (short)")
	ciMode := fs.Bool("ci", false, "CI mode")
	outputFormat := fs.String("output", "text", "Output format")
	outputFormatShort := fs.String("o", "text", "Output format (short)")
	configPath := fs.String("config", ".regrada.yaml", "Config file path")
	configPathShort := fs.String("c", ".regrada.yaml", "Config file path (short)")
	verbose := fs.Bool("verbose", false, "Verbose output")
	verboseShort := fs.Bool("v", false, "Verbose output (short)")

	fs.Parse(os.Args[2:])

	// Merge short and long flags
	if *testsPathShort != "" {
		*testsPath = *testsPathShort
	}
	if *baselinePathShort != "" {
		*baselinePath = *baselinePathShort
	}
	if *outputFormatShort != "text" {
		*outputFormat = *outputFormatShort
	}
	if *configPathShort != ".regrada.yaml" {
		*configPath = *configPathShort
	}
	if *verboseShort {
		*verbose = true
	}

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	if *outputFormat != "json" {
		fmt.Println()
		fmt.Println(titleStyle.Render("Regrada Eval Runner"))
		fmt.Println(dimStyle.Render("Running AI agent evaluations..."))
		fmt.Println()
	}

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil && *outputFormat != "json" {
		fmt.Printf("%s Config not found, using defaults\n", warnStyle.Render("Warning:"))
		cfg = config.Defaults(".")
	}

	// Find test suite
	if *testsPath == "" {
		*testsPath = filepath.Join(cfg.Evals.Path, "tests.yaml")
	}

	// Load test suite
	suite, err := eval.LoadSuite(*testsPath)
	if err != nil {
		if *outputFormat == "json" {
			jsonErr, _ := json.Marshal(map[string]string{"error": err.Error()})
			fmt.Println(string(jsonErr))
		} else {
			fmt.Printf("%s Failed to load test suite: %v\n", failStyle.Render("✗"), err)
		}
		os.Exit(1)
	}

	if *outputFormat != "json" {
		fmt.Printf("Test suite: %s\n", suite.Name)
		fmt.Printf("Tests: %d\n\n", len(suite.Tests))
	}

	// Load latest trace
	tr, err := eval.LoadLatestTrace()
	if err != nil {
		if *outputFormat == "json" {
			jsonErr, _ := json.Marshal(map[string]string{"error": err.Error()})
			fmt.Println(string(jsonErr))
		} else {
			fmt.Printf("%s Failed to load trace: %v\n", failStyle.Render("✗"), err)
		}
		os.Exit(1)
	}

	// Run tests
	result := &eval.EvalResult{
		Timestamp:   time.Now(),
		TestSuite:   suite.Name,
		TotalTests:  len(suite.Tests),
		TestResults: make([]eval.TestResult, 0, len(suite.Tests)),
	}

	for _, test := range suite.Tests {
		if *verbose {
			fmt.Printf("  Running: %s... ", test.Name)
		}

		testResult := eval.RunTest(test, tr)
		result.TestResults = append(result.TestResults, testResult)

		if testResult.Status == "passed" {
			result.Passed++
			if *verbose {
				fmt.Println(successStyle.Render("✓ passed"))
			}
		} else {
			result.Failed++
			if *verbose {
				fmt.Println(failStyle.Render("✗ failed"))
				for _, cr := range testResult.CheckResults {
					if !cr.Passed {
						fmt.Printf("      %s: %s\n", cr.Check, cr.Message)
					}
				}
			}
		}
	}

	// Compare with baseline if specified
	if *baselinePath == "" {
		*baselinePath = filepath.Join(".regrada", "baseline.json")
	}

	if comp, err := eval.CompareWithBaseline(result, *baselinePath); err == nil {
		result.Comparison = comp
		result.Regressions = len(comp.NewFailures)

		// Mark tests as regressions
		for i := range result.TestResults {
			for _, name := range comp.NewFailures {
				if result.TestResults[i].Name == name {
					result.TestResults[i].Regression = true
				}
			}
		}
	}

	// Output results
	switch *outputFormat {
	case "json":
		outputJSON(result)
	case "github":
		outputGitHub(result)
	default:
		outputText(result, successStyle, failStyle, warnStyle)
	}

	// Save results
	resultsPath := filepath.Join(".regrada", "results.json")
	eval.SaveResults(result, resultsPath)

	// Exit with error if in CI mode and there are regressions
	if *ciMode && result.Regressions > 0 {
		os.Exit(1)
	}
}

//
// HELPER FUNCTIONS
//

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

	// Handle interrupts
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case <-sigChan:
		cmd.Process.Signal(syscall.SIGTERM)
		return 130 // Standard exit code for SIGINT
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

func buildProxyEnv(proxyAddr string, cfg *config.RegradaConfig) []string {
	env := os.Environ()
	proxyURL := fmt.Sprintf("http://%s", proxyAddr)

	env = append(env, fmt.Sprintf("HTTP_PROXY=%s", proxyURL))
	env = append(env, fmt.Sprintf("HTTPS_PROXY=%s", proxyURL))
	env = append(env, fmt.Sprintf("http_proxy=%s", proxyURL))
	env = append(env, fmt.Sprintf("https_proxy=%s", proxyURL))

	// Provider-specific environment variables
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
