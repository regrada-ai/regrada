package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const version = "0.2.0"

var rootCmd = &cobra.Command{
	Use:   "regrada",
	Short: "Regrada - CI gate for LLM behavior",
	Long: `Regrada records LLM traces, converts them into test cases, and runs evals in CI.

Key commands:
  regrada init        Initialize a project (regrada.yml + example case)
  regrada record      Start the HTTP proxy recorder
  regrada accept      Convert recent traces into cases + baselines
  regrada baseline    Generate baseline snapshots for custom cases
  regrada test        Run evals, diff against baselines, apply policies`,
	Version:      version,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		code := 1
		if exitErr, ok := err.(ExitError); ok {
			code = exitErr.Code
			err = exitErr.Err
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(code)
	}
}

type ExitError struct {
	Code int
	Err  error
}

func (e ExitError) Error() string {
	if e.Err == nil {
		return "exit"
	}
	return e.Err.Error()
}
