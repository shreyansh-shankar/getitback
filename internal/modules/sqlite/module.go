package sqlite

import (
	"context"
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
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
	}, nil
}

func (m *SQLiteModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-sqlite-*")
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

func (m *SQLiteModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "sqlite3", Hint: "SQLite3 CLI"},
	}
}

func (m *SQLiteModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("sqlite3")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "sqlite3").Run()
}

func (m *SQLiteModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *SQLiteModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("sqlite")
	if restoreutil.CommandExists("sqlite3") {
		ver, err := restoreutil.CheckExecOutput("sqlite3", "--version")
		if err == nil {
			v.Version(strings.TrimSpace(ver))
		}
	}
	v.Check(restoreutil.CommandExists("sqlite3"), "sqlite3 installed")
	home := restoreutil.HomeDir()
	patterns := []string{"*.db", "*.sqlite", "*.sqlite3"}
	recoveredCount := 0
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(home, pattern))
		for _, path := range matches {
			if !strings.HasSuffix(path, ".getitback-bak") {
				v.Recovered(filepath.Base(path))
				recoveredCount++
			}
		}
	}
	v.Check(recoveredCount > 0 || restoreutil.CommandExists("sqlite3"), "SQLite databases found")
	v.Confidence(80)
	return v.Result(), nil
}

func (m *SQLiteModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()
	tmpDir := filepath.Join(os.TempDir(), "getitback-restore-sqlite")
	return []actions.Action{
		&actions.CreateDirectory{Path: tmpDir, Mode: 0755},
		&actions.ExtractArchive{Source: snap.Path, Destination: tmpDir},
		&restoreUtilAction{
			name: "sqlite_restore",
			desc: "Restore SQLite database files",
			fn: func(ctx *runtime.RestoreContext) error {
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
		},
	}, nil
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

var _ actions.Provider = (*SQLiteModule)(nil)
