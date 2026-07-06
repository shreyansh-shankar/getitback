package report

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/storage"
)

var Version = "v0.x.x"

func NewReport(results []*module.InventoryResult, backups []storage.BackupEntry, configHasEncryption bool, backedUp map[string]bool, score *module.RecoveryScore) *Report {
	r := &Report{}
	if score == nil {
		score = &module.RecoveryScore{}
	}

	hostname, _ := os.Hostname()
	now := time.Now()

	has := func(mod string) bool {
		for _, res := range results {
			if res.Module == mod && res.Detected {
				return true
			}
		}
		return false
	}

	getMeta := func(mod, key string) (any, bool) {
		for _, res := range results {
			if res.Module == mod && res.Detected && res.Metadata != nil {
				if v, ok := res.Metadata[key]; ok {
					return v, true
				}
			}
		}
		return nil, false
	}

	getMetaStr := func(mod, key string) string {
		if v, ok := getMeta(mod, key); ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}

	getMetaInt := func(mod, key string) int {
		if v, ok := getMeta(mod, key); ok {
			switch n := v.(type) {
			case int:
				return n
			case int64:
				return int(n)
			case float64:
				return int(n)
			}
		}
		return 0
	}

	getMetaStrList := func(mod, key string) []string {
		if v, ok := getMeta(mod, key); ok {
			switch list := v.(type) {
			case []string:
				return list
			case []any:
				var out []string
				for _, item := range list {
					if s, ok := item.(string); ok {
						out = append(out, s)
					}
				}
				return out
			}
		}
		return nil
	}

	isBackedUp := func(mod string) bool {
		if backedUp == nil {
			return false
		}
		return backedUp[mod]
	}

	allModules := []string{
		"system", "git", "ssh", "gpg", "shell", "dotfiles",
		"vscode", "neovim",
		"firefox", "chrome", "chromium", "brave", "vivaldi", "edge", "opera",
		"postgres", "mysql", "mongodb", "redis", "sqlite",
		"golang", "nodejs", "python", "rust", "java",
		"docker", "cloud", "kubernetes", "repos", "certs",
		"apt", "snap", "flatpak",
		"virtualization",
	}

	notDetected := []string{}
	for _, m := range allModules {
		if !has(m) {
			notDetected = append(notDetected, m)
		}
	}
	r.NotDetected = notDetected

	osName := getMetaStr("system", "os")
	if osName == "" {
		osName = getMetaStr("system", "kernel")
	}
	if osName == "" {
		osName = runtime.GOOS
	}

	buildHeader(r, hostname, now, osName, backups)
	buildOverview(r, has, isBackedUp, backups, configHasEncryption, score)
	buildSummary(r, has, getMetaStr, getMetaInt, getMetaStrList)
	buildMachine(r, has, getMetaStr, getMetaInt)
	buildDevelopment(r, has, getMetaStr, getMetaInt)
	buildIdentity(r, has, getMetaStr, getMetaInt, isBackedUp)
	buildBrowsers(r, has, getMetaStr, getMetaInt, results, isBackedUp)
	buildEditors(r, results, isBackedUp)
	buildDatabases(r, results, getMetaStr, getMetaInt, isBackedUp)
	buildContainers(r, has, getMetaStr, getMetaInt, isBackedUp)
	buildCloud(r, has, getMetaStr, getMetaStrList, isBackedUp)
	buildInfrastructure(r, results, getMetaStr, getMetaStrList, isBackedUp)
	buildPackages(r, has, getMetaStr, getMetaInt)
	buildSecurity(r, has, getMetaInt)
	buildProjects(r, has, getMetaInt)
	buildVirtualization(r, has, getMetaStrList, isBackedUp)
	buildBackupSummary(r, backups, configHasEncryption, results, isBackedUp)
	buildGaps(r, results, isBackedUp, has)
	buildStatistics(r, has, getMetaInt, getMetaStrList)
	buildCoverage(r, results, len(allModules))
	buildVerdict(r)
	buildMetadata(r, hostname, now)

	return r
}

