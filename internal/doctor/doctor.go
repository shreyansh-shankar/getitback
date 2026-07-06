package doctor

import (
	"fmt"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

type Report struct {
	Confidence      Confidence
	Risks           []Risk
	Coverage        BackupCoverage
	Timeline        RecoveryTimeline
	ActionPlan      []Action
	Readiness       []CategoryReadiness
	Summary         string
	Machine         MachineStatus
	DisasterPreview DisasterPreview
}

type Confidence struct {
	Score   int
	Grade   string
	Message string
	Reasons []string
}

type Risk struct {
	Severity string
	Module   string
	Message  string
	Impact   string
	Effort   string
	EstSize  string
	Command  string
}

type BackupCoverage struct {
	Protected        []string
	Unprotected      []string
	ProtectedCount   int
	UnprotectedCount int
	CoveragePercent  int
}

type RecoveryTimeline struct {
	Entries            []TimelineEntry
	Total              string
	Minutes            int
	EstimatedBackupSize string
	ManualSteps        int
}

type TimelineEntry struct {
	Category string
	Minutes  int
	Duration string
}

type Action struct {
	Priority  int
	Module    string
	Message   string
	Impact    string
	Effort    string
	Command   string
	Difficulty string
}

type CategoryReadiness struct {
	Name    string
	Score   int
	Max     int
	Percent int
	Status  string
	Reason  string
	Details []string
}

type MachineStatus struct {
	Overall    string
	Categories []StatusEntry
}

type StatusEntry struct {
	Category string
	Status   string
}

type DisasterPreview struct {
	WouldLose []string
	WouldKeep []string
}

type finding struct {
	severity string
	module   string
	message  string
	impact   string
	effort   string
	estSize  string
	command  string
	category string
}

type assetValue struct {
	category string
	level    int
}

var assetValues = map[string]assetValue{
	"ssh":          {"Identity", 5},
	"gpg":          {"Identity", 5},
	"dotfiles":     {"Configuration", 3},
	"shell":        {"Configuration", 2},
	"git":          {"Configuration", 3},
	"docker":       {"Containers", 4},
	"firefox":      {"Browsers", 3},
	"chrome":       {"Browsers", 3},
	"chromium":     {"Browsers", 2},
	"brave":        {"Browsers", 2},
	"vivaldi":      {"Browsers", 2},
	"edge":         {"Browsers", 2},
	"opera":        {"Browsers", 2},
	"vscode":       {"Editors", 3},
	"neovim":       {"Editors", 2},
	"postgres":     {"Databases", 4},
	"mysql":        {"Databases", 4},
	"mongodb":      {"Databases", 4},
	"redis":        {"Databases", 3},
	"sqlite":       {"Databases", 2},
	"golang":       {"Development", 2},
	"nodejs":       {"Development", 2},
	"python":       {"Development", 2},
	"rust":         {"Development", 2},
	"java":         {"Development", 2},
	"cloud":        {"Cloud", 4},
	"kubernetes":   {"Infrastructure", 4},
	"repos":        {"Projects", 3},
	"virtualization": {"Virtualization", 2},
	"certs":        {"Security", 3},
	"system":       {"System", 1},
}

var timelineEstimates = map[string]int{
	"Identity":        2,
	"Configuration":   5,
	"Development":     10,
	"Editors":         3,
	"Browsers":        10,
	"Packages":        5,
	"Databases":       20,
	"Containers":      15,
	"Cloud":           5,
	"Infrastructure":  10,
	"Projects":        8,
	"Virtualization":  5,
	"Security":        3,
}

var severityOrder = map[string]int{
	"Critical": 0,
	"High":     1,
	"Medium":   2,
	"Low":      3,
	"Info":     4,
}

var moduleCategories = map[string]string{
	"system": "System",
	"git":    "Development", "golang": "Development",
	"nodejs": "Development", "python": "Development", "rust": "Development",
	"java": "Development",
	"ssh": "Identity", "gpg": "Identity",
	"vscode": "Editors", "neovim": "Editors",
	"firefox": "Browsers", "chromium": "Browsers",
	"chrome": "Browsers", "brave": "Browsers", "vivaldi": "Browsers", "edge": "Browsers", "opera": "Browsers",
	"postgres": "Databases", "mongodb": "Databases", "redis": "Databases", "sqlite": "Databases",
	"mysql": "Databases",
	"shell": "Configuration", "dotfiles": "Configuration",
	"apt": "Packages", "snap": "Packages", "flatpak": "Packages",
	"docker": "Containers",
	"cloud": "Cloud",
	"kubernetes": "Infrastructure",
	"virtualization": "Virtualization",
	"certs": "Security",
	"repos": "Projects",
}

var backupSizeEstimates = map[string]string{
	"Identity":       "2 MB",
	"Configuration":  "5 MB",
	"Development":    "500 MB",
	"Editors":        "10 MB",
	"Browsers":       "2.4 GB",
	"Packages":       "50 MB",
	"Databases":      "1.5 GB",
	"Containers":     "15 GB",
	"Cloud":          "10 MB",
	"Infrastructure": "50 MB",
	"Projects":       "1 GB",
	"Virtualization": "5 GB",
	"Security":       "5 MB",
}

var categoryMaxes = map[string]int{
	"Identity":       15,
	"Configuration":  15,
	"Development":    20,
	"Editors":        15,
	"Browsers":       15,
	"Packages":       10,
	"Databases":      10,
	"Containers":     15,
	"Cloud":          10,
	"Infrastructure": 10,
	"Projects":       10,
	"Virtualization": 10,
}

var categoryOrder = []string{
	"Identity", "Configuration", "Development", "Editors",
	"Browsers", "Packages", "Databases", "Containers",
	"Cloud", "Infrastructure", "Projects", "Virtualization",
}

func NewReport(results []*module.InventoryResult, score *module.RecoveryScore, backedUp map[string]bool, configHasEncryption bool) *Report {
	r := &Report{}

	r.Readiness = computeReadiness(results, score, backedUp)
	r.Confidence = computeConfidence(r.Readiness, results, backedUp, configHasEncryption)
	r.Risks = computeRisks(results, backedUp, configHasEncryption)
	r.Coverage = computeCoverage(results, backedUp)
	r.Timeline = computeTimeline(r.Readiness, backedUp)
	r.ActionPlan = computeActionPlan(r.Risks)
	r.Machine = computeMachineStatus(r.Readiness)
	r.DisasterPreview = computeDisasterPreview(results, backedUp)
	r.Summary = computeSummary(r)

	return r
}

func computeReadiness(results []*module.InventoryResult, score *module.RecoveryScore, backedUp map[string]bool) []CategoryReadiness {
	readiness := make([]CategoryReadiness, 0, len(categoryOrder))
	has := func(mod string) bool {
		for _, res := range results {
			if res.Module == mod && res.Detected {
				return true
			}
		}
		return false
	}

	for _, cat := range categoryOrder {
		max := categoryMaxes[cat]
		s := 0
		switch cat {
		case "Identity":
			s = score.Identity
		case "Configuration":
			s = score.Configuration
		case "Development":
			s = score.Development
		case "Editors":
			s = score.Editors
		case "Browsers":
			s = score.Browsers
		case "Packages":
			s = score.Packages
		case "Databases":
			s = score.Databases
		case "Containers":
			s = score.Containers
		case "Cloud":
			s = score.Cloud
		case "Infrastructure":
			s = score.Infrastructure
		case "Projects":
			s = score.Projects
		case "Virtualization":
			s = score.Virtualization
		}

		pct := 0
		if max > 0 {
			pct = s * 100 / max
		}

		status := "At Risk"
		if pct >= 80 {
			status = "Recoverable"
		} else if pct >= 40 {
			status = "Partially Recoverable"
		}

		details := buildCategoryDetails(cat, results, backedUp, has)
		reason := buildReasonSummary(cat, results, backedUp, has)

		readiness = append(readiness, CategoryReadiness{
			Name: cat, Score: s, Max: max, Percent: pct,
			Status: status, Reason: reason, Details: details,
		})
	}
	return readiness
}

func buildReasonSummary(cat string, results []*module.InventoryResult, backedUp map[string]bool, has func(string) bool) string {
	switch cat {
	case "Identity":
		items := []string{}
		if has("ssh") {
			items = append(items, "SSH keys present")
		}
		if has("gpg") {
			items = append(items, "GPG keys present")
		}
		if len(items) == 0 {
			return "No identity credentials found"
		}
		return stringsJoin(items, ", ")
	case "Configuration":
		items := []string{}
		if has("dotfiles") {
			items = append(items, "Dotfiles tracked")
		}
		if has("shell") {
			items = append(items, "Shell configured")
		}
		if has("git") {
			items = append(items, "Git configured")
		}
		if len(items) > 0 {
			return stringsJoin(items, ", ")
		}
		return "Minimal configuration found"
	case "Development":
		var langs []string
		for _, l := range []string{"golang", "nodejs", "python", "rust", "java"} {
			if has(l) {
				langs = append(langs, moduleDisplayName(l))
			}
		}
		if len(langs) > 0 {
			return stringsJoin(langs, ", ") + " installed"
		}
		return "No development tools found"
	case "Editors":
		if has("vscode") {
			return "VS Code with extensions"
		}
		if has("neovim") {
			return "Neovim configured"
		}
		return "No editors found"
	case "Browsers":
		count := 0
		for _, b := range []string{"firefox", "chrome", "chromium", "brave", "vivaldi", "edge", "opera"} {
			if has(b) {
				count++
			}
		}
		if count > 0 {
			return fmtSprintf("%d browsers installed", count)
		}
		return "No browsers found"
	case "Packages":
		var pkg []string
		for _, p := range []string{"apt", "snap", "flatpak"} {
			if has(p) {
				pkg = append(pkg, p)
			}
		}
		if len(pkg) > 0 {
			return stringsJoin(pkg, ", ") + " package managers active"
		}
		return "No package managers found"
	case "Databases":
		var dbs []string
		for _, db := range []string{"postgres", "mysql", "mongodb", "redis", "sqlite"} {
			if has(db) {
				dbs = append(dbs, moduleDisplayName(db))
			}
		}
		if len(dbs) > 0 {
			return stringsJoin(dbs, ", ") + " installed"
		}
		return "No databases found"
	case "Containers":
		if has("docker") {
			return "Docker managing containers and volumes"
		}
		return "No container runtime found"
	case "Cloud":
		if has("cloud") {
			return "Cloud CLI tools available"
		}
		return "No cloud tools found"
	case "Infrastructure":
		if has("kubernetes") {
			return "Kubernetes and IaC tools available"
		}
		return "No infrastructure tools found"
	case "Projects":
		if has("repos") {
			return "Git repositories on disk"
		}
		return "No repositories found"
	case "Virtualization":
		if has("virtualization") {
			return "Virtualization platforms available"
		}
		return "No virtualization found"
	case "Security":
		if has("certs") {
			return "Certificate stores found"
		}
		return "No certificate management found"
	}
	return ""
}

func buildCategoryDetails(cat string, results []*module.InventoryResult, backedUp map[string]bool, has func(string) bool) []string {
	var details []string

	switch cat {
	case "Identity":
		if has("ssh") {
			details = append(details, "SSH keys  ✓")
			if backedUp["ssh"] {
				details = append(details, "  SSH backup  ✓")
			} else {
				details = append(details, "  SSH backup  ✗")
			}
		}
		if has("gpg") {
			details = append(details, "GPG keys  ✓")
			if backedUp["gpg"] {
				details = append(details, "  GPG backup  ✓")
			} else {
				details = append(details, "  GPG backup  ✗")
			}
		}
	case "Development":
		for _, l := range []string{"golang", "nodejs", "python", "rust", "java"} {
			if has(l) {
				status := "✓"
				if !backedUp[l] {
					status = "✗"
				}
				details = append(details, fmtSprintf("%s  %s  Backup %s", moduleDisplayName(l), "✓", status))
			}
		}
	case "Browsers":
		for _, b := range []string{"firefox", "chrome", "chromium", "brave", "vivaldi"} {
			if has(b) {
				status := "✓"
				if !backedUp[b] {
					status = "✗"
				}
				details = append(details, fmtSprintf("%s  %s  Backed up %s", moduleDisplayName(b), "✓", status))
			}
		}
	case "Databases":
		for _, db := range []string{"postgres", "mysql", "mongodb", "redis"} {
			if has(db) {
				status := "✓"
				if !backedUp[db] {
					status = "✗"
				}
				details = append(details, fmtSprintf("%s  %s  Backup %s", moduleDisplayName(db), "✓", status))
			}
		}
	case "Containers":
		if has("docker") {
			details = append(details, "Docker Engine installed  ✓")
			if backedUp["docker"] {
				details = append(details, "  Docker backup  ✓")
			} else {
				details = append(details, "  Docker backup  ✗")
			}
		}
	case "Configuration":
		if has("dotfiles") {
			status := "✓"
			if !backedUp["dotfiles"] {
				status = "✗"
			}
			details = append(details, fmtSprintf("Dotfiles  ✓  Backup %s", status))
		}
		if has("shell") {
			details = append(details, "Shell configured  ✓")
		}
		if has("git") {
			details = append(details, "Git configured  ✓")
		}
	}

	if len(details) == 0 {
		if has("system") {
			details = append(details, "System detected  ✓")
		}
	}

	return details
}

func computeConfidence(readiness []CategoryReadiness, results []*module.InventoryResult, backedUp map[string]bool, configHasEncryption bool) Confidence {
	var totalScore, maxScore int
	for _, r := range readiness {
		totalScore += r.Score
		maxScore += r.Max
	}
	pct := 0
	if maxScore > 0 {
		pct = totalScore * 100 / maxScore
	}

	grade := "CRITICAL"
	message := "Your workstation has significant recovery risks."
	switch {
	case pct >= 95:
		grade = "EXCELLENT"
		message = "Your workstation is fully recoverable. Critical assets are protected."
	case pct >= 85:
		grade = "GOOD"
		message = "Your workstation is mostly recoverable. A few areas need attention."
	case pct >= 70:
		grade = "FAIR"
		message = "Your workstation is partially recoverable. Several important assets remain uncovered."
	case pct >= 50:
		grade = "POOR"
		message = "Your workstation has significant recovery gaps. Prioritize backing up critical assets."
	}

	reasons := buildConfidenceReasons(results, backedUp, configHasEncryption)

	return Confidence{Score: pct, Grade: grade, Message: message, Reasons: reasons}
}

func buildConfidenceReasons(results []*module.InventoryResult, backedUp map[string]bool, configHasEncryption bool) []string {
	var reasons []string

	protected := 0
	unprotected := 0
	for _, res := range results {
		if !res.Detected {
			continue
		}
		if assetValues[res.Module].level >= 2 {
			if backedUp[res.Module] {
				protected++
			} else {
				unprotected++
			}
		}
	}

	reasons = append(reasons, fmtSprintf("✓ %d protected assets", protected))
	if unprotected > 0 {
		reasons = append(reasons, fmtSprintf("⚠ %d unprotected assets", unprotected))
	}

	has := func(mod string) bool {
		for _, res := range results {
			if res.Module == mod && res.Detected {
				return true
			}
		}
		return false
	}

	getMetaInt := func(mod, key string) (int, bool) {
		for _, res := range results {
			if res.Module == mod && res.Detected && res.Metadata != nil {
				if v, ok := res.Metadata[key]; ok {
					switch val := v.(type) {
					case int:
						return val, true
					case int64:
						return int(val), true
					}
				}
			}
		}
		return 0, false
	}

	if has("ssh") && has("gpg") {
		reasons = append(reasons, "✓ Critical identity (SSH + GPG) preserved")
	} else if has("ssh") {
		reasons = append(reasons, "✓ SSH keys present")
	} else if has("gpg") {
		reasons = append(reasons, "✓ GPG keys present")
	}

	hasUnprotectedDB := false
	for _, db := range []string{"postgres", "mysql", "mongodb", "redis"} {
		if has(db) && !backedUp[db] {
			hasUnprotectedDB = true
			break
		}
	}
	if hasUnprotectedDB {
		reasons = append(reasons, "⚠ Databases not protected")
	}

	if has("docker") && !backedUp["docker"] {
		if vols, ok := getMetaInt("docker", "volumes"); ok && vols > 0 {
			reasons = append(reasons, fmtSprintf("⚠ Docker volumes (%d) not backed up", vols))
		} else {
			reasons = append(reasons, "⚠ Docker environment not backed up")
		}
	}

	browserUnprotected := false
	for _, b := range []string{"firefox", "chrome"} {
		if has(b) && !backedUp[b] {
			browserUnprotected = true
			break
		}
	}
	if browserUnprotected {
		reasons = append(reasons, "⚠ Browser profiles not backed up")
	}

	if has("cloud") && !backedUp["cloud"] {
		for _, res := range results {
			if res.Module == "cloud" && res.Metadata != nil {
				if clis, ok := res.Metadata["detectedCLIs"]; ok {
					if list, ok := clis.([]string); ok && len(list) > 0 {
						reasons = append(reasons, fmtSprintf("⚠ Cloud credentials (%d services) not backed up", len(list)))
					}
				}
			}
		}
	}

	if !configHasEncryption {
		reasons = append(reasons, "⚠ No backup encryption configured")
	}

	return reasons
}

func computeRisks(results []*module.InventoryResult, backedUp map[string]bool, configHasEncryption bool) []Risk {
	var findings []finding

	has := func(mod string) bool {
		for _, res := range results {
			if res.Module == mod && res.Detected {
				return true
			}
		}
		return false
	}

	getMeta := func(mod, key string) (string, bool) {
		for _, res := range results {
			if res.Module == mod && res.Detected && res.Metadata != nil {
				if v, ok := res.Metadata[key]; ok {
					switch val := v.(type) {
					case string:
						return val, true
					case int:
						return fmtSprintf("%d", val), true
					case int64:
						return fmtSprintf("%d", val), true
					case bool:
						if val {
							return "true", true
						}
						return "false", true
					}
				}
			}
		}
		return "", false
	}

	getMetaInt := func(mod, key string) (int, bool) {
		for _, res := range results {
			if res.Module == mod && res.Detected && res.Metadata != nil {
				if v, ok := res.Metadata[key]; ok {
					switch val := v.(type) {
					case int:
						return val, true
					case int64:
						return int(val), true
					case float64:
						return int(val), true
					}
				}
			}
		}
		return 0, false
	}

	if !configHasEncryption {
		findings = append(findings, finding{
			severity: "Critical",
			message:  "No backup encryption configured",
			impact:   "All backup data would be stored in plain text. If storage is compromised, all credentials, keys, and personal data are exposed.",
			effort:   "2 min",
			command:  "Set encryption.enabled: true and encryption.key: /path/to/key in ~/.getitback/config.yaml",
			category: "Security",
		})
	}

	if has("ssh") {
		if n, ok := getMeta("ssh", "identityCount"); ok && n == "0" {
			findings = append(findings, finding{
				severity: "High",
				module:   "ssh",
				message:  "No SSH identities found",
				impact:   "Cannot authenticate to GitHub, servers, or other SSH-enabled services",
				effort:   "1 min",
				command:  "ssh-keygen -t ed25519 -a 100",
				category: "Identity",
			})
		} else if !backedUp["ssh"] {
			findings = append(findings, finding{
				severity: "High",
				module:   "ssh",
				message:  "SSH keys not backed up",
				impact:   "Loss of SSH keys means losing access to servers, GitHub, and other SSH-authenticated services. Regenerating and redistributing keys takes significant time.",
				effort:   "2 min",
				command:  "getitback backup --module ssh",
				category: "Identity",
			})
		}
	}

	if has("gpg") && !backedUp["gpg"] {
		findings = append(findings, finding{
			severity: "High",
			module:   "gpg",
			message:  "GPG keys not backed up",
			impact:   "GPG keys are used for signing commits, decrypting secrets, and authentication. If lost, all signed commits lose their verifiability and encrypted data becomes inaccessible.",
			effort:   "30 sec",
			command:  "gpg --export-secret-keys --armor > keys.asc",
			category: "Identity",
		})
	}

	if has("docker") {
		if !backedUp["docker"] {
			vols, _ := getMetaInt("docker", "volumes")
			compose, _ := getMetaInt("docker", "composeProjects")
			impact := "Docker configuration"
			if vols > 0 {
				impact += ", volume data"
			}
			if compose > 0 {
				impact += ", compose projects"
			}
			impact += " would be lost"

			findings = append(findings, finding{
				severity: "High",
				module:   "docker",
				message:  "Docker environment not backed up",
				impact:   impact,
				effort:   "5 min",
				command:  "getitback backup --module docker",
				category: "Containers",
			})
		}

		if n, ok := getMetaInt("docker", "danglingImages"); ok && n > 0 {
			findings = append(findings, finding{
				severity: "Low",
				module:   "docker",
				message:  fmtSprintf("%d dangling images consuming disk space", n),
				impact:   "Wasted disk space and potential confusion during restore",
				effort:   "1 min",
				command:  "docker image prune",
				category: "Containers",
			})
		}
	}

	for _, b := range []string{"firefox", "chrome", "chromium", "brave", "vivaldi"} {
		if has(b) && !backedUp[b] {
			if n, ok := getMetaInt(b, "profileCount"); ok && n > 0 {
				findings = append(findings, finding{
					severity: "High",
					module:   b,
					message:  fmtSprintf("%s profiles not backed up", moduleDisplayName(b)),
					impact:   "Bookmarks, saved passwords, extensions, browsing history, and site settings would be lost",
					effort:   "2 min",
					command:  fmtSprintf("getitback backup --module %s", b),
					category: "Browsers",
				})
			}
		}
	}

	for _, db := range []string{"postgres", "mysql", "mongodb", "redis"} {
		if has(db) && !backedUp[db] {
			findings = append(findings, finding{
				severity: "High",
				module:   db,
				message:  fmtSprintf("%s database not backed up", moduleDisplayName(db)),
				impact:   "All database data, schemas, and configurations would be lost. Recovery requires recreating from scratch or restoring from an external source.",
				effort:   "5 min",
				command:  fmtSprintf("getitback backup --module %s", db),
				category: "Databases",
			})
		}
	}

	if has("git") {
		if _, ok := getMeta("git", "signingKey"); !ok {
			findings = append(findings, finding{
				severity: "Medium",
				module:   "git",
				message:  "Git commit signing not configured",
				impact:   "Commits cannot be verified. This reduces trust in your commit history and may cause CI/CD pipeline rejections.",
				effort:   "2 min",
				command:  "git config --global user.signingkey <key-id> && git config --global commit.gpgsign true",
				category: "Configuration",
			})
		}
	}

	if has("dotfiles") && !backedUp["dotfiles"] {
		findings = append(findings, finding{
			severity: "Medium",
			module:   "dotfiles",
			message:  "Dotfiles not backed up",
			impact:   "Shell configuration, aliases, environment variables, and editor settings would be lost",
			effort:   "5 min",
			command:  "git init --bare $HOME/.dotfiles && alias dotfiles='git --git-dir=$HOME/.dotfiles/ --work-tree=$HOME'",
			category: "Configuration",
		})
	}

	if has("vscode") && !backedUp["vscode"] {
		if n, ok := getMetaInt("vscode", "extensions"); ok && n > 0 {
			findings = append(findings, finding{
				severity: "Medium",
				module:   "vscode",
				message:  fmtSprintf("VS Code extensions (%d) not backed up", n),
				impact:   "Extension list, settings, snippets, and keybindings would be lost. Recreating the setup takes significant time.",
				effort:   "2 min",
				command:  "code --list-extensions > vscode-extensions.txt",
				category: "Editors",
			})
		}
	}

	if has("cloud") && !backedUp["cloud"] {
		for _, res := range results {
			if res.Module == "cloud" && res.Metadata != nil {
				if clis, ok := res.Metadata["detectedCLIs"]; ok {
					if list, ok := clis.([]string); ok && len(list) > 0 {
						findings = append(findings, finding{
							severity: "High",
							module:   "cloud",
							message:  "Cloud CLI credentials not backed up",
							impact:   fmtSprintf("AWS, Vercel, and other cloud credentials would be lost. Re-authenticating to %d services is time-consuming", len(list)),
							effort:   "5 min",
							command:  "Check ~/.aws, ~/.azure, ~/.config/gcloud for credential files",
							category: "Cloud",
						})
					}
				}
			}
		}
	}

	if has("kubernetes") && !backedUp["kubernetes"] {
		if ctx, ok := getMeta("kubernetes", "currentContext"); ok && ctx != "" {
			findings = append(findings, finding{
				severity: "High",
				module:   "kubernetes",
				message:  fmtSprintf("Kubernetes context %q not backed up", ctx),
				impact:   "kubeconfig contains cluster access credentials. Loss means losing access to all Kubernetes clusters until credentials are re-provisioned.",
				effort:   "3 min",
				command:  "Back up ~/.kube/config and consider encrypting it",
				category: "Infrastructure",
			})
		}
	}

	if has("repos") && !backedUp["repos"] {
		if n, ok := getMetaInt("repos", "noRemoteRepos"); ok && n > 0 {
			findings = append(findings, finding{
				severity: "Medium",
				module:   "repos",
				message:  fmtSprintf("%d repositories have no remote", n),
				impact:   "Local-only repositories would be permanently lost if the machine fails. Code, commits, and branches unique to this machine would disappear.",
				effort:   "10 min",
				command:  "git remote add origin <url> for each affected repository",
				category: "Projects",
			})
		}
		if n, ok := getMetaInt("repos", "dirtyRepos"); ok && n > 0 {
			findings = append(findings, finding{
				severity: "Low",
				module:   "repos",
				message:  fmtSprintf("%d repositories have uncommitted changes", n),
				impact:   "Work-in-progress changes could be lost",
				effort:   "5 min",
				command:  "git status in each affected repo and commit or stash changes",
				category: "Projects",
			})
		}
	}

	if has("certs") {
		if n, ok := getMetaInt("certs", "expiredCerts"); ok && n > 0 {
			findings = append(findings, finding{
				severity: "Medium",
				module:   "certs",
				message:  fmtSprintf("%d expired certificates found", n),
				impact:   "Expired certificates can cause TLS handshake failures, breaking HTTPS connections and service authentication",
				effort:   "5 min",
				command:  "Review and renew expired certificates",
				category: "Security",
			})
		}
		if n, ok := getMetaInt("certs", "expiringCerts"); ok && n > 0 {
			findings = append(findings, finding{
				severity: "Low",
				module:   "certs",
				message:  fmtSprintf("%d certificates expiring within 30 days", n),
				impact:   "Services may become unreachable if certificates expire",
				effort:   "5 min",
				command:  "Renew certificates before they expire",
				category: "Security",
			})
		}
	}

	seen := make(map[string]bool)
	var unique []finding
	for _, f := range findings {
		if seen[f.message] {
			continue
		}
		seen[f.message] = true
		unique = append(unique, f)
	}

	risks := make([]Risk, len(unique))
	for i, f := range unique {
		risks[i] = Risk{
			Severity: f.severity,
			Module:   f.module,
			Message:  f.message,
			Impact:   f.impact,
			Effort:   f.effort,
			EstSize:  f.estSize,
			Command:  f.command,
		}
	}

	sortRisks(risks)
	return risks
}

func sortRisks(risks []Risk) {
	for i := 0; i < len(risks); i++ {
		for j := i + 1; j < len(risks); j++ {
			si := severityOrder[risks[i].Severity]
			sj := severityOrder[risks[j].Severity]
			if sj < si {
				risks[i], risks[j] = risks[j], risks[i]
			}
		}
	}
}

func computeCoverage(results []*module.InventoryResult, backedUp map[string]bool) BackupCoverage {
	var protected, unprotected []string

	for _, res := range results {
		if !res.Detected {
			continue
		}
		if assetValues[res.Module].level >= 2 {
			if backedUp[res.Module] {
				protected = append(protected, res.Module)
			} else {
				unprotected = append(unprotected, res.Module)
			}
		}
	}

	total := len(protected) + len(unprotected)
	pct := 0
	if total > 0 {
		pct = len(protected) * 100 / total
	}

	return BackupCoverage{
		Protected: protected, Unprotected: unprotected,
		ProtectedCount: len(protected), UnprotectedCount: len(unprotected),
		CoveragePercent: pct,
	}
}

func computeTimeline(readiness []CategoryReadiness, backedUp map[string]bool) RecoveryTimeline {
	var entries []TimelineEntry
	totalMin := 0
	totalManual := 0
	for _, r := range readiness {
		if r.Score == 0 {
			continue
		}
		mins := timelineEstimates[r.Name]
		if backedUp[mapCategoryToModule(r.Name)] {
			mins = mins / 2
		}
		entries = append(entries, TimelineEntry{
			Category: r.Name,
			Minutes:  mins,
			Duration: formatDuration(mins),
		})
		totalMin += mins

		if r.Score < r.Max {
			totalManual++
		}
	}

	totalSize := "18.2 GB"
	if len(entries) > 0 {
		entry := entries[0]
		if s, ok := backupSizeEstimates[entry.Category]; ok {
			totalSize = s
		}
	}

	return RecoveryTimeline{
		Entries:             entries,
		Total:               formatDuration(totalMin),
		Minutes:             totalMin,
		EstimatedBackupSize: totalSize,
		ManualSteps:         totalManual,
	}
}

func mapCategoryToModule(cat string) string {
	for mod, c := range moduleCategories {
		if c == cat {
			return mod
		}
	}
	return ""
}

func computeActionPlan(risks []Risk) []Action {
	var actions []Action
	priority := 1
	for _, r := range risks {
		if r.Severity == "Info" {
			continue
		}
		if r.Command == "" {
			continue
		}

		difficulty := estimateDifficulty(r)
		actions = append(actions, Action{
			Priority:   priority,
			Module:     r.Module,
			Message:    r.Message,
			Impact:     r.Severity,
			Effort:     r.Effort,
			Command:    r.Command,
			Difficulty: difficulty,
		})
		priority++
	}
	return actions
}

func estimateDifficulty(r Risk) string {
	if r.Effort == "30 sec" || r.Effort == "1 min" || r.Effort == "2 min" {
		return "Easy"
	}
	if r.Effort == "5 min" || r.Effort == "3 min" {
		return "Easy"
	}
	if r.Effort == "10 min" {
		return "Medium"
	}
	if strings.Contains(r.Message, "encryption") || strings.Contains(r.Message, "Kubernetes") {
		return "Medium"
	}
	return "Easy"
}

func computeMachineStatus(readiness []CategoryReadiness) MachineStatus {
	var entries []StatusEntry
	riskCount := 0
	for _, r := range readiness {
		entries = append(entries, StatusEntry{Category: r.Name, Status: r.Status})
		if r.Status == "At Risk" {
			riskCount++
		}
	}
	overall := "Mostly Recoverable"
	if riskCount == 0 {
		overall = "Fully Recoverable"
	} else if riskCount > len(readiness)/2 {
		overall = "Significantly At Risk"
	}
	return MachineStatus{Overall: overall, Categories: entries}
}

func computeDisasterPreview(results []*module.InventoryResult, backedUp map[string]bool) DisasterPreview {
	var wouldLose, wouldKeep []string

	has := func(mod string) bool {
		for _, res := range results {
			if res.Module == mod && res.Detected {
				return true
			}
		}
		return false
	}

	getMetaInt := func(mod, key string) (int, bool) {
		for _, res := range results {
			if res.Module == mod && res.Detected && res.Metadata != nil {
				if v, ok := res.Metadata[key]; ok {
					switch val := v.(type) {
					case int:
						return val, true
					case int64:
						return int(val), true
					}
				}
			}
		}
		return 0, false
	}

	if has("ssh") && !backedUp["ssh"] {
		wouldLose = append(wouldLose, "SSH identities and server access")
	} else if has("ssh") {
		wouldKeep = append(wouldKeep, "SSH identities")
	}

	if has("gpg") && !backedUp["gpg"] {
		wouldLose = append(wouldLose, "GPG keys and commit signing")
	} else if has("gpg") {
		wouldKeep = append(wouldKeep, "GPG keys")
	}

	if has("firefox") && !backedUp["firefox"] {
		if n, ok := getMetaInt("firefox", "profileCount"); ok && n > 0 {
			wouldLose = append(wouldLose, fmtSprintf("Firefox profiles (%d) with bookmarks, passwords, and extensions", n))
		}
	} else if has("firefox") {
		wouldKeep = append(wouldKeep, "Firefox profiles")
	}

	if has("chrome") && !backedUp["chrome"] {
		if n, ok := getMetaInt("chrome", "profileCount"); ok && n > 0 {
			wouldLose = append(wouldLose, fmtSprintf("Chrome profiles (%d) with bookmarks and saved passwords", n))
		}
	} else if has("chrome") {
		wouldKeep = append(wouldKeep, "Chrome profiles")
	}

	if has("docker") {
		if !backedUp["docker"] {
			if vols, ok := getMetaInt("docker", "volumes"); ok && vols > 0 {
				wouldLose = append(wouldLose, fmtSprintf("Docker volumes (%d) with application data", vols))
			}
			if imgs, ok := getMetaInt("docker", "images"); ok && imgs > 0 {
				wouldLose = append(wouldLose, fmtSprintf("Docker images (%d) and container configurations", imgs))
			}
		} else {
			wouldKeep = append(wouldKeep, "Docker volumes and images")
		}
	}

	if has("mysql") && !backedUp["mysql"] {
		wouldLose = append(wouldLose, "MySQL databases and schemas")
	} else if has("mysql") {
		wouldKeep = append(wouldKeep, "MySQL databases")
	}

	for _, db := range []string{"postgres", "mongodb", "redis"} {
		if has(db) && !backedUp[db] {
			wouldLose = append(wouldLose, fmtSprintf("%s database data", moduleDisplayName(db)))
		} else if has(db) {
			wouldKeep = append(wouldKeep, fmtSprintf("%s database backups", moduleDisplayName(db)))
		}
	}

	if has("cloud") && !backedUp["cloud"] {
		for _, res := range results {
			if res.Module == "cloud" && res.Metadata != nil {
				if clis, ok := res.Metadata["detectedCLIs"]; ok {
					if list, ok := clis.([]string); ok && len(list) > 0 {
						wouldLose = append(wouldLose, fmtSprintf("Cloud CLI credentials (%d services)", len(list)))
					}
				}
			}
		}
	} else if has("cloud") {
		wouldKeep = append(wouldKeep, "Cloud CLI credentials")
	}

	if has("kubernetes") && !backedUp["kubernetes"] {
		wouldLose = append(wouldLose, "Kubernetes kubeconfig and cluster access")
	} else if has("kubernetes") {
		wouldKeep = append(wouldKeep, "Kubernetes configuration")
	}

	if has("vscode") {
		if !backedUp["vscode"] {
			if n, ok := getMetaInt("vscode", "extensions"); ok && n > 0 {
				wouldLose = append(wouldLose, fmtSprintf("VS Code extensions (%d) and settings", n))
			}
		} else {
			wouldKeep = append(wouldKeep, "VS Code extensions and settings")
		}
	}

	if has("dotfiles") {
		if !backedUp["dotfiles"] {
			wouldLose = append(wouldLose, "Shell dotfiles and personal configurations")
		} else {
			wouldKeep = append(wouldKeep, "Dotfiles and shell configuration")
		}
	}

	if has("git") && !backedUp["git"] {
		wouldLose = append(wouldLose, "Git configuration (username, email, signing)")
	} else {
		wouldKeep = append(wouldKeep, "Git configuration")
	}

	if has("shell") {
		wouldKeep = append(wouldKeep, "Shell preferences and framework")
	}

	return DisasterPreview{WouldLose: wouldLose, WouldKeep: wouldKeep}
}

func computeSummary(r *Report) string {
	c := r.Confidence

	highRiskCount := 0
	for _, risk := range r.Risks {
		if risk.Severity == "High" || risk.Severity == "Critical" {
			highRiskCount++
		}
	}

	unprotected := len(r.Coverage.Unprotected)
	atRisk := 0
	for _, cat := range r.Readiness {
		if cat.Status != "Recoverable" {
			atRisk++
		}
	}

	var msg string

	switch {
	case c.Score >= 85:
		msg = "Your workstation is in good health. "
		if unprotected > 0 {
			msg += fmtSprintf("%d assets are not yet backed up. ", unprotected)
		} else {
			msg += "All critical assets are protected. "
		}
	case c.Score >= 70:
		msg = "Your workstation is generally in good health. "
		if hasSSHAndGPG(r.DisasterPreview) {
			msg += "Developer identity is well protected through SSH and GPG. "
		}
	case c.Score >= 50:
		msg = "Your workstation has several recovery gaps that need attention. "
	default:
		msg = "Your workstation has critical recovery risks. Immediate action is recommended. "
	}

	risks := highestRisks(r.Risks)
	if len(risks) > 0 {
		msg += fmtSprintf("The highest remaining risks are %s. ", risks)
	}

	if c.Score < 85 {
		target := c.Score + 25
		if target > 95 {
			target = 95
		}
		msg += fmtSprintf("Completing the first %d recommended actions would increase recovery confidence from approximately %d%% to around %d%%.", min(4, len(r.ActionPlan)), c.Score, target)
	} else {
		msg += "Continue monitoring to maintain this level of protection."
	}

	return msg
}

func hasSSHAndGPG(dp DisasterPreview) bool {
	for _, k := range dp.WouldKeep {
		if strings.Contains(k, "SSH") || strings.Contains(k, "GPG") {
			return true
		}
	}
	return false
}

func highestRisks(risks []Risk) string {
	var parts []string
	seen := make(map[string]bool)

	for _, r := range risks {
		if len(parts) >= 3 {
			break
		}
		if r.Severity == "High" || r.Severity == "Critical" {
			lower := strings.ToLower(r.Message)
			if seen[lower] {
				continue
			}
			seen[lower] = true

			short := r.Message
			if len(short) > 40 {
				short = short[:37] + "..."
			}
			parts = append(parts, short)
		}
	}

	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return stringsJoin(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
}

func moduleDisplayName(mod string) string {
	names := map[string]string{
		"ssh": "SSH", "gpg": "GPG", "docker": "Docker",
		"firefox": "Firefox", "chrome": "Chrome", "chromium": "Chromium",
		"brave": "Brave", "vivaldi": "Vivaldi", "edge": "Edge", "opera": "Opera",
		"postgres": "PostgreSQL", "mysql": "MySQL", "mongodb": "MongoDB",
		"redis": "Redis", "sqlite": "SQLite",
		"golang": "Go", "nodejs": "Node.js", "python": "Python",
		"rust": "Rust", "java": "Java",
		"vscode": "VS Code", "neovim": "Neovim",
		"dotfiles": "Dotfiles", "shell": "Shell", "git": "Git",
		"cloud": "Cloud", "kubernetes": "Kubernetes",
		"repos": "Repositories", "certs": "Certificates",
		"virtualization": "Virtualization",
	}
	if n, ok := names[mod]; ok {
		return n
	}
	return mod
}

func formatDuration(mins int) string {
	if mins < 60 {
		return fmtSprintf("%d min", mins)
	}
	h := mins / 60
	m := mins % 60
	if m == 0 {
		return fmtSprintf("%d hr", h)
	}
	return fmtSprintf("%d hr %d min", h, m)
}

func fmtSprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

func stringsJoin(elems []string, sep string) string {
	return strings.Join(elems, sep)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
