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
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
	"github.com/shreyansh-shankar/getitback/internal/runtime/restoreutil"
)

type SnapModule struct{}

func NewModule() *SnapModule { return &SnapModule{} }

func (m *SnapModule) Name() string        { return "snap" }
func (m *SnapModule) Description() string { return "Snap package manager" }

func (m *SnapModule) Detect() (bool, error) {
	return restoreutil.CommandExists("snap"), nil
}

func (m *SnapModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := restoreutil.CheckExecOutput("snap", "--version"); err == nil {
		lines := strings.Split(ver, "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "snap ") {
			result.Version = strings.TrimPrefix(lines[0], "snap ")
		}
	}

	out, err := restoreutil.CheckExecOutput("snap", "list")
	if err != nil {
		return result, nil
	}

	var snaps []string
	var channels = make(map[string]int)
	lines := strings.Split(out, "\n")
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
	out, err := restoreutil.CheckExecOutput("snap", "list")
	if err != nil {
		return nil, nil
	}

	var snaps []string
	lines := strings.Split(out, "\n")
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
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
	}, nil
}

func (m *SnapModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-snap-*")
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

func (m *SnapModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "snapd", Hint: "Snap package manager"},
	}
}

func (m *SnapModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("snapd")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "snapd").Run()
}

func (m *SnapModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *SnapModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("snap")

	v.Check(restoreutil.CommandExists("snap"), "snap installed")

	if ver, err := restoreutil.CheckExecOutput("snap", "list"); err == nil {
		lines := strings.Split(ver, "\n")
		count := 0
		for _, line := range lines[1:] {
			if strings.TrimSpace(line) != "" {
				count++
			}
		}
		if count > 0 {
			v.Recovered(fmt.Sprintf("%d snap packages", count))
		}
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *SnapModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: restoreutil.HomeDir()},
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

var _ actions.Provider = (*SnapModule)(nil)
