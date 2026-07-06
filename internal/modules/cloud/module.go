package cloud

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/module"
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
	{name: "AWS CLI", binary: "aws", versionFn: stdVersion, authFn: awsAuth, configFn: stdConfigPath},
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
		if _, err := exec.LookPath(cli.binary); err == nil {
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
		if _, err := exec.LookPath(cli.binary); err != nil {
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

		key := cli.binary + "Auth"
		meta[key] = auth
		if account != "" {
			meta[cli.binary+"Account"] = account
		}
		if config != "" {
			meta[cli.binary+"Config"] = config
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
	home, _ := os.UserHomeDir()
	config := home + "/.vercel/config.json"
	if _, err := os.Stat(config); err == nil {
		return true, ""
	}
	return false, ""
}

func netlifyAuth(binary string) (bool, string) {
	home, _ := os.UserHomeDir()
	config := home + "/.netlify/config.json"
	if _, err := os.Stat(config); err == nil {
		return true, ""
	}
	return false, ""
}

func wranglerAuth(binary string) (bool, string) {
	home, _ := os.UserHomeDir()
	config := home + "/.wrangler/config.json"
	if _, err := os.Stat(config); err == nil {
		return true, ""
	}
	return false, ""
}

func flyAuth(binary string) (bool, string) {
	home, _ := os.UserHomeDir()
	config := home + "/.fly/config.yml"
	if _, err := os.Stat(config); err == nil {
		return true, ""
	}
	return false, ""
}

func basicCheck(binary string) (bool, string) {
	return true, ""
}

func stdConfigPath(binary string) string {
	home, _ := os.UserHomeDir()
	dirs := map[string]string{
		"aws":      home + "/.aws",
		"az":       home + "/.azure",
		"gcloud":   home + "/.config/gcloud",
		"gh":       home + "/.config/gh",
		"vercel":   home + "/.vercel",
		"wrangler": home + "/.wrangler",
		"netlify":  home + "/.netlify",
	}
	if d, ok := dirs[binary]; ok {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
	}
	return ""
}

func ghConfigPath(binary string) string {
	home, _ := os.UserHomeDir()
	d := home + "/.config/gh"
	if info, err := os.Stat(d); err == nil && info.IsDir() {
		return d
	}
	return ""
}

func (m *CloudModule) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
	return nil, nil
}

func (m *CloudModule) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
	return nil
}

func (m *CloudModule) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
	return &module.VerifyResult{Module: m.Name(), Snapshot: snap, Valid: true}, nil
}

func (m *CloudModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{Module: m.Name(), Status: module.DoctorStatusOK}, nil
}
