package virtualization

import (
	"context"
	"os/exec"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

type VirtualizationModule struct{}

func NewModule() *VirtualizationModule { return &VirtualizationModule{} }

func (m *VirtualizationModule) Name() string        { return "virtualization" }
func (m *VirtualizationModule) Description() string { return "Virtualization and container platforms" }

type vmPlatform struct {
	name      string
	binary    string
	versionFn func(string) string
	infoFn    func(string) (int, int64)
}

var vmPlatforms = []vmPlatform{
	{name: "Podman", binary: "podman", versionFn: podmanVersion, infoFn: podmanInfo},
	{name: "LXD", binary: "lxc", versionFn: stdVersion, infoFn: lxdInfo},
	{name: "Multipass", binary: "multipass", versionFn: stdVersion, infoFn: multipassInfo},
	{name: "VirtualBox", binary: "vboxmanage", versionFn: stdVersion, infoFn: vboxInfo},
	{name: "QEMU", binary: "qemu-system-x86_64", versionFn: qemuVersion, infoFn: nil},
}

func (m *VirtualizationModule) Detect() (bool, error) {
	for _, p := range vmPlatforms {
		if _, err := exec.LookPath(p.binary); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func (m *VirtualizationModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}
	meta := make(map[string]any)

	var detected []string

	for _, p := range vmPlatforms {
		if _, err := exec.LookPath(p.binary); err != nil {
			continue
		}
		detected = append(detected, p.name)

		ver := ""
		if p.versionFn != nil {
			ver = p.versionFn(p.binary)
		}
		if ver != "" {
			meta[p.name+"Version"] = ver
		}

		if p.infoFn != nil {
			count, storage := p.infoFn(p.binary)
			if count > 0 {
				meta[p.name+"Instances"] = count
			}
			if storage > 0 {
				meta[p.name+"Storage"] = storage
			}
		}
	}

	if len(detected) > 0 {
		meta["detectedPlatforms"] = detected
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}
	return result, nil
}

func podmanVersion(binary string) string {
	out, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func qemuVersion(binary string) string {
	out, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return ""
	}
	first := strings.SplitN(string(out), "\n", 2)[0]
	return strings.TrimSpace(first)
}

func stdVersion(binary string) string {
	out, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return ""
	}
	first := strings.SplitN(string(out), "\n", 2)[0]
	return strings.TrimSpace(first)
}

func podmanInfo(binary string) (int, int64) {
	out, err := exec.Command(binary, "ps", "-a", "--format", "{{.ID}}").Output()
	if err != nil {
		return 0, 0
	}
	count := len(strings.Fields(string(out)))
	return count, 0
}

func lxdInfo(binary string) (int, int64) {
	out, err := exec.Command(binary, "list", "--format", "csv", "-c", "n").Output()
	if err != nil {
		return 0, 0
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, 0
}

func multipassInfo(binary string) (int, int64) {
	out, err := exec.Command(binary, "list", "--format", "json").Output()
	if err != nil {
		return 0, 0
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, `"name"`) {
			count++
		}
	}
	return count, 0
}

func vboxInfo(binary string) (int, int64) {
	out, err := exec.Command(binary, "list", "vms").Output()
	if err != nil {
		return 0, 0
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, 0
}

func (m *VirtualizationModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	return nil, nil
}

func (m *VirtualizationModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	return nil
}

func (m *VirtualizationModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *VirtualizationModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}
	if _, err := exec.LookPath("vboxmanage"); err == nil {
		out, err := exec.Command("vboxmanage", "list", "runningvms").Output()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "info",
				Message:  "VirtualBox VMs are running — ensure they are shut down before backup",
				Help:     "Run: vboxmanage controlvm <name> poweroff",
			})
		}
	}
	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}
