package cloud

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

type CloudModule struct{}

func NewModule() *CloudModule { return &CloudModule{} }

func (m *CloudModule) Name() string        { return "cloud" }
func (m *CloudModule) Description() string { return "Cloud CLI tools and authentication status" }

type cloudCLI struct {
	name      string
	binary    string
	versionFn func(string) string
	authFn    func(string) (bool, string)
	configFn  func(string) string
}

var cloudCLIs = []cloudCLI{
	{name: "AWS CLI", binary: "aws", versionFn: stdVersion, authFn: awsAuth, configFn: awsConfigPath},
	{name: "Azure CLI", binary: "az", versionFn: stdVersion, authFn: azureAuth, configFn: stdConfigPath},
	{name: "Google Cloud SDK", binary: "gcloud", versionFn: gcloudVersion, authFn: gcloudAuth, configFn: stdConfigPath},
	{name: "GitHub CLI", binary: "gh", versionFn: stdVersion, authFn: ghAuth, configFn: ghConfigPath},
	{name: "Vercel CLI", binary: "vercel", versionFn: stdVersion, authFn: vercelAuth, configFn: stdConfigPath},
	{name: "Netlify CLI", binary: "netlify", versionFn: stdVersion, authFn: netlifyAuth, configFn: stdConfigPath},
	{name: "Cloudflare Wrangler", binary: "wrangler", versionFn: wranglerVersion, authFn: wranglerAuth, configFn: stdConfigPath},
	{name: "Supabase CLI", binary: "supabase", versionFn: stdVersion, authFn: basicCheck, configFn: stdConfigPath},
	{name: "Fly.io CLI", binary: "flyctl", versionFn: stdVersion, authFn: flyAuth, configFn: stdConfigPath},
	{name: "Railway CLI", binary: "railway", versionFn: stdVersion, authFn: basicCheck, configFn: stdConfigPath},
}

func (m *CloudModule) Detect() (bool, error) {
	for _, cli := range cloudCLIs {
		if restoreutil.CommandExists(cli.binary) {
			return true, nil
		}
	}
	return false, nil
}

func (m *CloudModule) Inventory(ctx context.Context) (*module.InventoryResult, error) {
	result := &module.InventoryResult{Module: m.Name(), Detected: true}
	meta := make(map[string]any)

	var detected []string
	var details []string

	for _, cli := range cloudCLIs {
		if !restoreutil.CommandExists(cli.binary) {
			continue
		}
		detected = append(detected, cli.name)

		ver := ""
		if cli.versionFn != nil {
			ver = cli.versionFn(cli.binary)
		}
		auth, account := cli.authFn(cli.binary)
		config := cli.configFn(cli.binary)

		detail := cli.name
		if ver != "" {
			detail += " " + ver
		}
		if auth {
			detail += " (authenticated"
			if account != "" {
				detail += ": " + account
			}
			detail += ")"
		} else {
			detail += " (not authenticated)"
		}
		details = append(details, detail)

		safeKey := strings.ReplaceAll(strings.ToLower(cli.name), " ", "_")
		meta[safeKey+"_auth"] = map[bool]string{true: "yes", false: "no"}[auth]
		meta[safeKey+"_cli"] = cli.binary
		if account != "" {
			meta[safeKey+"_account"] = account
		}
		if config != "" {
			meta[safeKey+"_config"] = config
		}
		if auth && config != "" {
			if restoreutil.FileExists(config) || restoreutil.DirExists(config) {
				meta[safeKey+"_credentials"] = "present"
			}
		}
	}

	if len(detected) > 0 {
		meta["detectedCLIs"] = detected
	}

	if len(meta) > 0 {
		result.Metadata = meta
	}
	return result, nil
}

func stdVersion(binary string) string {
	out, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return ""
	}
	first := strings.SplitN(string(out), "\n", 2)[0]
	return strings.TrimSpace(first)
}

func gcloudVersion(binary string) string {
	out, err := exec.Command(binary, "--version").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Google Cloud SDK") {
			return strings.TrimSpace(line)
		}
	}
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
}

func wranglerVersion(binary string) string {
	out, err := exec.Command(binary, "--version").Output()
	if err != nil {
		out, err = exec.Command(binary, "version").Output()
		if err != nil {
			return ""
		}
	}
	return strings.TrimSpace(string(out))
}

