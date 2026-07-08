package rust

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

type RustModule struct{}

func NewModule() *RustModule { return &RustModule{} }

func (m *RustModule) Name() string        { return "rust" }
func (m *RustModule) Description() string { return "Rust toolchain and cargo packages" }

func (m *RustModule) Detect() (bool, error) {
	return restoreutil.CommandExists("rustc"), nil
}

func (m *RustModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true, Metadata: make(map[string]any)}

	if ver, err := restoreutil.CheckExecOutput("rustc", "--version"); err == nil {
		result.Version = ver
	}

	if ver, err := restoreutil.CheckExecOutput("cargo", "--version"); err == nil {
		result.Metadata["cargo"] = ver
	}

	if out, err := exec.Command("cargo", "install", "--list").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		var tools []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) > 0 {
				tools = append(tools, parts[0])
			}
		}
		result.Metadata["installedTools"] = tools
	}

	cargoConfig := filepath.Join(restoreutil.HomeDir(), ".cargo", "config.toml")
	if info, err := os.Stat(cargoConfig); err == nil {
		result.Resources = append(result.Resources, module.Resource{
			Name: "config.toml", Path: cargoConfig, Size: info.Size(), Modified: info.ModTime(), Type: "config",
		})
	}

	cargoCache := filepath.Join(restoreutil.HomeDir(), ".cargo", "registry", "cache")
	if info, err := os.Stat(cargoCache); err == nil && info.IsDir() {
		var cacheSize int64
		filepath.Walk(cargoCache, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			cacheSize += info.Size()
			return nil
		})
		if cacheSize > 0 {
			result.Metadata["cargoCache"] = cacheSize
		}
	}

	return result, nil
}

func (m *RustModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home := restoreutil.HomeDir()
	var entries []archive.Entry

	cargoConfig := filepath.Join(home, ".cargo", "config.toml")
	if _, err := os.Stat(cargoConfig); err == nil {
		entries = append(entries, archive.Entry{
			Source: cargoConfig, ArchivePath: ".cargo/config.toml",
		})
	}

	if out, err := exec.Command("cargo", "install", "--list").Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		var tools []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) > 0 {
				tools = append(tools, parts[0])
			}
		}
		if len(tools) > 0 {
			data, _ := json.Marshal(tools)
			tmpFile := filepath.Join(os.TempDir(), "getitback-rust-tools.json")
			os.WriteFile(tmpFile, data, 0600)
			defer os.Remove(tmpFile)
			entries = append(entries, archive.Entry{
				Source: tmpFile, ArchivePath: "cargo-tools.json",
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

func (m *RustModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	cargoDir := filepath.Join(home, ".cargo")
	os.MkdirAll(cargoDir, 0755)

	tmpDir, err := os.MkdirTemp(opts.WorkDir, "getitback-restore-rust-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if rt != nil {
		rt.Archive.Extract(snap.Path, tmpDir)
	} else {
		archive.Extract(snap.Path, tmpDir)
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

func (m *RustModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *RustModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}

	if !restoreutil.CommandExists("rustc") {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "error", Message: "rustc not found in PATH",
			Help: "Install Rust: curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh",
		})
	}
	if !restoreutil.CommandExists("cargo") {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "error", Message: "cargo not found in PATH",
			Help: "Cargo is part of the Rust toolchain; install via rustup",
		})
	}

	cargoDir := filepath.Join(restoreutil.HomeDir(), ".cargo")
	if info, err := os.Stat(cargoDir); err != nil {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "warning", Message: ".cargo directory does not exist",
			Help: fmt.Sprintf("mkdir -p %s", cargoDir),
		})
	} else if info.Mode().Perm()&0077 != 0 {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "warning", Message: ".cargo directory has overly permissive permissions",
			Help: fmt.Sprintf("chmod 755 %s", cargoDir),
		})
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

func (m *RustModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepCommand, Package: "rustc", Hint: "Rust compiler"},
		{Type: module.DepCommand, Package: "cargo", Hint: "Cargo package manager"},
		{Type: module.DepCommand, Package: "rustup", Hint: "Rust toolchain installer", Optional: true},
	}
}

func (m *RustModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("rustc")
	}
	if restoreutil.CommandExists("rustup") {
		return exec.Command("rustup", "install", "stable").Run()
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "rustc", "cargo").Run()
}

func (m *RustModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	os.MkdirAll(filepath.Join(home, ".cargo", "bin"), 0755)
	os.MkdirAll(filepath.Join(home, ".rustup"), 0755)
	return nil
}

func (m *RustModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("rust")

	ver, err := restoreutil.CheckExecOutput("rustc", "--version")
	if err == nil {
		v.Version(ver)
	}
	v.Check(restoreutil.CommandExists("rustc"), "Rust compiler installed")
	v.Check(restoreutil.CommandExists("cargo"), "Cargo package manager installed")

	home := restoreutil.HomeDir()
	cargoBin := filepath.Join(home, ".cargo", "bin")
	if restoreutil.DirExists(cargoBin) {
		entries, _ := os.ReadDir(cargoBin)
		for _, entry := range entries {
			if !entry.IsDir() {
				v.Recovered("tool: " + entry.Name())
			}
		}
	} else {
		v.Warn("No .cargo/bin directory")
	}

	if restoreutil.FileExists(filepath.Join(home, ".cargo", "config.toml")) {
		v.Recovered("cargo config")
	} else {
		v.Missing("cargo config.toml")
	}

	if restoreutil.DirExists(filepath.Join(home, ".cargo", "registry")) {
		v.Recovered("cargo registry cache")
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *RustModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}
	cargoDir := filepath.Join(home, ".cargo")

	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
		&actions.CreateDirectory{Path: cargoDir, Mode: 0755},
		&restoreUtilAction{
			name: "cargo_permissions",
			desc: "Set cargo directory permissions",
			fn: func(ctx *runtime.RestoreContext) error {
				os.Chmod(cargoDir, 0755)
				binDir := filepath.Join(cargoDir, "bin")
				if restoreutil.DirExists(binDir) {
					os.Chmod(binDir, 0755)
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

func (a *restoreUtilAction) Name() string                           { return a.name }
func (a *restoreUtilAction) Description() string                    { return a.desc }
func (a *restoreUtilAction) Execute(ctx *runtime.RestoreContext) error { return a.fn(ctx) }

var _ actions.Provider = (*RustModule)(nil)
