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
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
	"github.com/shreyansh-shankar/getitback/internal/runtime/restoreutil"
)

type NeovimModule struct{}

func NewModule() *NeovimModule { return &NeovimModule{} }

func (m *NeovimModule) Name() string        { return "neovim" }
func (m *NeovimModule) Description() string { return "Neovim editor configuration" }

func (m *NeovimModule) Detect() (bool, error) {
	if restoreutil.CommandExists("nvim") {
		return true, nil
	}
	home := restoreutil.HomeDir()
	_, err := os.Stat(filepath.Join(home, ".config", "nvim"))
	return err == nil, nil
}

func (m *NeovimModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true, Metadata: make(map[string]any)}

	if ver, err := restoreutil.CheckExecOutput("nvim", "--version"); err == nil {
		firstLine := string(ver)
		if idx := strings.Index(firstLine, "\n"); idx > 0 {
			firstLine = firstLine[:idx]
		}
		result.Version = firstLine
	}

	home := restoreutil.HomeDir()
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
	home := restoreutil.HomeDir()

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
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
	}, nil
}

func (m *NeovimModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	if rt, ok := opts.Runtime.(*runtime.Runtime); ok && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}
	tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-neovim-*")
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

func (m *NeovimModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepCommand, Package: "nvim", Hint: "Neovim editor"},
	}
}

func (m *NeovimModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	if restoreutil.CommandExists("nvim") {
		return nil
	}
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("neovim")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "neovim").Run()
}

func (m *NeovimModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	return os.MkdirAll(filepath.Join(home, ".config", "nvim"), 0755)
}

func (m *NeovimModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("neovim")

	if ver, err := restoreutil.CheckExecOutput("nvim", "--version"); err == nil {
		firstLine := ver
		if idx := strings.Index(firstLine, "\n"); idx > 0 {
			firstLine = firstLine[:idx]
		}
		v.Version(firstLine)
	}
	v.Check(restoreutil.CommandExists("nvim"), "Neovim installed")

	home := restoreutil.HomeDir()
	configDir := filepath.Join(home, ".config", "nvim")
	dirExists := restoreutil.DirExists(configDir)
	v.Check(dirExists, "Neovim config directory exists")

	if dirExists {
		initLua := filepath.Join(configDir, "init.lua")
		initVim := filepath.Join(configDir, "init.vim")
		if restoreutil.FileExists(initLua) {
			v.Recovered("init.lua")
		} else if restoreutil.FileExists(initVim) {
			v.Recovered("init.vim")
		} else {
			v.Warn("No init.lua or init.vim found")
		}

		pluginDirs := []string{"lua", "after", "plugin"}
		for _, dir := range pluginDirs {
			p := filepath.Join(configDir, dir)
			if restoreutil.DirExists(p) {
				v.Recovered("plugins: " + dir)
			}
		}
	}

	legacyDir := filepath.Join(home, ".vim")
	if restoreutil.DirExists(legacyDir) {
		v.Recovered(".vim directory")
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *NeovimModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()
	if rt, ok := opts.Runtime.(*runtime.Runtime); ok && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
	}, nil
}

type restoreUtilAction struct {
	actions.BaseAction
	name string
	desc string
	fn   func(ctx *runtime.RestoreContext) error
}

func (a *restoreUtilAction) Name() string                           { return a.name }
func (a *restoreUtilAction) Description() string                    { return a.desc }
func (a *restoreUtilAction) Execute(ctx *runtime.RestoreContext) error { return a.fn(ctx) }

var _ actions.Provider = (*NeovimModule)(nil)
