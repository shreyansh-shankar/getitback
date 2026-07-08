package vscode

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
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
	"github.com/shreyansh-shankar/getitback/internal/runtime/restoreutil"
)

type VSCodeModule struct{}

func NewModule() *VSCodeModule { return &VSCodeModule{} }

func (m *VSCodeModule) Name() string        { return "vscode" }
func (m *VSCodeModule) Description() string { return "Visual Studio Code configuration and extensions" }

func (m *VSCodeModule) Detect() (bool, error) {
	if restoreutil.CommandExists("code") {
		return true, nil
	}
	home := restoreutil.HomeDir()
	_, err := os.Stat(filepath.Join(home, ".config", "Code"))
	return err == nil, nil
}

func (m *VSCodeModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true, Metadata: make(map[string]any)}

	if ver, err := restoreutil.CheckExecOutput("code", "--version"); err == nil {
		lines := strings.SplitN(ver, "\n", 2)
		result.Version = lines[0]
	}

	home := restoreutil.HomeDir()
	userDir := filepath.Join(home, ".config", "Code", "User")

	for _, name := range []string{"settings.json", "keybindings.json"} {
		path := filepath.Join(userDir, name)
		if info, err := os.Stat(path); err == nil {
			result.Metadata[name[:len(name)-5]] = "yes"
			result.Resources = append(result.Resources, module.Resource{
				Name: name, Path: path, Size: info.Size(), Modified: info.ModTime(), Type: "config",
			})
		}
	}

	snippets := filepath.Join(userDir, "snippets")
	if info, err := os.Stat(snippets); err == nil {
		result.Metadata["snippets"] = "yes"
		result.Resources = append(result.Resources, module.Resource{
			Name: "snippets", Path: snippets, Size: info.Size(), Modified: info.ModTime(), Type: "config",
		})
	}

	if out, err := restoreutil.CheckExecOutput("code", "--list-extensions"); err == nil {
		exts := strings.Fields(out)
		result.Metadata["extensions"] = exts
		var themeCount, langPackCount int
		for _, ext := range exts {
			lower := strings.ToLower(ext)
			if strings.Contains(lower, "theme") {
				themeCount++
			}
			if strings.Contains(lower, "language") || strings.Contains(lower, "lang") || strings.Contains(lower, "spell") {
				langPackCount++
			}
		}
		if themeCount > 0 {
			result.Metadata["themes"] = themeCount
		}
		if langPackCount > 0 {
			result.Metadata["languagePacks"] = langPackCount
		}
	}

	profiles := filepath.Join(userDir, "profiles")
	if info, err := os.Stat(profiles); err == nil && info.IsDir() {
		profileEntries, _ := os.ReadDir(profiles)
		var profileNames []string
		for _, p := range profileEntries {
			if p.IsDir() {
				profileNames = append(profileNames, p.Name())
			}
		}
		if len(profileNames) > 0 {
			result.Metadata["profiles"] = profileNames
		}
	}

	workspaceStorage := filepath.Join(home, ".config", "Code", "User", "workspaceStorage")
	if info, err := os.Stat(workspaceStorage); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(workspaceStorage)
		if len(entries) > 0 {
			result.Metadata["workspaces"] = len(entries)
		}
	}

	return result, nil
}

func (m *VSCodeModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home := restoreutil.HomeDir()
	userDir := filepath.Join(home, ".config", "Code", "User")

	var entries []archive.Entry
	for _, name := range []string{"settings.json", "keybindings.json"} {
		path := filepath.Join(userDir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			entries = append(entries, archive.Entry{
				Source: path, ArchivePath: filepath.Join(".config", "Code", "User", name),
			})
		}
	}

	snippets := filepath.Join(userDir, "snippets")
	if info, err := os.Stat(snippets); err == nil && info.IsDir() {
		entries = append(entries, archive.Entry{
			Source: snippets, ArchivePath: filepath.Join(".config", "Code", "User", "snippets"),
		})
	}

	if out, err := restoreutil.CheckExecOutput("code", "--list-extensions"); err == nil {
		extData, _ := json.Marshal(strings.Fields(out))
		tmpFile := filepath.Join(os.TempDir(), "getitback-vscode-extensions.json")
		os.WriteFile(tmpFile, extData, 0600)
		defer os.Remove(tmpFile)
		entries = append(entries, archive.Entry{
			Source: tmpFile, ArchivePath: ".config/Code/extensions.json",
		})
	}

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), entries)
	if err != nil || snapshot == nil {
		return nil, err
	}
	return &module.BackupResult{
		Module: m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size,
			Checksum: snapshot.Checksum, OriginalSize: snapshot.OriginalSize,
			FileCount: snapshot.FileCount,
		}},
	}, nil
}

