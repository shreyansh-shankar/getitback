package golang

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

type GolangModule struct{}

func NewModule() *GolangModule { return &GolangModule{} }

func (m *GolangModule) Name() string        { return "golang" }
func (m *GolangModule) Description() string { return "Go programming language toolchain" }

func (m *GolangModule) Detect() (bool, error) {
	_, err := exec.LookPath("go")
	return err == nil, nil
}

func (m *GolangModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := exec.Command("go", "version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	meta := make(map[string]any)

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}
	meta["GOPATH"] = gopath
	binDir := filepath.Join(gopath, "bin")
	if entries, err := os.ReadDir(binDir); err == nil {
		var tools []string
		for _, e := range entries {
			if !e.IsDir() {
				tools = append(tools, e.Name())
			}
		}
		meta["binaries"] = len(tools)
		if len(tools) > 0 {
			samples := tools
			if len(samples) > 5 {
				samples = samples[:5]
			}
			meta["installedTools"] = samples
		}
	}

	if goroot := os.Getenv("GOROOT"); goroot != "" {
		meta["GOROOT"] = goroot
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}

	return result, nil
}

func (m *GolangModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	var entries []archive.Entry

	if gopath := os.Getenv("GOPATH"); gopath != "" {
		binDir := filepath.Join(gopath, "bin")
		if dirEntries, err := os.ReadDir(binDir); err == nil {
			var tools []string
			for _, e := range dirEntries {
				if !e.IsDir() {
					tools = append(tools, e.Name())
				}
			}
			if len(tools) > 0 {
				data, _ := json.Marshal(tools)
				tmpFile := filepath.Join(os.TempDir(), "getitback-golang-tools.json")
				os.WriteFile(tmpFile, data, 0600)
				defer os.Remove(tmpFile)
				entries = append(entries, archive.Entry{
					Source: tmpFile, ArchivePath: "go-tools.json",
				})
			}
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
		}},
	}, nil
}

func (m *GolangModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp("", "getitback-restore-golang-*")
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

func (m *GolangModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *GolangModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}
