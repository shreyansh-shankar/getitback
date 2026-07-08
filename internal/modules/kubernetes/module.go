package kubernetes

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

type KubernetesModule struct{}

func NewModule() *KubernetesModule { return &KubernetesModule{} }

func (m *KubernetesModule) Name() string        { return "kubernetes" }
func (m *KubernetesModule) Description() string { return "Kubernetes configuration and infrastructure tools" }

func (m *KubernetesModule) Detect() (bool, error) {
	return restoreutil.CommandExists("kubectl"), nil
}

func (m *KubernetesModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true, Metadata: make(map[string]any)}

	if ver, err := restoreutil.CheckExecOutput("kubectl", "version", "--client", "--output=json"); err == nil {
		var v struct {
			ClientVersion struct {
				GitVersion string `json:"gitVersion"`
			} `json:"clientVersion"`
		}
		if json.Unmarshal([]byte(ver), &v) == nil {
			result.Version = v.ClientVersion.GitVersion
		}
	}

	if out, err := restoreutil.CheckExecOutput("kubectl", "config", "get-contexts", "--output=name"); err == nil {
		contexts := strings.Fields(out)
		result.Metadata["contexts"] = len(contexts)
		if len(contexts) > 0 {
			result.Metadata["contextNames"] = contexts
		}
		if current, err := restoreutil.CheckExecOutput("kubectl", "config", "current-context"); err == nil {
			result.Metadata["currentContext"] = current
		}
	}

	if out, err := restoreutil.CheckExecOutput("kubectl", "get", "namespaces", "--output=name"); err == nil {
		result.Metadata["namespaces"] = len(strings.Fields(out))
	}

	kubeDir := filepath.Join(restoreutil.HomeDir(), ".kube")
	if restoreutil.DirExists(kubeDir) {
		result.Metadata["kubeconfigDir"] = kubeDir
	}

	if restoreutil.CommandExists("helm") {
		result.Metadata["helm"] = true
		helmDir := filepath.Join(restoreutil.HomeDir(), ".config", "helm")
		if restoreutil.DirExists(helmDir) {
			result.Metadata["helmConfigDir"] = helmDir
		}
	}

	if restoreutil.CommandExists("kind") {
		result.Metadata["kind"] = true
		if ver, err := restoreutil.CheckExecOutput("kind", "version"); err == nil {
			result.Metadata["kindVersion"] = ver
		}
	}

	if restoreutil.CommandExists("minikube") {
		result.Metadata["minikube"] = true
	}

	if restoreutil.CommandExists("terraform") {
		result.Metadata["terraform"] = true
	}

	return result, nil
}

type k8sBackupManifest struct {
	Kubeconfig  string   `json:"kubeconfig"`
	Contexts    []string `json:"contexts"`
	CurrentCtx  string   `json:"currentContext"`
	Namespaces  int      `json:"namespaces"`
	HelmEnabled bool     `json:"helmEnabled"`
	KindEnabled bool     `json:"kindEnabled"`
	Minikube    bool     `json:"minikube"`
	Kubeconfigs []string `json:"kubeconfigFiles,omitempty"`
}

func (m *KubernetesModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	var manifest k8sBackupManifest
	var entries []archive.Entry
	home := restoreutil.HomeDir()

	kubeDir := filepath.Join(home, ".kube")
	if restoreutil.DirExists(kubeDir) {
		entries = append(entries, archive.Entry{
			Source: kubeDir, ArchivePath: ".kube",
		})
		manifest.Kubeconfig = filepath.Join(kubeDir, "config")
	}

	if m, err := restoreutil.CheckExecOutput("kubectl", "config", "get-contexts", "--output=name"); err == nil {
		manifest.Contexts = strings.Fields(m)
	}
	if m, err := restoreutil.CheckExecOutput("kubectl", "config", "current-context"); err == nil {
		manifest.CurrentCtx = m
	}
	if m, err := restoreutil.CheckExecOutput("kubectl", "get", "namespaces", "--output=name"); err == nil {
		manifest.Namespaces = len(strings.Fields(m))
	}

	helmDir := filepath.Join(home, ".config", "helm")
	if restoreutil.CommandExists("helm") {
		manifest.HelmEnabled = true
		if restoreutil.DirExists(helmDir) {
			entries = append(entries, archive.Entry{
				Source: helmDir, ArchivePath: "helm",
			})
		}
	}

	kindDir := filepath.Join(home, ".config", "kind")
	if restoreutil.CommandExists("kind") {
		manifest.KindEnabled = true
		if restoreutil.DirExists(kindDir) {
			entries = append(entries, archive.Entry{
				Source: kindDir, ArchivePath: "kind",
			})
		}
	}

	if restoreutil.CommandExists("minikube") {
		manifest.Minikube = true
		minikubeDir := filepath.Join(home, ".minikube")
		if restoreutil.DirExists(minikubeDir) {
			entries = append(entries, archive.Entry{
				Source: minikubeDir, ArchivePath: "minikube",
			})
		}
	}

	tmpMeta, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("kubernetes: marshal manifest: %w", err)
	}
	metaFile := filepath.Join(os.TempDir(), "getitback-k8s-manifest.json")
	if err := os.WriteFile(metaFile, tmpMeta, 0600); err != nil {
		return nil, fmt.Errorf("kubernetes: write manifest: %w", err)
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
	if manifest.Kubeconfig != "" {
		contents = append(contents, "kubeconfig")
	}
	if len(manifest.Contexts) > 0 {
		contents = append(contents, fmt.Sprintf("contexts (%d)", len(manifest.Contexts)))
	}
	if manifest.HelmEnabled {
		contents = append(contents, "Helm configuration")
	}
	if manifest.KindEnabled {
		contents = append(contents, "Kind configuration")
	}
	if manifest.Minikube {
		contents = append(contents, "Minikube configuration")
	}
	return &module.BackupResult{
		Module: m.Name(),
		Snapshots: []module.Snapshot{{
			Module: m.Name(), Path: snapshot.Path, Size: snapshot.Size, Checksum: snapshot.Checksum,
			OriginalSize: snapshot.OriginalSize, FileCount: snapshot.FileCount,
		}},
		Contents: contents,
	}, nil
}