func awsAuth(binary string) (bool, string) {
	out, err := exec.Command(binary, "sts", "get-caller-identity", "--output", "text").Output()
	if err != nil {
		return false, ""
	}
	fields := strings.Fields(string(out))
	if len(fields) >= 1 {
		return true, fields[0]
	}
	return true, ""
}

func azureAuth(binary string) (bool, string) {
	out, err := exec.Command(binary, "account", "show", "--output", "tsv", "--query", "name").Output()
	if err != nil {
		return false, ""
	}
	return true, strings.TrimSpace(string(out))
}

func gcloudAuth(binary string) (bool, string) {
	out, err := exec.Command(binary, "auth", "list", "--format=value(account)", "--filter=status:ACTIVE").Output()
	if err != nil {
		return false, ""
	}
	account := strings.TrimSpace(string(out))
	return account != "", account
}

func ghAuth(binary string) (bool, string) {
	out, err := exec.Command(binary, "auth", "status").Output()
	if err != nil {
		return false, ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Logged in to") {
			parts := strings.Split(line, "account ")
			if len(parts) > 1 {
				return true, strings.TrimSpace(strings.Split(parts[1], " ")[0])
			}
			return true, ""
		}
	}
	return false, ""
}

func vercelAuth(binary string) (bool, string) {
	config := filepath.Join(restoreutil.HomeDir(), ".vercel", "config.json")
	if restoreutil.FileExists(config) {
		return true, ""
	}
	return false, ""
}

func netlifyAuth(binary string) (bool, string) {
	config := filepath.Join(restoreutil.HomeDir(), ".netlify", "config.json")
	if restoreutil.FileExists(config) {
		return true, ""
	}
	return false, ""
}

func wranglerAuth(binary string) (bool, string) {
	config := filepath.Join(restoreutil.HomeDir(), ".wrangler", "config.json")
	if restoreutil.FileExists(config) {
		return true, ""
	}
	return false, ""
}

func flyAuth(binary string) (bool, string) {
	config := filepath.Join(restoreutil.HomeDir(), ".fly", "config.yml")
	if restoreutil.FileExists(config) {
		return true, ""
	}
	return false, ""
}

func basicCheck(binary string) (bool, string) {
	return true, ""
}

func stdConfigPath(binary string) string {
	dirs := map[string]string{
		"aws":      filepath.Join(restoreutil.HomeDir(), ".aws"),
		"az":       filepath.Join(restoreutil.HomeDir(), ".azure"),
		"gcloud":   filepath.Join(restoreutil.HomeDir(), ".config", "gcloud"),
		"gh":       filepath.Join(restoreutil.HomeDir(), ".config", "gh"),
		"vercel":   filepath.Join(restoreutil.HomeDir(), ".vercel"),
		"wrangler": filepath.Join(restoreutil.HomeDir(), ".wrangler"),
		"netlify":  filepath.Join(restoreutil.HomeDir(), ".netlify"),
	}
	if d, ok := dirs[binary]; ok {
		if restoreutil.DirExists(d) {
			return d
		}
	}
	return ""
}

func awsConfigPath(binary string) string {
	return filepath.Join(restoreutil.HomeDir(), ".aws")
}

func ghConfigPath(binary string) string {
	d := filepath.Join(restoreutil.HomeDir(), ".config", "gh")
	if restoreutil.DirExists(d) {
		return d
	}
	return ""
}

type cloudBackupManifest struct {
	DetectedCLIs []string          `json:"detectedCLIs"`
	ConfigDirs   map[string]string `json:"configDirs,omitempty"`
	Credentials  map[string]bool   `json:"credentials,omitempty"`
	Accounts     map[string]string `json:"accounts,omitempty"`
}

