package ca

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const (
	CertFileName = "regrada-ca.pem"
	KeyFileName  = "regrada-ca-key.pem"
)

// CA represents a certificate authority for MITM proxy
type CA struct {
	cert *x509.Certificate
	key  *rsa.PrivateKey
	path string
}

// Generate creates a new root CA certificate and private key
func Generate(caPath string) (*CA, error) {
	if err := os.MkdirAll(caPath, 0755); err != nil {
		return nil, fmt.Errorf("create CA directory: %w", err)
	}

	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial number: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(10 * 365 * 24 * time.Hour) // 10 years

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Regrada"},
			CommonName:   "Regrada Root CA",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	// Self-sign the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	ca := &CA{
		cert: cert,
		key:  privateKey,
		path: caPath,
	}

	// Save to disk
	if err := ca.Save(); err != nil {
		return nil, err
	}

	return ca, nil
}

// Load loads an existing CA from disk
func Load(caPath string) (*CA, error) {
	certPath := filepath.Join(caPath, CertFileName)
	keyPath := filepath.Join(caPath, KeyFileName)

	// Read certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("read certificate: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	// Read private key
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode private key PEM")
	}

	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return &CA{
		cert: cert,
		key:  key,
		path: caPath,
	}, nil
}

// Save writes the CA certificate and key to disk
func (ca *CA) Save() error {
	certPath := filepath.Join(ca.path, CertFileName)
	keyPath := filepath.Join(ca.path, KeyFileName)

	// Write certificate
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("create cert file: %w", err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.cert.Raw,
	}); err != nil {
		return fmt.Errorf("encode certificate: %w", err)
	}

	// Write private key
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	defer keyFile.Close()

	if err := pem.Encode(keyFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(ca.key),
	}); err != nil {
		return fmt.Errorf("encode private key: %w", err)
	}

	return nil
}

// CertPath returns the path to the certificate file
func (ca *CA) CertPath() string {
	return filepath.Join(ca.path, CertFileName)
}

// KeyPath returns the path to the private key file
func (ca *CA) KeyPath() string {
	return filepath.Join(ca.path, KeyFileName)
}

// Cert returns the certificate
func (ca *CA) Cert() *x509.Certificate {
	return ca.cert
}

// Key returns the private key
func (ca *CA) Key() *rsa.PrivateKey {
	return ca.key
}

// Install installs the CA certificate into the OS trust store
func Install(caPath string) error {
	certPath := filepath.Join(caPath, CertFileName)

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return fmt.Errorf("CA certificate not found at %s. Run 'regrada ca init' first", certPath)
	}

	switch runtime.GOOS {
	case "darwin":
		return installMacOS(certPath)
	case "linux":
		return installLinux(certPath)
	case "windows":
		return installWindows(certPath)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Uninstall removes the CA certificate from the OS trust store
func Uninstall(caPath string) error {
	certPath := filepath.Join(caPath, CertFileName)

	switch runtime.GOOS {
	case "darwin":
		return uninstallMacOS(certPath)
	case "linux":
		return uninstallLinux(certPath)
	case "windows":
		return uninstallWindows(certPath)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func installMacOS(certPath string) error {
	cmd := exec.Command("sudo", "security", "add-trusted-cert",
		"-d", "-r", "trustRoot",
		"-k", "/Library/Keychains/System.keychain",
		certPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install CA (macOS): %w", err)
	}
	return nil
}

func uninstallMacOS(certPath string) error {
	cmd := exec.Command("sudo", "security", "delete-certificate",
		"-c", "Regrada Root CA",
		"/Library/Keychains/System.keychain")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("uninstall CA (macOS): %w", err)
	}
	return nil
}

func installLinux(certPath string) error {
	// Try update-ca-certificates (Debian/Ubuntu)
	destPath := "/usr/local/share/ca-certificates/regrada-ca.crt"
	if err := exec.Command("sudo", "cp", certPath, destPath).Run(); err == nil {
		if err := exec.Command("sudo", "update-ca-certificates").Run(); err == nil {
			return nil
		}
	}

	// Try update-ca-trust (RHEL/CentOS/Fedora)
	destPath = "/etc/pki/ca-trust/source/anchors/regrada-ca.pem"
	if err := exec.Command("sudo", "cp", certPath, destPath).Run(); err == nil {
		if err := exec.Command("sudo", "update-ca-trust").Run(); err == nil {
			return nil
		}
	}

	return fmt.Errorf("could not install CA on Linux (tried update-ca-certificates and update-ca-trust)")
}

func uninstallLinux(certPath string) error {
	// Try update-ca-certificates (Debian/Ubuntu)
	destPath := "/usr/local/share/ca-certificates/regrada-ca.crt"
	if _, err := os.Stat(destPath); err == nil {
		if err := exec.Command("sudo", "rm", destPath).Run(); err == nil {
			_ = exec.Command("sudo", "update-ca-certificates", "--fresh").Run()
			return nil
		}
	}

	// Try update-ca-trust (RHEL/CentOS/Fedora)
	destPath = "/etc/pki/ca-trust/source/anchors/regrada-ca.pem"
	if _, err := os.Stat(destPath); err == nil {
		if err := exec.Command("sudo", "rm", destPath).Run(); err == nil {
			_ = exec.Command("sudo", "update-ca-trust", "extract").Run()
			return nil
		}
	}

	return fmt.Errorf("CA certificate not found in trust store")
}

func installWindows(certPath string) error {
	cmd := exec.Command("certutil", "-addstore", "-f", "Root", certPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install CA (Windows): %w. Try running as Administrator", err)
	}
	return nil
}

func uninstallWindows(certPath string) error {
	cmd := exec.Command("certutil", "-delstore", "Root", "Regrada Root CA")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("uninstall CA (Windows): %w. Try running as Administrator", err)
	}
	return nil
}

// Exists checks if a CA certificate already exists at the given path
func Exists(caPath string) bool {
	certPath := filepath.Join(caPath, CertFileName)
	keyPath := filepath.Join(caPath, KeyFileName)

	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)

	return certErr == nil && keyErr == nil
}
