package nodejs

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

type NodeJSModule struct{}

func NewModule() *NodeJSModule { return &NodeJSModule{} }

func (m *NodeJSModule) Name() string        { return "nodejs" }
func (m *NodeJSModule) Description() string { return "Node.js runtime and npm packages" }

func (m *NodeJSModule) Detect() (bool, error) {
	_, err := exec.LookPath("node")
	return err == nil, nil
}

func (m *NodeJSModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := exec.Command("node", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	home, _ := os.UserHomeDir()
	meta := make(map[string]any)

	if ver, err := exec.Command("npm", "--version").Output(); err == nil {
		meta["npm"] = strings.TrimSpace(string(ver))
	}
	if ver, err := exec.Command("pnpm", "--version").Output(); err == nil {
		meta["pnpm"] = strings.TrimSpace(string(ver))
	}
	if ver, err := exec.Command("yarn", "--version").Output(); err == nil {
		meta["yarn"] = strings.TrimSpace(string(ver))
	}
	if ver, err := exec.Command("corepack", "--version").Output(); err == nil {
		meta["corepack"] = strings.TrimSpace(string(ver))
	}
	if ver, err := exec.Command("bun", "--version").Output(); err == nil {
		meta["bun"] = strings.TrimSpace(string(ver))
	}

	if out, err := exec.Command("npm", "list", "-g", "--depth=0").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		var pkgs []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "+--") || strings.HasPrefix(line, "└──") {
				pkgs = append(pkgs, strings.TrimLeft(line, "+─├└ "))
			}
		}
		meta["globalPackages"] = len(pkgs)
	} else {
		meta["globalPackages"] = 0
	}

	for _, config := range []string{".npmrc", ".nvmrc", ".node-version"} {
		path := filepath.Join(home, config)
		if info, err := os.Stat(path); err == nil {
			result.Resources = append(result.Resources, module.Resource{
				Name: config, Path: path, Size: info.Size(),
				Modified: info.ModTime(), Type: module.ResourceTypeConfig,
			})
		}
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}

	return result, nil
}

func (m *NodeJSModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()
	var entries []archive.Entry
	for _, config := range []string{".npmrc", ".nvmrc", ".node-version"} {
		path := filepath.Join(home, config)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			entries = append(entries, archive.Entry{
				Source: path, ArchivePath: config,
			})
		}
	}
	if out, err := exec.Command("npm", "list", "-g", "--depth=0", "--json").Output(); err == nil {
		tmpFile := filepath.Join(os.TempDir(), "getitback-nodejs-packages.json")
		os.WriteFile(tmpFile, out, 0600)
		defer os.Remove(tmpFile)
		entries = append(entries, archive.Entry{
			Source: tmpFile, ArchivePath: "npm-global.json",
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

func (m *NodeJSModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp("", "getitback-restore-nodejs-*")
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

func (m *NodeJSModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *NodeJSModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}
