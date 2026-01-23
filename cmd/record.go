package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/regrada-ai/regrada/internal/config"
	"github.com/regrada-ai/regrada/internal/record"
	"github.com/regrada-ai/regrada/internal/trace"
	"github.com/spf13/cobra"
)

var (
	recordConfigPath string
	recordStopAfter  int
	recordSessionID  string
)

var recordCmd = &cobra.Command{
	Use:   "record [-- command args...]",
	Short: "Start the HTTP proxy recorder",
	Long: `Start the HTTP proxy recorder to capture LLM traffic.

Usage:
  regrada record                    # Start proxy and wait for Ctrl+C
  regrada record -- python app.py   # Start proxy and run command with HTTP_PROXY set
  regrada record -- npm test        # Automatically proxy traffic from npm test`,
	RunE: runRecord,
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

	// Determine proxy mode
	var recorder interface {
		Start() error
		Stop() error
		TraceCount() int
		Session() *record.Session
	}

	if cfg.Capture.Proxy.Mode == "forward" {
		// Forward proxy with MITM
		if err := EnsureCA(cfg.Capture.Proxy.CAPath); err != nil {
			return ExitError{Code: 1, Err: err}
		}
		fpr, err := record.NewForwardProxyRecorder(cfg, store, redactor, session)
		if err != nil {
			return ExitError{Code: 1, Err: err}
		}
		recorder = fpr
	} else {
		// Reverse proxy mode (legacy)
		recorder = record.NewProxyRecorder(cfg, store, redactor, session)
	}

	if err := recorder.Start(); err != nil {
		return ExitError{Code: 1, Err: err}
	}

	proxyURL := fmt.Sprintf("http://%s", cfg.Capture.Proxy.Listen)

	fmt.Printf("Proxy listening on %s\n", cfg.Capture.Proxy.Listen)
	if cfg.Capture.Proxy.Mode == "forward" {
		fmt.Printf("Mode: Forward proxy (HTTPS MITM enabled)\n")
		fmt.Printf("Allowlisted hosts: %v\n", cfg.Capture.Proxy.AllowHosts)
	}

	// Check if user wants to run a command
	if len(args) > 0 {
		return runWithProxy(args, proxyURL, recorder, session, cfg)
	}

	// Interactive mode - wait for signal
	fmt.Println("Press Ctrl+C to stop")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	_ = recorder.Stop()

	session.Finalize()
	path, err := record.SaveSession(cfg.Record.SessionDir, session)
	if err != nil {
		return ExitError{Code: 1, Err: fmt.Errorf("save session: %w", err)}
	}

	fmt.Printf("\nRecorded %d traces\n", recorder.TraceCount())
	if recorder.TraceCount() > 0 {
		fmt.Printf("Session saved to %s\n", path)
	} else {
		fmt.Println("No traces recorded")
	}
	return nil
}

func runWithProxy(args []string, proxyURL string, recorder interface {
	Stop() error
	TraceCount() int
	Session() *record.Session
}, session *record.Session, cfg *config.ProjectConfig) error {

	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	fmt.Printf("Running command with proxy: %v\n\n", args)

	command := exec.Command(args[0], args[1:]...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin

	// Set proxy environment variables
	command.Env = append(os.Environ(),
		fmt.Sprintf("HTTP_PROXY=%s", proxyURL),
		fmt.Sprintf("HTTPS_PROXY=%s", proxyURL),
		fmt.Sprintf("http_proxy=%s", proxyURL),
		fmt.Sprintf("https_proxy=%s", proxyURL),
	)

	// Run command
	if err := command.Start(); err != nil {
		_ = recorder.Stop()
		return fmt.Errorf("start command: %w", err)
	}

	// Wait for command to complete
	cmdErr := command.Wait()

	// Stop proxy
	_ = recorder.Stop()

	// Save session
	session.Finalize()
	path, err := record.SaveSession(cfg.Record.SessionDir, session)
	if err != nil {
		return ExitError{Code: 1, Err: fmt.Errorf("save session: %w", err)}
	}

	fmt.Printf("\nRecorded %d traces\n", recorder.TraceCount())
	if recorder.TraceCount() > 0 {
		fmt.Printf("Session saved to %s\n", path)
	} else {
		fmt.Println("No traces recorded")
	}

	if cmdErr != nil {
		return ExitError{Code: command.ProcessState.ExitCode(), Err: fmt.Errorf("command failed: %w", cmdErr)}
	}

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
