package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/regrada-ai/regrada/internal/cases"
	"github.com/regrada-ai/regrada/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	initForce          bool
	initPath           string
	initNonInteractive bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a project with regrada.yml and an example case",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing config")
	initCmd.Flags().StringVar(&initPath, "path", "regrada.yml", "Path for the project config")
	initCmd.Flags().BoolVar(&initNonInteractive, "non-interactive", false, "Skip interactive prompts and use defaults")
}

func runInit(cmd *cobra.Command, args []string) error {
	if initPath == "" {
		initPath = "regrada.yml"
	}

	if _, err := os.Stat(initPath); err == nil && !initForce {
		return fmt.Errorf("config already exists at %s (use --force to overwrite)", initPath)
	}

	projectName := filepath.Base(mustGetwd())
	var cfg config.ProjectConfig

	if initNonInteractive {
		cfg = config.DefaultConfig(projectName)
	} else {
		var err error
		cfg, err = interactiveConfig(projectName)
		if err != nil {
			return fmt.Errorf("interactive setup: %w", err)
		}
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(initPath, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	dirs := []string{
		"regrada/cases",
		".regrada",
		".regrada/snapshots",
		".regrada/traces",
		".regrada/sessions",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	examplePath := filepath.Join("regrada", "cases", "example.yml")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) || initForce {
		example := cases.ExampleCase()
		if err := cases.WriteCase(examplePath, example); err != nil {
			return fmt.Errorf("write example case: %w", err)
		}
	}

	if err := appendGitignore(".regrada/", ".regrada/report.md", ".regrada/junit.xml"); err != nil {
		return fmt.Errorf("update .gitignore: %w", err)
	}

	fmt.Printf("\n✓ Initialized Regrada in %s\n", filepath.Dir(initPath))
	fmt.Printf("  - Config: %s\n", initPath)
	fmt.Printf("  - Example case: %s\n", examplePath)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Edit %s to configure your LLM provider\n", initPath)
	fmt.Printf("  2. Run 'regrada test' to execute your test cases\n")
	return nil
}

func appendGitignore(entries ...string) error {
	path := ".gitignore"
	data, _ := os.ReadFile(path)
	existing := string(data)
	lines := strings.Split(existing, "\n")
	seen := make(map[string]bool)
	for _, line := range lines {
		seen[strings.TrimSpace(line)] = true
	}

	var toAppend []string
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		if !seen[entry] {
			toAppend = append(toAppend, entry)
		}
	}

	if len(toAppend) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(existing) > 0 && !strings.HasSuffix(existing, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}

	for _, entry := range toAppend {
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return err
		}
	}

	return nil
}

func mustGetwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func interactiveConfig(projectName string) (config.ProjectConfig, error) {
	cfg := config.DefaultConfig(projectName)

	// Styling
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		MarginBottom(1)

	// Variables to capture user input
	var (
		provider         string
		projectNameInput string
		model            string
		deployment       string
		apiVersion       string
		region           string
		modelID          string
		runsStr          string
		timeoutStr       string
		concurrencyStr   string
		enableCapture    bool = true
		proxyListen      string
		proxyMode        string
		enableRedact     bool = true
		baselineMode     string
		gitRef           string
		addPIIPolicy     bool = true
		failOnError      bool = true
		commentOnPR      bool = true
	)

	// Welcome message
	fmt.Println(titleStyle.Render("Welcome to Regrada!"))
	fmt.Println("Let's configure your project with a few quick questions.")

	// Form 1: Project and Provider
	form1 := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project Name").
				Description("Name of your LLM testing project").
				Value(&projectNameInput).
				Placeholder(projectName),

			huh.NewSelect[string]().
				Title("LLM Provider").
				Description("Which provider will you use for testing?").
				Options(
					huh.NewOption("OpenAI", "openai"),
					huh.NewOption("Anthropic", "anthropic"),
					huh.NewOption("Azure OpenAI", "azure_openai"),
					huh.NewOption("AWS Bedrock", "bedrock"),
					huh.NewOption("Mock (for testing)", "mock"),
				).
				Value(&provider),
		),
	)

	if err := form1.Run(); err != nil {
		return cfg, err
	}

	// Apply project name (use default if empty)
	if projectNameInput != "" {
		cfg.Project.Name = projectNameInput
	}

	// Apply provider (use default if empty)
	if provider != "" {
		cfg.Providers.Default = provider
	} else {
		cfg.Providers.Default = "mock"
	}

	// Provider-specific configuration
	if provider != "mock" {
		var providerForm *huh.Form

		switch provider {
		case "openai":
			providerForm = huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title("OpenAI Configuration").
						Description("Set OPENAI_API_KEY environment variable for authentication"),

					huh.NewInput().
						Title("Model").
						Description("e.g., gpt-4o, gpt-4-turbo, gpt-3.5-turbo").
						Value(&model).
						Placeholder("gpt-4o"),
				),
			)
		case "anthropic":
			providerForm = huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title("Anthropic Configuration").
						Description("Set ANTHROPIC_API_KEY environment variable for authentication"),

					huh.NewInput().
						Title("Model").
						Description("e.g., claude-3-5-sonnet-20241022, claude-3-opus-20240229").
						Value(&model).
						Placeholder("claude-3-5-sonnet-20241022"),
				),
			)
		case "azure_openai":
			providerForm = huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title("Azure OpenAI Configuration").
						Description("Set AZURE_OPENAI_API_KEY and AZURE_OPENAI_ENDPOINT environment variables"),

					huh.NewInput().
						Title("Deployment Name").
						Description("Your Azure OpenAI deployment name").
						Value(&deployment).
						Placeholder("my-gpt4-deployment"),

					huh.NewInput().
						Title("API Version").
						Description("Azure OpenAI API version").
						Value(&apiVersion).
						Placeholder("2024-02-15-preview"),
				),
			)
		case "bedrock":
			providerForm = huh.NewForm(
				huh.NewGroup(
					huh.NewNote().
						Title("AWS Bedrock Configuration").
						Description("Set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables"),

					huh.NewInput().
						Title("AWS Region").
						Description("AWS region for Bedrock").
						Value(&region).
						Placeholder("us-east-1"),

					huh.NewInput().
						Title("Model ID").
						Description("e.g., anthropic.claude-v2, anthropic.claude-3-sonnet").
						Value(&modelID).
						Placeholder("anthropic.claude-v2"),
				),
			)
		}

		if providerForm != nil {
			if err := providerForm.Run(); err != nil {
				return cfg, err
			}
		}

		// Apply provider-specific config
		switch provider {
		case "openai":
			cfg.Providers.OpenAI.Model = model
		case "anthropic":
			cfg.Providers.Anthropic.Model = model
		case "azure_openai":
			cfg.Providers.AzureOpenAI.Deployment = deployment
			if apiVersion != "" {
				cfg.Providers.AzureOpenAI.APIVersion = apiVersion
			} else {
				cfg.Providers.AzureOpenAI.APIVersion = "2024-02-15-preview"
			}
		case "bedrock":
			if region != "" {
				cfg.Providers.Bedrock.Region = region
			} else {
				cfg.Providers.Bedrock.Region = "us-east-1"
			}
			cfg.Providers.Bedrock.ModelID = modelID
		}
	}

	// Form 2: Test Execution
	form2 := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Test Execution Settings").
				Description("Configure how your test cases will run"),

			huh.NewInput().
				Title("Runs per test").
				Description("Number of times to run each test (for statistical analysis)").
				Value(&runsStr).
				Placeholder("3"),

			huh.NewInput().
				Title("Timeout (ms)").
				Description("Maximum time for each test execution").
				Value(&timeoutStr).
				Placeholder("30000"),

			huh.NewInput().
				Title("Concurrency").
				Description("Maximum number of tests to run in parallel").
				Value(&concurrencyStr).
				Placeholder("8"),
		),
	)

	if err := form2.Run(); err != nil {
		return cfg, err
	}

	// Parse string inputs to integers
	runs := 3
	if runsStr != "" {
		if val, err := parseIntOrDefault(runsStr, 3); err == nil {
			runs = val
		}
	}

	timeout := 30000
	if timeoutStr != "" {
		if val, err := parseIntOrDefault(timeoutStr, 30000); err == nil {
			timeout = val
		}
	}

	concurrency := 8
	if concurrencyStr != "" {
		if val, err := parseIntOrDefault(concurrencyStr, 8); err == nil {
			concurrency = val
		}
	}

	cfg.Cases.Defaults.Runs = runs
	cfg.Cases.Defaults.TimeoutMS = timeout
	cfg.Cases.Defaults.Concurrency = concurrency

	// Form 3: Request Capture
	form3 := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Request Capture").
				Description("Capture LLM API requests via proxy for recording and testing"),

			huh.NewConfirm().
				Title("Enable proxy capture?").
				Description("Intercepts API calls for recording test cases").
				Value(&enableCapture),
		),
	)

	if err := form3.Run(); err != nil {
		return cfg, err
	}

	cfg.Capture.Enabled = &enableCapture

	if enableCapture {
		form3b := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Proxy listen address").
					Description("Address and port for the proxy server").
					Value(&proxyListen).
					Placeholder("127.0.0.1:8080"),

				huh.NewSelect[string]().
					Title("Proxy mode").
					Description("How the proxy intercepts requests").
					Options(
						huh.NewOption("Forward - Intercepts and forwards (configure app to use proxy)", "forward"),
						huh.NewOption("Reverse - Acts as reverse proxy (change app base URL)", "reverse"),
					).
					Value(&proxyMode),

				huh.NewConfirm().
					Title("Enable PII/secrets redaction?").
					Description("Automatically redact sensitive data in traces").
					Value(&enableRedact),
			),
		)

		if err := form3b.Run(); err != nil {
			return cfg, err
		}

		if proxyListen != "" {
			cfg.Capture.Proxy.Listen = proxyListen
		} else {
			cfg.Capture.Proxy.Listen = "127.0.0.1:8080"
		}

		if proxyMode != "" {
			cfg.Capture.Proxy.Mode = proxyMode
		} else {
			cfg.Capture.Proxy.Mode = "forward"
		}

		cfg.Capture.Redact.Enabled = &enableRedact
	}

	// Form 4: Baseline Comparison
	form4 := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Baseline Comparison").
				Description("Compare current results against a baseline (e.g., main branch)"),

			huh.NewSelect[string]().
				Title("Baseline mode").
				Options(
					huh.NewOption("Git - Compare against git ref (e.g., origin/main)", "git"),
					huh.NewOption("Local - Compare against local snapshots", "local"),
					huh.NewOption("Disabled - Skip baseline comparison", "disabled"),
				).
				Value(&baselineMode),
		),
	)

	if err := form4.Run(); err != nil {
		return cfg, err
	}

	if baselineMode != "" {
		cfg.Baseline.Mode = baselineMode
	} else {
		cfg.Baseline.Mode = "git"
	}

	if cfg.Baseline.Mode == "git" {
		form4b := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Git reference for baseline").
					Description("e.g., origin/main, origin/master, main").
					Value(&gitRef).
					Placeholder("origin/main"),
			),
		)

		if err := form4b.Run(); err != nil {
			return cfg, err
		}

		if gitRef != "" {
			cfg.Baseline.Git.Ref = gitRef
		} else {
			cfg.Baseline.Git.Ref = "origin/main"
		}
	}

	// Form 5: Policies and CI
	form5 := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("Policies & CI/CD").
				Description("Configure quality gates and CI integration"),

			huh.NewConfirm().
				Title("Add PII leak detection policy?").
				Description("Automatically detect and flag PII in LLM outputs").
				Value(&addPIIPolicy),

			huh.NewConfirm().
				Title("Fail CI builds on policy errors?").
				Description("Exit with error code when policies fail").
				Value(&failOnError),

			huh.NewConfirm().
				Title("Post results as PR comments?").
				Description("Automatically comment test results on pull requests").
				Value(&commentOnPR),
		),
	)

	if err := form5.Run(); err != nil {
		return cfg, err
	}

	if !addPIIPolicy {
		cfg.Policies = []config.Policy{}
	}

	if !failOnError {
		cfg.CI.FailOn = []config.FailOnSeverity{}
	}

	cfg.CI.CommentOnPR = &commentOnPR

	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("\n✓ Configuration complete!"))

	return cfg, nil
}

func parseIntOrDefault(s string, defaultVal int) (int, error) {
	if s == "" {
		return defaultVal, nil
	}
	var val int
	_, err := fmt.Sscanf(s, "%d", &val)
	if err != nil {
		return defaultVal, err
	}
	return val, nil
}
