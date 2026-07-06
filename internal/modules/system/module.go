package system

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/module"
)

type SystemModule struct{}

func NewModule() *SystemModule {
	return &SystemModule{}
}

func (m *SystemModule) Name() string        { return "system" }
func (m *SystemModule) Description() string { return "Operating system and hardware information" }

func (m *SystemModule) Detect() (bool, error) {
	return runtime.GOOS == "linux", nil
}

func (m *SystemModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{
		Module:   m.Name(),
		Detected: true,
		Version:  runtime.GOOS + " " + osReleaseField("PRETTY_NAME"),
	}

	host, _ := os.Hostname()
	currentUser, _ := user.Current()

	meta := map[string]any{
		"hostname":  host,
		"arch":      runtime.GOARCH,
		"goVersion": runtime.Version(),
	}

	if currentUser != nil {
		meta["user"] = currentUser.Username
		meta["homeDir"] = currentUser.HomeDir
	}

	if osRelease := readOSRelease(); len(osRelease) > 0 {
		if name, ok := osRelease["PRETTY_NAME"]; ok {
			meta["os"] = name
		} else if name, ok := osRelease["NAME"]; ok {
			ver := osRelease["VERSION_ID"]
			if ver != "" {
				meta["os"] = name + " " + ver
			} else {
				meta["os"] = name
			}
		}
		if codename, ok := osRelease["VERSION_CODENAME"]; ok {
			meta["codename"] = codename
		}
	}

	if kernel, err := exec.Command("uname", "-r").Output(); err == nil {
		meta["kernel"] = strings.TrimSpace(string(kernel))
	}

	if de := os.Getenv("XDG_CURRENT_DESKTOP"); de != "" {
		meta["desktop"] = de
	}
	if session := os.Getenv("XDG_SESSION_TYPE"); session != "" {
		meta["session"] = session
	}

	if tz, err := os.ReadFile("/etc/timezone"); err == nil {
		meta["timezone"] = strings.TrimSpace(string(tz))
	} else if tz, err := exec.Command("timedatectl", "show", "-p", "Timezone", "--value").Output(); err == nil {
		meta["timezone"] = strings.TrimSpace(string(tz))
	}

	if lang := os.Getenv("LANG"); lang != "" {
		meta["locale"] = lang
	} else if locale, err := exec.Command("locale").Output(); err == nil {
		for _, line := range strings.Split(string(locale), "\n") {
			if strings.HasPrefix(line, "LANG=") {
				meta["locale"] = strings.TrimPrefix(line, "LANG=")
				break
			}
		}
	}

	if disk, err := exec.Command("df", "-h", "/").Output(); err == nil {
		lines := strings.Split(string(disk), "\n")
		if len(lines) > 1 {
			fields := strings.Fields(lines[1])
			if len(fields) > 3 {
				meta["disk"] = map[string]string{
					"total": fields[1],
					"used":  fields[2],
					"avail": fields[3],
				}
			}
		}
	}

	result.Metadata = meta
	return result, nil
}

func (m *SystemModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	tmpDir, err := os.MkdirTemp("", "getitback-system")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	info := collectSystemInfo()
	infoPath := filepath.Join(tmpDir, "system-info.json")
	f, err := os.Create(infoPath)
	if err != nil {
		return nil, err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(info)
	f.Close()

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), []archive.Entry{
		{Source: infoPath, ArchivePath: "system-info.json"},
	})
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}

	return &module.BackupResult{
		Module: m.Name(),
		Snapshots: []module.Snapshot{
			{
				Module:   m.Name(),
				Path:     snapshot.Path,
				Size:     snapshot.Size,
				Checksum: snapshot.Checksum,
			},
		},
	}, nil
}

func (m *SystemModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	return fmt.Errorf("system information cannot be restored")
}

func (m *SystemModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{
		Module: m.Name(),
		Status: module.DoctorStatusOK,
	}, nil
}

func (m *SystemModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{
			Module: m.Name(),
			Snapshot: snap,
			Valid:   false,
			Errors:  []string{err.Error()},
		}, nil
	}

	if info.Size() == 0 {
		return &module.VerifyResult{
			Module: m.Name(),
			Snapshot: snap,
			Valid:   false,
			Errors:  []string{"snapshot is empty"},
		}, nil
	}

	return &module.VerifyResult{
		Module:   m.Name(),
		Snapshot: snap,
		Valid:    true,
	}, nil
}

func readOSRelease() map[string]string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return nil
	}
	defer f.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
			result[parts[0]] = strings.Trim(parts[1], "\"")
		}
	}
	return result
}

func osReleaseField(field string) string {
	release := readOSRelease()
	if release == nil {
		return "unknown"
	}
	return release[field]
}

type systemInfo struct {
	Hostname  string            `json:"hostname"`
	OS        string            `json:"os"`
	OSRelease map[string]string `json:"osRelease,omitempty"`
	Kernel    string            `json:"kernel,omitempty"`
	CPU       string            `json:"cpu,omitempty"`
	Memory    string            `json:"memory,omitempty"`
	Packages  []string          `json:"packages,omitempty"`
}

func collectSystemInfo() *systemInfo {
	info := &systemInfo{}
	info.Hostname, _ = os.Hostname()
	info.OS = runtime.GOOS + " " + runtime.GOARCH
	info.OSRelease = readOSRelease()

	if kernel, err := exec.Command("uname", "-r").Output(); err == nil {
		info.Kernel = strings.TrimSpace(string(kernel))
	}

	if cpu, err := exec.Command("nproc").Output(); err == nil {
		info.CPU = strings.TrimSpace(string(cpu)) + " cores"
	}

	if mem, err := exec.Command("free", "-h").Output(); err == nil {
		lines := strings.Split(string(mem), "\n")
		if len(lines) > 1 {
			fields := strings.Fields(lines[1])
			if len(fields) > 1 {
				info.Memory = "total: " + fields[1]
			}
		}
	}

	return info
}
