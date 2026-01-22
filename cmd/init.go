package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matias/regrada/internal/cases"
	"github.com/matias/regrada/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	initForce bool
	initPath  string
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
}

func runInit(cmd *cobra.Command, args []string) error {
	if initPath == "" {
		initPath = "regrada.yml"
	}

	if _, err := os.Stat(initPath); err == nil && !initForce {
		return fmt.Errorf("config already exists at %s (use --force to overwrite)", initPath)
	}

	projectName := filepath.Base(mustGetwd())
	cfg := config.DefaultConfig(projectName)

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

	fmt.Printf("Initialized Regrada in %s\n", filepath.Dir(initPath))
	fmt.Printf("- Config: %s\n", initPath)
	fmt.Printf("- Example case: %s\n", examplePath)
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
