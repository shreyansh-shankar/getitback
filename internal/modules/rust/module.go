package rust

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
)

type RustModule struct{}

func NewModule() *RustModule { return &RustModule{} }

func (m *RustModule) Name() string        { return "rust" }
func (m *RustModule) Description() string { return "Rust toolchain and cargo packages" }

func (m *RustModule) Detect() (bool, error) {
	_, err := exec.LookPath("rustc")
	return err == nil, nil
}

func (m *RustModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}
	home, _ := os.UserHomeDir()

	if ver, err := exec.Command("rustc", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	meta := make(map[string]any)

	if ver, err := exec.Command("cargo", "--version").Output(); err == nil {
		meta["cargo"] = strings.TrimSpace(string(ver))
	}

	if out, err := exec.Command("cargo", "install", "--list").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		var tools []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) > 0 {
				tools = append(tools, parts[0])
			}
		}
		meta["installedTools"] = tools
	}

	cargoConfig := filepath.Join(home, ".cargo", "config.toml")
	if info, err := os.Stat(cargoConfig); err == nil {
		result.Resources = append(result.Resources, module.Resource{
			Name: "config.toml", Path: cargoConfig, Size: info.Size(), Modified: info.ModTime(), Type: "config",
		})
	}

	cargoCache := filepath.Join(home, ".cargo", "registry", "cache")
	if info, err := os.Stat(cargoCache); err == nil && info.IsDir() {
		var cacheSize int64
		filepath.Walk(cargoCache, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			cacheSize += info.Size()
			return nil
		})
		if cacheSize > 0 {
			meta["cargoCache"] = cacheSize
		}
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}

	return result, nil
}

func (m *RustModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()
	var entries []archive.Entry

	cargoConfig := filepath.Join(home, ".cargo", "config.toml")
	if _, err := os.Stat(cargoConfig); err == nil {
		entries = append(entries, archive.Entry{
			Source: cargoConfig, ArchivePath: ".cargo/config.toml",
		})
	}

	if out, err := exec.Command("cargo", "install", "--list").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		var tools []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) > 0 {
				tools = append(tools, parts[0])
			}
		}
		if len(tools) > 0 {
			data, _ := json.Marshal(tools)
			tmpFile := filepath.Join(os.TempDir(), "getitback-rust-tools.json")
			os.WriteFile(tmpFile, data, 0600)
			defer os.Remove(tmpFile)
			entries = append(entries, archive.Entry{
				Source: tmpFile, ArchivePath: "cargo-tools.json",
			})
		}
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

func (m *RustModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp("", "getitback-restore-rust-*")
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

func (m *RustModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *RustModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}
