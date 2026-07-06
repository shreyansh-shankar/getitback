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
