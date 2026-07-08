package git

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

type GitModule struct{}

func NewModule() *GitModule { return &GitModule{} }

func (m *GitModule) Name() string        { return "git" }
func (m *GitModule) Description() string { return "Git version control configuration" }

func (m *GitModule) Detect() (bool, error) {
	return restoreutil.CommandExists("git"), nil
}

func (m *GitModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true, Metadata: make(map[string]any)}

	if ver, err := restoreutil.CheckExecOutput("git", "--version"); err == nil {
		result.Version = strings.TrimSpace(ver)
	}

	if name, err := restoreutil.CheckExecOutput("git", "config", "--global", "user.name"); err == nil {
		result.Metadata["username"] = name
	}
	if email, err := restoreutil.CheckExecOutput("git", "config", "--global", "user.email"); err == nil {
		result.Metadata["email"] = email
	}
	if helper, err := restoreutil.CheckExecOutput("git", "config", "--global", "credential.helper"); err == nil {
		result.Metadata["credentialHelper"] = helper
	}
	if signKey, err := restoreutil.CheckExecOutput("git", "config", "--global", "user.signingkey"); err == nil {
		result.Metadata["signingKey"] = signKey
		if gpgSign, err := restoreutil.CheckExecOutput("git", "config", "--global", "commit.gpgsign"); err == nil {
			result.Metadata["commitGpgSign"] = gpgSign
		}
	} else {
		result.Warnings = append(result.Warnings, "Git signing is not configured")
	}
	if defaultBranch, err := restoreutil.CheckExecOutput("git", "config", "--global", "init.defaultBranch"); err == nil {
		result.Metadata["defaultBranch"] = defaultBranch
	}
	if restoreutil.CommandExists("git-lfs") {
		result.Metadata["gitLFS"] = true
	}

	home := restoreutil.HomeDir()
	gitconfig := filepath.Join(home, ".gitconfig")
	if info, err := os.Stat(gitconfig); err == nil {
		result.Resources = append(result.Resources, module.Resource{
			Name: ".gitconfig", Path: gitconfig, Size: info.Size(),
			Modified: info.ModTime(), Type: module.ResourceTypeConfig,
		})
	}

	gitignore := filepath.Join(home, ".gitignore_global")
	if info, err := os.Stat(gitignore); err == nil {
		result.Metadata["globalIgnore"] = gitignore
		result.Resources = append(result.Resources, module.Resource{
			Name: ".gitignore_global", Path: gitignore, Size: info.Size(),
			Modified: info.ModTime(), Type: module.ResourceTypeConfig,
		})
	}

	hooksDir := filepath.Join(home, ".git", "hooks")
	if info, err := os.Stat(hooksDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(hooksDir)
		var hooks []string
		for _, e := range entries {
			if !e.IsDir() && !strings.HasSuffix(e.Name(), ".sample") {
				hooks = append(hooks, e.Name())
			}
		}
		if len(hooks) > 0 {
			result.Metadata["hooks"] = hooks
		}
	}

	return result, nil
}

func (m *GitModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home := restoreutil.HomeDir()
	var entries []archive.Entry

	gitconfig := filepath.Join(home, ".gitconfig")
	if _, err := os.Stat(gitconfig); err == nil {
		entries = append(entries, archive.Entry{Source: gitconfig, ArchivePath: ".gitconfig"})
	}
	gitignore := filepath.Join(home, ".gitignore_global")
	if _, err := os.Stat(gitignore); err == nil {
		entries = append(entries, archive.Entry{Source: gitignore, ArchivePath: ".gitignore_global"})
	}
	if len(entries) == 0 {
		return nil, nil
	}
	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), entries)
	if err != nil || snapshot == nil {
		return nil, err
	}
	var contents []string
	if restoreutil.FileExists(gitconfig) {
		contents = append(contents, "global gitconfig")
	}
	if restoreutil.FileExists(gitignore) {
		contents = append(contents, "global gitignore")
	}
	return &module.BackupResult{
		Module: m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size,
			Checksum: snapshot.Checksum, OriginalSize: snapshot.OriginalSize,
			FileCount: snapshot.FileCount,
		}},
		Contents: contents,
	}, nil
}

func (m *GitModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	if rt, ok := opts.Runtime.(*runtime.Runtime); ok && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}
	return restoreFiles(snap.Path, home, []string{".gitconfig", ".gitignore_global"})
}

func (m *GitModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *GitModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}

	if _, err := restoreutil.CheckExecOutput("git", "config", "--global", "user.name"); err != nil {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "warning", Message: "Git username not configured",
			Help: `git config --global user.name "Your Name"`,
		})
	}
	if _, err := restoreutil.CheckExecOutput("git", "config", "--global", "user.email"); err != nil {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "warning", Message: "Git email not configured",
			Help: `git config --global user.email "you@example.com"`,
		})
	}
	if _, err := restoreutil.CheckExecOutput("git", "config", "--global", "user.signingkey"); err != nil {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "info", Message: "Git commit signing is not configured",
			Help: "Set up signing: git config --global user.signingkey <key> && git config --global commit.gpgsign true",
		})
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

// --- Enhanced restore interfaces ---

func (m *GitModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "git", Hint: "Git VCS"},
	}
}

func (m *GitModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("git")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "git").Run()
}

func (m *GitModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *GitModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("git")

	if ver, err := restoreutil.CheckExecOutput("git", "--version"); err == nil {
		v.Version(strings.TrimSpace(ver))
	}
	v.Check(restoreutil.CommandExists("git"), "Git installed")

	home := restoreutil.HomeDir()
	if restoreutil.FileExists(filepath.Join(home, ".gitconfig")) {
		v.Recovered("global gitconfig")
	} else {
		v.Missing("global gitconfig")
	}
	if restoreutil.FileExists(filepath.Join(home, ".gitignore_global")) {
		v.Recovered("global gitignore")
	}

	if name, err := restoreutil.CheckExecOutput("git", "config", "--global", "user.name"); err == nil {
		v.Recovered(fmt.Sprintf("git user.name: %s", name))
	} else {
		v.Warn("git user.name not set")
	}
	if email, err := restoreutil.CheckExecOutput("git", "config", "--global", "user.email"); err == nil {
		v.Recovered(fmt.Sprintf("git user.email: %s", email))
	} else {
		v.Warn("git user.email not set")
	}
	if _, err := restoreutil.CheckExecOutput("git", "config", "--global", "user.signingkey"); err == nil {
		v.Recovered("commit signing configured")
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *GitModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()
	if rt, ok := opts.Runtime.(*runtime.Runtime); ok && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	return []actions.Action{
		&restoreUtilAction{
			name: "git_restore_configs",
			desc: "Restore .gitconfig and .gitignore_global",
			fn: func(ctx *runtime.RestoreContext) error {
				return restoreFiles(snap.Path, home, []string{".gitconfig", ".gitignore_global"})
			},
		},
	}, nil
}

func restoreFiles(snapshotPath, destRoot string, filenames []string) error {
	tmpDir, err := os.MkdirTemp("", "getitback-restore-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snapshotPath, tmpDir); err != nil {
		return fmt.Errorf("extract snapshot: %w", err)
	}

	for _, name := range filenames {
		src := filepath.Join(tmpDir, name)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		dst := filepath.Join(destRoot, name)
		if _, err := os.Stat(dst); err == nil {
			backupPath := dst + ".getitback-bak"
			if err := os.Rename(dst, backupPath); err != nil {
				return fmt.Errorf("backup existing %s: %w", name, err)
			}
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
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

var _ actions.Provider = (*GitModule)(nil)
