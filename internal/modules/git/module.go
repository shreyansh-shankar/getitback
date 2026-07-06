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
)

type GitModule struct{}

func NewModule() *GitModule { return &GitModule{} }

func (m *GitModule) Name() string        { return "git" }
func (m *GitModule) Description() string { return "Git version control configuration" }

func (m *GitModule) Detect() (bool, error) {
	_, err := exec.LookPath("git")
	return err == nil, nil
}

func (m *GitModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{
		Module:   m.Name(),
		Detected: true,
	}

	if ver, err := exec.Command("git", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	home, _ := os.UserHomeDir()
	meta := make(map[string]any)

	if name, err := exec.Command("git", "config", "--global", "user.name").Output(); err == nil {
		meta["username"] = strings.TrimSpace(string(name))
	}
	if email, err := exec.Command("git", "config", "--global", "user.email").Output(); err == nil {
		meta["email"] = strings.TrimSpace(string(email))
	}
	if helper, err := exec.Command("git", "config", "--global", "credential.helper").Output(); err == nil {
		meta["credentialHelper"] = strings.TrimSpace(string(helper))
	}
	if signKey, err := exec.Command("git", "config", "--global", "user.signingkey").Output(); err == nil {
		meta["signingKey"] = strings.TrimSpace(string(signKey))
		if gpgSign, err := exec.Command("git", "config", "--global", "commit.gpgsign").Output(); err == nil {
			meta["commitGpgSign"] = strings.TrimSpace(string(gpgSign))
		}
	} else {
		result.Warnings = append(result.Warnings, "Git signing is not configured")
	}
	if defaultBranch, err := exec.Command("git", "config", "--global", "init.defaultBranch").Output(); err == nil {
		meta["defaultBranch"] = strings.TrimSpace(string(defaultBranch))
	}
	if _, err := exec.LookPath("git-lfs"); err == nil {
		meta["gitLFS"] = true
	}

	gitconfig := filepath.Join(home, ".gitconfig")
	if info, err := os.Stat(gitconfig); err == nil {
		result.Resources = append(result.Resources, module.Resource{
			Name: ".gitconfig", Path: gitconfig, Size: info.Size(),
			Modified: info.ModTime(), Type: module.ResourceTypeConfig,
		})
	}

	gitignore := filepath.Join(home, ".gitignore_global")
	if info, err := os.Stat(gitignore); err == nil {
		meta["globalIgnore"] = gitignore
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
			meta["hooks"] = hooks
		}
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}

	return result, nil
}

func (m *GitModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()

	var entries []archive.Entry
	gitconfig := filepath.Join(home, ".gitconfig")
	if _, err := os.Stat(gitconfig); err == nil {
		entries = append(entries, archive.Entry{
			Source: gitconfig, ArchivePath: ".gitconfig",
		})
	}
	gitignore := filepath.Join(home, ".gitignore_global")
	if _, err := os.Stat(gitignore); err == nil {
		entries = append(entries, archive.Entry{
			Source: gitignore, ArchivePath: ".gitignore_global",
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

func (m *GitModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	return restoreFiles(snap.Path, home, map[string]string{
		".gitconfig":        ".gitconfig",
		".gitignore_global": ".gitignore_global",
	})
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
	result := &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}

	if _, err := exec.Command("git", "config", "--global", "user.name").Output(); err != nil {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "warning",
			Message:  "Git username not configured",
			Help:     "Run: git config --global user.name \"Your Name\"",
		})
	}
	if _, err := exec.Command("git", "config", "--global", "user.email").Output(); err != nil {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "warning",
			Message:  "Git email not configured",
			Help:     "Run: git config --global user.email \"you@example.com\"",
		})
	}
	if _, err := exec.Command("git", "config", "--global", "user.signingkey").Output(); err != nil {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "info",
			Message:  "Git commit signing is not configured",
			Help:     "Set up signing: git config --global user.signingkey <key> && git config --global commit.gpgsign true",
		})
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}

	return result, nil
}

func restoreFiles(snapshotPath, destRoot string, fileMap map[string]string) error {
	tmpDir, err := os.MkdirTemp("", "getitback-restore-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snapshotPath, tmpDir); err != nil {
		return fmt.Errorf("extract snapshot: %w", err)
	}

	for archiveName, targetRel := range fileMap {
		src := filepath.Join(tmpDir, archiveName)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}
		dst := filepath.Join(destRoot, targetRel)
		if _, err := os.Stat(dst); err == nil {
			backupPath := dst + ".getitback-bak"
			if err := os.Rename(dst, backupPath); err != nil {
				return fmt.Errorf("backup existing %s: %w", targetRel, err)
			}
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", archiveName, err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", targetRel, err)
		}
	}
	return nil
}