func (m *VSCodeModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	return restoreAndInstallExtensions(snap.Path, home)
}

func (m *VSCodeModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *VSCodeModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}, nil
}

// --- Enhanced restore interfaces ---

func (m *VSCodeModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepCommand, Package: "code", Hint: "VS Code CLI"},
	}
}

func (m *VSCodeModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	if restoreutil.CommandExists("code") {
		return nil
	}
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("code")
	}
	return exec.Command("sudo", "snap", "install", "code", "--classic").Run()
}

func (m *VSCodeModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	return os.MkdirAll(filepath.Join(home, ".config", "Code", "User"), 0755)
}

func (m *VSCodeModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("vscode")

	if ver, err := restoreutil.CheckExecOutput("code", "--version"); err == nil {
		lines := strings.SplitN(ver, "\n", 2)
		v.Version(lines[0])
	}
	v.Check(restoreutil.CommandExists("code"), "VS Code CLI available")

	home := restoreutil.HomeDir()
	userDir := filepath.Join(home, ".config", "Code", "User")
	dirExists := restoreutil.DirExists(userDir)
	v.Check(dirExists, "VS Code user data directory exists")

	if dirExists {
		for _, name := range []string{"settings.json", "keybindings.json"} {
			path := filepath.Join(userDir, name)
			if restoreutil.FileExists(path) {
				v.Recovered(name)
			} else {
				v.Warn("missing: %s", name)
			}
		}
		if restoreutil.DirExists(filepath.Join(userDir, "snippets")) {
			v.Recovered("snippets directory")
		}
	}

	if out, err := restoreutil.CheckExecOutput("code", "--list-extensions"); err == nil {
		count := len(strings.Fields(out))
		v.Recovered(fmt.Sprintf("%d extensions", count))
	}
	v.Confidence(80)
	return v.Result(), nil
}

func (m *VSCodeModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()

	var actionsList []actions.Action

	actionsList = append(actionsList,
		&actions.CreateDirectory{Path: filepath.Join(home, ".config", "Code", "User"), Mode: 0755},
	)

	actionsList = append(actionsList, &restoreUtilAction{
		name: "vscode_extract_configs",
		desc: "Extract VS Code settings and keybindings",
		fn: func(ctx *runtime.RestoreContext) error {
			tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-vscode-*")
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
		},
	})

	actionsList = append(actionsList, &restoreUtilAction{
		name: "vscode_install_extensions",
		desc: "Install VS Code extensions from backup",
		fn: func(ctx *runtime.RestoreContext) error {
			tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-extensions-*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)

			if err := archive.Extract(snap.Path, tmpDir); err != nil {
				return fmt.Errorf("extract: %w", err)
			}

			extFile := filepath.Join(tmpDir, ".config", "Code", "extensions.json")
			data, err := os.ReadFile(extFile)
			if err != nil {
				return nil
			}

			var exts []string
			if err := json.Unmarshal(data, &exts); err != nil {
				return nil
			}

			for _, ext := range exts {
				exec.Command("code", "--install-extension", ext).Run()
			}
			return nil
		},
	})

	return actionsList, nil
}

func restoreAndInstallExtensions(snapshotPath, home string) error {
	tmpDir, err := os.MkdirTemp("", "getitback-restore-vscode-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snapshotPath, tmpDir); err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	if err := filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
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
	}); err != nil {
		return err
	}

	extFile := filepath.Join(tmpDir, ".config", "Code", "extensions.json")
	if data, err := os.ReadFile(extFile); err == nil {
		var exts []string
		if json.Unmarshal(data, &exts) == nil {
			for _, ext := range exts {
				exec.Command("code", "--install-extension", ext).Run()
			}
		}
	}
	return nil
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

var _ actions.Provider = (*VSCodeModule)(nil)