func (m *KubernetesModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	tmpDir, err := os.MkdirTemp("", "getitback-restore-k8s-*")
	if err != nil {
		return fmt.Errorf("kubernetes: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if rt != nil {
		rt.Archive.Extract(snap.Path, tmpDir)
	} else {
		archive.Extract(snap.Path, tmpDir)
	}

	restoreDir := func(src, dst string) {
		if restoreutil.DirExists(src) {
			os.MkdirAll(filepath.Dir(dst), 0755)
			exec.Command("cp", "-r", src, dst).Run()
		}
	}

	kubeSrc := filepath.Join(tmpDir, ".kube")
	kubeDst := filepath.Join(home, ".kube")
	if restoreutil.DirExists(kubeSrc) {
		os.MkdirAll(home, 0755)
		exec.Command("cp", "-r", kubeSrc, kubeDst).Run()
	}

	restoreDir(filepath.Join(tmpDir, "helm"), filepath.Join(home, ".config", "helm"))
	restoreDir(filepath.Join(tmpDir, "kind"), filepath.Join(home, ".config", "kind"))
	restoreDir(filepath.Join(tmpDir, "minikube"), filepath.Join(home, ".minikube"))

	return nil
}

func (m *KubernetesModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *KubernetesModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}

	if !restoreutil.CommandExists("kubectl") {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "error",
			Message:  "kubectl not installed",
			Help:     "Install kubectl to manage Kubernetes clusters",
		})
	}

	kubeConfig := filepath.Join(restoreutil.HomeDir(), ".kube", "config")
	if !restoreutil.FileExists(kubeConfig) {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "warning",
			Message:  "No kubeconfig found",
			Help:     "Configure kubectl with: kubectl config set-cluster ...",
		})
	}

	if out, err := restoreutil.CheckExecOutput("kubectl", "config", "get-contexts", "--output=name"); err != nil || len(strings.TrimSpace(out)) == 0 {
		result.Issues = append(result.Issues, module.DoctorIssue{
			Severity: "warning",
			Message:  "No Kubernetes contexts configured",
			Help:     "Add a cluster context with: kubectl config set-context ...",
		})
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

func (m *KubernetesModule) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "kubectl", Hint: "Kubernetes CLI"},
		{Type: module.DepSystemPkg, Package: "helm", Hint: "Helm package manager"},
	}
}

func (m *KubernetesModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	if rt != nil {
		return rt.Pkg.Install("kubectl")
	}
	if restoreutil.CommandExists("snap") {
		return exec.Command("sudo", "snap", "install", "kubectl", "--classic").Run()
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq", "kubectl").Run()
}

func (m *KubernetesModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	kubeDir := filepath.Join(restoreutil.HomeDir(), ".kube")
	if !restoreutil.DirExists(kubeDir) {
		return os.MkdirAll(kubeDir, 0700)
	}
	return nil
}

func (m *KubernetesModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("kubernetes")

	ver, err := restoreutil.CheckExecOutput("kubectl", "version", "--client", "--output=json")
	if err == nil {
		var cv struct {
			ClientVersion struct {
				GitVersion string `json:"gitVersion"`
			} `json:"clientVersion"`
		}
		if json.Unmarshal([]byte(ver), &cv) == nil {
			v.Version(cv.ClientVersion.GitVersion)
		}
	}

	v.Check(restoreutil.CommandExists("kubectl"), "kubectl installed")
	v.Check(restoreutil.CommandExists("helm"), "helm installed")

	kubeConfig := filepath.Join(restoreutil.HomeDir(), ".kube", "config")
	if restoreutil.FileExists(kubeConfig) {
		v.Recovered("kubeconfig")
	} else {
		v.Missing("kubeconfig")
	}

	kubeDir := filepath.Join(restoreutil.HomeDir(), ".kube")
	if restoreutil.DirExists(kubeDir) {
		v.Recovered(".kube directory")
	} else {
		v.Warn("No .kube directory found")
	}

	if restoreutil.CommandExists("helm") {
		if out, err := restoreutil.CheckExecOutput("helm", "repo", "list"); err == nil {
			if strings.Contains(out, "stable") || strings.Contains(out, "bitnami") {
				v.Recovered("Helm repositories configured")
			} else {
				v.Warn("Helm installed but no standard repos found")
			}
		} else {
			v.Warn("Unable to list Helm repos")
		}
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *KubernetesModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}
	kubeDir := filepath.Join(home, ".kube")

	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
		&actions.CreateDirectory{Path: kubeDir, Mode: 0700},
		&restoreUtilAction{
			name: "kubeconfig_restore",
			desc: "Restore kubeconfig with backup handling",
			fn: func(ctx *runtime.RestoreContext) error {
				configPath := filepath.Join(kubeDir, "config")
				if restoreutil.FileExists(configPath) {
					os.Chmod(configPath, 0600)
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

func (a *restoreUtilAction) Name() string                          { return a.name }
func (a *restoreUtilAction) Description() string                    { return a.desc }
func (a *restoreUtilAction) Execute(ctx *runtime.RestoreContext) error { return a.fn(ctx) }

var _ actions.Provider = (*KubernetesModule)(nil)
