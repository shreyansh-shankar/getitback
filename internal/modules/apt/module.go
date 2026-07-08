package apt

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

type AptModule struct{}

func NewModule() *AptModule { return &AptModule{} }

func (m *AptModule) Name() string        { return "apt" }
func (m *AptModule) Description() string { return "APT package manager (Debian/Ubuntu)" }

func (m *AptModule) Detect() (bool, error) {
	return restoreutil.CommandExists("apt"), nil
}

func (m *AptModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := restoreutil.CheckExecOutput("apt", "--version"); err == nil {
		result.Version = strings.Fields(ver)[0]
	}

	meta := make(map[string]any)

	out, err := restoreutil.CheckExecOutput("apt-mark", "showmanual")
	if err == nil {
		packages := strings.Fields(out)
		meta["manualPackages"] = len(packages)
	} else {
		out, err := restoreutil.CheckExecOutput("dpkg", "--get-selections")
		if err == nil {
			count := 0
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, "install") {
					count++
				}
			}
			meta["installedPackages"] = count
		}
	}

	if held, err := restoreutil.CheckExecOutput("apt-mark", "showhold"); err == nil {
		heldPkgs := strings.Fields(held)
		if len(heldPkgs) > 0 {
			meta["heldPackages"] = len(heldPkgs)
		}
	}

	sources := "/etc/apt/sources.list"
	if restoreutil.FileExists(sources) {
		data, _ := os.ReadFile(sources)
		lines := strings.Split(string(data), "\n")
		var repos []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) > 2 && (parts[0] == "deb" || parts[0] == "deb-src") {
				repos = append(repos, parts[1])
			}
		}
		if len(repos) > 0 {
			if len(repos) > 5 {
				meta["repositories"] = len(repos)
			} else {
				meta["repositories"] = repos
			}
		}
	}

	sourcesDir := "/etc/apt/sources.list.d"
	if entries, err := os.ReadDir(sourcesDir); err == nil {
		var additionalRepos int
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".list") {
				additionalRepos++
			}
		}
		if additionalRepos > 0 {
			meta["additionalRepos"] = additionalRepos
		}
	}

	result.Metadata = meta
	return result, nil
}

func (m *AptModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	var packages []string

	out, err := restoreutil.CheckExecOutput("apt-mark", "showmanual")
	if err == nil {
		packages = strings.Fields(out)
	} else {
		out, err := restoreutil.CheckExecOutput("dpkg", "--get-selections")
		if err != nil {
			return nil, nil
		}
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "install") {
				packages = append(packages, strings.Fields(line)[0])
			}
		}
	}

	if len(packages) == 0 {
		return nil, nil
	}

	tmpFile := filepath.Join(os.TempDir(), "getitback-apt-packages.json")
	defer os.Remove(tmpFile)
	data, _ := json.Marshal(packages)
	os.WriteFile(tmpFile, data, 0600)

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: tmpFile, ArchivePath: "apt-packages.json"},
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

func (m *AptModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	tmpDir, err := os.MkdirTemp("", "getitback-restore-apt-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("extract snapshot: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(tmpDir, "apt-packages.json"))
	if err != nil {
		return fmt.Errorf("read package list: %w", err)
	}
	var packages []string
	if err := json.Unmarshal(data, &packages); err != nil {
		return fmt.Errorf("parse package list: %w", err)
	}
	if len(packages) == 0 {
		return nil
	}
	cmd := exec.Command("sudo", append([]string{"apt", "install", "-y"}, packages...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (m *AptModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *AptModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}
	out, err := restoreutil.CheckExecOutput("apt", "list", "--upgradable")
	if err == nil {
		lines := strings.Split(strings.TrimSpace(out), "\n")
		pending := 0
		for _, line := range lines {
			if !strings.HasPrefix(line, "Listing...") && line != "" {
				pending++
			}
		}
		if pending > 0 {
			result.Status = module.DoctorStatusWarning
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "warning",
				Message:  fmt.Sprintf("%d packages can be upgraded", pending),
				Help:     "Run 'sudo apt upgrade' to update packages",
			})
		}
	}
	return result, nil
}

func (m *AptModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "apt", Hint: "APT package manager"},
	}
}

func (m *AptModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *AptModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	return nil
}

func (m *AptModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("apt")

	v.Check(restoreutil.CommandExists("dpkg"), "dpkg available")
	v.Check(restoreutil.CommandExists("apt-get"), "apt-get available")

	if restoreutil.FileExists("/etc/apt/sources.list") {
		v.Recovered("sources.list")
	}
	if restoreutil.DirExists("/etc/apt/sources.list.d") {
		entries, _ := os.ReadDir("/etc/apt/sources.list.d")
		var count int
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".list") {
				count++
			}
		}
		if count > 0 {
			v.Recovered(fmt.Sprintf("%d additional source lists", count))
		}
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *AptModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	return []actions.Action{
		&actions.ExtractArchive{
			Source:      snap.Path,
			Destination: "/etc/apt",
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

var _ actions.Provider = (*AptModule)(nil)
