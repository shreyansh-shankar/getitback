package edge

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/modules/browserutil"
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
	"github.com/shreyansh-shankar/getitback/internal/runtime/restoreutil"
)

type EdgeModule struct{}

func NewModule() *EdgeModule { return &EdgeModule{} }

func (m *EdgeModule) Name() string        { return "edge" }
func (m *EdgeModule) Description() string { return "Microsoft Edge browser" }

func (m *EdgeModule) Detect() (bool, error) {
	cfg := browserutil.BrowserConfig{
		Name:      "microsoft-edge-stable",
		Binaries:  []string{"microsoft-edge", "microsoft-edge-stable"},
		FlatpakID: "com.microsoft.Edge",
		ConfigDir: "~/.config/microsoft-edge",
	}
	return browserutil.DetectInstallation(cfg) != nil, nil
}

func (m *EdgeModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	cfg := browserutil.BrowserConfig{
		Name:      "microsoft-edge-stable",
		Binaries:  []string{"microsoft-edge", "microsoft-edge-stable"},
		FlatpakID: "com.microsoft.Edge",
		ConfigDir: "~/.config/microsoft-edge",
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

	prof := browserutil.DetectChromeProfiles("~/.config/microsoft-edge")
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
		result.Warnings = append(result.Warnings, "Edge is installed but no profiles have been created yet")
	}

	result.Metadata = meta
	return result, nil
}

func (m *EdgeModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()
	basePath := filepath.Join(home, ".config", "microsoft-edge")
	if _, err := os.Stat(basePath); err != nil {
		return nil, nil
	}
	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: basePath, ArchivePath: ".config/microsoft-edge"},
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
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
	}, nil
}

func (m *EdgeModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-edge-*")
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

func (m *EdgeModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *EdgeModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *EdgeModule) Dependencies(ctx context.Context) []module.Dependency {
	return nil
}

func (m *EdgeModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *EdgeModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	return os.MkdirAll(filepath.Join(home, ".config", "microsoft-edge"), 0755)
}

func (m *EdgeModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation(m.Name())

	home := restoreutil.HomeDir()
	edgeDir := filepath.Join(home, ".config", "microsoft-edge")

	if restoreutil.DirExists(edgeDir) {
		v.Recovered("Edge config directory")
		for _, name := range []string{"Bookmarks", "Preferences", "History", "Cookies", "Login Data"} {
			if restoreutil.FileExists(filepath.Join(edgeDir, "Default", name)) {
				v.Recovered(name)
			} else {
				v.Warn("missing %s in Default profile", name)
			}
		}
	} else {
		v.Error("Edge config directory not found")
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *EdgeModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()

	return []actions.Action{
		&restoreUtilAction{
			name: "edge_backup_config",
			desc: "Backup existing Edge configuration",
			fn: func(ctx *runtime.RestoreContext) error {
				edgeDir := filepath.Join(home, ".config", "microsoft-edge")
				if !restoreutil.DirExists(edgeDir) {
					return nil
				}
				return filepath.Walk(edgeDir, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return err
					}
					bakPath := path + ".getitback-bak"
					if _, err := os.Stat(bakPath); os.IsNotExist(err) {
						return os.Rename(path, bakPath)
					}
					return nil
				})
			},
		},
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
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

var _ actions.Provider = (*EdgeModule)(nil)
