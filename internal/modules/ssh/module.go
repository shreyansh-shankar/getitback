package ssh

import (
	"context"
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

var privateKeyNames = map[string]bool{
	"id_rsa": true, "id_ecdsa": true, "id_ecdsa_sk": true,
	"id_ed25519": true, "id_ed25519_sk": true, "id_dsa": true, "id_xmss": true,
}

type SSHModule struct{}

func NewModule() *SSHModule { return &SSHModule{} }

func (m *SSHModule) Name() string        { return "ssh" }
func (m *SSHModule) Description() string { return "SSH configuration and keys" }

func (m *SSHModule) Detect() (bool, error) {
	return restoreutil.CommandExists("ssh"), nil
}

func (m *SSHModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true, Metadata: make(map[string]any)}

	if ver, err := restoreutil.CheckExecOutput("ssh", "-V"); err == nil {
		parts := strings.Fields(ver)
		if len(parts) > 0 {
			result.Version = strings.TrimPrefix(parts[0], "OpenSSH_")
		}
	}

	sshDir := filepath.Join(restoreutil.HomeDir(), ".ssh")
	if !restoreutil.DirExists(sshDir) {
		return result, nil
	}

	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return result, nil
	}

	var keys, configs int
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		rtype := module.ResourceTypeConfig
		if isPrivateKey(entry.Name()) {
			keys++
			rtype = module.ResourceTypeSecret
		} else {
			configs++
		}
		result.Resources = append(result.Resources, module.Resource{
			Name: entry.Name(), Path: filepath.Join(sshDir, entry.Name()),
			Size: info.Size(), Modified: info.ModTime(), Type: rtype,
		})
	}
	result.Metadata["keys"] = keys
	result.Metadata["configs"] = configs

	return result, nil
}

func (m *SSHModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home := restoreutil.HomeDir()
	sshDir := filepath.Join(home, ".ssh")
	if !restoreutil.DirExists(sshDir) {
		return nil, nil
	}
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return nil, nil
	}
	var archiveEntries []archive.Entry
	for _, entry := range entries {
		archiveEntries = append(archiveEntries, archive.Entry{
			Source:      filepath.Join(sshDir, entry.Name()),
			ArchivePath: filepath.Join(".ssh", entry.Name()),
		})
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

func (m *SSHModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}
	sshDir := filepath.Join(home, ".ssh")

	os.MkdirAll(sshDir, 0700)

	entries, _ := os.ReadDir(sshDir)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".getitback-bak") {
			continue
		}
		os.Rename(filepath.Join(sshDir, entry.Name()), filepath.Join(sshDir, entry.Name()+".getitback-bak"))
	}

	if rt != nil {
		rt.Archive.Extract(snap.Path, home)
	} else {
		archive.Extract(snap.Path, home)
	}

	restored, _ := os.ReadDir(sshDir)
	for _, entry := range restored {
		if strings.HasSuffix(entry.Name(), ".getitback-bak") {
			continue
		}
		path := filepath.Join(sshDir, entry.Name())
		if isPrivateKey(entry.Name()) {
			os.Chmod(path, 0600)
		} else {
			os.Chmod(path, 0644)
		}
	}
	return nil
}

func (m *SSHModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *SSHModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}
	sshDir := filepath.Join(restoreutil.HomeDir(), ".ssh")

	sshInfo, err := os.Stat(sshDir)
	if err != nil {
		return result, nil
	}
	if sshInfo.Mode().Perm()&0077 != 0 {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "error", Message: ".ssh directory has overly permissive permissions",
			Help: fmt.Sprintf("chmod 700 %s", sshDir),
		})
	}

	entries, _ := os.ReadDir(sshDir)
	for _, entry := range entries {
		if !isPrivateKey(entry.Name()) {
			continue
		}
		info, _ := entry.Info()
		if info == nil {
			continue
		}
		if info.Mode().Perm()&0077 != 0 {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "error", Message: fmt.Sprintf("Private key %s has weak permissions (%o)", entry.Name(), info.Mode().Perm()),
				Help:     fmt.Sprintf("chmod 600 %s", filepath.Join(sshDir, entry.Name())),
			})
		}
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