func buildHeader(r *Report, hostname string, now time.Time, osName string, backups []storage.BackupEntry) {
	h := ReportHeader{
		Hostname:         hostname,
		GeneratedAt:      now.Format("2006-01-02 15:04 MST"),
		GeneratedAtUnix:  now.Unix(),
		OperatingSystem:  osName,
		GetItBackVersion: Version,
	}
	if len(backups) > 0 {
		h.RecoverySnapshot = backups[0].ID
		h.SnapshotCount = len(backups)
	}
	r.Header = h
}

func buildSummary(r *Report, has func(string) bool, getMetaStr func(string, string) string, getMetaInt func(string, string) int, getMetaStrList func(string, string) []string) {
	s := ExecutiveSummary{
		OperatingSystem: r.Header.OperatingSystem,
	}

	if has("shell") {
		s.PrimaryShell = getMetaStr("shell", "basename")
	}
	if has("vscode") {
		s.PrimaryEditor = "VS Code"
	} else if has("neovim") {
		s.PrimaryEditor = "Neovim"
	}
	for _, b := range []string{"firefox", "chrome", "chromium", "brave", "vivaldi", "edge", "opera"} {
		if has(b) {
			s.PrimaryBrowser = browserDisplayName(b)
			break
		}
	}

	var langs []string
	for _, l := range []string{"golang", "nodejs", "python", "rust", "java"} {
		if has(l) {
			langs = append(langs, langDisplayName(l))
		}
	}
	s.Languages = langs

	if has("docker") {
		s.Containers = getMetaInt("docker", "containers")
		s.DockerVolumes = getMetaInt("docker", "volumes")
	}
	if has("repos") {
		s.Repositories = getMetaInt("repos", "totalRepos")
	}
	if has("cloud") {
		s.CloudProviders = getMetaStrList("cloud", "detectedCLIs")
	}
	s.BackupSnapshots = r.Header.SnapshotCount

	var dbs []string
	for _, db := range []string{"postgres", "mysql", "mongodb", "redis", "sqlite"} {
		if has(db) {
			dbs = append(dbs, dbDisplayName(db))
		}
	}
	s.Databases = dbs

	r.Summary = s
}

func buildMachine(r *Report, has func(string) bool, getMetaStr func(string, string) string, getMetaInt func(string, string) int) {
	m := MachineProfile{}
	if has("system") {
		m.Architecture = getMetaStr("system", "arch")
		m.Kernel = getMetaStr("system", "kernel")
		m.Desktop = getMetaStr("system", "desktop")
		m.Session = getMetaStr("system", "session")
		m.Timezone = getMetaStr("system", "timezone")
		m.Locale = getMetaStr("system", "locale")
		if cpu := getMetaStr("system", "cpu"); cpu != "" {
			m.CPU = cpu
		} else {
			if c := getMetaInt("system", "cpuCores"); c > 0 {
				m.CPU = fmt.Sprintf("%d cores", c)
			}
		}
		if ram := getMetaStr("system", "memory"); ram != "" {
			m.RAM = ram
		} else if r := getMetaInt("system", "memoryMB"); r > 0 {
			m.RAM = fmt.Sprintf("%d MB", r)
		}
		if disk := getMetaStr("system", "disk_total"); disk != "" {
			m.Storage = disk
		}
		if u := getMetaStr("system", "disk_used"); u != "" {
			m.StorageUsed = u
		}
		if a := getMetaStr("system", "disk_avail"); a != "" {
			m.StorageAvail = a
		}
	}
	r.Machine = m
}

func buildDevelopment(r *Report, has func(string) bool, getMetaStr func(string, string) string, getMetaInt func(string, string) int) {
	d := DevelopmentStack{}

	var langs []LanguageInfo
	for _, l := range []struct{ mod, name string }{
		{"golang", "Go"}, {"nodejs", "Node.js"}, {"python", "Python"},
		{"rust", "Rust"}, {"java", "Java"},
	} {
		if has(l.mod) {
			li := LanguageInfo{
				Name:         l.name,
				Version:      getMetaStr(l.mod, "version"),
				PackageCount: getMetaInt(l.mod, "packageCount"),
				GlobalPkgs:   getMetaInt(l.mod, "globalPackages"),
			}
			if li.Version == "" {
				li.Version = getMetaStr(l.mod, "goVersion")
			}
			langs = append(langs, li)
		}
	}
	d.Languages = langs

	var vcs []VCInfo
	if has("git") {
		details := ""
		if sk := getMetaStr("git", "signingKey"); sk != "" {
			details = "Signing: " + sk
		}
		vcs = append(vcs, VCInfo{
			Name: "Git", Version: getMetaStr("git", "version"),
			Configured: true, Details: details,
		})
	}
	vcs = append(vcs, VCInfo{Name: "SSH", Configured: has("ssh")})
	vcs = append(vcs, VCInfo{Name: "GPG", Configured: has("gpg")})
	d.VersionControl = vcs

	d.PackageManagers = buildPackageMgrs(has, getMetaStr, getMetaInt)

	r.Development = d
}

