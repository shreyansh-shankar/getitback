package kubernetes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

type KubernetesModule struct{}

func NewModule() *KubernetesModule { return &KubernetesModule{} }

func (m *KubernetesModule) Name() string        { return "kubernetes" }
func (m *KubernetesModule) Description() string { return "Kubernetes and infrastructure tools" }

type infraTool struct {
	name      string
	binary    string
	versionFn func(string) string
}

var infraTools = []infraTool{
	{name: "kubectl", binary: "kubectl", versionFn: stdVersion},
	{name: "Helm", binary: "helm", versionFn: stdVersion},
	{name: "Minikube", binary: "minikube", versionFn: stdVersion},
	{name: "Kind", binary: "kind", versionFn: stdVersion},
	{name: "k3d", binary: "k3d", versionFn: stdVersion},
	{name: "Terraform", binary: "terraform", versionFn: stdVersion},
	{name: "OpenTofu", binary: "tofu", versionFn: stdVersion},
	{name: "Ansible", binary: "ansible", versionFn: ansibleVersion},
}

func (m *KubernetesModule) Detect() (bool, error) {
	for _, t := range infraTools {
		if _, err := exec.LookPath(t.binary); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func (m *KubernetesModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}
	meta := make(map[string]any)

	var detectedTools []string

	for _, t := range infraTools {
		if _, err := exec.LookPath(t.binary); err != nil {
			continue
		}
		detectedTools = append(detectedTools, t.name)
		ver := ""
		if t.versionFn != nil {
			ver = t.versionFn(t.binary)
		}
		if ver != "" {
			meta[t.name+"Version"] = ver
		}
	}

	if len(detectedTools) > 0 {
		meta["detectedTools"] = detectedTools
	}

	// kubectl specific info
	if _, err := exec.LookPath("kubectl"); err == nil {
		contexts, currentCtx := getKubeContexts()
		if len(contexts) > 0 {
			meta["kubeContexts"] = contexts
			meta["currentContext"] = currentCtx
		}

		namespaces := getNamespaces()
		if len(namespaces) > 0 {
			meta["namespaces"] = namespaces
		}
	}

	// Helm specific
	if _, err := exec.LookPath("helm"); err == nil {
		repos := getHelmRepos()
		if len(repos) > 0 {
			meta["helmRepos"] = repos
		}
	}

	// Terraform specific
	if _, err := exec.LookPath("terraform"); err == nil {
		if cache := getTerraformCache(); cache != "" {
			meta["terraformPluginCache"] = cache
		}
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}
	return result, nil
}

func stdVersion(binary string) string {
	out, err := exec.Command(binary, "version").Output()
	if err != nil {
		return ""
	}
	first := strings.SplitN(string(out), "\n", 2)[0]
	return strings.TrimSpace(first)
}

func ansibleVersion(binary string) string {
	out, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return ""
	}
	first := strings.SplitN(string(out), "\n", 2)[0]
	return strings.TrimSpace(first)
}

func getKubeContexts() ([]string, string) {
	out, err := exec.Command("kubectl", "config", "get-contexts", "-o", "name").Output()
	if err != nil {
		return nil, ""
	}
	contexts := strings.Fields(string(out))
	if len(contexts) == 0 {
		return nil, ""
	}

	current, err := exec.Command("kubectl", "config", "current-context").Output()
	if err != nil {
		return contexts, ""
	}
	return contexts, strings.TrimSpace(string(current))
}

func getNamespaces() []string {
	out, err := exec.Command("kubectl", "get", "namespaces", "-o", "name").Output()
	if err != nil {
		return nil
	}
	var ns []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			ns = append(ns, strings.TrimPrefix(line, "namespace/"))
		}
	}
	return ns
}

func getHelmRepos() []string {
	out, err := exec.Command("helm", "repo", "list", "-q").Output()
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}

func getTerraformCache() string {
	home, _ := os.UserHomeDir()
	candidates := []string{
		home + "/.terraform.d/plugin-cache",
		"/usr/share/terraform/plugins",
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}

func (m *KubernetesModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	return nil, nil
}

func (m *KubernetesModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	return nil
}

func (m *KubernetesModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *KubernetesModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}

	if _, err := exec.LookPath("kubectl"); err == nil {
		currentCtx, _ := exec.Command("kubectl", "config", "current-context").Output()
		ctxStr := strings.TrimSpace(string(currentCtx))
		if ctxStr != "" && ctxStr != "minikube" && ctxStr != "kind-kind" {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "info",
				Message:  fmt.Sprintf("Current kubectl context is %q — verify this is intentional", ctxStr),
				Help:     "Run: kubectl config use-context <context> to switch contexts",
			})
		}
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}
