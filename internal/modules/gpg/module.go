package gpg

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

type GPGModule struct{}

func NewModule() *GPGModule { return &GPGModule{} }

func (m *GPGModule) Name() string        { return "gpg" }
func (m *GPGModule) Description() string { return "GnuPG encryption keys and configuration" }

func (m *GPGModule) Detect() (bool, error) {
	return restoreutil.CommandExists("gpg"), nil
}

func (m *GPGModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}

	if ver, err := restoreutil.CheckExecOutput("gpg", "--version"); err == nil {
		lines := strings.Split(ver, "\n")
		if len(lines) > 0 {
			result.Version = lines[0]
		}
	}

	gnupgDir := filepath.Join(restoreutil.HomeDir(), ".gnupg")
	if info, err := os.Stat(gnupgDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(gnupgDir)
		for _, entry := range entries {
			info, _ := entry.Info()
			if info == nil {
				continue
			}
			resType := "config"
			if strings.HasPrefix(entry.Name(), "private-keys") || entry.Name() == "secring.gpg" {
				resType = "secret"
			}
			result.Resources = append(result.Resources, module.Resource{
				Name: entry.Name(), Path: filepath.Join(gnupgDir, entry.Name()),
				Size: info.Size(), Modified: info.ModTime(), Type: resType,
			})
		}
	}

	out, err := exec.Command("gpg", "--list-keys", "--keyid-format", "LONG").Output()
	if err == nil {
		var keys []string
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "pub") || strings.HasPrefix(line, "sec") {
				fields := strings.Fields(line)
				if len(fields) > 1 {
					keys = append(keys, fields[1])
				}
			}
		}
		result.Metadata = map[string]any{"keys": keys, "keyCount": len(keys)}
	}

	return result, nil
}

func (m *GPGModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	gnupgDir := filepath.Join(restoreutil.HomeDir(), ".gnupg")
	if _, err := os.Stat(gnupgDir); err != nil {
		return nil, nil
	}
	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: gnupgDir, ArchivePath: ".gnupg"},
	})
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

func (m *GPGModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	gnupgDir := filepath.Join(home, ".gnupg")
	os.MkdirAll(gnupgDir, 0700)

	if info, err := os.Stat(gnupgDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(gnupgDir)
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".getitback-bak") {
				os.Rename(filepath.Join(gnupgDir, entry.Name()), filepath.Join(gnupgDir, entry.Name()+".getitback-bak"))
			}
		}
	}

	if err := archive.Extract(snap.Path, home); err != nil {
		return fmt.Errorf("extract gpg snapshot: %w", err)
	}

	filepath.Walk(gnupgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		os.Chmod(path, 0600)
		return nil
	})
	return nil
}

func (m *GPGModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *GPGModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}
	out, err := exec.Command("gpg", "--list-keys", "--keyid-format", "LONG").Output()
	if err != nil {
		return result, nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "pub") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[2] == "RSA" || fields[2] == "rsa1024" || fields[2] == "dsa1024" {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "warning", Message: fmt.Sprintf("key %s uses weak algorithm: %s", fields[1], fields[2]),
				Help: "Consider generating a new key with ed25519: gpg --full-generate-key",
			})
		}
	}
	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

// --- Enhanced restore interfaces ---

func (m *GPGModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "gnupg", Hint: "GnuPG"},
	}
}

func (m *GPGModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("gnupg")
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "gnupg").Run()
}

func (m *GPGModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	gnupgDir := filepath.Join(restoreutil.HomeDir(), ".gnupg")
	os.MkdirAll(gnupgDir, 0700)
	return nil
}

func (m *GPGModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("gpg")

	if restoreutil.CommandExists("gpg") {
		ver, err := restoreutil.CheckExecOutput("gpg", "--version")
		if err == nil {
			lines := strings.Split(ver, "\n")
			if len(lines) > 0 {
				v.Version(lines[0])
			}
		}
	}
	v.Check(restoreutil.CommandExists("gpg"), "GnuPG installed")

	gnupgDir := filepath.Join(restoreutil.HomeDir(), ".gnupg")
	dirExists := restoreutil.DirExists(gnupgDir)
	v.Check(dirExists, "GNUPG directory exists")

	if dirExists {
		if info, err := os.Stat(gnupgDir); err == nil {
			if info.Mode().Perm() != 0700 {
				v.Warn(".gnupg permissions should be 0700")
			}
		}
	}

	out, err := exec.Command("gpg", "--list-keys", "--keyid-format", "LONG").Output()
	if err == nil {
		var pubKeys, secKeys int
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "pub") {
				pubKeys++
			}
			if strings.HasPrefix(line, "sec") {
				secKeys++
			}
		}
		if pubKeys > 0 {
			v.Recovered(fmt.Sprintf("%d public keys", pubKeys))
		}
		if secKeys > 0 {
			v.Recovered(fmt.Sprintf("%d secret keys", secKeys))
		}
		v.Check(secKeys > 0, "Secret keys present")
	} else {
		// gpg --list-keys might fail if gpg-agent is stale or permissions are wrong
		// Try killing gpg-agent and re-trying
		exec.Command("gpgconf", "--kill", "gpg-agent").Run()
		out2, err2 := exec.Command("gpg", "--list-keys", "--keyid-format", "LONG").Output()
		if err2 == nil {
			var pubKeys, secKeys int
			for _, line := range strings.Split(string(out2), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "pub") {
					pubKeys++
				}
				if strings.HasPrefix(line, "sec") {
					secKeys++
				}
			}
			if pubKeys > 0 {
				v.Recovered(fmt.Sprintf("%d public keys", pubKeys))
			}
			if secKeys > 0 {
				v.Recovered(fmt.Sprintf("%d secret keys", secKeys))
			}
			v.Check(secKeys > 0, "Secret keys present")
		} else {
			v.Warn("Could not list GPG keys after agent restart: %v", err)
		}
	}

	exec.Command("gpg", "--check-trustdb").Run()
	v.Recovered("trustdb verified")

	v.Confidence(85)
	return v.Result(), nil
}

func (m *GPGModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()
	gnupgDir := filepath.Join(home, ".gnupg")

	return []actions.Action{
		&actions.CreateDirectory{Path: gnupgDir, Mode: 0700},
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
		&gpgPermissionAction{name: "gpg_permissions", desc: "Set GPG directory permissions"},
		&gpgRestartAgentAction{name: "gpg_restart_agent", desc: "Restart gpg-agent to pick up restored keys"},
	}, nil
}

type gpgPermissionAction struct {
	actions.BaseAction
	name string
	desc string
}

func (a *gpgPermissionAction) Name() string        { return a.name }
func (a *gpgPermissionAction) Description() string  { return a.desc }
func (a *gpgPermissionAction) Execute(ctx *runtime.RestoreContext) error {
	gnupgDir := filepath.Join(restoreutil.HomeDir(), ".gnupg")
	return filepath.Walk(gnupgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.Chmod(path, 0700)
		}
		return os.Chmod(path, 0600)
	})
}

type gpgRestartAgentAction struct {
	actions.BaseAction
	name string
	desc string
}

func (a *gpgRestartAgentAction) Name() string        { return a.name }
func (a *gpgRestartAgentAction) Description() string  { return a.desc }
func (a *gpgRestartAgentAction) Execute(ctx *runtime.RestoreContext) error {
	exec.Command("gpgconf", "--kill", "gpg-agent").Run()
	return nil
}

var _ actions.Provider = (*GPGModule)(nil)