func buildPackageMgrs(has func(string) bool, getMetaStr func(string, string) string, getMetaInt func(string, string) int) []PackageManagerInfo {
	var pkgs []PackageManagerInfo
	for _, p := range []string{"apt", "snap", "flatpak"} {
		if has(p) {
			info := PackageManagerInfo{
				Name:    p,
				Version: getMetaStr(p, "version"),
				Count:   getMetaInt(p, fmt.Sprintf("%sCount", p)),
			}
			if info.Count == 0 {
				if p == "apt" {
					info.Count = getMetaInt("apt", "installedPackages")
				} else if p == "snap" {
					info.Count = getMetaInt("snap", "snapCount")
				}
			}
			pkgs = append(pkgs, info)
		}
	}
	if pkgs == nil {
		pkgs = []PackageManagerInfo{}
	}
	return pkgs
}

func buildIdentity(r *Report, has func(string) bool, getMetaStr func(string, string) string, getMetaInt func(string, string) int, isBackedUp func(string) bool) {
	id := IdentitySection{}
	if has("ssh") {
		id.SSH = &SSHInfo{
			Version:       getMetaStr("ssh", "version"),
			IdentityCount: getMetaInt("ssh", "identityCount"),
			Keys:          getMetaInt("ssh", "keys"),
			BackedUp:      isBackedUp("ssh"),
		}
	}
	if has("gpg") {
		id.GPG = &GPGInfo{
			Version:  getMetaStr("gpg", "version"),
			KeyCount: getMetaInt("gpg", "keyCount"),
			BackedUp: isBackedUp("gpg"),
		}
	}
	r.Identity = id
}

func buildBrowsers(r *Report, has func(string) bool, getMetaStr func(string, string) string, getMetaInt func(string, string) int, results []*module.InventoryResult, isBackedUp func(string) bool) {
	var browsers []BrowserInfo
	for _, b := range []string{"firefox", "chrome", "chromium", "brave", "vivaldi", "edge", "opera"} {
		if has(b) {
			storage := ""
			for _, res := range results {
				if res.Module == b {
					var total int64
					for _, rsrc := range res.Resources {
						total += rsrc.Size
					}
					if total > 0 {
						storage = formatBytes(total)
					}
					break
				}
			}
			level := recoveryLevels[b]
			if level == 0 {
				level = 2
			}
			browsers = append(browsers, BrowserInfo{
				Name:           browserDisplayName(b),
				Version:        getMetaStr(b, "version"),
				ProfileCount:   getMetaInt(b, "profileCount"),
				DefaultProfile: getMetaStr(b, "defaultProfile"),
				Storage:        storage,
				InstallMethod:  getMetaStr(b, "installMethod"),
				BackedUp:       isBackedUp(b),
				RecoveryLevel:  level,
			})
		}
	}
	if browsers == nil {
		browsers = []BrowserInfo{}
	}
	r.Browsers = browsers
}

func buildEditors(r *Report, results []*module.InventoryResult, isBackedUp func(string) bool) {
	var editors []EditorInfo
	for _, res := range results {
		switch res.Module {
		case "vscode":
			editors = append(editors, EditorInfo{
				Name:          "VS Code",
				Version:       res.Version,
				Extensions:    getMetaIntFromRes(res, "extensions"),
				Settings:      getMetaIntFromRes(res, "settings"),
				Themes:        getMetaIntFromRes(res, "themes"),
				Snippets:      getMetaIntFromRes(res, "snippets"),
				BackedUp:      isBackedUp("vscode"),
				RecoveryLevel: recoveryLevels["vscode"],
			})
		case "neovim":
			editors = append(editors, EditorInfo{
				Name:          "Neovim",
				Version:       res.Version,
				BackedUp:      isBackedUp("neovim"),
				RecoveryLevel: recoveryLevels["neovim"],
			})
		}
	}
	if editors == nil {
		editors = []EditorInfo{}
	}
	r.Editors = editors
}

