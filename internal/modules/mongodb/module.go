package mongodb

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

type MongoDBModule struct{}

func NewModule() *MongoDBModule { return &MongoDBModule{} }

func (m *MongoDBModule) Name() string        { return "mongodb" }
func (m *MongoDBModule) Description() string { return "MongoDB databases" }

func (m *MongoDBModule) Detect() (bool, error) {
	if _, err := exec.LookPath("mongosh"); err == nil {
		return true, nil
	}
	_, err := exec.LookPath("mongo")
	return err == nil, nil
}

func (m *MongoDBModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := exec.Command("mongosh", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	} else if ver, err := exec.Command("mongo", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	return result, nil
}

func (m *MongoDBModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	dumpDir, err := os.MkdirTemp("", "getitback-mongodb-dump-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dumpDir)

	if err := exec.Command("mongodump", "--out", dumpDir).Run(); err != nil {
		return nil, nil
	}

	var entries []archive.Entry
	entries = append(entries, archive.Entry{
		Source: dumpDir, ArchivePath: "mongodb-dump",
	})

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

func (m *MongoDBModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	tmpDir, err := os.MkdirTemp("", "getitback-restore-mongodb-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("extract snapshot: %w", err)
	}

	dumpDir := filepath.Join(tmpDir, "mongodb-dump")
	if _, err := os.Stat(dumpDir); err != nil {
		filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() && info.Name() == "dump" {
				dumpDir = path
			}
			return nil
		})
	}

	if _, err := os.Stat(dumpDir); err != nil {
		return fmt.Errorf("no mongodump directory found in snapshot")
	}

	cmd := exec.Command("mongorestore", dumpDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restore mongodb dump: %w", err)
	}
	return nil
}

func (m *MongoDBModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *MongoDBModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *MongoDBModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "mongodb-database-tools", Hint: "MongoDB database tools"},
	}
}

func (m *MongoDBModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("mongodb-database-tools")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "mongodb-database-tools").Run()
}

func (m *MongoDBModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *MongoDBModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("mongodb")
	if restoreutil.CommandExists("mongosh") {
		ver, err := restoreutil.CheckExecOutput("mongosh", "--version")
		if err == nil {
			v.Version(strings.TrimSpace(ver))
		}
	} else if restoreutil.CommandExists("mongo") {
		ver, err := restoreutil.CheckExecOutput("mongo", "--version")
		if err == nil {
			v.Version(strings.TrimSpace(ver))
		}
	}
	v.Check(restoreutil.CommandExists("mongosh") || restoreutil.CommandExists("mongo"), "MongoDB shell installed")
	v.Check(restoreutil.CommandExists("mongodump"), "mongodump installed")
	v.Check(restoreutil.CommandExists("mongorestore"), "mongorestore installed")
	if restoreutil.CommandExists("mongodump") {
		v.Recovered("MongoDB database tools")
	}
	v.Confidence(80)
	return v.Result(), nil
}

func (m *MongoDBModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	tmpDir := filepath.Join(os.TempDir(), "getitback-restore-mongodb")
	return []actions.Action{
		&actions.CreateDirectory{Path: tmpDir, Mode: 0755},
		&actions.ExtractArchive{Source: snap.Path, Destination: tmpDir},
		&restoreUtilAction{
			name: "mongodb_restore",
			desc: "Restore MongoDB databases from dump",
			fn: func(ctx *runtime.RestoreContext) error {
				dumpDir := filepath.Join(tmpDir, "mongodb-dump")
				if _, err := os.Stat(dumpDir); err != nil {
					filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
						if err != nil {
							return err
						}
						if info.IsDir() && info.Name() == "dump" {
							dumpDir = path
						}
						return nil
					})
				}
				if _, err := os.Stat(dumpDir); err != nil {
					return fmt.Errorf("no mongodump directory found in snapshot")
				}
				bakDir := filepath.Join(tmpDir, "pre-restore.getitback-bak")
				exec.Command("mongodump", "--out", bakDir).Run()
				cmd := exec.Command("mongorestore", dumpDir)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				return cmd.Run()
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

var _ actions.Provider = (*MongoDBModule)(nil)
