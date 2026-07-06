package sqlite

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
)

type SQLiteModule struct{}

func NewModule() *SQLiteModule { return &SQLiteModule{} }

func (m *SQLiteModule) Name() string        { return "sqlite" }
func (m *SQLiteModule) Description() string { return "SQLite databases" }

func (m *SQLiteModule) Detect() (bool, error) {
	if _, err := exec.LookPath("sqlite3"); err == nil {
		return true, nil
	}
	home, _ := os.UserHomeDir()
	matches, _ := filepath.Glob(filepath.Join(home, "*.db"))
	return len(matches) > 0, nil
}

func (m *SQLiteModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := exec.Command("sqlite3", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	home, _ := os.UserHomeDir()
	patterns := []string{
		filepath.Join(home, "*.db"),
		filepath.Join(home, "*.sqlite"),
		filepath.Join(home, "*.sqlite3"),
		filepath.Join(home, "Documents", "*.db"),
	}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			result.Resources = append(result.Resources, module.Resource{
				Name: filepath.Base(path), Path: path, Size: info.Size(), Modified: info.ModTime(), Type: "data",
			})
		}
	}

	return result, nil
}

func (m *SQLiteModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()

	var entries []archive.Entry
	for _, pattern := range []string{"*.db", "*.sqlite", "*.sqlite3"} {
		matches, _ := filepath.Glob(filepath.Join(home, pattern))
		for _, path := range matches {
			rel, _ := filepath.Rel(home, path)
			entries = append(entries, archive.Entry{
				Source: path, ArchivePath: rel,
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
		}},
	}, nil
}

func (m *SQLiteModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp("", "getitback-restore-sqlite-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return err
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

func (m *SQLiteModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *SQLiteModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}