func buildDatabases(r *Report, results []*module.InventoryResult, getMetaStr func(string, string) string, getMetaInt func(string, string) int, isBackedUp func(string) bool) {
	var databases []DatabaseInfo
	for _, db := range []string{"postgres", "mysql", "mongodb", "redis", "sqlite"} {
		for _, res := range results {
			if res.Module == db && res.Detected {
				storage := ""
				var total int64
				for _, rsrc := range res.Resources {
					total += rsrc.Size
				}
				if total > 0 {
					storage = formatBytes(total)
				}
				level := recoveryLevels[db]
				if level == 0 {
					level = 3
				}
				databases = append(databases, DatabaseInfo{
					Name:      dbDisplayName(db),
					Version:   res.Version,
					Databases: getMetaInt(db, "databases"),
					DataDir:   getMetaStr(db, "dataDir"),
					ConfigFile: getMetaStr(db, "config"),
					Storage:   storage,
					BackedUp:  isBackedUp(db),
					RecoveryLevel: level,
				})
				break
			}
		}
	}
	if databases == nil {
		databases = []DatabaseInfo{}
	}
	r.Databases = databases
}

func buildContainers(r *Report, has func(string) bool, getMetaStr func(string, string) string, getMetaInt func(string, string) int, isBackedUp func(string) bool) {
	if !has("docker") {
		return
	}
	r.Containers = &ContainerInfo{
		Version:         getMetaStr("docker", "version"),
		Containers:      getMetaInt("docker", "containers"),
		Running:         getMetaInt("docker", "runningContainers"),
		Stopped:         getMetaInt("docker", "stoppedContainers"),
		Images:          getMetaInt("docker", "images"),
		Volumes:          getMetaInt("docker", "volumes"),
		Networks:        getMetaInt("docker", "networks"),
		CustomNetworks:  getMetaInt("docker", "customNetworks"),
		ComposeProjects: getMetaInt("docker", "composeProjects"),
		BuildCache:      getMetaStr("docker", "buildCache"),
		DanglingImages:  getMetaInt("docker", "danglingImages"),
		ImageStorage:    getMetaStr("docker", "imageStorage"),
		VolumeStorage:   getMetaStr("docker", "volumeStorage"),
		RootDir:         getMetaStr("docker", "rootDir"),
		Rootless:        getMetaStr("docker", "rootless") == "yes" || getMetaStr("docker", "rootless") == "true",
		BackedUp:        isBackedUp("docker"),
		RecoveryLevel:   recoveryLevels["docker"],
	}
}

func buildCloud(r *Report, has func(string) bool, getMetaStr func(string, string) string, getMetaStrList func(string, string) []string, isBackedUp func(string) bool) {
	if !has("cloud") {
		return
	}
	clis := getMetaStrList("cloud", "detectedCLIs")
	var providers []CloudProviderInfo
	for _, cli := range clis {
		safeKey := strings.ReplaceAll(strings.ToLower(cli), " ", "_")
		providers = append(providers, CloudProviderInfo{
			Name:          cli,
			CliInstalled:  true,
			Authenticated: getMetaStr("cloud", safeKey+"_auth") == "yes",
			AccountID:     getMetaStr("cloud", safeKey+"_account"),
			Credentials:   getMetaStr("cloud", safeKey+"_credentials") == "present",
			BackedUp:      isBackedUp("cloud"),
			RecoveryLevel: recoveryLevels["cloud"],
		})
	}
	r.Cloud = &CloudInfo{Providers: providers}
}