func (m *CloudModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	var manifest cloudBackupManifest
	var entries []archive.Entry
	home := restoreutil.HomeDir()

	manifest.ConfigDirs = make(map[string]string)
	manifest.Credentials = make(map[string]bool)
	manifest.Accounts = make(map[string]string)

	for _, cli := range cloudCLIs {
		if !restoreutil.CommandExists(cli.binary) {
			continue
		}
		manifest.DetectedCLIs = append(manifest.DetectedCLIs, cli.name)
		auth, account := cli.authFn(cli.binary)
		manifest.Accounts[cli.name] = account
		manifest.Credentials[cli.name] = auth

		config := cli.configFn(cli.binary)
		if config != "" && config != "~" {
			if restoreutil.FileExists(config) || restoreutil.DirExists(config) {
				manifest.ConfigDirs[cli.name] = config
				rel := strings.TrimPrefix(config, home)
				rel = strings.TrimPrefix(rel, "/")
				entries = append(entries, archive.Entry{
					Source: config, ArchivePath: "configs/" + rel,
				})
			}
		}
	}

	tmpMeta, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("cloud: marshal manifest: %w", err)
	}
	metaFile := filepath.Join(os.TempDir(), "getitback-cloud-manifest.json")
	if err := os.WriteFile(metaFile, tmpMeta, 0600); err != nil {
		return nil, fmt.Errorf("cloud: write manifest: %w", err)
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
	for _, cli := range manifest.DetectedCLIs {
		contents = append(contents, cli+" configuration")
		if manifest.Credentials[cli] {
			contents = append(contents, cli+" credentials")
		}
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

func (m *CloudModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}
	tmpDir, err := os.MkdirTemp("", "getitback-restore-cloud-*")
	if err != nil {
		return fmt.Errorf("cloud: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if rt != nil {
		rt.Archive.Extract(snap.Path, tmpDir)
	} else {
		archive.Extract(snap.Path, tmpDir)
	}

	configsDir := filepath.Join(tmpDir, "configs")
	if info, err := os.Stat(configsDir); err == nil && info.IsDir() {
		filepath.Walk(configsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(configsDir, path)
			if rel == "" {
				return nil
			}
			dst := filepath.Join(home, rel)
			if info.IsDir() {
				return os.MkdirAll(dst, 0700)
			}
			os.MkdirAll(filepath.Dir(dst), 0755)
			data, _ := os.ReadFile(path)
			return os.WriteFile(dst, data, 0600)
		})
	}

	return nil
}

func (m *CloudModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	info, err := os.Stat(snap.Path)
	if err != nil {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{err.Error()}}, nil
	}
	if info.Size() == 0 {
		return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: false, Errors: []string{"empty snapshot"}}, nil
	}
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *CloudModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	result := &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}
	for _, cli := range cloudCLIs {
		if !restoreutil.CommandExists(cli.binary) {
			continue
		}
		auth, _ := cli.authFn(cli.binary)
		if !auth {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "warning",
				Message:  fmt.Sprintf("%s is installed but not authenticated", cli.name),
				Help:     fmt.Sprintf("Run: %s login or %s auth", cli.binary, cli.binary),
			})
		}
	}

	if restoreutil.CommandExists("aws") {
		credFile := filepath.Join(restoreutil.HomeDir(), ".aws", "credentials")
		if !restoreutil.FileExists(credFile) {
			result.Issues = append(result.Issues, module.DoctorIssue{
				Severity: "warning",
				Message:  "AWS credentials file not found",
				Help:     "Run: aws configure",
			})
		}
	}

	if len(result.Issues) > 0 {
		result.Status = module.DoctorStatusWarning
	}
	return result, nil
}

// --- Enhanced restore interfaces ---

func (m *CloudModule) Dependencies(ctx context.Context) []module.Dependency {
	var deps []module.Dependency
	if !restoreutil.CommandExists("aws") {
		deps = append(deps, module.Dependency{
			Type: module.DepSystemPkg, Package: "awscli", Hint: "AWS CLI",
		})
	}
	if !restoreutil.CommandExists("gcloud") {
		deps = append(deps, module.Dependency{
			Type: module.DepDownload, URL: "https://cloud.google.com/sdk/docs/install",
			Hint: "Google Cloud SDK",
		})
	}
	if !restoreutil.CommandExists("az") {
		deps = append(deps, module.Dependency{
			Type: module.DepSystemPkg, Package: "azure-cli", Hint: "Azure CLI",
		})
	}
	if !restoreutil.CommandExists("kubectl") {
		deps = append(deps, module.Dependency{
			Type: module.DepDownload, URL: "https://kubernetes.io/docs/tasks/tools/",
			Hint: "kubectl",
		})
	}
	if !restoreutil.CommandExists("helm") {
		deps = append(deps, module.Dependency{
			Type: module.DepDownload, URL: "https://helm.sh/docs/intro/install/",
			Hint: "Helm",
		})
	}
	return deps
}

