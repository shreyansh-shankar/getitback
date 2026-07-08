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
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
	"github.com/shreyansh-shankar/getitback/internal/runtime/restoreutil"
)

type FlatpakModule struct{}

func NewModule() *FlatpakModule { return &FlatpakModule{} }

func (m *FlatpakModule) Name() string        { return "flatpak" }
func (m *FlatpakModule) Description() string { return "Flatpak package manager" }

func (m *FlatpakModule) Detect() (bool, error) {
	return restoreutil.CommandExists("flatpak"), nil
}

func (m *FlatpakModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := restoreutil.CheckExecOutput("flatpak", "--version"); err == nil {
		result.Version = strings.TrimSpace(ver)
	}

	meta := make(map[string]any)

	if out, err := restoreutil.CheckExecOutput("flatpak", "list", "--app", "--columns=application"); err == nil {
		var count int
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && line != "Application ID" {
				count++
			}
		}
		meta["apps"] = count
	}

	if out, err := restoreutil.CheckExecOutput("flatpak", "list", "--runtime", "--columns=application"); err == nil {
		runtimes := 0
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && line != "Application ID" {
				runtimes++
			}
		}
		meta["runtimes"] = runtimes
	}

	if out, err := restoreutil.CheckExecOutput("flatpak", "remotes", "--columns=name"); err == nil {
		var remotes []string
		for _, line := range strings.Split(out, "\n") {
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
	out, err := restoreutil.CheckExecOutput("flatpak", "list", "--app", "--columns=application")
	if err != nil {
		return nil, nil
	}
	var apps []string
	for _, line := range strings.Split(out, "\n") {
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
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
	}, nil
}

func (m *FlatpakModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-flatpak-*")
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

func (m *FlatpakModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "flatpak", Hint: "Flatpak package manager"},
	}
}

func (m *FlatpakModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("flatpak")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "flatpak").Run()
}

func (m *FlatpakModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *FlatpakModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("flatpak")

	v.Check(restoreutil.CommandExists("flatpak"), "flatpak installed")

	if ver, err := restoreutil.CheckExecOutput("flatpak", "--version"); err == nil {
		v.Version(strings.TrimSpace(ver))
	}

	if out, err := restoreutil.CheckExecOutput("flatpak", "remotes", "--columns=name"); err == nil {
		var remotes []string
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && line != "Name" {
				remotes = append(remotes, line)
			}
		}
		if len(remotes) > 0 {
			v.Recovered(fmt.Sprintf("%d flatpak remotes", len(remotes)))
		}
	}

	if out, err := restoreutil.CheckExecOutput("flatpak", "list", "--app", "--columns=application"); err == nil {
		var apps []string
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if line != "" && line != "Application ID" {
				apps = append(apps, line)
			}
		}
		if len(apps) > 0 {
			v.Recovered(fmt.Sprintf("%d flatpak apps", len(apps)))
		}
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *FlatpakModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()
	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
		&restoreUtilAction{
			name: "flatpak_add_remotes",
			desc: "Add flatpak remotes",
			fn: func(ctx *runtime.RestoreContext) error {
				out, err := restoreutil.CheckExecOutput("flatpak", "remote-list", "--columns=name,url")
				if err != nil {
					return nil
				}
				for _, line := range strings.Split(out, "\n") {
					line = strings.TrimSpace(line)
					if line == "" || strings.HasPrefix(line, "Name") {
						continue
					}
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						name := fields[0]
						url := fields[1]
						if err := exec.Command("flatpak", "remote-add", "--if-not-exists", name, url).Run(); err != nil {
							return fmt.Errorf("add flatpak remote %s: %w", name, err)
						}
					}
				}
				return nil
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

var _ actions.Provider = (*FlatpakModule)(nil)