func buildInfrastructure(r *Report, results []*module.InventoryResult, getMetaStr func(string, string) string, getMetaStrList func(string, string) []string, isBackedUp func(string) bool) {
	infra := &InfrastructureInfo{
		Tools:          nil,
		BackedUp:       isBackedUp("kubernetes"),
		RecoveryLevel:  recoveryLevels["kubernetes"],
	}

	for _, res := range results {
		if res.Module == "kubernetes" && res.Detected {
			infra.Kubernetes = &KubernetesInfo{
				Version:        res.Version,
				CurrentContext: getMetaStr("kubernetes", "currentContext"),
				Contexts:       getMetaStrList("kubernetes", "kubeContexts"),
				Namespaces:     getMetaStrList("kubernetes", "namespaces"),
				HelmRepos:      getMetaStrList("kubernetes", "helmRepos"),
			}
		}
	}

	tools := getMetaStrList("kubernetes", "detectedTools")
	if len(tools) > 0 {
		infra.Tools = tools
	}

	if getMetaStr("kubernetes", "currentContext") != "" {
		infra.KubeconfigFound = true
	}

	if infra.Kubernetes == nil && len(infra.Tools) == 0 {
		return
	}

	r.Infra = infra
}

func buildPackages(r *Report, has func(string) bool, getMetaStr func(string, string) string, getMetaInt func(string, string) int) {
	p := PackageInfo{}
	if has("apt") {
		c := getMetaInt("apt", "installedPackages")
		if c == 0 {
			c = getMetaInt("apt", "manualPackages")
		}
		p.Apt = &PackageManagerInfo{
			Name: "apt", Version: getMetaStr("apt", "version"),
			Count: c,
		}
	}
	if has("snap") {
		p.Snap = &PackageManagerInfo{
			Name: "snap", Version: getMetaStr("snap", "version"),
			Count: getMetaInt("snap", "snapCount"),
		}
	}
	if has("flatpak") {
		p.Flatpak = &PackageManagerInfo{
			Name: "flatpak", Version: getMetaStr("flatpak", "version"),
			Count: getMetaInt("flatpak", "apps"),
		}
	}
	r.Packages = p
}

func buildSecurity(r *Report, has func(string) bool, getMetaInt func(string, string) int) {
	if !has("certs") {
		return
	}
	r.Security = &SecurityInfo{
		CertStores: getMetaInt("certs", "certificateStores"),
		ValidCerts: getMetaInt("certs", "validCerts"),
		Expiring:   getMetaInt("certs", "expiringCerts"),
		Expired:    getMetaInt("certs", "expiredCerts"),
		CABundles:  getMetaInt("certs", "customCABundles"),
	}
}

func buildProjects(r *Report, has func(string) bool, getMetaInt func(string, string) int) {
	if !has("repos") {
		return
	}
	r.Projects = &ProjectsInfo{
		TotalRepos:  getMetaInt("repos", "totalRepos"),
		DirtyRepos:  getMetaInt("repos", "dirtyRepos"),
		NoRemote:    getMetaInt("repos", "noRemoteRepos"),
		GitHubRepos: getMetaInt("repos", "githubRepos"),
		GitLabRepos: getMetaInt("repos", "gitlabRepos"),
		LocalOnly:   getMetaInt("repos", "localOnlyRepos"),
	}
}

func buildVirtualization(r *Report, has func(string) bool, getMetaStrList func(string, string) []string, isBackedUp func(string) bool) {
	if !has("virtualization") {
		return
	}
	r.Virtualization = &VirtualizationInfo{
		Platforms:     getMetaStrList("virtualization", "detectedPlatforms"),
		BackedUp:      isBackedUp("virtualization"),
		RecoveryLevel: recoveryLevels["virtualization"],
	}
}

func buildBackupSummary(r *Report, backups []storage.BackupEntry, configHasEncryption bool, results []*module.InventoryResult, isBackedUp func(string) bool) {
	bs := BackupSummary{
		SnapshotCount:   len(backups),
		Encryption:      "Disabled",
		StorageProvider: "Local",
		RecoverableCount: 0,
		TotalCount:      0,
	}
	if configHasEncryption {
		bs.Encryption = "Enabled"
	}
	if len(backups) > 0 {
		bs.LatestSnapshot = backups[0].ID
		bs.CreatedAt = backups[0].CreatedAt.Format("2006-01-02 15:04 MST")
		var totalSize int64
		for _, b := range backups {
			totalSize += b.Size
		}
		if totalSize > 0 {
			bs.TotalSize = formatBytes(totalSize)
		}
	}

	for _, res := range results {
		if res.Detected {
			bs.TotalCount++
		}
	}

	recoverable := 0
	for _, res := range results {
		if res.Detected {
			mod := res.Module
			if _, ok := recoveryLevels[mod]; ok && recoveryLevels[mod] >= 2 {
				if isBackedUp(mod) {
					recoverable++
				}
			}
		}
	}
	bs.RecoverableCount = recoverable
	bs.RestoreTime = estimateRestoreTime(results)
	r.Backups = bs
}

