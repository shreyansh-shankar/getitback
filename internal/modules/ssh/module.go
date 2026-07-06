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
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")
	info, err := os.Stat(sshDir)
	if err != nil {
		return false, nil
	}
	return info.IsDir(), nil
}

func (m *SSHModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{
		Module:   m.Name(),
		Detected: true,
	}

	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")

	if ver, err := exec.Command("ssh", "-V").CombinedOutput(); err == nil {
		parts := strings.Fields(string(ver))
		if len(parts) > 0 {
			result.Version = strings.TrimPrefix(parts[0], "OpenSSH_")
		}
	}

	entries, err := os.ReadDir(sshDir)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, nil
	}

	var identityNames []string
	var configFile, knownHosts, authKeys string

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := entry.Name()
		path := filepath.Join(sshDir, name)

		if isPrivateKey(name) {
			identityNames = append(identityNames, name)
			result.Resources = append(result.Resources, module.Resource{
				Name: name, Path: path, Size: info.Size(),
				Modified: info.ModTime(), Type: module.ResourceTypeSecret,
			})
			continue
		}

		resType := module.ResourceTypeConfig
		if name == "known_hosts" || name == "authorized_keys" {
			resType = module.ResourceTypeData
		}
		if name == "config" {
			configFile = path
		}
		if name == "known_hosts" {
			knownHosts = path
		}
		if name == "authorized_keys" {
			authKeys = path
		}
		result.Resources = append(result.Resources, module.Resource{
			Name: name, Path: path, Size: info.Size(),
			Modified: info.ModTime(), Type: resType,
		})
	}

	meta := make(map[string]any)
	meta["identityCount"] = len(identityNames)
	if len(identityNames) > 0 {
		meta["identities"] = identityNames
	}
	if configFile != "" {
		meta["config"] = configFile
	}
	if knownHosts != "" {
		meta["knownHosts"] = knownHosts
	}
	if authKeys != "" {
		meta["authorizedKeys"] = authKeys
	}
	result.Metadata = meta

	return result, nil
}

func (m *SSHModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")

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

func (m *SSHModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")

	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("create .ssh dir: %w", err)
	}

	entries, err := os.ReadDir(sshDir)
	if err == nil {
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".getitback-bak") {
				continue
			}
			oldPath := filepath.Join(sshDir, entry.Name())
			bakPath := oldPath + ".getitback-bak"
			os.Rename(oldPath, bakPath)
		}
	}

	if err := archive.Extract(snap.Path, home); err != nil {
		return fmt.Errorf("extract ssh snapshot: %w", err)
	}

	restoredEntries, _ := os.ReadDir(sshDir)
	for _, entry := range restoredEntries {
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
	result := &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}

	home, _ := os.UserHomeDir()
	sshDir := filepath.Join(home, ".ssh")

	sshInfo, err := os.Stat(sshDir)
	if err != nil {
		return result, nil
	}

	if sshInfo.Mode().Perm()&0077 != 0 {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "error",
			Message:  ".ssh directory has overly permissive permissions",
			Help:     fmt.Sprintf("Run: chmod 700 %s", sshDir),
		})
	}

	entries, err := os.ReadDir(sshDir)
	if err != nil {
		return result, nil
	}

	for _, entry := range entries {
		if !isPrivateKey(entry.Name()) {
			continue
		}
		info, _ := entry.Info()
		if info == nil {
			continue
		}
		perm := info.Mode().Perm()
		if perm&0077 != 0 {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "error",
				Message:  fmt.Sprintf("Private key %s has weak permissions (%o)", entry.Name(), perm),
				Help:     fmt.Sprintf("Run: chmod 600 %s", filepath.Join(sshDir, entry.Name())),
			})
		}

		pubName := entry.Name() + ".pub"
		if _, err := os.Stat(filepath.Join(sshDir, pubName)); os.IsNotExist(err) {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "warning",
				Message:  fmt.Sprintf("Private key %s has no matching public key %s", entry.Name(), pubName),
				Help:     fmt.Sprintf("Generate with: ssh-keygen -y -f ~/.ssh/%s > ~/.ssh/%s.pub", entry.Name(), entry.Name()),
			})
		}
	}

	configPath := filepath.Join(sshDir, "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "warning",
			Message:  "No SSH config file found",
			Help:     "Create ~/.ssh/config for organized host configuration",
		})
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}

	return result, nil
}

func isPrivateKey(name string) bool {
	if privateKeyNames[name] {
		return true
	}
	if strings.HasSuffix(name, ".pub") {
		return false
	}
	knownFiles := map[string]bool{
		"config": true, "known_hosts": true, "known_hosts.old": true,
		"authorized_keys": true, "authorized_keys2": true,
		"environment": true, "rc": true,
	}
	return !knownFiles[name]
}
