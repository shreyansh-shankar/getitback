package node

import (
	"context"
	"encoding/json"
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

type NodeModule struct{}

func NewModule() *NodeModule { return &NodeModule{} }

func (m *NodeModule) Name() string        { return "nodejs" }
func (m *NodeModule) Description() string { return "Node.js runtime and npm packages" }

func (m *NodeModule) Detect() (bool, error) {
	return restoreutil.CommandExists("node"), nil
}

func (m *NodeModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true, Metadata: make(map[string]any)}

	if ver, err := restoreutil.CheckExecOutput("node", "--version"); err == nil {
		result.Version = ver
	}

	if ver, err := restoreutil.CheckExecOutput("npm", "--version"); err == nil {
		result.Metadata["npm"] = ver
	}
	if ver, err := restoreutil.CheckExecOutput("pnpm", "--version"); err == nil {
		result.Metadata["pnpm"] = ver
	}
	if ver, err := restoreutil.CheckExecOutput("yarn", "--version"); err == nil {
		result.Metadata["yarn"] = ver
	}
	if ver, err := restoreutil.CheckExecOutput("corepack", "--version"); err == nil {
		result.Metadata["corepack"] = ver
	}
	if ver, err := restoreutil.CheckExecOutput("bun", "--version"); err == nil {
		result.Metadata["bun"] = ver
	}

	if out, err := restoreutil.CheckExecOutput("npm", "list", "-g", "--depth=0"); err == nil {
		lines := strings.Split(out, "\n")
		var pkgs int
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "+--") || strings.HasPrefix(line, "└──") {
				pkgs++
			}
		}
		result.Metadata["globalPackages"] = pkgs
	}

	home := restoreutil.HomeDir()
	for _, config := range []string{".npmrc", ".yarnrc", ".nvmrc", ".node-version"} {
		path := filepath.Join(home, config)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			result.Resources = append(result.Resources, module.Resource{
				Name: config, Path: path, Size: info.Size(),
				Modified: info.ModTime(), Type: module.ResourceTypeConfig,
			})
		}
	}

	return result, nil
}

func (m *NodeModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home := restoreutil.HomeDir()
	var entries []archive.Entry
	for _, config := range []string{".npmrc", ".yarnrc", ".nvmrc", ".node-version"} {
		path := filepath.Join(home, config)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			entries = append(entries, archive.Entry{
				Source: path, ArchivePath: config,
			})
		}
	}
	if out, err := exec.Command("npm", "list", "-g", "--depth=0", "--json").Output(); err == nil {
		tmpFile := filepath.Join(os.TempDir(), "getitback-nodejs-packages.json")
		os.WriteFile(tmpFile, out, 0600)
		defer os.Remove(tmpFile)
		entries = append(entries, archive.Entry{
			Source: tmpFile, ArchivePath: "npm-global.json",
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

func (m *NodeModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	tmpDir, err := os.MkdirTemp("", "getitback-restore-nodejs-*")
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
		if rel == "npm-global.json" {
			return nil
		}
		dst := filepath.Join(home, rel)
		if _, err := os.Stat(dst); err == nil {
			os.Rename(dst, dst+".getitback-bak")
		}
		os.MkdirAll(filepath.Dir(dst), 0755)
		data, _ := os.ReadFile(path)
		return os.WriteFile(dst, data, 0644)
	})
}

func (m *NodeModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *NodeModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}

	if !restoreutil.CommandExists("node") {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "error", Message: "Node.js is not installed",
			Help:     "Install Node.js via nvm, apt, or official installer",
		})
	}
	if !restoreutil.CommandExists("npm") {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "warning", Message: "npm is not available",
			Help:     "npm should be bundled with Node.js",
		})
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

// --- Enhanced restore interfaces ---

func (m *NodeModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "nodejs", Hint: "Node.js runtime"},
		{Type: module.DepCommand, Command: "npm", Hint: "npm package manager", Optional: false},
		{Type: module.DepCommand, Command: "yarn", Hint: "Yarn package manager", Optional: true},
		{Type: module.DepCommand, Command: "pnpm", Hint: "pnpm package manager", Optional: true},
	}
}

func (m *NodeModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("nodejs")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "nodejs").Run()
}

func (m *NodeModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()

	npmDir := filepath.Join(home, ".npm-global")
	if err := os.MkdirAll(npmDir, 0755); err == nil {
		exec.Command("npm", "config", "set", "prefix", npmDir).Run()
	}

	if restoreutil.CommandExists("yarn") {
		exec.Command("yarn", "config", "set", "global-folder", filepath.Join(home, ".yarn-global")).Run()
	}

	return nil
}

func (m *NodeModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("nodejs")

	nodeVer, err := restoreutil.CheckExecOutput("node", "--version")
	if err == nil {
		v.Version(nodeVer)
	}
	v.Check(restoreutil.CommandExists("node"), "Node.js installed")
	v.Check(restoreutil.CommandExists("npm"), "npm available")

	if snap.Path != "" {
		tmpDir, err := os.MkdirTemp("", "getitback-validate-nodejs-*")
		if err == nil {
			defer os.RemoveAll(tmpDir)
			archive.Extract(snap.Path, tmpDir)
			pkgPath := filepath.Join(tmpDir, "npm-global.json")
			if data, err := os.ReadFile(pkgPath); err == nil {
				var pkgList struct {
					Dependencies map[string]struct {
						Version string `json:"version"`
					} `json:"dependencies"`
				}
				if json.Unmarshal(data, &pkgList) == nil {
					for name := range pkgList.Dependencies {
						if restoreutil.CommandExists(name) {
							v.Recovered("global package: " + name)
						} else {
							v.Missing("global package: " + name)
						}
					}
				}
			}
			for _, config := range []string{".npmrc", ".yarnrc", ".nvmrc", ".node-version"} {
				if restoreutil.FileExists(filepath.Join(tmpDir, config)) {
					if restoreutil.FileExists(filepath.Join(restoreutil.HomeDir(), config)) {
						v.Recovered(config)
					} else {
						v.Missing(config)
					}
				}
			}
		}
	}

	v.Check(restoreutil.CommandExists("node"), "Node.js runtime present")
	v.Confidence(90)
	return v.Result(), nil
}

func (m *NodeModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
		&restoreUtilAction{
			name: "install_global_packages",
			desc: "Reinstall global npm packages from snapshot",
			fn: func(ctx *runtime.RestoreContext) error {
				tmpDir, err := os.MkdirTemp("", "getitback-npm-global-*")
				if err != nil {
					return err
				}
				defer os.RemoveAll(tmpDir)

				if rt != nil {
					rt.Archive.Extract(snap.Path, tmpDir)
				} else {
					archive.Extract(snap.Path, tmpDir)
				}

				data, err := os.ReadFile(filepath.Join(tmpDir, "npm-global.json"))
				if err != nil {
					return nil
				}

				var pkgList struct {
					Dependencies map[string]struct {
						Version string `json:"version"`
					} `json:"dependencies"`
				}
				if json.Unmarshal(data, &pkgList) != nil {
					return nil
				}

				var pkgArgs []string
				for name, info := range pkgList.Dependencies {
					if info.Version != "" {
						pkgArgs = append(pkgArgs, name+"@"+info.Version)
					} else {
						pkgArgs = append(pkgArgs, name)
					}
				}
				if len(pkgArgs) == 0 {
					return nil
				}

				args := append([]string{"install", "-g"}, pkgArgs...)
				cmd := exec.Command("npm", args...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				return cmd.Run()
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

var _ actions.Provider = (*NodeModule)(nil)