func estimateRestoreTime(results []*module.InventoryResult) string {
	estimates := map[string]int{
		"Identity": 2, "Configuration": 5, "Development": 10, "Editors": 3,
		"Browsers": 10, "Packages": 5, "Databases": 20, "Containers": 15,
		"Cloud": 5, "Infrastructure": 10, "Projects": 8, "Virtualization": 5,
	}
	catForMod := map[string]string{
		"ssh": "Identity", "gpg": "Identity", "dotfiles": "Configuration",
		"shell": "Configuration", "git": "Configuration",
		"golang": "Development", "nodejs": "Development", "python": "Development",
		"rust": "Development", "java": "Development",
		"vscode": "Editors", "neovim": "Editors",
		"firefox": "Browsers", "chrome": "Browsers", "chromium": "Browsers",
		"brave": "Browsers", "vivaldi": "Browsers", "edge": "Browsers", "opera": "Browsers",
		"postgres": "Databases", "mysql": "Databases", "mongodb": "Databases",
		"redis": "Databases", "sqlite": "Databases",
		"docker": "Containers", "cloud": "Cloud",
		"kubernetes": "Infrastructure", "repos": "Projects",
		"virtualization": "Virtualization", "apt": "Packages",
		"snap": "Packages", "flatpak": "Packages",
	}
	seenCats := make(map[string]bool)
	totalMin := 0
	for _, res := range results {
		if !res.Detected {
			continue
		}
		if cat, ok := catForMod[res.Module]; ok {
			if !seenCats[cat] {
				seenCats[cat] = true
				if mins, ok := estimates[cat]; ok {
					totalMin += mins
				}
			}
		}
	}
	if totalMin < 60 {
		return fmt.Sprintf("%d min", totalMin)
	}
	h := totalMin / 60
	m := totalMin % 60
	if m == 0 {
		return fmt.Sprintf("%d hr", h)
	}
	return fmt.Sprintf("%d hr %d min", h, m)
}

func buildStatistics(r *Report, has func(string) bool, getMetaInt func(string, string) int, getMetaStrList func(string, string) []string) {
	s := AssetStats{}
	for _, l := range []string{"golang", "nodejs", "python", "rust", "java"} {
		if has(l) {
			s.Languages++
		}
	}
	for _, b := range []string{"firefox", "chrome", "chromium", "brave", "vivaldi", "edge", "opera"} {
		if has(b) {
			s.Browsers++
		}
	}
	for _, e := range []string{"vscode", "neovim"} {
		if has(e) {
			s.Editors++
		}
	}
	for _, db := range []string{"postgres", "mysql", "mongodb", "redis", "sqlite"} {
		if has(db) {
			s.Databases++
		}
	}
	if has("docker") {
		s.Containers = getMetaInt("docker", "containers")
		s.DockerVolumes = getMetaInt("docker", "volumes")
		s.ComposeProjects = getMetaInt("docker", "composeProjects")
	}
	if has("repos") {
		s.Repositories = getMetaInt("repos", "totalRepos")
	}
	if has("certs") {
		s.Certificates = getMetaInt("certs", "certificateFiles")
	}
	if has("ssh") {
		s.SSHKeys = getMetaInt("ssh", "keys")
	}
	if has("gpg") {
		s.GPGKeys = getMetaInt("gpg", "keyCount")
	}
	if has("cloud") {
		s.CloudProviders = len(getMetaStrList("cloud", "detectedCLIs"))
	}
	r.Statistics = s
}