func (m *CloudModule) Install(ctx context.Context, opts module.RestoreOptions) error {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	packages := []string{}
	if !restoreutil.CommandExists("aws") {
		packages = append(packages, "awscli")
	}
	if !restoreutil.CommandExists("az") {
		packages = append(packages, "azure-cli")
	}
	if len(packages) == 0 {
		return nil
	}
	if rt != nil {
		return rt.Pkg.Install(packages...)
	}
	return exec.Command("sudo", "apt-get", "install", "-y", "-qq").Run()
}

func (m *CloudModule) Configure(ctx context.Context, opts module.RestoreOptions) error {
	home := restoreutil.HomeDir()
	dirs := []string{
		filepath.Join(home, ".aws"),
		filepath.Join(home, ".config", "gcloud"),
		filepath.Join(home, ".azure"),
	}
	for _, d := range dirs {
		os.MkdirAll(d, 0700)
	}
	return nil
}

func (m *CloudModule) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
	v := restoreutil.NewValidation("cloud")

	ver, err := restoreutil.CheckExecOutput("aws", "--version")
	if err == nil {
		v.Version("aws: " + strings.Fields(ver)[0])
	}

	v.Check(restoreutil.CommandExists("aws"), "AWS CLI installed")
	v.Check(restoreutil.CommandExists("gcloud"), "Google Cloud SDK installed")
	v.Check(restoreutil.CommandExists("az"), "Azure CLI installed")
	v.Check(restoreutil.CommandExists("kubectl"), "kubectl installed")
	v.Check(restoreutil.CommandExists("helm"), "Helm installed")

	home := restoreutil.HomeDir()
	configDirs := map[string]string{
		"AWS credentials":   filepath.Join(home, ".aws"),
		"GCloud config":     filepath.Join(home, ".config", "gcloud"),
		"Azure config":      filepath.Join(home, ".azure"),
		"kubectl config":    filepath.Join(home, ".kube"),
		"Helm config":       filepath.Join(home, ".config", "helm"),
	}

	for name, dir := range configDirs {
		if restoreutil.DirExists(dir) {
			v.Recovered(name)
		} else {
			v.Missing(name)
		}
	}

	awsCreds := filepath.Join(home, ".aws", "credentials")
	if restoreutil.FileExists(awsCreds) {
		v.Recovered("AWS credentials file")
	} else {
		v.Warn("No AWS credentials file (manual configure needed)")
	}

	gcloudCreds := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
	if restoreutil.FileExists(gcloudCreds) {
		v.Recovered("GCloud application default credentials")
	}

	v.Confidence(85)
	return v.Result(), nil
}

func (m *CloudModule) Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]actions.Action, error) {
	rt, _ := opts.Runtime.(*runtime.Runtime)
	home := restoreutil.HomeDir()
	if rt != nil && rt.OS.HomeDir != "" {
		home = rt.OS.HomeDir
	}

	return []actions.Action{
		&actions.ExtractArchive{Source: snap.Path, Destination: home},
		&restoreUtilAction{
			name: "cloud_credential_helpers",
			desc: "Restore cloud credential helpers and permissions",
			fn: func(ctx *runtime.RestoreContext) error {
				awsDir := filepath.Join(home, ".aws")
				if restoreutil.DirExists(awsDir) {
					entries, _ := os.ReadDir(awsDir)
					for _, entry := range entries {
						path := filepath.Join(awsDir, entry.Name())
						os.Chmod(path, 0600)
					}
				}
				gcloudDir := filepath.Join(home, ".config", "gcloud")
				if restoreutil.DirExists(gcloudDir) {
					filepath.Walk(gcloudDir, func(path string, info os.FileInfo, err error) error {
						if err != nil || info == nil {
							return err
						}
						if info.IsDir() {
							os.Chmod(path, 0700)
						} else {
							os.Chmod(path, 0600)
						}
						return nil
					})
				}
				azureDir := filepath.Join(home, ".azure")
				if restoreutil.DirExists(azureDir) {
					filepath.Walk(azureDir, func(path string, info os.FileInfo, err error) error {
						if err != nil || info == nil {
							return err
						}
						if info.IsDir() {
							os.Chmod(path, 0700)
						} else {
							os.Chmod(path, 0600)
						}
						return nil
					})
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

func (a *restoreUtilAction) Name() string        { return a.name }
func (a *restoreUtilAction) Description() string  { return a.desc }
func (a *restoreUtilAction) Execute(ctx *runtime.RestoreContext) error { return a.fn(ctx) }

var _ actions.Provider = (*CloudModule)(nil)
