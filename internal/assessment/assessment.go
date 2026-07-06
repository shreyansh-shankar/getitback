package assessment

import (
	"sort"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

func ComputeCoverage(results []*module.InventoryResult) map[string]module.Coverage {
	cov := make(map[string]module.Coverage)
	for _, res := range results {
		if !res.Detected {
			continue
		}
		c := module.Coverage{}
		for _, r := range res.Resources {
			c.Resources++
			switch r.Type {
			case module.ResourceTypeConfig:
				c.Configs++
				c.HasConfig = true
			case module.ResourceTypeSecret:
				c.Secrets++
				c.HasSecret = true
			case module.ResourceTypeData:
				c.Data++
				c.HasData = true
			}
		}
		cov[res.Module] = c
	}
	return cov
}

func ComputeScore(results []*module.InventoryResult, coverage map[string]module.Coverage, categories map[string]string) module.RecoveryScore {
	var identity, config, dev, editors, browsers, pkgs, databases int
	var containers, cloud, infra, projects, virt int

	for _, res := range results {
		if !res.Detected {
			continue
		}
		cat := categories[res.Module]
		c := coverage[res.Module]
		base := 1
		if c.Resources > 0 {
			base = 5
			if c.HasConfig {
				base += 2
			}
			if c.HasData {
				base += 2
			}
		}

		switch cat {
		case "Identity":
			switch res.Module {
			case "ssh":
				identity += scoreIdentity(base, res, c)
			case "gpg":
				identity += 3
			default:
				identity += base
			}
		case "Configuration":
			switch res.Module {
			case "dotfiles":
				config += scoreDotfiles(c)
			case "shell":
				config += scoreShell(res)
			case "git":
				config += scoreGit(res)
			case "system":
				config += 1
			default:
				config += base
			}
		case "Development":
			dev += scoreLanguage(res)
		case "Editors":
			editors += scoreEditor(res, c)
		case "Browsers":
			browsers += scoreBrowser(res)
		case "Packages":
			pkgs += scorePackage(res)
		case "Databases":
			databases += scoreDatabase(res)
		case "Containers":
			containers += scoreContainers(res)
		case "Cloud":
			cloud += scoreCloud(res)
		case "Infrastructure":
			infra += scoreInfrastructure(res)
		case "Projects":
			projects += scoreProjects(res)
		case "Virtualization":
			virt += scoreVirtualization(res)
		default:
			if cat != "" {
				dev += base
			}
		}
	}

	if identity > 15 {
		identity = 15
	}
	if config > 15 {
		config = 15
	}
	if dev > 20 {
		dev = 20
	}
	if editors > 15 {
		editors = 15
	}
	if browsers > 15 {
		browsers = 15
	}
	if pkgs > 10 {
		pkgs = 10
	}
	if databases > 10 {
		databases = 10
	}
	if containers > 15 {
		containers = 15
	}
	if cloud > 10 {
		cloud = 10
	}
	if infra > 10 {
		infra = 10
	}
	if projects > 10 {
		projects = 10
	}
	if virt > 10 {
		virt = 10
	}

	total := identity + config + dev + editors + browsers + pkgs + databases + containers + cloud + infra + projects + virt
	return module.RecoveryScore{
		Total:          total,
		Identity:       identity,
		Configuration:  config,
		Development:    dev,
		Editors:        editors,
		Browsers:       browsers,
		Packages:       pkgs,
		Databases:      databases,
		Containers:     containers,
		Cloud:          cloud,
		Infrastructure: infra,
		Projects:       projects,
		Virtualization: virt,
	}
}

func scoreIdentity(base int, res *module.InventoryResult, c module.Coverage) int {
	s := base
	if n, ok := res.Metadata["identityCount"]; ok {
		if count, ok := toInt(n); ok {
			s += count * 3
		}
	}
	if len(res.Warnings) == 0 {
		s += 3
	}
	return s
}

func scoreDotfiles(c module.Coverage) int {
	s := 0
	if c.Configs > 0 {
		s += 5
		if c.Configs >= 5 {
			s += 3
		}
		if c.Configs >= 10 {
			s += 2
		}
	}
	return s
}

func scoreShell(res *module.InventoryResult) int {
	s := 2
	if res.Version != "" {
		s += 1
	}
	if meta, ok := res.Metadata["starship"]; ok {
		if v, ok := meta.(string); ok && v == "yes" {
			s += 2
		}
	}
	if _, ok := res.Metadata["frameworks"]; ok {
		s += 2
	}
	return s
}

func scoreGit(res *module.InventoryResult) int {
	s := 3
	if u, ok := res.Metadata["username"]; ok && u != "" {
		s += 2
	}
	if e, ok := res.Metadata["email"]; ok && e != "" {
		s += 2
	}
	if res.Metadata["signingkey"] != nil {
		s += 3
	}
	return s
}

func scoreLanguage(res *module.InventoryResult) int {
	s := 2
	if res.Version != "" {
		s += 2
	}
	if c, ok := res.Metadata["packageCount"]; ok {
		if count, ok := toInt(c); ok && count > 0 {
			s += 3
		}
	}
	if n, ok := res.Metadata["globalPackages"]; ok {
		if count, ok := toInt(n); ok && count > 0 {
			s += 2
		}
	}
	if _, ok := res.Metadata["poetry"]; ok {
		s += 2
	}
	if _, ok := res.Metadata["uv"]; ok {
		s += 2
	}
	if _, ok := res.Metadata["corepack"]; ok {
		s += 1
	}
	return s
}

func scoreEditor(res *module.InventoryResult, c module.Coverage) int {
	s := 3
	if res.Version != "" {
		s += 2
	}
	if c.Configs > 0 {
		s += 3
	}
	if n, ok := res.Metadata["extensions"]; ok {
		if count, ok := toInt(n); ok && count > 0 {
			s += 3
		}
	}
	return s
}

func scoreBrowser(res *module.InventoryResult) int {
	s := 2
	if res.Version != "" {
		s += 2
	}
	if n, ok := res.Metadata["profileCount"]; ok {
		if count, ok := toInt(n); ok && count > 0 {
			s += 3
		}
	}
	return s
}

func scorePackage(res *module.InventoryResult) int {
	s := 2
	if res.Version != "" {
		s += 1
	}
	if n, ok := res.Metadata["snapCount"]; ok {
		if count, ok := toInt(n); ok && count > 0 {
			s += 3
		}
	}
	if n, ok := res.Metadata["manualPackages"]; ok {
		if count, ok := toInt(n); ok && count > 0 {
			s += 2
		}
	}
	if n, ok := res.Metadata["apps"]; ok {
		if count, ok := toInt(n); ok && count > 0 {
			s += 2
		}
	}
	return s
}

func scoreDatabase(res *module.InventoryResult) int {
	s := 3
	if res.Version != "" {
		s += 3
	}
	if n, ok := res.Metadata["databases"]; ok {
		if list, ok := n.([]string); ok && len(list) > 0 {
			s += 4
		}
	}
	return s
}

func scoreContainers(res *module.InventoryResult) int {
	s := 3
	if res.Version != "" {
		s += 3
	}
	if n, ok := res.Metadata["containers"]; ok {
		if count, ok := toInt(n); ok && count > 0 {
			s += 3
		}
	}
	if n, ok := res.Metadata["volumes"]; ok {
		if count, ok := toInt(n); ok && count > 0 {
			s += 3
		}
	}
	if n, ok := res.Metadata["composeProjects"]; ok {
		if count, ok := toInt(n); ok && count > 0 {
			s += 3
		}
	}
	return s
}

func scoreCloud(res *module.InventoryResult) int {
	s := 2
	if n, ok := res.Metadata["detectedCLIs"]; ok {
		if list, ok := n.([]string); ok {
			s += len(list) * 2
		}
	}
	return s
}

func scoreInfrastructure(res *module.InventoryResult) int {
	s := 2
	if n, ok := res.Metadata["detectedTools"]; ok {
		if list, ok := n.([]string); ok {
			s += len(list) * 2
		}
	}
	if n, ok := res.Metadata["kubeContexts"]; ok {
		if list, ok := n.([]string); ok && len(list) > 0 {
			s += 3
		}
	}
	return s
}

func scoreProjects(res *module.InventoryResult) int {
	s := 2
	if n, ok := res.Metadata["totalRepos"]; ok {
		if count, ok := toInt(n); ok && count > 0 {
			s += 5
			if count >= 10 {
				s += 3
			}
		}
	}
	return s
}

func scoreVirtualization(res *module.InventoryResult) int {
	s := 2
	if n, ok := res.Metadata["detectedPlatforms"]; ok {
		if list, ok := n.([]string); ok {
			s += len(list) * 2
		}
	}
	return s
}

func ComputeWeightedScore(score module.RecoveryScore) int {
	return score.Total
}

func ScoreGrade(score int) string {
	switch {
	case score >= 85:
		return "A"
	case score >= 70:
		return "B"
	case score >= 50:
		return "C"
	case score >= 30:
		return "D"
	default:
		return "F"
	}
}

func GenerateRecommendations(results []*module.InventoryResult, coverage map[string]module.Coverage, categories map[string]string) []module.Recommendation {
	var recs []module.Recommendation

	seen := make(map[string]bool)

	addRec := func(priority module.RecommendationPriority, category, message, help string) {
		key := message
		if seen[key] {
			return
		}
		seen[key] = true
		recs = append(recs, module.Recommendation{
			Priority: priority,
			Category: category,
			Message:  message,
			Help:     help,
		})
	}

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
					if s, ok := v.(string); ok {
						return s, true
					}
				}
			}
		}
		return "", false
	}

	moduleWarnings := func(mod string) []string {
		for _, res := range results {
			if res.Module == mod && res.Detected {
				return res.Warnings
			}
		}
		return nil
	}

	// Encrypted backups
	if !has("encryption") {
		addRec(module.RecPriorityHigh, "Security",
			"No encryption configured for backups",
			"Set encryption.enabled: true and encryption.key: /path/to/key in ~/.getitback/config.yaml")
	}

	// SSH
	if has("ssh") {
		for _, w := range moduleWarnings("ssh") {
			addRec(module.RecPriorityHigh, "Identity", w, "Run 'ssh-keygen -t ed25519 -a 100' to generate a new key")
		}
		if identities, ok := getMeta("ssh", "identityCount"); ok && identities == "0" {
			addRec(module.RecPriorityMedium, "Identity",
				"No SSH identities found",
				"Generate an SSH key: ssh-keygen -t ed25519 -a 100")
		}
	} else {
		addRec(module.RecPriorityLow, "Identity",
			"SSH not detected — are you on a server or CI environment?",
			"")
	}

	// Git
	if has("git") {
		if u, ok := getMeta("git", "username"); !ok || u == "" {
			addRec(module.RecPriorityHigh, "Configuration",
				"Git username not set",
				"Run: git config --global user.name \"Your Name\"")
		}
		if e, ok := getMeta("git", "email"); !ok || e == "" {
			addRec(module.RecPriorityHigh, "Configuration",
				"Git email not set",
				"Run: git config --global user.email \"you@example.com\"")
		}
		if _, ok := getMeta("git", "signingkey"); !ok {
			addRec(module.RecPriorityMedium, "Configuration",
				"Git commit signing not configured",
				"Run: git config --global user.signingkey <key-id> && git config --global commit.gpgsign true")
		}
	} else {
		addRec(module.RecPriorityLow, "Configuration",
			"Git not detected",
			"Install git to track your dotfiles and configuration")
	}

	// GPG
	if has("gpg") {
		addRec(module.RecPriorityMedium, "Identity",
			"GPG keys detected — ensure they are backed up",
			"Export keys: gpg --export-secret-keys --armor > keys.asc")
	}

	// Dotfiles
	if has("dotfiles") {
		c := coverage["dotfiles"]
		if c.Configs > 0 {
			addRec(module.RecPriorityMedium, "Configuration",
				"Dotfiles detected — consider version-controlling them",
				"Initialize a dotfiles repo: git init --bare $HOME/.dotfiles && alias dotfiles='git --git-dir=$HOME/.dotfiles/ --work-tree=$HOME'")
		}
		if c.Secrets > 0 {
			addRec(module.RecPriorityHigh, "Security",
				"Secret files detected among dotfiles — review and separate them",
				"Run: getitback secrets  (or: getitback inventory -v)")
		}
	} else {
		addRec(module.RecPriorityLow, "Configuration",
			"No dotfiles directory found",
			"Initialize a dotfiles repo to track your configuration")
	}

	// Browsers
	var browserCount int
	for _, res := range results {
		if categories[res.Module] == "Browsers" && res.Detected {
			browserCount++
			if n, ok := res.Metadata["profileCount"]; ok {
				if count, ok := toInt(n); ok && count > 0 {
					addRec(module.RecPriorityMedium, "Browsers",
						"Browser profiles detected — backup bookmarks, extensions, and settings",
						"Run: getitback backup --module "+res.Module)
				}
			}
		}
	}
	if browserCount > 0 {
		addRec(module.RecPriorityLow, "Browsers",
			"Multiple browsers detected — consider which is your primary",
			"Browser profiles are some of the most valuable data to back up")
	}

	// Languages
	for _, lang := range []string{"golang", "nodejs", "python", "rust"} {
		if has(lang) {
			addRec(module.RecPriorityLow, "Development",
				lang+" detected — ensure your development environment is reproducible",
				"Use go.mod, package.json, pyproject.toml, Cargo.toml to pin dependencies")
		}
	}

	// VSCode
	if has("vscode") {
		if n, ok := getMeta("vscode", "extensions"); ok {
			if n != "0" {
				addRec(module.RecPriorityLow, "Editors",
					"VS Code extensions detected — export the list for easy reinstall",
					"Run: code --list-extensions > vscode-extensions.txt")
			}
		}
	}

	// SSH key backup recommendation
	if em, ok := getMeta("ssh", "identityCount"); ok && em != "0" {
		addRec(module.RecPriorityHigh, "Identity",
			"SSH keys detected — ensure they are securely backed up",
			"Run: getitback backup --module ssh")
	}

	// Docker
	if has("docker") {
		addRec(module.RecPriorityMedium, "Development",
			"Docker detected — back up Dockerfiles, compose files, and volumes",
			"Run: getitback backup --module docker")
	}

	// Databases
	for _, db := range []string{"postgres", "mongodb", "redis", "mysql"} {
		if has(db) {
			addRec(module.RecPriorityMedium, "Databases",
				db+" detected — schedule regular dumps",
				"Run: getitback backup --module "+db)
		}
	}

	// Docker
	if has("docker") {
		if n, ok := getMeta("docker", "volumes"); ok && n != "0" {
			addRec(module.RecPriorityMedium, "Containers",
				"Docker volumes detected — enable volume backups",
				"Run: getitback backup --module docker")
		}
		if n, ok := getMeta("docker", "composeProjects"); ok && n != "0" {
			addRec(module.RecPriorityMedium, "Containers",
				"Docker Compose projects detected — back up compose files",
				"Run: getitback backup --module docker")
		}
	}

	// Cloud CLI
	if has("cloud") {
		for _, res := range results {
			if res.Module == "cloud" && res.Metadata != nil {
				if clis, ok := res.Metadata["detectedCLIs"]; ok {
					if list, ok := clis.([]string); ok && len(list) > 0 {
						addRec(module.RecPriorityMedium, "Cloud",
							"Cloud CLI tools detected — ensure credentials are securely backed up",
							"Check ~/.aws, ~/.azure, ~/.config/gcloud for credential files")
					}
				}
			}
		}
	}

	// Kubernetes
	if has("kubernetes") {
		currentCtx, ok := getMeta("kubernetes", "currentContext")
		if ok && currentCtx != "" && currentCtx != "minikube" && currentCtx != "kind-kind" {
			addRec(module.RecPriorityMedium, "Infrastructure",
				"Current kubectl context is "+currentCtx+" — recommend encrypted kubeconfig backup",
				"Run: getitback backup --module kubernetes")
		}
	}

	// Repos
	if has("repos") {
		if n, ok := getMeta("repos", "noRemoteRepos"); ok && n != "0" {
			addRec(module.RecPriorityMedium, "Projects",
				"Repositories without remotes detected — back them up locally",
				"Run: git remote add origin <url> to add a remote")
		}
		if n, ok := getMeta("repos", "dirtyRepos"); ok && n != "0" {
			addRec(module.RecPriorityLow, "Projects",
				"Repositories with uncommitted changes detected — commit or stash them",
				"Run: git status in each affected repo")
		}
	}

	// Java
	if has("java") {
		addRec(module.RecPriorityLow, "Development",
			"Java detected — ensure your build is reproducible",
			"Use Gradle or Maven wrapper to pin build tool versions")
	}

	sort.Slice(recs, func(i, j int) bool {
		order := map[module.RecommendationPriority]int{
			module.RecPriorityCritical: 0,
			module.RecPriorityHigh:     1,
			module.RecPriorityMedium:   2,
			module.RecPriorityLow:      3,
		}
		if order[recs[i].Priority] != order[recs[j].Priority] {
			return order[recs[i].Priority] < order[recs[j].Priority]
		}
		return recs[i].Message < recs[j].Message
	})

	return recs
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	case string:
		return 0, false
	}
	return 0, false
}
