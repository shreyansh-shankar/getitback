package dotfiles

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

var knownConfig = map[string]bool{
	".zshrc": true, ".zprofile": true, ".zshenv": true, ".zlogin": true, ".zlogout": true,
	".bashrc": true, ".bash_profile": true, ".bash_logout": true, ".profile": true,
	".gitconfig": true, ".gitignore_global": true,
	".tmux.conf": true,
	".editorconfig": true, ".gitattributes": true,
	".inputrc": true, ".selected_editor": true,
	".Xresources": true, ".xinitrc": true, ".xsession": true,
}

var knownTemp = map[string]bool{
	".bash_history": true, ".zsh_history": true, ".zhistory": true,
	".python_history": true, ".mysql_history": true, ".psql_history": true,
	".rediscli_history": true, ".node_repl_history": true,
	".lesshst": true, ".wget-hsts": true,
	".sudo_as_admin_successful": true,
	".zcompdump": true, ".zcompdump.zwc": true,
	".cache": true, ".local": true, ".dbus": true, ".pki": true,
	".Xauthority": true, ".ICEauthority": true, ".esd_auth": true,
	".directory": true,
}

var secretPrefixes = []string{".env", "token", "secret", "credential", "api", ".pem", ".key"}
var secretSuffixes = []string{".pem", ".key", ".cert", ".crt"}

var skipDirs = map[string]bool{
	".ssh": true, ".gnupg": true, ".mozilla": true, ".config": true,
	".npm": true, ".cargo": true, ".rustup": true, ".nvm": true, ".vscode": true,
	".oh-my-zsh": true, ".zprezto": true, ".zim": true, ".bash_it": true, ".oh-my-bash": true,
}

var dotConfigDirs = []string{
	"nvim", "tmux", "btop", "htop", "ranger",
	"kitty", "alacritty", "wezterm",
	"picom", "i3", "i3status", "polybar",
	"waybar", "sway", "hypr",
}

type DotfilesModule struct{}

func NewModule() *DotfilesModule { return &DotfilesModule{} }

func (m *DotfilesModule) Name() string        { return "dotfiles" }
func (m *DotfilesModule) Description() string { return "User dotfiles and configuration files" }

func (m *DotfilesModule) Detect() (bool, error) {
	home, _ := os.UserHomeDir()
	entries, err := os.ReadDir(home)
	if err != nil {
		return false, nil
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".") || entry.IsDir() {
			continue
		}
		if knownTemp[name] || skipDirs[name] {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (m *DotfilesModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{
		Module:   m.Name(),
		Detected: true,
	}

	home, _ := os.UserHomeDir()
	files := findDotfiles(home)

	var configCount, tempCount, secretCount, unknownCount int

	for _, f := range files {
		switch f.class {
		case "config":
			result.Resources = append(result.Resources, module.Resource{
				Name: f.name, Path: f.path, Size: f.size, Type: module.ResourceTypeConfig,
			})
			configCount++
		case "secret":
			if isSecretLike(f.name) {
				secretCount++
			}
		case "temp":
			tempCount++
		default:
			unknownCount++
		}
	}

	meta := make(map[string]any)
	if configCount > 0 {
		meta["configuration"] = configCount
	}
	if tempCount > 0 {
		meta["temporary"] = tempCount
	}
	if secretCount > 0 {
		meta["secrets"] = secretCount
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d potentially sensitive files detected (use 'getitback secrets' to inspect)", secretCount))
	}
	if unknownCount > 0 {
		meta["unknown"] = unknownCount
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}

	return result, nil
}

func (m *DotfilesModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()
	files := findDotfiles(home)

	var entries []archive.Entry
	for _, d := range files {
		if d.class != "config" {
			continue
		}
		rel, err := filepath.Rel(home, d.path)
		if err != nil {
			continue
		}
		entries = append(entries, archive.Entry{
			Source: d.path, ArchivePath: rel,
		})
	}

	for _, dir := range dotConfigDirs {
		configPath := filepath.Join(home, ".config", dir)
		if info, err := os.Stat(configPath); err == nil && info.IsDir() {
			rel, _ := filepath.Rel(home, configPath)
			entries = append(entries, archive.Entry{
				Source: configPath, ArchivePath: rel,
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
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
	}, nil
}

func (m *DotfilesModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()

	tmpDir, err := os.MkdirTemp("", "getitback-restore-dotfiles-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("extract dotfiles snapshot: %w", err)
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

func (m *DotfilesModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *DotfilesModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *DotfilesModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "git", Hint: "Git VCS"},
	}
}

func (m *DotfilesModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("git")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "git").Run()
}

func (m *DotfilesModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *DotfilesModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("dotfiles")

	home := restoreutil.HomeDir()
	entries, err := os.ReadDir(home)
	if err != nil {
		v.Error("cannot read home directory")
		return v.Result(), nil
	}

	var found int
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".") || entry.IsDir() {
			continue
		}
		if knownTemp[name] {
			continue
		}
		if knownConfig[name] {
			v.Recovered(name)
			found++
		}
	}

	if found == 0 {
		v.Warn("no dotfiles found in home directory")
		v.Missing("dotfiles")
	}

	v.Confidence(90)
	return v.Result(), nil
}

func (m *DotfilesModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	return []actions.Action{
		&restoreUtilAction{
			name: "dotfiles_restore",
			desc: "Restore dotfiles to home directory",
			fn: func(ctx *runtime.RestoreContext) error {
				return m.Restore(ctx, snap, opts)
			},
		},
	}, nil
}

type dotfile struct {
	name  string
	path  string
	size  int64
	class string
}

func findDotfiles(home string) []dotfile {
	var result []dotfile

	entries, err := os.ReadDir(home)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".") || entry.IsDir() {
			continue
		}
		if skipDirs[name] {
			continue
		}

		info, _ := entry.Info()
		if info == nil {
			continue
		}

		class := classifyFile(name)
		result = append(result, dotfile{
			name: name, path: filepath.Join(home, name),
			size: info.Size(), class: class,
		})
	}

	for _, dir := range dotConfigDirs {
		configPath := filepath.Join(home, ".config", dir)
		if info, err := os.Stat(configPath); err == nil && info.IsDir() {
			result = append(result, dotfile{
				name: ".config/" + dir, path: configPath,
				size: info.Size(), class: "config",
			})
		}
	}

	return result
}

func classifyFile(name string) string {
	if knownConfig[name] {
		return "config"
	}
	if knownTemp[name] {
		return "temp"
	}
	if strings.HasPrefix(name, ".pending-") {
		return "temp"
	}
	if isSecretLike(name) {
		return "secret"
	}
	return "unknown"
}

func isSecretLike(name string) bool {
	if name == ".env" {
		return true
	}
	if strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key") {
		return true
	}
	lower := strings.ToLower(name)
	secretKeywords := []string{"token", "secret", "credential", "api", "auth", "password", "key"}
	for _, kw := range secretKeywords {
		if strings.Contains(lower, kw) && !strings.HasSuffix(lower, ".pub") {
			return true
		}
	}
	return false
}

type restoreUtilAction struct {
	actions.BaseAction
	name string
	desc string
	fn   func(ctx *runtime.RestoreContext) error
}

func (a *restoreUtilAction) Name() string                       { return a.name }
func (a *restoreUtilAction) Description() string                 { return a.desc }
func (a *restoreUtilAction) Execute(ctx *runtime.RestoreContext) error { return a.fn(ctx) }

var _ actions.Provider = (*DotfilesModule)(nil)
