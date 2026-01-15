package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "regrada",
	Short: "Regrada - CI/CD for AI applications",
	Long: `Regrada helps you trace LLM calls and run evaluations to catch regressions.

Key commands:
  regrada init                   Initialize new project with interactive setup
  regrada trace -- <command>     Trace LLM API calls from command
  regrada run [options]          Run evaluations and detect regressions
  regrada version                Show version information`,
	Version:      version,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here if needed
}
