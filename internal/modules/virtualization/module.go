package virtualization

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
		if restoreutil.CommandExists(p.binary) {
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
		if !restoreutil.CommandExists(p.binary) {
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
	out, err := restoreutil.CheckExecOutput(binary, "--version")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func qemuVersion(binary string) string {
	out, err := restoreutil.CheckExecOutput(binary, "--version")
	if err != nil {
		return ""
	}
	first := strings.SplitN(out, "\n", 2)[0]
	return strings.TrimSpace(first)
}

func stdVersion(binary string) string {
	out, err := restoreutil.CheckExecOutput(binary, "--version")
	if err != nil {
		return ""
	}
	first := strings.SplitN(out, "\n", 2)[0]
	return strings.TrimSpace(first)
}

func podmanInfo(binary string) (int, int64) {
	out, err := restoreutil.CheckExecOutput(binary, "ps", "-a", "--format", "{{.ID}}")
	if err != nil {
		return 0, 0
	}
	count := len(strings.Fields(out))
	return count, 0
}

func lxdInfo(binary string) (int, int64) {
	out, err := restoreutil.CheckExecOutput(binary, "list", "--format", "csv", "-c", "n")
	if err != nil {
		return 0, 0
	}
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, 0
}

func multipassInfo(binary string) (int, int64) {
	out, err := restoreutil.CheckExecOutput(binary, "list", "--format", "json")
	if err != nil {
		return 0, 0
	}
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, `"name"`) {
			count++
		}
	}
	return count, 0
}

func vboxInfo(binary string) (int, int64) {
	out, err := restoreutil.CheckExecOutput(binary, "list", "vms")
	if err != nil {
		return 0, 0
	}
	count := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count, 0
}

type virtBackupManifest struct {
	DetectedPlatforms []string `json:"detectedPlatforms"`
	LibvirtDomains    []string `json:"libvirtDomains,omitempty"`
	QEMUConfigs       []string `json:"qemuConfigs,omitempty"`
	VagrantMachines   []string `json:"vagrantMachines,omitempty"`
}

func (m *VirtualizationModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	var manifest virtBackupManifest
	var entries []archive.Entry

	qemuConfigs := []string{
		"/etc/qemu",
		"/usr/local/etc/qemu",
	}

	for _, cfg := range qemuConfigs {
		if info, err := os.Stat(cfg); err == nil && info.IsDir() {
			manifest.QEMUConfigs = append(manifest.QEMUConfigs, cfg)
			entries = append(entries, archive.Entry{
				Source: cfg, ArchivePath: "qemu/" + filepath.Base(cfg),
			})
		}
	}

	home, _ := os.UserHomeDir()
	qemuUserConfig := filepath.Join(home, ".config", "qemu")
	if info, err := os.Stat(qemuUserConfig); err == nil && info.IsDir() {
		manifest.QEMUConfigs = append(manifest.QEMUConfigs, qemuUserConfig)
		entries = append(entries, archive.Entry{
			Source: qemuUserConfig, ArchivePath: "qemu/user-config",
		})
	}

	if restoreutil.CommandExists("virsh") {
		out, err := restoreutil.CheckExecOutput("virsh", "list", "--name", "--all")
		if err == nil {
			for _, domain := range strings.Fields(out) {
				manifest.LibvirtDomains = append(manifest.LibvirtDomains, domain)
				xmlOut, err := exec.Command("virsh", "dumpxml", domain).Output()
				if err != nil {
					continue
				}
				tmpFile := filepath.Join(os.TempDir(), "getitback-virsh-"+domain+".xml")
				os.WriteFile(tmpFile, xmlOut, 0600)
				defer os.Remove(tmpFile)
				entries = append(entries, archive.Entry{
					Source: tmpFile, ArchivePath: "libvirt-domains/" + domain + ".xml",
				})
			}
		}

		libvirtConfigs := []string{
			"/etc/libvirt",
			filepath.Join(home, ".config", "libvirt"),
		}
		for _, cfg := range libvirtConfigs {
			if info, err := os.Stat(cfg); err == nil && info.IsDir() {
				entries = append(entries, archive.Entry{
					Source: cfg, ArchivePath: "libvirt-config/" + filepath.Base(cfg),
				})
			}
		}
	}

	vagrantDir := filepath.Join(home, ".vagrant.d")
	if info, err := os.Stat(vagrantDir); err == nil && info.IsDir() {
		entries = append(entries, archive.Entry{
			Source: vagrantDir, ArchivePath: "vagrant",
		})
	}

	for _, p := range vmPlatforms {
		if restoreutil.CommandExists(p.binary) {
			manifest.DetectedPlatforms = append(manifest.DetectedPlatforms, p.name)
		}
	}

	tmpMeta, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("virtualization: marshal manifest: %w", err)
	}
	metaFile := filepath.Join(os.TempDir(), "getitback-virt-manifest.json")
	if err := os.WriteFile(metaFile, tmpMeta, 0600); err != nil {
		return nil, fmt.Errorf("virtualization: write manifest: %w", err)
	}
	defer os.Remove(metaFile)
	entries = append(entries, archive.Entry{
		Source: metaFile, ArchivePath: "manifest.json",
	})

	if len(entries) == 0 {
		return nil, nil
	}

	snapshot, err := archive.CreateSnapshot(opts.SnapshotsDir, m.Name(), entries)
	if err != nil {
		return nil, err
	}
	if snapshot == nil {
		return nil, nil
	}
	contents := []string{}
	if len(manifest.QEMUConfigs) > 0 {
		contents = append(contents, "QEMU configuration")
	}
	if len(manifest.LibvirtDomains) > 0 {
		contents = append(contents, fmt.Sprintf("libvirt domains (%d)", len(manifest.LibvirtDomains)))
	}
	if len(manifest.DetectedPlatforms) > 0 {
		for _, p := range manifest.DetectedPlatforms {
			contents = append(contents, p)
		}
	}
	return &module.BackupResult{
		Module:    m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size, Checksum: snapshot.Checksum,
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
		Contents: contents,
	}, nil
}

