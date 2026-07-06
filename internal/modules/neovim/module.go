package neovim

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

type NeovimModule struct{}

func NewModule() *NeovimModule { return &NeovimModule{} }

func (m *NeovimModule) Name() string        { return "neovim" }
func (m *NeovimModule) Description() string { return "Neovim editor configuration" }

func (m *NeovimModule) Detect() (bool, error) {
	if _, err := exec.LookPath("nvim"); err == nil {
		return true, nil
	}
	home, _ := os.UserHomeDir()
	_, err := os.Stat(filepath.Join(home, ".config", "nvim"))
	return err == nil, nil
}

func (m *NeovimModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := exec.Command("nvim", "--version").Output(); err == nil {
		firstLine := string(ver)
		if idx := strings.Index(firstLine, "\n"); idx > 0 {
			firstLine = firstLine[:idx]
		}
		result.Version = firstLine
	}

	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "nvim")

	if info, err := os.Stat(configDir); err == nil {
		result.Resources = append(result.Resources, module.Resource{
			Name: "nvim", Path: configDir, Size: info.Size(), Type: "config",
		})
	}

	legacyDir := filepath.Join(home, ".vim")
	if info, err := os.Stat(legacyDir); err == nil {
		result.Resources = append(result.Resources, module.Resource{
			Name: ".vim", Path: legacyDir, Size: info.Size(), Type: "config",
		})
	}

	return result, nil
}

func (m *NeovimModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()

	var entries []archive.Entry

	configDir := filepath.Join(home, ".config", "nvim")
	if info, err := os.Stat(configDir); err == nil && info.IsDir() {
		entries = append(entries, archive.Entry{
			Source: configDir, ArchivePath: ".config/nvim",
		})
	}

	legacyDir := filepath.Join(home, ".vim")
	if info, err := os.Stat(legacyDir); err == nil && info.IsDir() {
		entries = append(entries, archive.Entry{
			Source: legacyDir, ArchivePath: ".vim",
		})
	}

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), entries)
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

func (m *NeovimModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp("", "getitback-restore-neovim-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	return filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(tmpDir, path)
		dst := filepath.Join(home, rel)
		if _, err := os.Stat(dst); err == nil {
			os.Rename(dst, dst+".getitback-bak")
		}
		os.MkdirAll(filepath.Dir(dst), 0755)
		data, _ := os.ReadFile(path)
		return os.WriteFile(dst, data, 0644)
	})
}

func (m *NeovimModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *NeovimModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}
