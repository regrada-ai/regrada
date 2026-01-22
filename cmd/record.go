package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/matias/regrada/internal/config"
	"github.com/matias/regrada/internal/record"
	"github.com/matias/regrada/internal/trace"
	"github.com/spf13/cobra"
)

var (
	recordConfigPath string
	recordStopAfter  int
	recordSessionID  string
)

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Start the HTTP proxy recorder",
	RunE:  runRecord,
}

func init() {
	rootCmd.AddCommand(recordCmd)

	recordCmd.Flags().StringVarP(&recordConfigPath, "config", "c", "", "Path to config file (default: regrada.yml/regrada.yaml)")
	recordCmd.Flags().IntVar(&recordStopAfter, "stop-after", 0, "Stop after N traces are recorded")
	recordCmd.Flags().StringVar(&recordSessionID, "session", "", "Session ID (default: timestamp)")
}

func runRecord(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadProjectConfig(recordConfigPath)
	if err != nil {
		return ExitError{Code: 3, Err: err}
	}
	if cfg.Capture.Enabled != nil && !*cfg.Capture.Enabled {
		return ExitError{Code: 1, Err: fmt.Errorf("capture is disabled in config")}
	}
	if cfg.Capture.Mode != "proxy" {
		return ExitError{Code: 1, Err: fmt.Errorf("capture.mode must be proxy")}
	}

	if err := os.MkdirAll(cfg.Record.TracesDir, 0755); err != nil {
		return ExitError{Code: 1, Err: err}
	}

	redactor, err := buildRedactor(cfg)
	if err != nil {
		return ExitError{Code: 1, Err: err}
	}

	session := record.NewSession(recordSessionID)
	store := trace.NewLocalStore(cfg.Record.TracesDir)
	recorder := record.NewProxyRecorder(cfg, store, redactor, session)
	if recordStopAfter > 0 {
		recorder.SetStopAfter(recordStopAfter)
	}

	if err := recorder.Start(); err != nil {
		return ExitError{Code: 1, Err: err}
	}

	fmt.Printf("Recorder listening on %s\n", cfg.Capture.Proxy.Listen)
	fmt.Println("Set provider base URL to the proxy (OPENAI_BASE_URL/ANTHROPIC_BASE_URL).")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	select {
	case <-sig:
		_ = recorder.Stop()
	case <-recorder.Done():
		// stopped by stop-after or shutdown
	}

	session.Finalize()
	path, err := record.SaveSession(cfg.Record.SessionDir, session)
	if err != nil {
		return ExitError{Code: 1, Err: err}
	}

	fmt.Printf("Recorded %d traces\n", recorder.TraceCount())
	fmt.Printf("Session saved to %s\n", path)
	return nil
}

func buildRedactor(cfg *config.ProjectConfig) (trace.Redactor, error) {
	redactCfg := cfg.Capture.Redact
	if redactCfg.Enabled != nil && !*redactCfg.Enabled {
		return nil, nil
	}

	presetPatterns := trace.PresetPatterns(redactCfg.Presets)
	redactCfg.Patterns = append(presetPatterns, redactCfg.Patterns...)
	return trace.NewRedactor(redactCfg)
}