func buildCoverage(r *Report, results []*module.InventoryResult, totalModules int) {
	detected := 0
	for _, res := range results {
		if res.Detected {
			detected++
		}
	}
	missing := totalModules - detected
	pct := 0
	if totalModules > 0 {
		pct = detected * 100 / totalModules
	}
	r.Coverage = CoverageInfo{
		DetectedModules: detected,
		TotalModules:    totalModules,
		MissingModules:  missing,
		CoveragePercent: pct,
	}
}

func buildMetadata(r *Report, hostname string, now time.Time) {
	r.Meta = ReportMetadata{
		GeneratedBy: "GetItBack",
		Version:     Version,
		GeneratedAt: now.UTC().Format(time.RFC3339),
		MachineID:   hostname,
		Format:      "v2",
	}
}

var recoveryLevels = map[string]int{
	"ssh": 5, "gpg": 5,
	"postgres": 5, "mysql": 5, "mongodb": 5, "redis": 4, "sqlite": 3,
	"docker": 5, "cloud": 5, "kubernetes": 5,
	"firefox": 4, "chrome": 4, "chromium": 3, "brave": 3, "vivaldi": 2, "edge": 2, "opera": 2,
	"vscode": 4, "neovim": 3,
	"repos": 4, "certs": 4, "dotfiles": 4,
	"shell": 2, "git": 3, "golang": 2, "nodejs": 2, "python": 2, "rust": 2, "java": 2,
	"apt": 1, "snap": 1, "flatpak": 1, "system": 1,
	"virtualization": 2,
}

func recoveryStars(level int) string {
	switch level {
	case 5:
		return "★★★★★ Critical"
	case 4:
		return "★★★★☆ High"
	case 3:
		return "★★★☆☆ Medium"
	case 2:
		return "★★☆☆☆ Low"
	default:
		return "★☆☆☆☆ Minimal"
	}
}

func buildOverview(r *Report, has func(string) bool, isBackedUp func(string) bool, backups []storage.BackupEntry, configHasEncryption bool, score *module.RecoveryScore) {
	protected := 0
	unprotected := 0
	for mod, level := range recoveryLevels {
		if level >= 2 && has(mod) {
			if isBackedUp(mod) {
				protected++
			} else {
				unprotected++
			}
		}
	}

	categoryMaxes := []int{15, 15, 20, 15, 15, 10, 10, 15, 10, 10, 10, 10}
	var maxTotal int
	for _, c := range categoryMaxes {
		maxTotal += c
	}
	pct := 0
	if maxTotal > 0 {
		pct = score.Total * 100 / maxTotal
	}
	grade := "POOR"
	status := "At Risk"
	switch {
	case pct >= 80:
		grade = "GOOD"
		status = "Mostly Recoverable"
	case pct >= 50:
		grade = "FAIR"
		status = "Partially Recoverable"
	case pct >= 30:
		grade = "POOR"
		status = "Significantly At Risk"
	}

	latestBackup := ""
	if len(backups) > 0 {
		latestBackup = backups[0].ID
	}

	var risks []string
	if !configHasEncryption {
		risks = append(risks, "Backup encryption disabled")
	}
	for mod, level := range recoveryLevels {
		if level >= 4 && has(mod) && !isBackedUp(mod) {
			display := modDisplayName(mod)
			if display == "" {
				display = mod
			}
			risks = append(risks, display+" not backed up")
		}
	}
	if len(risks) > 3 {
		risks = risks[:3]
	}

	r.Overview = RecoveryOverview{
		ConfidenceScore:   pct,
		ConfidenceGrade:   grade,
		MachineStatus:     status,
		ProtectedAssets:   protected,
		UnprotectedAssets: unprotected,
		LatestBackup:      latestBackup,
		BackupCount:       len(backups),
		HighestRisks:      risks,
	}
}

