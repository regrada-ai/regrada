package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/matias/regrada/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	initForce       bool
	initUseDefaults bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize new project with interactive setup",
	Long:  `Initialize a new Regrada project with interactive configuration or use defaults.`,
	Run:   runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Force initialization even if project exists")
	initCmd.Flags().BoolVarP(&initUseDefaults, "yes", "y", false, "Use default values without interactive prompts")
}

func runInit(cmd *cobra.Command, args []string) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	fmt.Println()
	fmt.Println(titleStyle.Render("Regrada Initialize"))
	fmt.Println(dimStyle.Render("Setting up your AI testing environment..."))
	fmt.Println()

	if _, err := os.Stat(".regrada.yaml"); err == nil && !initForce {
		fmt.Printf("%s Project already initialized. Use --force to reinitialize.\n", warnStyle.Render("Warning:"))
		os.Exit(1)
	}

	var cfg *config.RegradaConfig
	if initUseDefaults {
		cfg = config.Defaults(".")
	} else {
		cfg = runInteractiveSetup()
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		fmt.Printf("%s Failed to serialize config: %v\n", warnStyle.Render("Error:"), err)
		os.Exit(1)
	}

	if err := os.WriteFile(".regrada.yaml", data, 0644); err != nil {
		fmt.Printf("%s Failed to write config: %v\n", warnStyle.Render("Error:"), err)
		os.Exit(1)
	}

	os.MkdirAll(".regrada/traces", 0755)
	os.MkdirAll("evals/schemas", 0755)

	createExampleEval()

	fmt.Println()
	fmt.Println(successStyle.Render("✓ Project initialized successfully!"))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Run your app:", dimStyle.Render("regrada trace --save-baseline -- <your-command>"))
	fmt.Println("  2. Edit generated tests in evals/tests.yaml to add checks")
	fmt.Println("  3. Validate:", dimStyle.Render("regrada run"))
	fmt.Println()
}

func runInteractiveSetup() *config.RegradaConfig {
	cfg := config.Defaults(".")

	cwd, _ := os.Getwd()
	defaultProject := filepath.Base(cwd)

	var projectName string
	var env string
	var providerType string
	var model string
	var baseURL string
	var captureOptions []string
	var evalTypes []string
	var gateEnabled bool
	var gateThreshold string
	var gateFailOn string
	var outputFormat string
	var outputVerbose bool

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

		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Evaluation Types").
				Description("Select evaluation methods to use").
				Options(
					huh.NewOption("Semantic", "semantic"),
					huh.NewOption("Exact", "exact"),
					huh.NewOption("LLM Judge", "llm-judge"),
				).
				Value(&evalTypes).
				Limit(3),
		),

		huh.NewGroup(
			huh.NewConfirm().
				Title("Enable Quality Gate?").
				Description("Automatically fail CI/CD if tests don't meet quality thresholds").
				Value(&gateEnabled).
				Affirmative("Yes").
				Negative("No"),
		),

		huh.NewGroup(
			huh.NewInput().
				Title("Quality Threshold").
				Description("Minimum pass rate (0.0-1.0, e.g., 0.85 for 85%)").
				Value(&gateThreshold).
				Placeholder("0.85"),

			huh.NewSelect[string]().
				Title("Fail On").
				Description("When to fail the quality gate").
				Options(
					huh.NewOption("Any Failure", "any-failure"),
					huh.NewOption("Regression Only", "regression"),
					huh.NewOption("Below Threshold", "threshold"),
				).
				Value(&gateFailOn),
		),

		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Output Format").
				Description("Default output format for results").
				Options(
					huh.NewOption("Text", "text"),
					huh.NewOption("JSON", "json"),
					huh.NewOption("GitHub", "github"),
				).
				Value(&outputFormat),

			huh.NewConfirm().
				Title("Verbose Output?").
				Description("Show detailed information during execution").
				Value(&outputVerbose).
				Affirmative("Yes").
				Negative("No"),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

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

	if projectName == "" {
		projectName = defaultProject
	}
	if model == "" {
		model = "gpt-4o"
	}

	cfg.Project = projectName
	cfg.Env = env
	cfg.Provider.Type = providerType
	cfg.Provider.Model = model
	cfg.Provider.BaseURL = baseURL

	cfg.Capture.Requests = contains(captureOptions, "requests")
	cfg.Capture.Responses = contains(captureOptions, "responses")
	cfg.Capture.Traces = contains(captureOptions, "traces")
	cfg.Capture.Latency = contains(captureOptions, "latency")

	if len(captureOptions) == 0 {
		cfg.Capture.Requests = true
		cfg.Capture.Responses = true
		cfg.Capture.Traces = true
		cfg.Capture.Latency = true
	}

	if len(evalTypes) > 0 {
		cfg.Evals.Types = evalTypes
	}

	cfg.Gate.Enabled = gateEnabled
	if gateThreshold != "" {
		var threshold float64
		fmt.Sscanf(gateThreshold, "%f", &threshold)
		cfg.Gate.Threshold = threshold
	}
	if gateFailOn != "" {
		cfg.Gate.FailOn = gateFailOn
	}

	if outputFormat != "" {
		cfg.Output.Format = outputFormat
	}
	cfg.Output.Verbose = outputVerbose

	return cfg
}

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
description: Run "regrada trace --save-baseline -- <your-command>" to auto-generate tests

tests:
  - name: example_trace
    trace_index: 0
    description: "Example test - will be replaced with auto-generated tests"
    checks:
      - "tool_called:example_tool"
      - "contains:expected text"
`

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
	os.WriteFile("evals/schemas/response.json", []byte(exampleSchema), 0644)
}
