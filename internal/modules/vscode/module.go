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
)

type VSCodeModule struct{}

func NewModule() *VSCodeModule { return &VSCodeModule{} }

func (m *VSCodeModule) Name() string        { return "vscode" }
func (m *VSCodeModule) Description() string { return "Visual Studio Code configuration and extensions" }

func (m *VSCodeModule) Detect() (bool, error) {
	if _, err := exec.LookPath("code"); err == nil {
		return true, nil
	}
	home, _ := os.UserHomeDir()
	_, err := os.Stat(filepath.Join(home, ".config", "Code"))
	return err == nil, nil
}

func (m *VSCodeModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := exec.Command("code", "--version").Output(); err == nil {
		lines := strings.SplitN(string(ver), "\n", 2)
		result.Version = lines[0]
	}

	home, _ := os.UserHomeDir()
	userDir := filepath.Join(home, ".config", "Code", "User")
	codeDir := filepath.Join(home, ".config", "Code")

	meta := make(map[string]any)

	for _, name := range []string{"settings.json", "keybindings.json"} {
		path := filepath.Join(userDir, name)
		if info, err := os.Stat(path); err == nil {
			meta[name[:len(name)-5]] = "yes" // "settings", "keybindings"
			result.Resources = append(result.Resources, module.Resource{
				Name: name, Path: path, Size: info.Size(), Modified: info.ModTime(), Type: "config",
			})
		}
	}

	snippets := filepath.Join(userDir, "snippets")
	if info, err := os.Stat(snippets); err == nil {
		meta["snippets"] = "yes"
		result.Resources = append(result.Resources, module.Resource{
			Name: "snippets", Path: snippets, Size: info.Size(), Modified: info.ModTime(), Type: "config",
		})
	}

	if out, err := exec.Command("code", "--list-extensions").Output(); err == nil {
		exts := strings.Fields(string(out))
		meta["extensions"] = exts
		// Count themes and language packs
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
			meta["themes"] = themeCount
		}
		if langPackCount > 0 {
			meta["languagePacks"] = langPackCount
		}
	}

	profiles := filepath.Join(userDir, "profiles")
	if info, err := os.Stat(profiles); err == nil && info.IsDir() {
		profileEntries, err := os.ReadDir(profiles)
		if err == nil {
			var profileNames []string
			for _, p := range profileEntries {
				if p.IsDir() {
					profileNames = append(profileNames, p.Name())
				}
			}
			if len(profileNames) > 0 {
				meta["profiles"] = profileNames
			}
		}
	}

	// Workspace storage
	workspaceStorage := filepath.Join(codeDir, "User", "workspaceStorage")
	if info, err := os.Stat(workspaceStorage); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(workspaceStorage)
		if len(entries) > 0 {
			meta["workspaces"] = len(entries)
		}
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}

	return result, nil
}

func (m *VSCodeModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()
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

	if out, err := exec.Command("code", "--list-extensions").Output(); err == nil {
		extData, _ := json.Marshal(strings.Fields(string(out)))
		tmpFile := filepath.Join(os.TempDir(), "getitback-vscode-extensions.json")
		os.WriteFile(tmpFile, extData, 0600)
		defer os.Remove(tmpFile)
		entries = append(entries, archive.Entry{
			Source: tmpFile, ArchivePath: ".config/Code/extensions.json",
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

func (m *VSCodeModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp("", "getitback-restore-vscode-*")
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

func (m *VSCodeModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
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
