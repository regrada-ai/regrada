package cmd

import (
	"fmt"
	"os"

	"github.com/regrada-ai/regrada/internal/ca"
	"github.com/regrada-ai/regrada/internal/config"
	"github.com/spf13/cobra"
)

var caConfigPath string

var caCmd = &cobra.Command{
	Use:   "ca",
	Short: "Manage CA certificate for HTTPS MITM",
	Long: `Manage the Regrada Root CA certificate for HTTPS interception.

Required for forward proxy mode with HTTPS MITM.`,
}

var caInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a new CA certificate",
	Long: `Generate a new Regrada Root CA certificate for HTTPS interception.

This creates a self-signed CA certificate that will be used to MITM HTTPS traffic
through the forward proxy. The certificate is saved to .regrada/ca/ by default.`,
	RunE: runCAInit,
}

var caInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install CA into OS trust store",
	Long: `Install the Regrada Root CA into the operating system's trust store.

This command requires sudo/administrator privileges and will:
- macOS: Add certificate to System keychain
- Linux: Add to /usr/local/share/ca-certificates or /etc/pki/ca-trust
- Windows: Add to Root certificate store

SECURITY NOTE: The certificate is only used by the Regrada proxy when explicitly
running 'regrada record'. It does NOT intercept any traffic unless:
1. The Regrada proxy is actively running
2. Your application is configured to use the proxy (via HTTP_PROXY env vars)

The certificate sits dormant in your trust store and is only used when you explicitly
proxy traffic through Regrada.`,
	RunE: runCAInstall,
}

var caUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove CA from OS trust store",
	Long: `Remove the Regrada Root CA from the operating system's trust store.

This will untrust the Regrada CA certificate. HTTPS traffic through the proxy
will no longer be trusted after running this command.`,
	RunE: runCAUninstall,
}

var caStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check CA certificate status",
	Long: `Check if the Regrada CA certificate exists and display its details.

Shows certificate information including validity period and file locations.`,
	RunE: runCAStatus,
}

func init() {
	rootCmd.AddCommand(caCmd)
	caCmd.AddCommand(caInitCmd)
	caCmd.AddCommand(caInstallCmd)
	caCmd.AddCommand(caUninstallCmd)
	caCmd.AddCommand(caStatusCmd)

	caCmd.PersistentFlags().StringVarP(&caConfigPath, "config", "c", "", "Path to config file (default: regrada.yml/regrada.yaml)")
}

func runCAInit(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadProjectConfig(caConfigPath)
	if err != nil {
		return ExitError{Code: 3, Err: err}
	}

	caPath := cfg.Capture.Proxy.CAPath

	if ca.Exists(caPath) {
		fmt.Printf("CA certificate already exists at %s\n", caPath)
		fmt.Println("To regenerate, delete the existing certificate first:")
		fmt.Printf("  rm -rf %s\n", caPath)
		return nil
	}

	fmt.Printf("Generating CA certificate at %s...\n", caPath)
	caObj, err := ca.Generate(caPath)
	if err != nil {
		return ExitError{Code: 1, Err: fmt.Errorf("generate CA: %w", err)}
	}

	fmt.Println("✓ CA certificate generated successfully")
	fmt.Printf("  Certificate: %s\n", caObj.CertPath())
	fmt.Printf("  Private Key: %s\n", caObj.KeyPath())
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  Local development:")
	fmt.Println("    1. Install the CA: regrada ca install")
	fmt.Println("    2. Start recording: regrada record -- <your-command>")
	fmt.Println()
	fmt.Println("  CI environments:")
	fmt.Println("    Skip 'regrada ca install' - the cert will be automatically")
	fmt.Println("    configured via environment variables when you run 'regrada record'.")

	return nil
}

func runCAInstall(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadProjectConfig(caConfigPath)
	if err != nil {
		return ExitError{Code: 3, Err: err}
	}

	caPath := cfg.Capture.Proxy.CAPath

	fmt.Println("Installing CA certificate into OS trust store...")
	fmt.Println("This requires sudo/administrator privileges.")
	fmt.Println()

	if err := ca.Install(caPath); err != nil {
		return ExitError{Code: 1, Err: fmt.Errorf("install CA: %w", err)}
	}

	fmt.Println("✓ CA certificate installed successfully")
	fmt.Println()
	fmt.Println("The certificate is now trusted by your system.")
	fmt.Println()
	fmt.Println("IMPORTANT: The certificate ONLY intercepts traffic when:")
	fmt.Println("  - You run 'regrada record -- <command>'")
	fmt.Println("  - The command's HTTP_PROXY env vars point to the Regrada proxy")
	fmt.Println()
	fmt.Println("Normal system traffic is NOT affected by this certificate.")
	fmt.Println("To remove: regrada ca uninstall")

	return nil
}

func runCAUninstall(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadProjectConfig(caConfigPath)
	if err != nil {
		return ExitError{Code: 3, Err: err}
	}

	caPath := cfg.Capture.Proxy.CAPath

	fmt.Println("Removing CA certificate from OS trust store...")
	fmt.Println("This requires sudo/administrator privileges.")
	fmt.Println()

	if err := ca.Uninstall(caPath); err != nil {
		return ExitError{Code: 1, Err: fmt.Errorf("uninstall CA: %w", err)}
	}

	fmt.Println("✓ CA certificate removed successfully")

	return nil
}

func runCAStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadProjectConfig(caConfigPath)
	if err != nil {
		return ExitError{Code: 3, Err: err}
	}

	caPath := cfg.Capture.Proxy.CAPath

	fmt.Printf("CA Path: %s\n", caPath)

	if ca.Exists(caPath) {
		fmt.Println("Status: ✓ CA certificate exists")

		caObj, err := ca.Load(caPath)
		if err != nil {
			fmt.Printf("Error loading CA: %v\n", err)
			return ExitError{Code: 1, Err: err}
		}

		cert := caObj.Cert()
		fmt.Printf("Subject: %s\n", cert.Subject.CommonName)
		fmt.Printf("Valid From: %s\n", cert.NotBefore.Format("2006-01-02"))
		fmt.Printf("Valid Until: %s\n", cert.NotAfter.Format("2006-01-02"))
		fmt.Printf("Serial: %s\n", cert.SerialNumber.String())
		fmt.Println()
		fmt.Println("Certificate file:", caObj.CertPath())
		fmt.Println("Private key file:", caObj.KeyPath())
	} else {
		fmt.Println("Status: ✗ CA certificate not found")
		fmt.Println()
		fmt.Println("Run 'regrada ca init' to generate a CA certificate")
		return ExitError{Code: 1, Err: fmt.Errorf("CA not found")}
	}

	return nil
}

// EnsureCA checks if CA exists and provides helpful error if not
func EnsureCA(caPath string) error {
	if !ca.Exists(caPath) {
		fmt.Fprintln(os.Stderr, "Error: CA certificate not found")
		fmt.Fprintf(os.Stderr, "Run the following commands to set up the CA:\n\n")
		fmt.Fprintf(os.Stderr, "  regrada ca init      # Generate CA certificate\n")
		fmt.Fprintf(os.Stderr, "  regrada ca install   # Install into OS trust store (requires sudo)\n\n")
		return fmt.Errorf("CA not found at %s", caPath)
	}
	return nil
}
