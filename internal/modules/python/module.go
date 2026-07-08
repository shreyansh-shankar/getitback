package python

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

type PythonModule struct{}

func NewModule() *PythonModule { return &PythonModule{} }

func (m *PythonModule) Name() string        { return "python" }
func (m *PythonModule) Description() string { return "Python runtime and pip packages" }

func (m *PythonModule) Detect() (bool, error) {
	return restoreutil.CommandExists("python3") || restoreutil.CommandExists("python"), nil
}

func (m *PythonModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}
	home := restoreutil.HomeDir()

	pythonCmd := "python3"
	if !restoreutil.CommandExists(pythonCmd) {
		pythonCmd = "python"
	}

	if ver, err := restoreutil.CheckExecOutput(pythonCmd, "--version"); err == nil {
		result.Version = ver
	}

	meta := make(map[string]any)

	if ver, err := restoreutil.CheckExecOutput("pip3", "--version"); err == nil {
		meta["pip"] = ver
	} else if ver, err := restoreutil.CheckExecOutput("pip", "--version"); err == nil {
		meta["pip"] = ver
	}

	if out, err := exec.Command("pip3", "list", "--format=json").Output(); err == nil {
		var pkgs []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}
		if json.Unmarshal(out, &pkgs) == nil {
			meta["packageCount"] = len(pkgs)
		}
	}

	if restoreutil.CommandExists("poetry") {
		meta["poetry"] = true
	}
	if restoreutil.CommandExists("pipx") {
		meta["pipx"] = true
		if out, err := exec.Command("pipx", "list", "--short").Output(); err == nil {
			count := 0
			for _, line := range strings.Split(string(out), "\n") {
				if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, " ") {
					count++
				}
			}
			if count > 0 {
				meta["pipxPackages"] = count
			}
		}
	}
	if restoreutil.CommandExists("uv") {
		meta["uv"] = true
	}
	if restoreutil.CommandExists("virtualenv") {
		meta["virtualenv"] = true
	}
	if restoreutil.DirExists(filepath.Join(home, ".virtualenvs")) {
		meta["virtualenvsDir"] = true
	}

	pipConfig := filepath.Join(home, ".config", "pip", "pip.conf")
	if info, err := os.Stat(pipConfig); err == nil {
		result.Resources = append(result.Resources, module.Resource{
			Name: "pip.conf", Path: pipConfig, Size: info.Size(),
			Modified: info.ModTime(), Type: module.ResourceTypeConfig,
		})
	}

	pypirc := filepath.Join(home, ".pypirc")
	if info, err := os.Stat(pypirc); err == nil {
		result.Resources = append(result.Resources, module.Resource{
			Name: ".pypirc", Path: pypirc, Size: info.Size(),
			Modified: info.ModTime(), Type: module.ResourceTypeSecret,
		})
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}

	return result, nil
}

func (m *PythonModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home := restoreutil.HomeDir()
	var entries []archive.Entry

	pipConfig := filepath.Join(home, ".config", "pip", "pip.conf")
	if _, err := os.Stat(pipConfig); err == nil {
		entries = append(entries, archive.Entry{
			Source: pipConfig, ArchivePath: ".config/pip/pip.conf",
		})
	}

	pypirc := filepath.Join(home, ".pypirc")
	if _, err := os.Stat(pypirc); err == nil {
		entries = append(entries, archive.Entry{
			Source: pypirc, ArchivePath: ".pypirc",
		})
	}

	if out, err := exec.Command("pip3", "list", "--format=json").Output(); err == nil {
		tmpFile := filepath.Join(os.TempDir(), "getitback-python-packages.json")
		os.WriteFile(tmpFile, out, 0600)
		defer os.Remove(tmpFile)
		entries = append(entries, archive.Entry{
			Source: tmpFile, ArchivePath: "pip-packages.json",
		})
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
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size,
			Checksum: snapshot.Checksum, OriginalSize: snapshot.OriginalSize,
			FileCount: snapshot.FileCount,
		}},
	}, nil
}

func (m *PythonModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	pipDir := filepath.Join(home, ".config", "pip")
	os.MkdirAll(pipDir, 0755)

	entries, _ := os.ReadDir(pipDir)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".getitback-bak") {
			continue
		}
		os.Rename(filepath.Join(pipDir, entry.Name()), filepath.Join(pipDir, entry.Name()+".getitback-bak"))
	}

	pypircPath := filepath.Join(home, ".pypirc")
	if _, err := os.Stat(pypircPath); err == nil {
		os.Rename(pypircPath, pypircPath+".getitback-bak")
	}

	if rt != nil {
		rt.Archive.Extract(snap.Path, home)
	} else {
		archive.Extract(snap.Path, home)
	}

	return nil
}

