package certs

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

type CertsModule struct{}

func NewModule() *CertsModule { return &CertsModule{} }

func (m *CertsModule) Name() string        { return "certs" }
func (m *CertsModule) Description() string { return "TLS/SSL certificates and credential stores" }

var certSearchDirs = []string{
	"/etc/ssl/certs",
	"/etc/pki/tls/certs",
	"/usr/local/share/ca-certificates",
	"/usr/share/ca-certificates",
}

var sshCertDirs = []string{
	"~/.ssh",
}

func (m *CertsModule) Detect() (bool, error) {
	for _, dir := range certSearchDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return true, nil
		}
	}
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	if entries, err := os.ReadDir(sshDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), "-cert.pub") {
				return true, nil
			}
		}
	}
	return false, nil
}

func (m *CertsModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}
	meta := make(map[string]any)

	var certFiles []string
	var expiredCount int
	var expiringCount int
	var validCount int

	for _, dir := range certSearchDirs {
		expanded := strings.Replace(dir, "~", func() string { h, _ := os.UserHomeDir(); return h }(), 1)
		entries, err := os.ReadDir(expanded)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			path := filepath.Join(expanded, entry.Name())
			certFiles = append(certFiles, path)

			expired, expiring, err := checkCertExpiry(path)
			if err != nil {
				continue
			}
			if expired {
				expiredCount++
			} else if expiring {
				expiringCount++
			} else {
				validCount++
			}
		}
	}

	var sshCerts []string
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	if entries, err := os.ReadDir(sshDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), "-cert.pub") {
				path := filepath.Join(sshDir, e.Name())
				sshCerts = append(sshCerts, path)
				expired, expiring, err := checkSSHCertExpiry(path)
				if err != nil {
					continue
				}
				if expired {
					expiredCount++
				} else if expiring {
					expiringCount++
				} else {
					validCount++
				}
			}
		}
	}

	meta["certificateStores"] = int64(len(certSearchDirs))
	meta["certificateFiles"] = int64(len(certFiles) + len(sshCerts))
	meta["validCerts"] = validCount
	meta["expiringCerts"] = expiringCount
	meta["expiredCerts"] = expiredCount

	if expiredCount > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d expired certificates found", expiredCount))
	}
	if expiringCount > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d certificates expiring within 30 days", expiringCount))
	}

	if customCA := findCustomCABundles(); len(customCA) > 0 {
		meta["customCABundles"] = customCA
		for _, ca := range customCA {
			result.Resources = append(result.Resources, module.Resource{
				Name: filepath.Base(ca), Path: ca, Type: module.ResourceTypeConfig,
			})
		}
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}
	return result, nil
}

func checkCertExpiry(path string) (expired bool, expiring bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return false, false, fmt.Errorf("not a PEM file")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, false, err
	}
	now := time.Now()
	if now.After(cert.NotAfter) {
		return true, false, nil
	}
	if cert.NotAfter.Sub(now) < 30*24*time.Hour {
		return false, true, nil
	}
	return false, false, nil
}

func checkSSHCertExpiry(path string) (expired bool, expiring bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, false, err
	}
	for len(data) > 0 {
		block, rest := pem.Decode(data)
		if block == nil {
			break
		}
		data = rest
		if block.Type == "SSH CERTIFICATE" || strings.Contains(block.Type, "CERTIFICATE") {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				continue
			}
			now := time.Now()
			if now.After(cert.NotAfter) {
				return true, false, nil
			}
			if cert.NotAfter.Sub(now) < 30*24*time.Hour {
				return false, true, nil
			}
			return false, false, nil
		}
	}
	return false, false, fmt.Errorf("no certificate found")
}

func findCustomCABundles() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{
		home + "/.ssh",
		home + "/.config",
		"/usr/local/share/ca-certificates",
	}
	var bundles []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if strings.Contains(name, "ca-bundle") || strings.Contains(name, "ca-cert") || strings.HasSuffix(name, ".crt") {
				bundles = append(bundles, filepath.Join(dir, e.Name()))
			}
		}
	}
	return bundles
}

func (m *CertsModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	return nil, nil
}

func (m *CertsModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	return nil
}

func (m *CertsModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *CertsModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}
	for _, dir := range certSearchDirs {
		expanded := strings.Replace(dir, "~", func() string { h, _ := os.UserHomeDir(); return h }(), 1)
		entries, _ := os.ReadDir(expanded)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			expired, expiring, _ := checkCertExpiry(filepath.Join(expanded, entry.Name()))
			if expired {
				result.Issues = append(result.Issues, module.DoctorIssue{
					Severity: "warning",
					Message:  fmt.Sprintf("Certificate %s has expired", entry.Name()),
					Help:     "Renew the certificate or remove the expired file",
				})
			} else if expiring {
				result.Issues = append(result.Issues, module.DoctorIssue{
					Severity: "info",
					Message:  fmt.Sprintf("Certificate %s expires within 30 days", entry.Name()),
					Help:     "Renew the certificate before it expires",
				})
			}
		}
	}

	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	if entries, err := os.ReadDir(sshDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), "-cert.pub") {
				expired, expiring, _ := checkSSHCertExpiry(filepath.Join(sshDir, e.Name()))
				if expired {
					result.Issues = append(result.Issues, module.DoctorIssue{
						Severity: "warning",
						Message:  fmt.Sprintf("SSH certificate %s has expired", e.Name()),
						Help:     "Request a new SSH certificate from your CA",
					})
				} else if expiring {
					result.Issues = append(result.Issues, module.DoctorIssue{
						Severity: "info",
						Message:  fmt.Sprintf("SSH certificate %s expires within 30 days", e.Name()),
						Help:     "Request a new SSH certificate before expiry",
					})
				}
			}
		}
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}