func (m *VirtualizationModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	home, _ := os.UserHomeDir()
	tmpDir, err := os.MkdirTemp("", "getitback-restore-virt-*")
	if err != nil {
		return fmt.Errorf("virtualization: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := archive.Extract(snap.Path, tmpDir); err != nil {
		return fmt.Errorf("virtualization: extract snapshot: %w", err)
	}

	restoreDir := func(src, dst string) {
		if info, err := os.Stat(src); err == nil && info.IsDir() {
			os.MkdirAll(filepath.Dir(dst), 0755)
			exec.Command("cp", "-r", src+"/.", dst).Run()
		}
	}

	restoreDir(filepath.Join(tmpDir, "qemu"), "/etc/qemu")
	restoreDir(filepath.Join(tmpDir, "qemu", "user-config"), filepath.Join(home, ".config", "qemu"))
	restoreDir(filepath.Join(tmpDir, "vagrant"), filepath.Join(home, ".vagrant.d"))

	libvirtDomains := filepath.Join(tmpDir, "libvirt-domains")
	if info, err := os.Stat(libvirtDomains); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(libvirtDomains)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".xml") {
				exec.Command("virsh", "define", filepath.Join(libvirtDomains, e.Name())).Run()
			}
		}
	}

	return nil
}

func (m *VirtualizationModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *VirtualizationModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}
	if restoreutil.CommandExists("vboxmanage") {
		out, err := restoreutil.CheckExecOutput("vboxmanage", "list", "runningvms")
		if err == nil && len(strings.TrimSpace(out)) > 0 {
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

func (m *VirtualizationModule) Dependencies(ctx context.Context) []module.Dependency {
	var deps []module.Dependency
	if restoreutil.CommandExists("virsh") {
		deps = append(deps, module.Dependency{Type: module.DepSystemPkg, Package: "libvirt-daemon-system", Hint: "libvirt"})
	}
	if restoreutil.CommandExists("vagrant") {
		deps = append(deps, module.Dependency{Type: module.DepSystemPkg, Package: "vagrant", Hint: "Vagrant"})
	}
	if restoreutil.CommandExists("vboxmanage") {
		deps = append(deps, module.Dependency{Type: module.DepSystemPkg, Package: "virtualbox", Hint: "VirtualBox"})
	}
	if len(deps) == 0 {
		return nil
	}
	return deps
}

func (m *VirtualizationModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	var pkgs []string
	if restoreutil.CommandExists("virsh") {
		pkgs = append(pkgs, "libvirt-daemon-system")
	}
	if restoreutil.CommandExists("vagrant") {
		pkgs = append(pkgs, "vagrant")
	}
	if restoreutil.CommandExists("vboxmanage") {
		pkgs = append(pkgs, "virtualbox")
	}
	if len(pkgs) == 0 {
		return nil
	}
	if rt != nil {
		return rt.Pkg.Install(pkgs...)
	}
	args := append([]string{"apt-get", "install", "-y", "-qq"}, pkgs...)
	return exec.Command("sudo", args...).Run()
}

func (m *VirtualizationModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	os.MkdirAll(filepath.Join(home, ".config", "qemu"), 0755)
	os.MkdirAll(filepath.Join(home, ".config", "libvirt"), 0755)
	os.MkdirAll(filepath.Join(home, ".vagrant.d"), 0755)
	return nil
}

func (m *VirtualizationModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("virtualization")

	if restoreutil.CommandExists("virsh") {
		v.Recovered("libvirt detected")
	}
	if restoreutil.CommandExists("vagrant") {
		v.Recovered("vagrant detected")
	}
	if restoreutil.CommandExists("vboxmanage") {
		v.Recovered("VirtualBox detected")
	}
	if restoreutil.CommandExists("qemu-system-x86_64") {
		v.Recovered("QEMU detected")
	}

	home := restoreutil.HomeDir()
	if restoreutil.DirExists(filepath.Join(home, ".config", "qemu")) {
		v.Recovered("qemu config restored")
	}
	if restoreutil.DirExists(filepath.Join(home, ".vagrant.d")) {
		v.Recovered("vagrant config restored")
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *VirtualizationModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	home := restoreutil.HomeDir()
	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
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

var _ actions.Provider = (*VirtualizationModule)(nil)