func (m *PythonModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *PythonModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}
	home := restoreutil.HomeDir()

	pipConf := filepath.Join(home, ".config", "pip", "pip.conf")
	if info, err := os.Stat(pipConf); err == nil {
		if info.Mode().Perm()&0077 != 0 {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "error", Message: "pip.conf has overly permissive permissions",
				Help:     fmt.Sprintf("chmod 600 %s", pipConf),
			})
		}
	}

	pypirc := filepath.Join(home, ".pypirc")
	if info, err := os.Stat(pypirc); err == nil {
		if info.Mode().Perm()&0077 != 0 {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "error", Message: ".pypirc has overly permissive permissions",
				Help:     fmt.Sprintf("chmod 600 %s", pypirc),
			})
		}
	}

	if restoreutil.CommandExists("pip3") {
		if out, err := restoreutil.CheckExecOutput("pip3", "list", "--outdated", "--format=columns"); err == nil {
			lines := strings.Split(out, "\n")
			if len(lines) > 2 {
				result.Issues = append(result.Issues, module.DoctorIssue{
					Severity: "warning", Message: fmt.Sprintf("%d outdated pip packages", len(lines)-2),
					Help: "pip3 list --outdated --format=columns",
				})
			}
		}
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

// --- Enhanced restore interfaces ---

func (m *PythonModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "python3", Hint: "Python 3 runtime"},
		{Type: module.DepSystemPkg, Package: "python3-pip", Hint: "Pip package manager"},
		{Type: module.DepCommand, Package: "pipenv", Hint: "Pipenv (optional)", Optional: true},
		{Type: module.DepCommand, Package: "poetry", Hint: "Poetry (optional)", Optional: true},
	}
}

func (m *PythonModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("python3-pip")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "python3-pip").Run()
}

func (m *PythonModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	os.MkdirAll(filepath.Join(home, ".config", "pip"), 0755)
	os.MkdirAll(filepath.Join(home, ".pyenv"), 0755)
	os.MkdirAll(filepath.Join(home, ".local", "share", "virtualenvs"), 0755)
	return nil
}

func (m *PythonModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("python")

	ver, err := restoreutil.CheckExecOutput("python3", "--version")
	if err == nil {
		v.Version(ver)
	}
	v.Check(restoreutil.CommandExists("python3"), "Python 3 installed")
	v.Check(restoreutil.CommandExists("pip3"), "Pip3 installed")

	home := restoreutil.HomeDir()

	pipConf := filepath.Join(home, ".config", "pip", "pip.conf")
	if restoreutil.FileExists(pipConf) {
		v.Recovered("pip.conf")
	} else {
		v.Warn("No pip.conf found")
	}

	pypirc := filepath.Join(home, ".pypirc")
	if restoreutil.FileExists(pypirc) {
		v.Recovered(".pypirc")
	} else {
		v.Missing(".pypirc")
	}

	if restoreutil.CommandExists("pipenv") {
		v.Recovered("pipenv")
	}
	if restoreutil.CommandExists("poetry") {
		v.Recovered("poetry")
	}

	pkgFile := filepath.Join(home, "pip-packages.json")
	if restoreutil.FileExists(pkgFile) {
		data, err := os.ReadFile(pkgFile)
		if err == nil {
			var expected []struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			}
			if json.Unmarshal(data, &expected) == nil {
				out, err := exec.Command("pip3", "list", "--format=json").Output()
				if err == nil {
					var installed []struct {
						Name    string `json:"name"`
						Version string `json:"version"`
					}
					if json.Unmarshal(out, &installed) == nil {
						installedMap := make(map[string]string)
						for _, p := range installed {
							installedMap[strings.ToLower(p.Name)] = p.Version
						}
						var missing []string
						for _, p := range expected {
							if _, ok := installedMap[strings.ToLower(p.Name)]; !ok {
								missing = append(missing, p.Name)
							}
						}
						if len(missing) == 0 {
							v.Check(true, "All %d pip packages installed", len(expected))
							v.Recovered("pip packages")
						} else {
							v.Missing(fmt.Sprintf("%d pip packages: %s", len(missing), strings.Join(missing, ", ")))
							v.Warn("Package mismatch: %d missing out of %d expected", len(missing), len(expected))
						}
					}
				}
			}
		}
	}

	if restoreutil.CommandExists("python3") && restoreutil.CommandExists("pip3") {
		v.Confidence(85)
	} else {
		v.Confidence(30)
	}

	return v.Result(), nil
}

func (m *PythonModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
		&actions.CreateDirectory{Path: filepath.Join(home, ".config", "pip"), Mode: 0755},
		&restoreUtilAction{
			name: "reinstall_pip_packages",
			desc: "Reinstall global pip packages from backup manifest",
			fn: func(ctx *runtime.RestoreContext) error {
				pkgFile := filepath.Join(home, "pip-packages.json")
				if !restoreutil.FileExists(pkgFile) {
					return nil
				}
				data, err := os.ReadFile(pkgFile)
				if err != nil {
					return err
				}
				var pkgs []struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				}
				if err := json.Unmarshal(data, &pkgs); err != nil {
					return err
				}
				var pkgNames []string
				for _, p := range pkgs {
					pkgNames = append(pkgNames, p.Name)
				}
				if len(pkgNames) == 0 {
					return nil
				}
				return exec.Command("pip3", append([]string{"install"}, pkgNames...)...).Run()
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

func (a *restoreUtilAction) Name() string                       { return a.name }
func (a *restoreUtilAction) Description() string                 { return a.desc }
func (a *restoreUtilAction) Execute(ctx *runtime.RestoreContext) error { return a.fn(ctx) }

var _ actions.Provider = (*PythonModule)(nil)