func buildGaps(r *Report, results []*module.InventoryResult, isBackedUp func(string) bool, has func(string) bool) {
	var gaps []RecoveryGap

	for mod, level := range recoveryLevels {
		if level < 2 {
			continue
		}
		if !has(mod) {
			continue
		}
		if isBackedUp(mod) {
			continue
		}

		display := modDisplayName(mod)
		if display == "" {
			display = mod
		}

		cat := moduleCategory(mod)
		gaps = append(gaps, RecoveryGap{
			Name:     display,
			Category: cat,
			Issue:    "Not backed up",
		})
	}

	dedup := make(map[string]bool)
	var unique []RecoveryGap
	for _, g := range gaps {
		key := g.Name + ":" + g.Issue
		if dedup[key] {
			continue
		}
		dedup[key] = true
		unique = append(unique, g)
	}

	// Sort by importance: higher recovery level first
	levelForMod := make(map[string]int)
	for mod, level := range recoveryLevels {
		levelForMod[modDisplayName(mod)] = level
	}
	sort.Slice(unique, func(i, j int) bool {
		li := levelForMod[unique[i].Name]
		if li == 0 {
			li = 1
		}
		lj := levelForMod[unique[j].Name]
		if lj == 0 {
			lj = 1
		}
		return li > lj
	})
	r.Gaps = unique
}

func buildVerdict(r *Report) {
	c := r.Overview.ConfidenceScore
	summary := "This workstation is fully recoverable."
	if c < 30 {
		summary = "This workstation has critical recovery risks. Immediate action is required."
	} else if c < 50 {
		summary = "This workstation has significant recovery gaps that need attention."
	} else if c < 80 {
		summary = "This workstation is mostly recoverable."
	}

	target := c + 25
	if target > 95 {
		target = 95
	}

	actions := make([]string, 0, 5)
	for _, gap := range r.Gaps {
		if len(actions) >= 5 {
			break
		}
		actions = append(actions, "Backup "+gap.Name)
	}
	if r.Backups.Encryption == "Disabled" {
		actions = append([]string{"Enable backup encryption"}, actions...)
		if len(actions) > 5 {
			actions = actions[:5]
		}
	}

	r.Verdict = RecoveryVerdict{
		Summary:           summary,
		Confidence:        c,
		TargetConfidence:  target,
		CriticalActions:   actions,
	}
}

func modDisplayName(mod string) string {
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
		"apt": "APT", "snap": "Snap", "flatpak": "Flatpak",
	}
	if n, ok := names[mod]; ok {
		return n
	}
	return ""
}

func moduleCategory(mod string) string {
	cat := map[string]string{
		"ssh": "Identity", "gpg": "Identity",
		"docker": "Containers", "cloud": "Cloud", "kubernetes": "Infrastructure",
		"firefox": "Browsers", "chrome": "Browsers", "chromium": "Browsers",
		"brave": "Browsers", "vivaldi": "Browsers", "edge": "Browsers", "opera": "Browsers",
		"vscode": "Editors", "neovim": "Editors",
		"postgres": "Databases", "mysql": "Databases", "mongodb": "Databases",
		"redis": "Databases", "sqlite": "Databases",
		"golang": "Development", "nodejs": "Development", "python": "Development",
		"rust": "Development", "java": "Development",
		"repos": "Projects", "certs": "Security",
		"shell": "Configuration", "dotfiles": "Configuration", "git": "Configuration",
		"apt": "Packages", "snap": "Packages", "flatpak": "Packages",
	}
	if c, ok := cat[mod]; ok {
		return c
	}
	return "Other"
}

var browserNames = map[string]string{
	"firefox": "Firefox", "chrome": "Chrome", "chromium": "Chromium",
	"brave": "Brave", "vivaldi": "Vivaldi", "edge": "Edge", "opera": "Opera",
}

var langNames = map[string]string{
	"golang": "Go", "nodejs": "Node.js", "python": "Python",
	"rust": "Rust", "java": "Java",
}

var dbNames = map[string]string{
	"postgres": "PostgreSQL", "mysql": "MySQL", "mongodb": "MongoDB",
	"redis": "Redis", "sqlite": "SQLite",
}

func browserDisplayName(mod string) string {
	if n, ok := browserNames[mod]; ok {
		return n
	}
	return mod
}

func langDisplayName(mod string) string {
	if n, ok := langNames[mod]; ok {
		return n
	}
	return mod
}

func dbDisplayName(mod string) string {
	if n, ok := dbNames[mod]; ok {
		return n
	}
	return mod
}

func getMetaIntFromRes(res *module.InventoryResult, key string) int {
	if res.Metadata == nil {
		return 0
	}
	if v, ok := res.Metadata[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