// --- Enhanced restore interfaces ---

func (m *SSHModule) Dependencies(ctx context.Context) []module.Dependency {
	deps := []module.Dependency{
		{Type: module.DepSystemPkg, Package: "openssh-client", Hint: "SSH client"},
	}
	return deps
}

func (m *SSHModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("openssh-client")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "openssh-client").Run()
}

func (m *SSHModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	exec.Command("bash", "-c", "eval $(ssh-agent) && echo $SSH_AUTH_SOCK").Run()
	sshDir := filepath.Join(restoreutil.HomeDir(), ".ssh")
	if _, err := os.Stat(filepath.Join(sshDir, "config")); os.IsNotExist(err) {
		return nil
	}
	return nil
}

func (m *SSHModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("ssh")

	ver, err := restoreutil.CheckExecOutput("ssh", "-V")
	if err == nil {
		parts := strings.Fields(ver)
		if len(parts) > 0 {
			v.Version(strings.TrimPrefix(parts[0], "OpenSSH_"))
		}
	}
	v.Check(restoreutil.CommandExists("ssh"), "SSH client installed")
	v.Check(restoreutil.CommandExists("ssh-agent"), "SSH agent available")

	sshDir := filepath.Join(restoreutil.HomeDir(), ".ssh")
	dirExists := restoreutil.DirExists(sshDir)
	v.Check(dirExists, "SSH directory exists")

	if dirExists {
		entries, _ := os.ReadDir(sshDir)
		var keyCount, validKeys, pubKeyCount int
		for _, entry := range entries {
			// Skip backup files created during restore
			if strings.HasSuffix(entry.Name(), ".getitback-bak") {
				continue
			}
			if isPrivateKey(entry.Name()) {
				keyCount++
				info, _ := entry.Info()
				if info != nil && info.Mode().Perm()&0077 == 0 {
					validKeys++
					v.Recovered("key: " + entry.Name())
				} else {
					v.Missing("proper permissions: " + entry.Name())
				}
			}
			if strings.HasSuffix(entry.Name(), ".pub") {
				pubKeyCount++
				v.Recovered("public key: " + entry.Name())
			}
		}
		if keyCount > 0 {
			v.Check(validKeys == keyCount, "%d SSH keys with correct permissions", keyCount)
		}
	}

	if restoreutil.FileExists(filepath.Join(sshDir, "config")) {
		v.Recovered("SSH config")
	} else {
		v.Warn("No SSH config file")
	}

	if restoreutil.FileExists(filepath.Join(sshDir, "authorized_keys")) {
		v.Recovered("authorized_keys")
	}

	v.Confidence(90)
	return v.Result(), nil
}

func (m *SSHModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}
	sshDir := filepath.Join(home, ".ssh")

	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
		&actions.CreateDirectory{Path: sshDir, Mode: 0700},
		&restoreUtilAction{
			name: "ssh_permissions",
			desc: "Set SSH key permissions",
			fn: func(ctx *runtime.RestoreContext) error {
				entries, _ := os.ReadDir(sshDir)
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), ".getitback-bak") {
						continue
					}
					path := filepath.Join(sshDir, entry.Name())
					if isPrivateKey(entry.Name()) {
						os.Chmod(path, 0600)
					} else {
						os.Chmod(path, 0644)
					}
				}
				return nil
			},
		},
	}, nil
}

func isPrivateKey(name string) bool {
	// Strip backup suffix if present
	base := name
	if strings.HasSuffix(base, ".getitback-bak") {
		base = strings.TrimSuffix(base, ".getitback-bak")
	}

	// Public key files are never private keys
	if strings.HasSuffix(base, ".pub") {
		return false
	}

	if privateKeyNames[base] {
		return true
	}

	knownFiles := map[string]bool{
		"config": true, "known_hosts": true, "known_hosts.old": true,
		"authorized_keys": true, "authorized_keys2": true,
		"environment": true, "rc": true,
	}
	return !knownFiles[base]
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

var _ actions.Provider = (*SSHModule)(nil)
