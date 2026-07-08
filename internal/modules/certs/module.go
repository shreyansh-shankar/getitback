package certs

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
	"github.com/shreyansh-shankar/getitback/internal/runtime/restoreutil"
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

type certBackupManifest struct {
	SystemDirs      []string `json:"systemCertDirs"`
	CustomCABundles []string `json:"customCABundles,omitempty"`
	SSHCertFiles    []string `json:"sshCertFiles,omitempty"`
	ClientCerts     []string `json:"clientCerts,omitempty"`
	TotalFiles      int      `json:"totalFiles"`
}

func (m *CertsModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	var manifest certBackupManifest
	var entries []archive.Entry
	home, _ := os.UserHomeDir()

	for _, dir := range certSearchDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			manifest.SystemDirs = append(manifest.SystemDirs, dir)
			entries = append(entries, archive.Entry{
				Source: dir, ArchivePath: "system/" + filepath.Base(dir),
			})
		}
	}

	sshDir := filepath.Join(home, ".ssh")
	if entries2, err := os.ReadDir(sshDir); err == nil {
		for _, e := range entries2 {
			if strings.HasSuffix(e.Name(), "-cert.pub") {
				path := filepath.Join(sshDir, e.Name())
				manifest.SSHCertFiles = append(manifest.SSHCertFiles, e.Name())
				entries = append(entries, archive.Entry{
					Source: path, ArchivePath: "ssh-certs/" + e.Name(),
				})
			}
		}
	}

	customCAs := findCustomCABundles()
	for _, ca := range customCAs {
		manifest.CustomCABundles = append(manifest.CustomCABundles, ca)
		rel := strings.TrimPrefix(ca, home)
		rel = strings.TrimPrefix(rel, "/")
		entries = append(entries, archive.Entry{
			Source: ca, ArchivePath: "custom/" + rel,
		})
	}

	clientCertDirs := []string{
		filepath.Join(home, ".config", "ssl"),
		filepath.Join(home, ".ssl"),
	}
	for _, dir := range clientCertDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			entries2, _ := os.ReadDir(dir)
			for _, e := range entries2 {
				if e.IsDir() {
					continue
				}
				name := strings.ToLower(e.Name())
				if strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".crt") || strings.HasSuffix(name, ".cert") {
					path := filepath.Join(dir, e.Name())
					manifest.ClientCerts = append(manifest.ClientCerts, e.Name())
					entries = append(entries, archive.Entry{
						Source: path, ArchivePath: "client/" + e.Name(),
					})
				}
			}
		}
	}

	manifest.TotalFiles = len(manifest.SystemDirs) + len(manifest.SSHCertFiles) +
		len(manifest.CustomCABundles) + len(manifest.ClientCerts)

	tmpMeta, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("certs: marshal manifest: %w", err)
	}
	metaFile := filepath.Join(os.TempDir(), "getitback-certs-manifest.json")
	if err := os.WriteFile(metaFile, tmpMeta, 0600); err != nil {
		return nil, fmt.Errorf("certs: write manifest: %w", err)
	}
	defer os.Remove(metaFile)
	entries = append(entries, archive.Entry{
		Source: metaFile, ArchivePath: "manifest.json",
	})

	if len(entries) == 0 {
		return nil, nil
	}

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), entries)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	contents := []string{}
	if len(manifest.SystemDirs) > 0 {
		contents = append(contents, fmt.Sprintf("system cert stores (%d)", len(manifest.SystemDirs)))
	}
	if len(manifest.SSHCertFiles) > 0 {
		contents = append(contents, fmt.Sprintf("SSH certificates (%d)", len(manifest.SSHCertFiles)))
	}
	if len(manifest.CustomCABundles) > 0 {
		contents = append(contents, fmt.Sprintf("custom CA bundles (%d)", len(manifest.CustomCABundles)))
	}
	if len(manifest.ClientCerts) > 0 {
		contents = append(contents, fmt.Sprintf("client certificates (%d)", len(manifest.ClientCerts)))
	}
	return &module.BackupResult{
		Module:    m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size, Checksum: snapshot.Checksum,
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
		Contents: contents,
	}, nil
}

func (m *CertsModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-certs-*")
	if err != nil {
		return fmt.Errorf("certs: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("certs: extract snapshot: %w", err)
	}

	restoreDir := func(src, dst string) {
		if info, err := os.Stat(src); err == nil && info.IsDir() {
			os.MkdirAll(filepath.Dir(dst), 0755)
			exec.Command("cp", "-r", src+"/.", dst).Run()
		}
	}

	restoreDir(filepath.Join(tmpDir, "ssh-certs"), filepath.Join(home, ".ssh"))
	restoreDir(filepath.Join(tmpDir, "custom"), home)

	for _, dir := range []string{filepath.Join(home, ".config", "ssl"), filepath.Join(home, ".ssl")} {
		restoreDir(filepath.Join(tmpDir, "client"), dir)
	}

	return nil
}

func (m *CertsModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
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

func (m *CertsModule) Dependencies(ctx context.Context) []module.Dependency {
	return nil
}

func (m *CertsModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *CertsModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	certDir := filepath.Join(restoreutil.HomeDir(), ".cert")
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		os.MkdirAll(certDir, 0700)
	}
	return nil
}

func (m *CertsModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("certs")

	certDir := filepath.Join(restoreutil.HomeDir(), ".cert")
	if restoreutil.DirExists(certDir) {
		entries, _ := os.ReadDir(certDir)
		if len(entries) > 0 {
			v.Recovered(fmt.Sprintf("%d cert files", len(entries)))
		}
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *CertsModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()
	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
		&restoreUtilAction{
			name: "certs_set_permissions",
			desc: "Set correct permissions on certificate files",
			fn: func(ctx *runtime.RestoreContext) error {
				certDir := filepath.Join(home, ".cert")
				if _, err := os.Stat(certDir); os.IsNotExist(err) {
					return nil
				}
				return filepath.Walk(certDir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() {
						os.Chmod(path, 0600)
					}
					return nil
				})
			},
		},
	}, nil
}

type restoreUtilAction struct {
	actions.BaseAction
	name string
	desc string
	fn   func(ctx *runtime.RestoreContext) error
}

func (a *restoreUtilAction) Name() string        { return a.name }
func (a *restoreUtilAction) Description() string  { return a.desc }
func (a *restoreUtilAction) Execute(ctx *runtime.RestoreContext) error { return a.fn(ctx) }

var _ actions.Provider = (*CertsModule)(nil)
