package gpg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
)

type GPGModule struct{}

func NewModule() *GPGModule { return &GPGModule{} }

func (m *GPGModule) Name() string        { return "gpg" }
func (m *GPGModule) Description() string { return "GnuPG encryption keys and configuration" }

func (m *GPGModule) Detect() (bool, error) {
	_, err := exec.LookPath("gpg")
	return err == nil, nil
}

func (m *GPGModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := exec.Command("gpg", "--version").Output(); err == nil {
		lines := strings.Split(string(ver), "\n")
		if len(lines) > 0 {
			result.Version = lines[0]
		}
	}

	home, _ := os.UserHomeDir()
	gnupgDir := filepath.Join(home, ".gnupg")
	if info, err := os.Stat(gnupgDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(gnupgDir)
		if err == nil {
			for _, entry := range entries {
				info, _ := entry.Info()
				if info == nil {
					continue
				}
				resType := "config"
				if strings.HasPrefix(entry.Name(), "private-keys") || entry.Name() == "secring.gpg" {
					resType = "secret"
				}
				result.Resources = append(result.Resources, module.Resource{
					Name: entry.Name(), Path: filepath.Join(gnupgDir, entry.Name()),
					Size: info.Size(), Modified: info.ModTime(), Type: resType,
				})
			}
		}
	}

	out, err := exec.Command("gpg", "--list-keys", "--keyid-format", "LONG").Output()
	if err == nil {
		var keys []string
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "pub") || strings.HasPrefix(line, "sec") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					keys = append(keys, fields[1])
				}
			}
		}
		result.Metadata = map[string]any{"keys": keys}
	}

	return result, nil
}

func (m *GPGModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()
	gnupgDir := filepath.Join(home, ".gnupg")
	if _, err := os.Stat(gnupgDir); err != nil {
		return nil, nil
	}

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: gnupgDir, ArchivePath: ".gnupg"},
	})
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	return &module.BackupResult{
		Module: m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size, Checksum: snapshot.Checksum,
		}},
	}, nil
}

func (m *GPGModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	gnupgDir := filepath.Join(home, ".gnupg")

	if err := os.MkdirAll(gnupgDir, 0700); err != nil {
		return fmt.Errorf("create .gnupg dir: %w", err)
	}

	if info, err := os.Stat(gnupgDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(gnupgDir)
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".getitback-bak") {
				os.Rename(filepath.Join(gnupgDir, entry.Name()), filepath.Join(gnupgDir, entry.Name()+".getitback-bak"))
			}
		}
	}

	if err := archive.Extract(snap.Path, home); err != nil {
		return fmt.Errorf("extract gpg snapshot: %w", err)
	}

	filepath.Walk(gnupgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		os.Chmod(path, 0600)
		return nil
	})

	return nil
}

func (m *GPGModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *GPGModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}

	out, err := exec.Command("gpg", "--list-keys", "--keyid-format", "LONG").Output()
	if err != nil {
		return result, nil
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "pub") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		algo := fields[2]
		if algo == "RSA" || algo == "rsa1024" || algo == "dsa1024" {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "warning",
				Message:  fmt.Sprintf("key %s uses weak algorithm: %s", fields[1], algo),
				Help:     "Consider generating a new key with ed25519: gpg --full-generate-key",
			})
		}
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}

	return result, nil
}
