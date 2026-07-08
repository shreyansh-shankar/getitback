package golang

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
	"github.com/shreyansh-shankar/getitback/internal/runtime/restoreutil"
)

type GolangModule struct{}

func NewModule() *GolangModule { return &GolangModule{} }

func (m *GolangModule) Name() string        { return "golang" }
func (m *GolangModule) Description() string { return "Go programming language toolchain" }

func (m *GolangModule) Detect() (bool, error) {
	return restoreutil.CommandExists("go"), nil
}

func (m *GolangModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true, Metadata: make(map[string]any)}

	if ver, err := restoreutil.CheckExecOutput("go", "version"); err == nil {
		result.Version = ver
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(restoreutil.HomeDir(), "go")
	}
	result.Metadata["GOPATH"] = gopath

	binDir := filepath.Join(gopath, "bin")
	if restoreutil.DirExists(binDir) {
		entries, err := os.ReadDir(binDir)
		if err == nil {
			var tools []string
			for _, e := range entries {
				if !e.IsDir() {
					tools = append(tools, e.Name())
				}
			}
			result.Metadata["binaries"] = len(tools)
			if len(tools) > 0 {
				samples := tools
				if len(samples) > 5 {
					samples = samples[:5]
				}
				result.Metadata["installedTools"] = samples
			}
		}
	}

	if goroot := os.Getenv("GOROOT"); goroot != "" {
		result.Metadata["GOROOT"] = goroot
	}

	return result, nil
}

func (m *GolangModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(restoreutil.HomeDir(), "go")
	}

	var archiveEntries []archive.Entry
	for _, dir := range []string{"bin", "pkg"} {
		srcDir := filepath.Join(gopath, dir)
		if !restoreutil.DirExists(srcDir) {
			continue
		}
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			archiveEntries = append(archiveEntries, archive.Entry{
				Source:      filepath.Join(srcDir, entry.Name()),
				ArchivePath: filepath.Join("go", dir, entry.Name()),
			})
		}
	}

	if len(archiveEntries) == 0 {
		return nil, nil
	}

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), archiveEntries)
	if err != nil || snapshot == nil {
		return nil, err
	}

	return &module.BackupResult{
		Module: m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size,
			Checksum: snapshot.Checksum, OriginalSize: snapshot.OriginalSize,
			FileCount: snapshot.FileCount,
		}},
	}, nil
}

func (m *GolangModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	if rt != nil {
		return rt.Archive.Extract(snap.Path, home)
	}
	return archive.Extract(snap.Path, home)
}

func (m *GolangModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *GolangModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "info",
			Message:  "GOPATH not set — using default",
			Help:     "Set GOPATH to organize Go development",
		})
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}

	return result, nil
}

// --- Enhanced restore interfaces ---

func (m *GolangModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "golang-go", Hint: "Go compiler and tools"},
	}
}

func (m *GolangModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("golang-go")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "golang-go").Run()
}

func (m *GolangModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(restoreutil.HomeDir(), "go")
	}
	os.MkdirAll(filepath.Join(gopath, "bin"), 0755)
	os.MkdirAll(filepath.Join(gopath, "pkg"), 0755)
	return nil
}

func (m *GolangModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("golang")

	ver, err := restoreutil.CheckExecOutput("go", "version")
	if err == nil {
		v.Version(ver)
	}
	v.Check(restoreutil.CommandExists("go"), "Go toolchain installed")

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(restoreutil.HomeDir(), "go")
	}
	gopathExists := restoreutil.DirExists(gopath)
	v.Check(gopathExists, "GOPATH directory exists")

	binDir := filepath.Join(gopath, "bin")
	if restoreutil.DirExists(binDir) {
		entries, _ := os.ReadDir(binDir)
		for _, entry := range entries {
			if !entry.IsDir() {
				v.Recovered("binary: " + entry.Name())
			}
		}
	}

	if gopathExists {
		v.Recovered("GOPATH: " + gopath)
	} else {
		v.Missing("GOPATH directory")
	}

	v.Confidence(90)
	return v.Result(), nil
}

func (m *GolangModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(home, "go")
	}

	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
		&restoreUtilAction{
			name: "go_gopath_structure",
			desc: "Restore GOPATH directory structure",
			fn: func(ctx *runtime.RestoreContext) error {
				os.MkdirAll(filepath.Join(gopath, "bin"), 0755)
				os.MkdirAll(filepath.Join(gopath, "pkg"), 0755)
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

var _ actions.Provider = (*GolangModule)(nil)
