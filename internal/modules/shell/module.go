package shell

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

type ShellModule struct{}

func NewModule() *ShellModule { return &ShellModule{} }

func (m *ShellModule) Name() string        { return "shell" }
func (m *ShellModule) Description() string { return "Shell configuration" }

func (m *ShellModule) Detect() (bool, error) {
	return os.Getenv("SHELL") != "", nil
}

func (m *ShellModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{
		Module:   m.Name(),
		Detected: true,
	}

	shell := os.Getenv("SHELL")
	home, _ := os.UserHomeDir()
	shellName := filepath.Base(shell)
	meta := map[string]any{
		"basename": shellName,
	}

	if ver, err := exec.Command(shell, "--version").Output(); err == nil {
		firstLine := strings.SplitN(string(ver), "\n", 2)[0]
		result.Version = firstLine
	}

	// Framework detection
	frameworks := detectFrameworks(home, shellName)
	if len(frameworks) > 0 {
		meta["frameworks"] = frameworks
	}

	// Starship detection
	if _, err := exec.LookPath("starship"); err == nil {
		meta["starship"] = true
		if _, err := os.Stat(filepath.Join(home, ".config", "starship.toml")); err == nil {
			result.Resources = append(result.Resources, module.Resource{
				Name: "starship.toml", Path: filepath.Join(home, ".config", "starship.toml"),
				Type: "config",
			})
		}
	}

	result.Metadata = meta

	configFiles := shellConfigFiles(shellName, home)

	for _, file := range configFiles {
		if info, err := os.Stat(file); err == nil {
			result.Resources = append(result.Resources, module.Resource{
				Name:     filepath.Base(file),
				Path:     file,
				Size:     info.Size(),
				Modified: info.ModTime(),
				Type:     "config",
			})
		}
	}

	return result, nil
}

func (m *ShellModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()
	shell := os.Getenv("SHELL")
	shellName := filepath.Base(shell)
	configPaths := shellConfigFiles(shellName, home)

	var entries []archive.Entry
	for _, path := range configPaths {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		rel, err := filepath.Rel(home, path)
		if err != nil {
			continue
		}
		entries = append(entries, archive.Entry{
			Source:      path,
			ArchivePath: rel,
		})
	}

	starship := filepath.Join(home, ".config", "starship.toml")
	if _, err := os.Stat(starship); err == nil {
		entries = append(entries, archive.Entry{
			Source: starship, ArchivePath: ".config/starship.toml",
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
		Snapshots: []module.Snapshot{
			{
				Module:   m.Name(),
				Path:     snapshot.Path,
				Size:     snapshot.Size,
				Checksum: snapshot.Checksum,
			},
		},
	}, nil
}

func (m *ShellModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()

	tmpDir, err := os.MkdirTemp("", "getitback-restore-shell-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("extract shell snapshot: %w", err)
	}

	return filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(tmpDir, path)
		if err != nil {
			return err
		}

		dst := filepath.Join(home, rel)

		if _, err := os.Stat(dst); err == nil {
			os.Rename(dst, dst+".getitback-bak")
		}

		os.MkdirAll(filepath.Dir(dst), 0755)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0644)
	})
}

func (m *ShellModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *ShellModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{
			Module:   m.Name(),
			Snapshot: snap,
			Valid:    false,
			Errors:   []string{err.Error()},
		}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{
			Module:   m.Name(),
			Snapshot: snap,
			Valid:    false,
			Errors:   []string{"snapshot is empty"},
		}, nil
	}
	return &module.VerifyResult{
		Module:   m.Name(),
		Snapshot: snap,
		Valid:    true,
	}, nil
}

func detectFrameworks(home, shellName string) []string {
	var frameworks []string
	switch shellName {
	case "zsh":
		if _, err := os.Stat(filepath.Join(home, ".oh-my-zsh")); err == nil {
			frameworks = append(frameworks, "oh-my-zsh")
			themeFile := filepath.Join(home, ".oh-my-zsh", "custom", "themes")
			if entries, err := os.ReadDir(themeFile); err == nil {
				for _, e := range entries {
					if strings.Contains(e.Name(), "powerlevel10k") {
						frameworks = append(frameworks, "powerlevel10k")
						break
					}
				}
			}
		}
		if _, err := os.Stat(filepath.Join(home, ".zprezto")); err == nil {
			frameworks = append(frameworks, "prezto")
		}
		if _, err := os.Stat(filepath.Join(home, ".antigen.zsh")); err == nil {
			frameworks = append(frameworks, "antigen")
		}
		if _, err := os.Stat(filepath.Join(home, ".zim")); err == nil {
			frameworks = append(frameworks, "zim")
		}
	case "bash":
		if _, err := os.Stat(filepath.Join(home, ".bash_it")); err == nil {
			frameworks = append(frameworks, "bash-it")
		}
		if _, err := os.Stat(filepath.Join(home, ".oh-my-bash")); err == nil {
			frameworks = append(frameworks, "oh-my-bash")
		}
	case "fish":
		if _, err := os.Stat(filepath.Join(home, ".config", "fish", "functions")); err == nil {
			entries, _ := os.ReadDir(filepath.Join(home, ".config", "fish", "functions"))
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".fish") {
					frameworks = append(frameworks, "custom-functions:"+e.Name())
				}
			}
		}
	}
	return frameworks
}

func shellConfigFiles(shell, home string) []string {
	files := map[string][]string{
		"bash": {".bashrc", ".bash_profile", ".bash_logout", ".profile"},
		"zsh":  {".zshrc", ".zprofile", ".zshenv", ".zlogin", ".zlogout"},
		"fish": {".config/fish/config.fish", ".config/fish/functions"},
	}
	if paths, ok := files[shell]; ok {
		result := make([]string, len(paths))
		for i, p := range paths {
			result[i] = filepath.Join(home, p)
		}
		return result
	}
	return nil
}
