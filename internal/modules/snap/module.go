package snap

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

type SnapModule struct{}

func NewModule() *SnapModule { return &SnapModule{} }

func (m *SnapModule) Name() string        { return "snap" }
func (m *SnapModule) Description() string { return "Snap package manager" }

func (m *SnapModule) Detect() (bool, error) {
	_, err := exec.LookPath("snap")
	return err == nil, nil
}

func (m *SnapModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := exec.Command("snap", "--version").Output(); err == nil {
		lines := strings.Split(string(ver), "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "snap ") {
			result.Version = strings.TrimPrefix(lines[0], "snap ")
		}
	}

	out, err := exec.Command("snap", "list").Output()
	if err != nil {
		return result, nil
	}

	var snaps []string
	var channels = make(map[string]int)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			snaps = append(snaps, fields[0])
			if len(fields) > 2 {
				channels[fields[2]]++
			}
		}
	}

	meta := map[string]any{
		"snapCount": len(snaps),
	}
	if len(snaps) <= 20 {
		meta["snaps"] = snaps
	}
	if len(channels) > 0 {
		meta["channels"] = channels
	}
	result.Metadata = meta

	return result, nil
}

func (m *SnapModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	out, err := exec.Command("snap", "list").Output()
	if err != nil {
		return nil, nil
	}

	var snaps []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			snaps = append(snaps, fields[0])
		}
	}

	if len(snaps) == 0 {
		return nil, nil
	}

	tmpFile := filepath.Join(os.TempDir(), "getitback-snap-packages.json")
	defer os.Remove(tmpFile)
	data, _ := json.Marshal(snaps)
	os.WriteFile(tmpFile, data, 0600)

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: tmpFile, ArchivePath: "snap-packages.json"},
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

func (m *SnapModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	tmpDir, err := os.MkdirTemp("", "getitback-restore-snap-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("extract snapshot: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "snap-packages.json"))
	if err != nil {
		return fmt.Errorf("read package list: %w", err)
	}

	var snaps []string
	if err := json.Unmarshal(data, &snaps); err != nil {
		return fmt.Errorf("parse package list: %w", err)
	}

	for _, s := range snaps {
		cmd := exec.Command("sudo", "snap", "install", s)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install snap %s: %w", s, err)
		}
	}
	return nil
}

func (m *SnapModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *SnapModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}
