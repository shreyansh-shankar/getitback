package chromium

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

type ChromiumModule struct{}

func NewModule() *ChromiumModule { return &ChromiumModule{} }

func (m *ChromiumModule) Name() string        { return "chromium" }
func (m *ChromiumModule) Description() string { return "Chromium browser profiles" }

func (m *ChromiumModule) Detect() (bool, error) {
	cfg := browserutil.BrowserConfig{
		Name:      "chromium-browser",
		Binaries:  []string{"chromium", "chromium-browser"},
		SnapName:  "chromium",
		FlatpakID: "org.chromium.Chromium",
		ConfigDir: "~/.config/chromium",
	}
	return browserutil.DetectInstallation(cfg) != nil, nil
}

func (m *ChromiumModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	cfg := browserutil.BrowserConfig{
		Name:      "chromium-browser",
		Binaries:  []string{"chromium", "chromium-browser"},
		SnapName:  "chromium",
		FlatpakID: "org.chromium.Chromium",
		ConfigDir: "~/.config/chromium",
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

	prof := browserutil.DetectChromeProfiles("~/.config/chromium")
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
		result.Warnings = append(result.Warnings, "Chromium is installed but no profiles have been created yet")
	}

	result.Metadata = meta
	return result, nil
}

func (m *ChromiumModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()
	basePath := filepath.Join(home, ".config", "chromium")
	if _, err := os.Stat(basePath); err != nil {
		return nil, nil
	}
	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: basePath, ArchivePath: ".config/chromium"},
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

func (m *ChromiumModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp("", "getitback-restore-chromium-*")
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

func (m *ChromiumModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *ChromiumModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *ChromiumModule) Dependencies(ctx context.Context) []module.Dependency {
	return nil
}

func (m *ChromiumModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *ChromiumModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	return os.MkdirAll(filepath.Join(home, ".config", "chromium"), 0755)
}

func (m *ChromiumModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation(m.Name())

	home := restoreutil.HomeDir()
	chromiumDir := filepath.Join(home, ".config", "chromium")

	if restoreutil.DirExists(chromiumDir) {
		v.Recovered("Chromium config directory")
		for _, name := range []string{"Bookmarks", "Preferences", "History", "Cookies", "Login Data"} {
			if restoreutil.FileExists(filepath.Join(chromiumDir, "Default", name)) {
				v.Recovered(name)
			} else {
				v.Warn("missing %s in Default profile", name)
			}
		}
	} else {
		v.Error("Chromium config directory not found")
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *ChromiumModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()

	return []actions.Action{
		&restoreUtilAction{
			name: "chromium_backup_config",
			desc: "Backup existing Chromium configuration",
			fn: func(ctx *runtime.RestoreContext) error {
				chromiumDir := filepath.Join(home, ".config", "chromium")
				if !restoreutil.DirExists(chromiumDir) {
					return nil
				}
				return filepath.Walk(chromiumDir, func(path string, info os.FileInfo, err error) error {
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

var _ actions.Provider = (*ChromiumModule)(nil)
