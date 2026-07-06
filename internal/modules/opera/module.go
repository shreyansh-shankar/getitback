package opera

import (
	"context"
	"os"
	"path/filepath"

	"fmt"
	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/modules/browserutil"
)

type OperaModule struct{}

func NewModule() *OperaModule { return &OperaModule{} }

func (m *OperaModule) Name() string        { return "opera" }
func (m *OperaModule) Description() string { return "Opera browser" }

func (m *OperaModule) Detect() (bool, error) {
	cfg := browserutil.BrowserConfig{
		Name:      "opera-stable",
		Binaries:  []string{"opera"},
		FlatpakID: "com.opera.Opera",
		ConfigDir: "~/.config/opera",
	}
	return browserutil.DetectInstallation(cfg) != nil, nil
}

func (m *OperaModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	cfg := browserutil.BrowserConfig{
		Name:      "opera-stable",
		Binaries:  []string{"opera"},
		FlatpakID: "com.opera.Opera",
		ConfigDir: "~/.config/opera",
	}
	install := browserutil.DetectInstallation(cfg)
	if install == nil {
		return result, nil
	}

	meta := make(map[string]any)
	meta["installMethod"] = string(install.Method)

	if install.Version != "" {
		result.Version = install.Version
	}

	prof := browserutil.DetectChromeProfiles("~/.config/opera")
	meta["profileCount"] = prof.Count

	if prof.Default != "" {
		meta["defaultProfile"] = prof.Default
	}

	if prof.Available && prof.Count > 0 {
		for _, p := range prof.Profiles {
			size := browserutil.DirSize(p.Path)
			result.Resources = append(result.Resources, module.Resource{
				Name: p.Name, Path: p.Path, Size: size, Type: module.ResourceTypeData,
			})
		}
	} else if prof.Available && prof.Count == 0 {
		result.Warnings = append(result.Warnings, "Opera is installed but no profiles have been created yet")
	}

	result.Metadata = meta
	return result, nil
}

func (m *OperaModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()
	basePath := filepath.Join(home, ".config", "opera")
	if _, err := os.Stat(basePath); err != nil {
		return nil, nil
	}
	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: basePath, ArchivePath: ".config/opera"},
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

func (m *OperaModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp("", "getitback-restore-opera-*")
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

func (m *OperaModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *OperaModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}
