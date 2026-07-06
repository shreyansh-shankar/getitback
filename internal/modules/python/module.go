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
)

type PythonModule struct{}

func NewModule() *PythonModule { return &PythonModule{} }

func (m *PythonModule) Name() string        { return "python" }
func (m *PythonModule) Description() string { return "Python runtime and pip packages" }

func (m *PythonModule) Detect() (bool, error) {
	if _, err := exec.LookPath("python3"); err == nil {
		return true, nil
	}
	_, err := exec.LookPath("python")
	return err == nil, nil
}

func (m *PythonModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}
	home, _ := os.UserHomeDir()

	pythonCmd := "python3"
	if _, err := exec.LookPath(pythonCmd); err != nil {
		pythonCmd = "python"
	}

	if ver, err := exec.Command(pythonCmd, "--version").Output(); err == nil {
		result.Version = strings.TrimSpace(string(ver))
	}

	meta := make(map[string]any)

	if ver, err := exec.Command("pip3", "--version").Output(); err == nil {
		meta["pip"] = strings.TrimSpace(string(ver))
	} else if ver, err := exec.Command("pip", "--version").Output(); err == nil {
		meta["pip"] = strings.TrimSpace(string(ver))
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

	if _, err := exec.LookPath("poetry"); err == nil {
		meta["poetry"] = true
	}
	if _, err := exec.LookPath("pipx"); err == nil {
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
	if _, err := exec.LookPath("uv"); err == nil {
		meta["uv"] = true
	}
	if _, err := exec.LookPath("virtualenv"); err == nil {
		meta["virtualenv"] = true
	}
	if _, err := os.Stat(filepath.Join(home, ".virtualenvs")); err == nil {
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
	home, _ := os.UserHomeDir()
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
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size, Checksum: snapshot.Checksum,
		}},
	}, nil
}

func (m *PythonModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp("", "getitback-restore-python-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("extract: %w", err)
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
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}
