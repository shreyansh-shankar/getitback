package firefox

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

type FirefoxModule struct{}

func NewModule() *FirefoxModule { return &FirefoxModule{} }

func (m *FirefoxModule) Name() string        { return "firefox" }
func (m *FirefoxModule) Description() string { return "Firefox browser profiles and configuration" }

func (m *FirefoxModule) Detect() (bool, error) {
	cfg := browserutil.BrowserConfig{
		Name:      "firefox",
		Binaries:  []string{"firefox"},
		SnapName:  "firefox",
		FlatpakID: "org.mozilla.firefox",
		ConfigDir: "~/.mozilla/firefox",
	}
	info := browserutil.DetectInstallation(cfg)
	return info != nil, nil
}

func (m *FirefoxModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	cfg := browserutil.BrowserConfig{
		Name:      "firefox",
		Binaries:  []string{"firefox"},
		SnapName:  "firefox",
		FlatpakID: "org.mozilla.firefox",
		ConfigDir: "~/.mozilla/firefox",
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

	prof := browserutil.DetectFirefoxProfiles()
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
		result.Warnings = append(result.Warnings, "Firefox is installed but no profiles have been created yet")
	}

	result.Metadata = meta
	return result, nil
}

func (m *FirefoxModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	prof := browserutil.DetectFirefoxProfiles()
	if prof.Count == 0 {
		return nil, nil
	}

	var entries []archive.Entry
	for _, p := range prof.Profiles {
		archiveRel := ".mozilla/firefox/" + filepath.Base(p.Path)
		entries = append(entries, archive.Entry{
			Source: p.Path, ArchivePath: archiveRel,
		})
	}

	profilesDir := filepath.Dir(prof.Profiles[0].Path)
	iniPath := filepath.Join(profilesDir, "profiles.ini")
	if _, err := os.Stat(iniPath); err == nil {
		entries = append(entries, archive.Entry{
			Source: iniPath, ArchivePath: ".mozilla/firefox/profiles.ini",
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
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
	}, nil
}

func (m *FirefoxModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp("", "getitback-restore-firefox-*")
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

func (m *FirefoxModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *FirefoxModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *FirefoxModule) Dependencies(ctx context.Context) []module.Dependency {
	return nil
}

func (m *FirefoxModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *FirefoxModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	return os.MkdirAll(filepath.Join(home, ".mozilla", "firefox"), 0755)
}

func (m *FirefoxModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation(m.Name())

	home := restoreutil.HomeDir()
	firefoxDir := filepath.Join(home, ".mozilla", "firefox")

	if restoreutil.DirExists(firefoxDir) {
		v.Recovered("Firefox config directory")

		prof := browserutil.DetectFirefoxProfiles()
		if prof.Available && prof.Count > 0 {
			for _, p := range prof.Profiles {
				for _, name := range []string{"places.sqlite", "prefs.js", "bookmarksbackup"} {
					if restoreutil.FileExists(filepath.Join(p.Path, name)) {
						v.Recovered(fmt.Sprintf("%s in %s", name, p.Name))
					} else {
						v.Warn("missing %s in %s", name, p.Name)
					}
				}
			}
		}

		if restoreutil.FileExists(filepath.Join(firefoxDir, "profiles.ini")) {
			v.Recovered("profiles.ini")
		} else {
			v.Warn("missing profiles.ini")
		}
	} else {
		v.Error("Firefox config directory not found")
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *FirefoxModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()

	return []actions.Action{
		&restoreUtilAction{
			name: "firefox_backup_config",
			desc: "Backup existing Firefox configuration",
			fn: func(ctx *runtime.RestoreContext) error {
				firefoxDir := filepath.Join(home, ".mozilla", "firefox")
				if !restoreutil.DirExists(firefoxDir) {
					return nil
				}
				return filepath.Walk(firefoxDir, func(path string, info os.FileInfo, err error) error {
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

var _ actions.Provider = (*FirefoxModule)(nil)
