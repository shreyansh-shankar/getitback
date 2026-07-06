package flatpak

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

type FlatpakModule struct{}

func NewModule() *FlatpakModule { return &FlatpakModule{} }

func (m *FlatpakModule) Name() string        { return "flatpak" }
func (m *FlatpakModule) Description() string { return "Flatpak package manager" }

func (m *FlatpakModule) Detect() (bool, error) {
	_, err := exec.LookPath("flatpak")
	return err == nil, nil
}

func (m *FlatpakModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := exec.Command("flatpak", "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	meta := make(map[string]any)

	if out, err := exec.Command("flatpak", "list", "--app", "--columns=application").Output(); err == nil {
		var count int
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && line != "Application ID" {
				count++
			}
		}
		meta["apps"] = count
	}

	if out, err := exec.Command("flatpak", "list", "--runtime", "--columns=application").Output(); err == nil {
		runtimes := 0
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && line != "Application ID" {
				runtimes++
			}
		}
		meta["runtimes"] = runtimes
	}

	if out, err := exec.Command("flatpak", "remotes", "--columns=name").Output(); err == nil {
		var remotes []string
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && line != "Name" {
				remotes = append(remotes, line)
			}
		}
		if len(remotes) > 0 {
			meta["remotes"] = remotes
		}
	}

	result.Metadata = meta

	return result, nil
}

func (m *FlatpakModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	out, err := exec.Command("flatpak", "list", "--app", "--columns=application").Output()
	if err != nil {
		return nil, nil
	}
	var apps []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != "Application ID" {
			apps = append(apps, line)
		}
	}
	if len(apps) == 0 {
		return nil, nil
	}
	tmpFile := filepath.Join(os.TempDir(), "getitback-flatpak-apps.json")
	defer os.Remove(tmpFile)
	data, _ := json.Marshal(apps)
	os.WriteFile(tmpFile, data, 0600)

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: tmpFile, ArchivePath: "flatpak-apps.json"},
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

func (m *FlatpakModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	tmpDir, err := os.MkdirTemp("", "getitback-restore-flatpak-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("extract snapshot: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(tmpDir, "flatpak-apps.json"))
	if err != nil {
		return fmt.Errorf("read app list: %w", err)
	}
	var apps []string
	if err := json.Unmarshal(data, &apps); err != nil {
		return fmt.Errorf("parse app list: %w", err)
	}
	for _, app := range apps {
		cmd := exec.Command("flatpak", "install", "-y", app)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("install flatpak %s: %w", app, err)
		}
	}
	return nil
}

func (m *FlatpakModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *FlatpakModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}
