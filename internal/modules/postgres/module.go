package postgres

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

type PostgresModule struct{}

func NewModule() *PostgresModule { return &PostgresModule{} }

func (m *PostgresModule) Name() string        { return "postgres" }
func (m *PostgresModule) Description() string { return "PostgreSQL databases" }

func (m *PostgresModule) Detect() (bool, error) {
	if _, err := exec.LookPath("psql"); err == nil {
		return true, nil
	}
	_, err := exec.LookPath("pg_dump")
	return err == nil, nil
}

func (m *PostgresModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := exec.Command("psql", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	out, err := exec.Command("psql", "-U", os.Getenv("USER"), "-lqt").Output()
	if err != nil {
		return result, nil
	}

	var databases []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && !strings.HasPrefix(fields[0], " ") && fields[0] != "" {
			name := fields[0]
			if name != "template0" && name != "template1" {
				databases = append(databases, name)
			}
		}
	}

	if len(databases) > 0 {
		result.Metadata = map[string]any{"databases": databases}
	}

	return result, nil
}

func (m *PostgresModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	out, err := exec.Command("pg_dumpall", "-U", os.Getenv("USER"), "--no-password").Output()
	if err != nil {
		return nil, nil
	}

	tmpFile := filepath.Join(os.TempDir(), "getitback-postgres-dump.sql")
	if err := os.WriteFile(tmpFile, out, 0600); err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile)

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: tmpFile, ArchivePath: "postgres-dump.sql"},
	})
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

func (m *PostgresModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	tmpDir, err := os.MkdirTemp("", "getitback-restore-postgres-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("extract snapshot: %w", err)
	}

	var dumpFile string
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(path, ".sql") || strings.HasSuffix(path, ".dump")) {
			dumpFile = path
		}
		return nil
	})

	if dumpFile == "" {
		return fmt.Errorf("no SQL dump found in snapshot")
	}

	user := os.Getenv("USER")
	cmd := exec.Command("psql", "-U", user, "-f", dumpFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restore postgres dump: %w", err)
	}
	return nil
}

func (m *PostgresModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *PostgresModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}
