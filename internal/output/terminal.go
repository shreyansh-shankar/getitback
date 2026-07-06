package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/doctor"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/report"
)

type Format string

const (
	FormatTerminal Format = "terminal"
	FormatJSON     Format = "json"
	FormatYAML     Format = "yaml"
	FormatMarkdown Format = "markdown"
)

func ParseFormat(s string) Format {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON
	case "yaml", "yml":
		return FormatYAML
	case "markdown", "md":
		return FormatMarkdown
	default:
		return FormatTerminal
	}
}

type RenderOptions struct {
	Verbose    bool
	Categories map[string]string
	Coverage   map[string]module.Coverage
	Score      *module.RecoveryScore
	Grade      string
	Recs       []module.Recommendation
}

type Renderer interface {
	RenderInventory(w io.Writer, results []*module.InventoryResult, opts RenderOptions) error
	RenderDoctor(w io.Writer, report *doctor.Report) error
	RenderReport(w io.Writer, report *report.Report) error
}

func NewRenderer(format Format) Renderer {
	switch format {
	case FormatJSON:
		return &JSONRenderer{}
	case FormatYAML:
		return &YAMLRenderer{}
	case FormatMarkdown:
		return &MarkdownRenderer{}
	default:
		return &TerminalRenderer{}
	}
}

type TerminalRenderer struct{}

var sectionOrder = []string{
	"System",
	"Identity",
	"Configuration",
	"Development",
	"Editors",
	"Browsers",
	"Packages",
	"Databases",
	"Containers",
	"Cloud",
	"Infrastructure",
	"Projects",
	"Virtualization",
	"Security",
}

var metaLabels = map[string]string{
	"version":          "Version",
	"profileCount":     "Profiles",
	"installMethod":    "Installation",
	"defaultProfile":   "Default Profile",
	"identityCount":    "SSH Identities",
	"identities":       "Identities",
	"manualPackages":   "Manual Packages",
	"installedPackages": "Installed Packages",
	"heldPackages":     "Held Packages",
	"repositories":     "Repositories",
	"additionalRepos":  "Additional Repos",
	"snapCount":        "Snaps",
	"snaps":            "Installed Snaps",
	"configuration":    "Configuration Files",
	"temporary":        "Temporary Files",
	"secrets":          "Potential Secrets",
	"unknown":          "Unknown Files",
	"globalPackages":   "Global Packages",
	"packageCount":     "Package Count",
	"keyCount":         "Keys",
	"keys":             "Keys",
	"extensions":       "Extensions",
	"languagePacks":    "Language Packs",
	"themes":           "Installed Themes",
	"snippets":         "Snippets",
	"settings":         "Settings",
	"workspaces":       "Workspaces",
	"binaries":         "Installed Binaries",
	"installedTools":   "Tools",
	"channels":         "Channels",
	"signingKey":       "Signing Key",
	"commitGpgSign":    "Commit Signing",
	"credentialHelper": "Credential Helper",
	"defaultBranch":    "Default Branch",
	"gitLFS":           "Git LFS",
	"globalIgnore":     "Global Ignore",
	"hostname":         "Hostname",
	"kernel":           "Kernel",
	"desktop":          "Desktop",
	"session":          "Session",
	"locale":           "Locale",
	"timezone":         "Timezone",
	"codename":         "Codename",
	"goVersion":        "Go Version",
	"homeDir":          "Home Directory",
	"authorizedKeys":   "Authorized Keys",
	"knownHosts":       "Known Hosts",
	"config":           "Config",
	"runtimes":         "Runtimes",
	"remotes":          "Remotes",
	"apps":             "Applications",
	"profiles":         "Profiles",
	"hooks":            "Hooks",
	"basename":         "Shell",
	"frameworks":       "Frameworks",
	"starship":         "Starship",
	"disk":             "Disk",
	"user":             "User",
	"arch":             "Architecture",
	"os":               "OS",
	"GOPATH":           "GOPATH",
	"GOROOT":           "GOROOT",
	"containers":       "Containers",
	"runningContainers": "Running",
	"stoppedContainers": "Stopped",
	"images":           "Images",
	"imageStorage":     "Image Storage",
	"danglingImages":   "Dangling Images",
	"volumes":          "Volumes",
	"volumeStorage":    "Volume Storage",
	"networks":         "Networks",
	"customNetworks":   "Custom Networks",
	"composeProjects":  "Compose Projects",
	"composeProjectNames": "Projects",
	"rootDir":          "Root Directory",
	"rootless":         "Rootless",
	"flavor":           "Flavor",
	"databases":        "Databases",
	"dataDir":          "Data Directory",
	"storage":          "Storage",
	"detectedCLIs":     "CLI Tools",
	"detectedTools":    "Infra Tools",
	"kubeContexts":     "Contexts",
	"currentContext":   "Current Context",
	"namespaces":       "Namespaces",
	"helmRepos":        "Helm Repos",
	"terraformPluginCache": "Plugin Cache",
	"detectedPlatforms": "Platforms",
	"certificateStores": "Certificate Stores",
	"certificateFiles": "Certificate Files",
	"validCerts":       "Valid",
	"expiringCerts":    "Expiring",
	"expiredCerts":     "Expired",
	"customCABundles":  "Custom CA Bundles",
	"totalRepos":       "Total Repositories",
	"dirtyRepos":       "Uncommitted Changes",
	"noRemoteRepos":    "Without Remote",
	"githubRepos":      "GitHub",
	"gitlabRepos":      "GitLab",
	"localOnlyRepos":   "Local Only",
	"remoteProviders":  "Remote Providers",
	"jdk":              "JDK",
	"jre":              "JRE",
	"javaHome":         "Java Home",
	"gradle":           "Gradle",
	"gradleHome":       "Gradle Home",
	"gradleCache":      "Gradle Cache",
	"maven":            "Maven",
	"mavenVersion":     "Maven Version",
	"mavenCache":       "Maven Cache",
	"sbt":              "SBT",
	"bun":              "Bun",
	"pipx":             "pipx",
	"pipxPackages":     "pipx Packages",
	"cargoCache":       "Cargo Cache",
}

var metaPriority = map[string]int{
	"version":    0,
	"os":         0,
	"kernel":     0,
	"desktop":    0,
	"session":    0,
	"arch":       0,
	"locale":     0,
	"timezone":   0,
	"installMethod": 1,
	"shell":      1,
	"basename":   1,
	"installation": 1,
}

func displayLabel(key string) string {
	if label, ok := metaLabels[key]; ok {
		return label
	}
	return key
}

func metaSortKey(key string) (int, string) {
	priority := 10
	if p, ok := metaPriority[key]; ok {
		priority = p
	}
	return priority, displayLabel(key)
}

func computeTotalStorage(res *module.InventoryResult) int64 {
	var total int64
	for _, r := range res.Resources {
		total += r.Size
	}
	return total
}

func (r *TerminalRenderer) RenderInventory(w io.Writer, results []*module.InventoryResult, opts RenderOptions) error {
	grouped := make(map[string][]*module.InventoryResult)
	var modulesHealthy, modulesWarning, modulesError, modulesSkipped int
	var allSecrets int

	for _, res := range results {
		if !res.Detected {
			modulesSkipped++
			continue
		}
		g := opts.Categories[res.Module]
		if g == "" {
			g = "Other"
		}
		grouped[g] = append(grouped[g], res)

		if len(res.Warnings) > 0 {
			modulesWarning++
		} else {
			modulesHealthy++
		}
		if len(res.Errors) > 0 {
			modulesError++
		}
		for _, rsrc := range res.Resources {
			if rsrc.Type == module.ResourceTypeSecret {
				allSecrets++
			}
		}
	}

	fmt.Fprint(w, SectionHeader("Inventory"))

	detectedCount := 0
	for _, section := range sectionOrder {
		mods := grouped[section]
		if len(mods) == 0 {
			continue
		}
		fmt.Fprintf(w, "  %s%s%s\n", ColorBold+ColorBlue, section, ColorReset)
		for _, res := range mods {
			detectedCount++
			r.renderModule(w, res, opts)
		}
		fmt.Fprintln(w)
	}
	// Render any uncategorized modules
	for g, mods := range grouped {
		isKnown := false
		for _, s := range sectionOrder {
			if s == g {
				isKnown = true
				break
			}
		}
		if isKnown {
			continue
		}
		fmt.Fprintf(w, "  %s%s%s\n", ColorBold+ColorBlue, g, ColorReset)
		for _, res := range mods {
			detectedCount++
			r.renderModule(w, res, opts)
		}
		fmt.Fprintln(w)
	}

	totalModules := modulesHealthy + modulesWarning + modulesError + modulesSkipped

	// Module Health Summary
	fmt.Fprintf(w, "  %sModule Health%s\n", ColorBold+ColorBlue, ColorReset)
	if modulesHealthy == totalModules && modulesHealthy > 0 {
		fmt.Fprintf(w, "    %s✓%s All systems nominal — %d/%d modules healthy\n",
			ColorGreen, ColorReset, modulesHealthy, totalModules)
	} else {
		fmt.Fprintf(w, "    %s✓%sHealthy:  %d\n", ColorGreen, ColorReset, modulesHealthy)
		if modulesWarning > 0 {
			fmt.Fprintf(w, "    %s⚠%sWarnings: %d\n", ColorYellow, ColorReset, modulesWarning)
		}
		if modulesError > 0 {
			fmt.Fprintf(w, "    %s✗%sErrors:   %d\n", ColorRed, ColorReset, modulesError)
		}
		if modulesSkipped > 0 {
			fmt.Fprintf(w, "    %s-%sSkipped:  %d\n", ColorCyan, ColorReset, modulesSkipped)
		}
		if totalModules > 0 {
			pct := modulesHealthy * 100 / totalModules
			fmt.Fprintf(w, "    %sℹ%sCoverage: %s%d%%%s\n",
				ColorCyan, ColorReset, colorForScore(pct), pct, ColorReset)
		}
	}

	// Secrets Summary
	if allSecrets > 0 {
		fmt.Fprintf(w, "\n  %sSecrets%s\n    %s⚠%s %d potential secret files detected (use 'getitback secrets' to inspect)\n",
			ColorBold+ColorRed, ColorReset, ColorYellow, ColorReset, allSecrets)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s%d%s modules detected\n", ColorGreen, detectedCount, ColorReset)

	if opts.Score != nil {
		fmt.Fprintln(w)
		fmt.Fprint(w, SectionHeader("Recovery Readiness"))

		categoryMaxes := []int{15, 15, 20, 15, 15, 10, 10, 15, 10, 10, 10, 10}
		var maxTotal int
		for _, c := range categoryMaxes {
			maxTotal += c
		}
		pct := 0
		if maxTotal > 0 {
			pct = opts.Score.Total * 100 / maxTotal
		}

		bar := scoreBar(pct)
		grade := "F"
		if opts.Grade != "" {
			grade = opts.Grade
		}
		fmt.Fprintf(w, "  %sScore:%s  %s[%s%d%%%s]%s\n",
			ColorBold, gradeColor(grade),
			ColorCyan, colorForScore(pct),
			pct, ColorCyan, ColorReset)
		fmt.Fprintln(w, "  "+bar)

		fmt.Fprintln(w)
		categories := []struct {
			name     string
			score    int
			max      int
			explain  string
		}{
			{"Identity", opts.Score.Identity, 15, ""},
			{"Configuration", opts.Score.Configuration, 15, ""},
			{"Development", opts.Score.Development, 20, ""},
			{"Editors", opts.Score.Editors, 15, ""},
			{"Browsers", opts.Score.Browsers, 15, ""},
			{"Packages", opts.Score.Packages, 10, ""},
			{"Databases", opts.Score.Databases, 10, ""},
			{"Containers", opts.Score.Containers, 15, ""},
			{"Cloud", opts.Score.Cloud, 10, ""},
			{"Infrastructure", opts.Score.Infrastructure, 10, ""},
			{"Projects", opts.Score.Projects, 10, ""},
			{"Virtualization", opts.Score.Virtualization, 10, ""},
		}

		// Generate explanations based on scores
		for i := range categories {
			pct := 0
			if categories[i].max > 0 {
				pct = categories[i].score * 100 / categories[i].max
			}
			if pct >= 100 {
				categories[i].explain = "Complete"
			} else if pct >= 50 {
				categories[i].explain = "Mostly configured"
			} else if categories[i].score == 0 {
				switch categories[i].name {
				case "Identity":
					categories[i].explain = "No SSH keys or GPG keys detected"
				case "Configuration":
					categories[i].explain = "Missing shell, dotfiles, or git config"
				case "Development":
					categories[i].explain = "No development tools detected"
				case "Editors":
					categories[i].explain = "No editors detected"
				case "Browsers":
					categories[i].explain = "No browsers detected"
				case "Packages":
					categories[i].explain = "No package managers detected"
				case "Databases":
					categories[i].explain = "No supported databases detected"
				}
			} else {
				categories[i].explain = fmt.Sprintf("%d/%d points", categories[i].score, categories[i].max)
			}
		}

		for _, cat := range categories {
			pct := 0
			if cat.max > 0 {
				pct = cat.score * 100 / cat.max
			}
			pctColor := colorForScore(pct)
			bar := miniBarColor(cat.score, cat.max, 10, pctColor)
			explainColor := ColorCyan
			if cat.explain == "Complete" {
				explainColor = ColorGreen
			}
			fmt.Fprintf(w, "    %-16s%s  %s%d%%%s  %s(%s)%s\n",
				cat.name+":", bar,
				pctColor, pct, ColorReset,
				explainColor, cat.explain, ColorReset)
		}

		if !opts.Verbose && len(opts.Recs) > 0 {
			maxRecs := 5
			if len(opts.Recs) < maxRecs {
				maxRecs = len(opts.Recs)
			}
			fmt.Fprintln(w)
			fmt.Fprintf(w, "  %sRecommendations%s\n", ColorBold+ColorBlue, ColorReset)
			for i := 0; i < maxRecs; i++ {
				r := opts.Recs[i]
				pColor := priorityColor(r.Priority)
				fmt.Fprintf(w, "    %s•%s [%s%s%s] %s\n",
					ColorCyan, ColorReset,
					pColor, r.Priority, ColorReset, r.Message)
				if r.Help != "" {
					fmt.Fprintf(w, "      %sℹ%s %s\n", ColorCyan, ColorReset, r.Help)
				}
			}
			fmt.Fprintf(w, "    %sUse 'getitback doctor' for full analysis%s\n",
				ColorCyan, ColorReset)
		}
	}

	return nil
}

func (r *TerminalRenderer) renderModule(w io.Writer, res *module.InventoryResult, opts RenderOptions) {
	version := ""
	if res.Version != "" {
		version = " " + ColorCyan + res.Version + ColorReset
	}
	fmt.Fprintf(w, "    %s✓%s %s%s%s\n",
		ColorGreen, ColorReset, ColorBold, res.Module, ColorReset+version)

	// Compute storage total
	storageTotal := computeTotalStorage(res)

	// Collect metadata keys and sort by priority then display label
	type metaEntry struct {
		key   string
		label string
		value any
	}
	var entries []metaEntry
	if res.Metadata != nil {
		for k, v := range res.Metadata {
			entries = append(entries, metaEntry{k, displayLabel(k), v})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		pi, pj := 10, 10
		if p, ok := metaPriority[entries[i].key]; ok {
			pi = p
		}
		if p, ok := metaPriority[entries[j].key]; ok {
			pj = p
		}
		if pi != pj {
			return pi < pj
		}
		return entries[i].label < entries[j].label
	})

	// Show storage if significant
	showStorage := storageTotal > 0 && res.Module != "system"

	// Track whether we're showing resources in metadata or separately
	resourceCount := 0
	
	// For browsers, handle profile paths separately
	isBrowser := false
	for _, b := range []string{"firefox", "chromium", "chrome", "brave", "vivaldi", "edge", "opera"} {
		if res.Module == b {
			isBrowser = true
			break
		}
	}

	// Count profile directories and non-profile resources
	var profileResources []module.Resource
	var otherResources []module.Resource
	for _, rsrc := range res.Resources {
		if rsrc.Type == module.ResourceTypeTemp {
			continue
		}
		if !opts.Verbose && rsrc.Type == module.ResourceTypeSecret {
			continue
		}
		if isBrowser {
			// Check if this is a profile directory (starts with a hash pattern or "Default"/"Profile N")
			if rsrc.Type == module.ResourceTypeData {
				profileResources = append(profileResources, rsrc)
				continue
			}
		}
		otherResources = append(otherResources, rsrc)
	}

	// Show non-profile resources
	for _, rsrc := range otherResources {
		resourceCount++
		meta := rsrc.Path
		if rsrc.Size > 0 {
			meta += fmt.Sprintf("  (%s)", formatSize(rsrc.Size))
		}
		fmt.Fprintf(w, "      %s•%s %s\n", ColorCyan, ColorReset, meta)
	}

	if isBrowser {
		if opts.Verbose {
			for _, rsrc := range profileResources {
				meta := rsrc.Path
				if rsrc.Size > 0 {
					meta += fmt.Sprintf("  (%s)", formatSize(rsrc.Size))
				}
				fmt.Fprintf(w, "      %s•%s %s\n", ColorCyan, ColorReset, meta)
			}
		}
	}

	if len(entries) > 0 {
		for i, e := range entries {
			branch := "├"
			if i == len(entries)-1 && !showStorage && resourceCount == 0 && len(res.Warnings) == 0 && len(res.Errors) == 0 {
				branch = "└"
			}
			display := formatMetaValue(e.value)
			if !opts.Verbose && len(display) > 80 {
				display = display[:77] + "..."
			}

			// Format value with storage suffix for profile-related entries
			if e.key == "profileCount" && showStorage {
				display = fmt.Sprintf("%d  (%s)", e.value, formatSize(storageTotal))
			} else if e.key == "extensions" && showStorage {
				display = fmt.Sprintf("%s  (%s)", display, formatSize(storageTotal))
			}

			fmt.Fprintf(w, "      %s%s %s%s: %s\n",
				ColorCyan, branch, ColorReset,
				ColorBlue+e.label+ColorReset, display)
		}
	}

	// Show storage for non-browser modules with significant data
	if showStorage && !isBrowser && len(entries) > 0 && resourceCount == 0 {
		branch := "├"
		if len(res.Warnings) == 0 && len(res.Errors) == 0 {
			branch = "└"
		}
		fmt.Fprintf(w, "      %s%s %s%s: %s\n",
			ColorCyan, branch, ColorReset,
			ColorBlue+"Storage"+ColorReset, formatSize(storageTotal))
	}

	// Warnings
	for i, warn := range res.Warnings {
		branch := "├"
		if i == len(res.Warnings)-1 && len(res.Errors) == 0 {
			branch = "└"
		}
		fmt.Fprintf(w, "      %s%s %s⚠ %s%s\n",
			ColorYellow, branch, ColorReset, warn, ColorReset)
	}

	// Errors
	for i, err := range res.Errors {
		branch := "├"
		if i == len(res.Errors)-1 {
			branch = "└"
		}
		fmt.Fprintf(w, "      %s%s %s✗ %s%s\n",
			ColorRed, branch, ColorReset, err, ColorReset)
	}
}

func scoreBar(score int) string {
	const width = 30
	filled := score * width / 100
	c := colorForScore(score)
	bar := strings.Builder{}
	bar.Grow(width)
	for i := 0; i < width; i++ {
		if i < filled {
			bar.WriteString("█")
		} else {
			bar.WriteString("░")
		}
	}
	return ColorCyan + bar.String() + ColorReset + "  " + c + fmt.Sprintf("%d%%", score) + ColorReset
}

func miniBar(score, max, width int) string {
	return miniBarColor(score, max, width, "")
}

func miniBarColor(score, max, width int, c string) string {
	filled := 0
	if max > 0 {
		filled = score * width / max
	}
	bar := strings.Builder{}
	bar.Grow(width)
	for i := 0; i < width; i++ {
		if i < filled {
			bar.WriteString("█")
		} else {
			bar.WriteString("░")
		}
	}
	if c == "" {
		c = colorForScore(score * 100 / max)
	}
	return c + bar.String() + ColorReset
}

func gradeColor(grade string) string {
	switch grade {
	case "A":
		return ColorGreen
	case "B":
		return ColorGreen
	case "C":
		return ColorYellow
	case "D":
		return ColorYellow
	default:
		return ColorRed
	}
}

func colorForScore(score int) string {
	switch {
	case score >= 70:
		return ColorGreen
	case score >= 40:
		return ColorYellow
	default:
		return ColorRed
	}
}

func priorityColor(p module.RecommendationPriority) string {
	switch p {
	case module.RecPriorityCritical:
		return ColorRed
	case module.RecPriorityHigh:
		return ColorYellow
	case module.RecPriorityMedium:
		return ColorGreen
	default:
		return ColorCyan
	}
}

func formatMetaValue(v any) string {
	switch val := v.(type) {
	case []string:
		if len(val) > 10 {
			return fmt.Sprintf("%d items", len(val))
		}
		return strings.Join(val, ", ")
	case []any:
		if len(val) > 10 {
			return fmt.Sprintf("%d items", len(val))
		}
		parts := make([]string, len(val))
		for i, item := range val {
			if m, ok := item.(map[string]any); ok {
				if name, has := m["filename"]; has {
					parts[i] = fmt.Sprintf("%v", name)
				} else {
					parts[i] = fmt.Sprintf("%v", item)
				}
			} else {
				parts[i] = fmt.Sprint(item)
			}
		}
		return strings.Join(parts, ", ")
	case map[string]string:
		if len(val) <= 5 {
			parts := make([]string, 0, len(val))
			for k, v := range val {
				parts = append(parts, k+":"+v)
			}
			return strings.Join(parts, ", ")
		}
		return fmt.Sprintf("%d entries", len(val))
	case map[string]int:
		if len(val) <= 5 {
			parts := make([]string, 0, len(val))
			for k, v := range val {
				parts = append(parts, fmt.Sprintf("%s:%d", k, v))
			}
			return strings.Join(parts, ", ")
		}
		return fmt.Sprintf("%d channels", len(val))
	case map[string]any:
		if len(val) <= 5 {
			parts := make([]string, 0, len(val))
			for k, v := range val {
				parts = append(parts, fmt.Sprintf("%s:%v", k, v))
			}
			return strings.Join(parts, ", ")
		}
		return fmt.Sprintf("%d entries", len(val))
	case bool:
		if val {
			return "yes"
		}
		return "no"
	default:
		return fmt.Sprint(v)
	}
}

func formatSize(bytes int64) string {
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

func (r *TerminalRenderer) RenderDoctor(w io.Writer, report *doctor.Report) error {
	fmt.Fprintln(w)
	fmt.Fprint(w, SectionHeader("Recovery Assessment"))

	renderConfidence(w, report.Confidence)
	fmt.Fprintln(w)
	renderDisasterPreview(w, report.DisasterPreview)
	fmt.Fprintln(w)
	renderRisks(w, report.Risks)
	fmt.Fprintln(w)
	renderCoverage(w, report.Coverage)
	fmt.Fprintln(w)
	renderTimeline(w, report.Timeline)
	fmt.Fprintln(w)
	renderActionPlan(w, report.ActionPlan, report.Confidence)
	fmt.Fprintln(w)
	renderReadiness(w, report.Readiness)
	fmt.Fprintln(w)
	renderMachineStatus(w, report.Machine)
	fmt.Fprintln(w)
	renderDoctorSummary(w, report.Summary)

	return nil
}

func (r *TerminalRenderer) RenderReport(w io.Writer, rep *report.Report) error {
	renderReportHeader(w, rep.Header)
	fmt.Fprintln(w)

	renderRecoveryOverview(w, rep.Overview)
	fmt.Fprintln(w)

	renderReportSummary(w, rep.Summary)
	fmt.Fprintln(w)

	renderMachineProfile(w, rep.Machine)
	fmt.Fprintln(w)

	renderDevStack(w, rep.Development)
	fmt.Fprintln(w)

	renderIdentitySection(w, rep.Identity)
	fmt.Fprintln(w)

	renderReportBrowsers(w, rep.Browsers)
	fmt.Fprintln(w)

	renderReportEditors(w, rep.Editors)
	fmt.Fprintln(w)

	renderReportDatabases(w, rep.Databases)
	fmt.Fprintln(w)

	if rep.Containers != nil {
		renderContainerInfo(w, rep.Containers)
		fmt.Fprintln(w)
	}

	if rep.Cloud != nil {
		renderCloudInfo(w, rep.Cloud)
		fmt.Fprintln(w)
	}

	if rep.Infra != nil {
		renderInfraInfo(w, rep.Infra)
		fmt.Fprintln(w)
	}

	renderPackageInfo(w, rep.Packages)
	fmt.Fprintln(w)

	if rep.Security != nil {
		renderSecurityInfo(w, rep.Security)
		fmt.Fprintln(w)
	}

	if rep.Projects != nil {
		renderProjectsInfo(w, rep.Projects)
		fmt.Fprintln(w)
	}

	if rep.Virtualization != nil {
		renderVirtInfo(w, rep.Virtualization)
		fmt.Fprintln(w)
	}

	if len(rep.Gaps) > 0 {
		renderRecoveryGaps(w, rep.Gaps)
		fmt.Fprintln(w)
	}

	renderBackupSummary(w, rep.Backups)
	fmt.Fprintln(w)

	renderAssetStats(w, rep.Statistics)
	fmt.Fprintln(w)

	if len(rep.NotDetected) > 0 {
		renderNotDetected(w, rep.NotDetected)
		fmt.Fprintln(w)
	}

	renderCoverageInfo(w, rep.Coverage)
	fmt.Fprintln(w)

	renderRecoveryVerdict(w, rep.Verdict)
	fmt.Fprintln(w)

	renderReportMetadata(w, rep.Meta)

	return nil
}

func sectionHeading(w io.Writer, icon, title string) {
	fmt.Fprintf(w, "\n  %s%s %s%s\n", ColorBold+ColorCyan, icon, title, ColorReset)
	fmt.Fprintf(w, "  %s%s%s\n", ColorBold+ColorCyan, strings.Repeat("━", 50), ColorReset)
}

func dotLeader(label, value string, pad int) string {
	dots := pad - len(label)
	if dots < 1 {
		dots = 1
	}
	return label + " " + strings.Repeat(".", dots) + " " + value
}

func renderReportHeader(w io.Writer, h report.ReportHeader) {
	fmt.Fprintf(w, "\n  %s%s%s\n", ColorBold+ColorCyan, "Machine Audit Report", ColorReset)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %sHostname%s\n", ColorBold, ColorReset)
	fmt.Fprintf(w, "    %s\n", h.Hostname)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %sGenerated%s\n", ColorBold, ColorReset)
	fmt.Fprintf(w, "    %s\n", h.GeneratedAt)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %sOperating System%s\n", ColorBold, ColorReset)
	fmt.Fprintf(w, "    %s\n", h.OperatingSystem)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %sGetItBack Version%s\n", ColorBold, ColorReset)
	fmt.Fprintf(w, "    %s\n", h.GetItBackVersion)
	if h.RecoverySnapshot != "" {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "    %sRecovery Snapshot%s\n", ColorBold, ColorReset)
		fmt.Fprintf(w, "    %s\n", h.RecoverySnapshot)
	}
}

func renderRecoveryOverview(w io.Writer, o report.RecoveryOverview) {
	sectionHeading(w, "🔍", "Recovery Overview")

	fmt.Fprintf(w, "\n    %sRecovery Confidence%s\n", ColorBold, ColorReset)
	gradeColor := colorForScore(o.ConfidenceScore)
	fmt.Fprintf(w, "    %s%d%% (%s)%s\n", gradeColor, o.ConfidenceScore, o.ConfidenceGrade, ColorReset)

	statusColor := ColorGreen
	if o.MachineStatus == "Significantly At Risk" {
		statusColor = ColorRed
	} else if o.MachineStatus == "Partially Recoverable" {
		statusColor = ColorYellow
	}
	fmt.Fprintf(w, "\n    %sRecovery Status%s\n", ColorBold, ColorReset)
	fmt.Fprintf(w, "    %s%s%s\n", statusColor, o.MachineStatus, ColorReset)

	fmt.Fprintf(w, "\n    %sProtected Assets%s     %d\n", ColorBold, ColorReset, o.ProtectedAssets)
	fmt.Fprintf(w, "    %sUnprotected Assets%s   %d\n", ColorBold, ColorReset, o.UnprotectedAssets)

	if o.LatestBackup != "" {
		fmt.Fprintf(w, "\n    %sLatest Backup%s\n", ColorBold, ColorReset)
		fmt.Fprintf(w, "    %s (%d snapshots)\n", o.LatestBackup, o.BackupCount)
	}

	if len(o.HighestRisks) > 0 {
		fmt.Fprintf(w, "\n    %sHighest Risks%s\n", ColorBold, ColorReset)
		for _, risk := range o.HighestRisks {
			fmt.Fprintf(w, "    %s•%s %s%s%s\n", ColorRed, ColorReset, ColorYellow, risk, ColorReset)
		}
	}
}

func renderReportSummary(w io.Writer, s report.ExecutiveSummary) {
	sectionHeading(w, "📋", "Summary")

	pad := 20
	if s.OperatingSystem != "" {
		fmt.Fprintf(w, "\n    %s\n", ColorCyan+dotLeader("Operating System", s.OperatingSystem, pad)+ColorReset)
	}
	if s.PrimaryShell != "" {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Primary Shell", s.PrimaryShell, pad)+ColorReset)
	}
	if s.PrimaryEditor != "" {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Primary Editor", s.PrimaryEditor, pad)+ColorReset)
	}
	if s.PrimaryBrowser != "" {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Primary Browser", s.PrimaryBrowser, pad)+ColorReset)
	}
	if len(s.Languages) > 0 {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Languages", commaList(s.Languages), pad)+ColorReset)
	}
	if len(s.Databases) > 0 {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Databases", commaList(s.Databases), pad)+ColorReset)
	}
	if s.Containers > 0 {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Containers", fmt.Sprintf("%d", s.Containers), pad)+ColorReset)
	}
	if s.DockerVolumes > 0 {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Docker Volumes", fmt.Sprintf("%d", s.DockerVolumes), pad)+ColorReset)
	}
	if s.Repositories > 0 {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Repositories", fmt.Sprintf("%d", s.Repositories), pad)+ColorReset)
	}
	if len(s.CloudProviders) > 0 {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Cloud Providers", commaList(s.CloudProviders), pad)+ColorReset)
	}
	if s.BackupSnapshots > 0 {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Backup Snapshots", fmt.Sprintf("%d", s.BackupSnapshots), pad)+ColorReset)
	}
}

func renderMachineProfile(w io.Writer, m report.MachineProfile) {
	sectionHeading(w, "🖥", "Machine Profile")

	pad := 16
	if m.Architecture != "" {
		fmt.Fprintf(w, "\n    %s\n", ColorCyan+dotLeader("Architecture", m.Architecture, pad)+ColorReset)
	}
	if m.CPU != "" {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("CPU", m.CPU, pad)+ColorReset)
	}
	if m.RAM != "" {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("RAM", m.RAM, pad)+ColorReset)
	}
	if m.Storage != "" {
		storage := m.Storage
		if m.StorageUsed != "" && m.StorageAvail != "" {
			storage = fmt.Sprintf("%s (%s used, %s free)", m.Storage, m.StorageUsed, m.StorageAvail)
		}
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Storage", storage, pad)+ColorReset)
	}
	if m.Kernel != "" {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Kernel", m.Kernel, pad)+ColorReset)
	}
	if m.Desktop != "" {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Desktop", m.Desktop, pad)+ColorReset)
	}
	if m.Session != "" {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Session", m.Session, pad)+ColorReset)
	}
	if m.Timezone != "" {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Timezone", m.Timezone, pad)+ColorReset)
	}
	if m.Locale != "" {
		fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader("Locale", m.Locale, pad)+ColorReset)
	}
}

func recoveryBadge(level int) string {
	switch level {
	case 5:
		return ColorRed + "Critical" + ColorReset
	case 4:
		return ColorYellow + "High" + ColorReset
	case 3:
		return ColorGreen + "Medium" + ColorReset
	case 2:
		return ColorCyan + "Low" + ColorReset
	default:
		return ColorBlue + "Minimal" + ColorReset
	}
}

func renderDevStack(w io.Writer, d report.DevelopmentStack) {
	sectionHeading(w, "🛠", "Development Stack")

	if len(d.Languages) > 0 {
		fmt.Fprintf(w, "\n    %sLanguages%s\n", ColorBold, ColorReset)
		for _, l := range d.Languages {
			v := ""
			if l.Version != "" {
				v = "   " + l.Version
			}
			fmt.Fprintf(w, "    %s•%s %s%s\n", ColorCyan, ColorReset, l.Name, v)
		}
	}

	fmt.Fprintf(w, "\n    %sVersion Control%s\n", ColorBold, ColorReset)
	for _, vc := range d.VersionControl {
		vci := ColorGreen + "✓" + ColorReset
		if !vc.Configured {
			vci = ColorCyan + "∘" + ColorReset
		}
		v := ""
		if vc.Version != "" {
			v = "   " + vc.Version
		}
		fmt.Fprintf(w, "    %s•%s %s %s%s\n", ColorCyan, ColorReset, vci, vc.Name, v)
	}

	if len(d.PackageManagers) > 0 {
		fmt.Fprintf(w, "\n    %sPackage Managers%s\n", ColorBold, ColorReset)
		for _, p := range d.PackageManagers {
			detail := ""
			if p.Count > 0 {
				detail = fmt.Sprintf("   %d packages", p.Count)
			}
			fmt.Fprintf(w, "    %s•%s %s%s\n", ColorCyan, ColorReset, p.Name, detail)
		}
	}
}

func renderIdentitySection(w io.Writer, id report.IdentitySection) {
	if id.SSH == nil && id.GPG == nil {
		return
	}
	sectionHeading(w, "🔐", "Identity")

	if id.SSH != nil {
		fmt.Fprintf(w, "\n    %sSSH%s\n", ColorBold, ColorReset)
		backupIcon := ColorGreen + "✓ Backed up" + ColorReset
		if !id.SSH.BackedUp {
			backupIcon = ColorYellow + "✗ Not backed up" + ColorReset
		}
		fmt.Fprintf(w, "      %sStatus:%s  %s\n", ColorCyan, ColorReset, ColorGreen+"✓ Detected"+ColorReset)
		fmt.Fprintf(w, "      %sBackup:%s   %s\n", ColorCyan, ColorReset, backupIcon)
		if id.SSH.Version != "" {
			fmt.Fprintf(w, "      %sVersion:%s   %s\n", ColorCyan, ColorReset, id.SSH.Version)
		}
		fmt.Fprintf(w, "      %sIdentities:%s %d\n", ColorCyan, ColorReset, id.SSH.IdentityCount)
	}

	if id.GPG != nil {
		if id.SSH != nil {
			fmt.Fprintln(w)
		} else {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "    %sGPG%s\n", ColorBold, ColorReset)
		backupIcon := ColorGreen + "✓ Backed up" + ColorReset
		if !id.GPG.BackedUp {
			backupIcon = ColorYellow + "✗ Not backed up" + ColorReset
		}
		fmt.Fprintf(w, "      %sStatus:%s  %s\n", ColorCyan, ColorReset, ColorGreen+"✓ Detected"+ColorReset)
		fmt.Fprintf(w, "      %sBackup:%s   %s\n", ColorCyan, ColorReset, backupIcon)
		if id.GPG.Version != "" {
			fmt.Fprintf(w, "      %sVersion:%s   %s\n", ColorCyan, ColorReset, id.GPG.Version)
		}
		fmt.Fprintf(w, "      %sKeys:%s      %d\n", ColorCyan, ColorReset, id.GPG.KeyCount)
	}
}

func renderReportBrowsers(w io.Writer, browsers []report.BrowserInfo) {
	if len(browsers) == 0 {
		return
	}
	sectionHeading(w, "🌐", "Browsers")

	for _, b := range browsers {
		fmt.Fprintf(w, "\n    %s%s%s\n", ColorBold, b.Name, ColorReset)
		fmt.Fprintf(w, "      %sRecovery Value:%s %s\n", ColorCyan, ColorReset, recoveryBadge(b.RecoveryLevel))
		if b.Version != "" {
			fmt.Fprintf(w, "      %sVersion:%s  %s\n", ColorCyan, ColorReset, b.Version)
		}
		backupIcon := ColorGreen + "✓ Backed up" + ColorReset
		if !b.BackedUp {
			backupIcon = ColorYellow + "✗ Not backed up" + ColorReset
		}
		fmt.Fprintf(w, "      %sBackup:%s   %s\n", ColorCyan, ColorReset, backupIcon)
		if b.ProfileCount > 0 {
			fmt.Fprintf(w, "      %sProfiles:%s %d\n", ColorCyan, ColorReset, b.ProfileCount)
			if b.DefaultProfile != "" {
				fmt.Fprintf(w, "      %sDefault:%s  %s\n", ColorCyan, ColorReset, b.DefaultProfile)
			}
		} else {
			fmt.Fprintf(w, "      %sNo profiles created.%s\n", ColorCyan, ColorReset)
		}
		if b.Storage != "" {
			fmt.Fprintf(w, "      %sStorage:%s %s\n", ColorCyan, ColorReset, b.Storage)
		}
		if b.InstallMethod != "" {
			fmt.Fprintf(w, "      %sInstalled Via:%s %s\n", ColorCyan, ColorReset, b.InstallMethod)
		}
	}
}

func renderReportEditors(w io.Writer, editors []report.EditorInfo) {
	if len(editors) == 0 {
		return
	}
	sectionHeading(w, "✏️", "Editors")

	for _, e := range editors {
		fmt.Fprintf(w, "\n    %s%s%s\n", ColorBold, e.Name, ColorReset)
		fmt.Fprintf(w, "      %sRecovery Value:%s %s\n", ColorCyan, ColorReset, recoveryBadge(e.RecoveryLevel))
		if e.Version != "" {
			fmt.Fprintf(w, "      %sVersion:%s    %s\n", ColorCyan, ColorReset, e.Version)
		}
		backupIcon := ColorGreen + "✓ Backed up" + ColorReset
		if !e.BackedUp {
			backupIcon = ColorYellow + "✗ Not backed up" + ColorReset
		}
		fmt.Fprintf(w, "      %sBackup:%s     %s\n", ColorCyan, ColorReset, backupIcon)
		if e.Extensions > 0 {
			fmt.Fprintf(w, "      %sExtensions:%s %d\n", ColorCyan, ColorReset, e.Extensions)
		}
		if e.Settings > 0 {
			fmt.Fprintf(w, "      %sSettings:%s   %d\n", ColorCyan, ColorReset, e.Settings)
		}
		if e.Themes > 0 {
			fmt.Fprintf(w, "      %sThemes:%s     %d\n", ColorCyan, ColorReset, e.Themes)
		}
		if e.Snippets > 0 {
			fmt.Fprintf(w, "      %sSnippets:%s   %d\n", ColorCyan, ColorReset, e.Snippets)
		}
	}
}

func renderReportDatabases(w io.Writer, databases []report.DatabaseInfo) {
	if len(databases) == 0 {
		return
	}
	sectionHeading(w, "🗄", "Databases")

	for _, d := range databases {
		fmt.Fprintf(w, "\n    %s%s%s\n", ColorBold, d.Name, ColorReset)
		fmt.Fprintf(w, "      %sRecovery Value:%s %s\n", ColorCyan, ColorReset, recoveryBadge(d.RecoveryLevel))
		backupIcon := ColorGreen + "✓ Backed up" + ColorReset
		if !d.BackedUp {
			backupIcon = ColorYellow + "✗ Not backed up" + ColorReset
		}
		fmt.Fprintf(w, "      %sBackup:%s   %s\n", ColorCyan, ColorReset, backupIcon)
		if d.Version != "" {
			fmt.Fprintf(w, "      %sVersion:%s  %s\n", ColorCyan, ColorReset, d.Version)
		}
		if d.ConfigFile != "" {
			fmt.Fprintf(w, "      %sConfig:%s   %s\n", ColorCyan, ColorReset, d.ConfigFile)
		}
		if d.DataDir != "" {
			fmt.Fprintf(w, "      %sData Dir:%s %s\n", ColorCyan, ColorReset, d.DataDir)
		}
		if d.Storage != "" {
			fmt.Fprintf(w, "      %sStorage:%s  %s\n", ColorCyan, ColorReset, d.Storage)
		}
	}
}

func renderContainerInfo(w io.Writer, c *report.ContainerInfo) {
	sectionHeading(w, "🐳", "Docker")

	fmt.Fprintf(w, "\n    %sRecovery Value:%s %s\n", ColorCyan, ColorReset, recoveryBadge(c.RecoveryLevel))
	backupIcon := ColorGreen + "✓ Backed up" + ColorReset
	if !c.BackedUp {
		backupIcon = ColorYellow + "✗ Not backed up" + ColorReset
	}
	fmt.Fprintf(w, "    %sBackup:%s %s\n", ColorCyan, ColorReset, backupIcon)
	if c.Version != "" {
		fmt.Fprintf(w, "    %sVersion:%s %s\n", ColorCyan, ColorReset, c.Version)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %s%s%s\n", ColorBold, "Containers", ColorReset)
	fmt.Fprintf(w, "      %sTotal:%s   %d\n", ColorCyan, ColorReset, c.Containers)
	fmt.Fprintf(w, "      %sRunning:%s  %d\n", ColorCyan, ColorReset, c.Running)
	fmt.Fprintf(w, "      %sStopped:%s  %d\n", ColorCyan, ColorReset, c.Stopped)

	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %s%s%s\n", ColorBold, "Images", ColorReset)
	fmt.Fprintf(w, "      %sTotal:%s   %d\n", ColorCyan, ColorReset, c.Images)
	if c.DanglingImages > 0 {
		fmt.Fprintf(w, "      %sDangling:%s %d\n", ColorCyan, ColorReset, c.DanglingImages)
	}
	if c.ImageStorage != "" {
		fmt.Fprintf(w, "    %sImage Storage:%s %s\n", ColorCyan, ColorReset, c.ImageStorage)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %s%s%s\n", ColorBold, "Storage", ColorReset)
	if c.Volumes > 0 {
		fmt.Fprintf(w, "      %sVolumes:%s  %d\n", ColorCyan, ColorReset, c.Volumes)
	}
	if c.VolumeStorage != "" {
		fmt.Fprintf(w, "      %sStorage:%s  %s\n", ColorCyan, ColorReset, c.VolumeStorage)
	}
	if c.BuildCache != "" {
		fmt.Fprintf(w, "      %sBuild Cache:%s %s\n", ColorCyan, ColorReset, c.BuildCache)
	}
	if c.RootDir != "" {
		fmt.Fprintf(w, "      %sRoot Dir:%s  %s\n", ColorCyan, ColorReset, c.RootDir)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %s%s%s\n", ColorBold, "Networks", ColorReset)
	fmt.Fprintf(w, "      %sTotal:%s   %d\n", ColorCyan, ColorReset, c.Networks)
	if c.CustomNetworks > 0 {
		fmt.Fprintf(w, "      %sCustom:%s  %d\n", ColorCyan, ColorReset, c.CustomNetworks)
	}
	if c.ComposeProjects > 0 {
		fmt.Fprintf(w, "    %sCompose Projects:%s %d\n", ColorCyan, ColorReset, c.ComposeProjects)
	}
	fmt.Fprintf(w, "    %sRootless:%s %v\n", ColorCyan, ColorReset, c.Rootless)
}

func renderCloudInfo(w io.Writer, c *report.CloudInfo) {
	sectionHeading(w, "☁", "Cloud Accounts")

	for _, p := range c.Providers {
		fmt.Fprintf(w, "\n    %s%s%s\n", ColorBold, p.Name, ColorReset)
		fmt.Fprintf(w, "      %sRecovery Value:%s %s\n", ColorCyan, ColorReset, recoveryBadge(p.RecoveryLevel))

		fmt.Fprintf(w, "      %sStatus%s\n", ColorBold, ColorReset)
		cliIcon := ColorGreen + "✓ Installed" + ColorReset
		backupIcon := ColorGreen + "✓ Backed up" + ColorReset
		if !p.BackedUp {
			backupIcon = ColorYellow + "✗ Not backed up" + ColorReset
		}
		fmt.Fprintf(w, "        %s•%s CLI %s\n", ColorCyan, ColorReset, cliIcon)
		fmt.Fprintf(w, "        %s•%s Backup %s\n", ColorCyan, ColorReset, backupIcon)

		fmt.Fprintf(w, "      %sAuthentication%s\n", ColorBold, ColorReset)
		authIcon := ColorYellow + "✗ Not authenticated" + ColorReset
		if p.Authenticated {
			authIcon = ColorGreen + "✓ Authenticated" + ColorReset
		}
		fmt.Fprintf(w, "        %s•%s %s\n", ColorCyan, ColorReset, authIcon)

		credIcon := ColorYellow + "✗ Not detected" + ColorReset
		if p.Credentials {
			credIcon = ColorGreen + "✓ Present" + ColorReset
		}
		fmt.Fprintf(w, "        %s•%s Credentials %s\n", ColorCyan, ColorReset, credIcon)
		if p.AccountID != "" {
			fmt.Fprintf(w, "      %sAccount ID:%s %s\n", ColorCyan, ColorReset, p.AccountID)
		}
	}
}

func renderInfraInfo(w io.Writer, i *report.InfrastructureInfo) {
	sectionHeading(w, "⚙", "Infrastructure")

	fmt.Fprintf(w, "\n    %sRecovery Value:%s %s\n", ColorCyan, ColorReset, recoveryBadge(i.RecoveryLevel))

	if len(i.Tools) > 0 {
		fmt.Fprintf(w, "\n    %sTools%s\n", ColorBold, ColorReset)
		for _, tool := range i.Tools {
			fmt.Fprintf(w, "      %s✓%s %s\n", ColorGreen, ColorReset, tool)
		}
	}
	if i.Kubernetes != nil {
		k := i.Kubernetes
		fmt.Fprintf(w, "\n    %sKubernetes%s\n", ColorBold, ColorReset)
		if k.Version != "" {
			fmt.Fprintf(w, "      %sVersion:%s        %s\n", ColorCyan, ColorReset, k.Version)
		}
		kcIcon := ColorGreen + "✓ Detected" + ColorReset
		fmt.Fprintf(w, "      %sKubeconfig:%s %s\n", ColorCyan, ColorReset, kcIcon)
		if k.CurrentContext != "" {
			ctxColor := ColorGreen
			fmt.Fprintf(w, "      %sCurrent Context:%s %s%s%s\n", ColorCyan, ColorReset, ctxColor, k.CurrentContext, ColorReset)
		}
		if len(k.Contexts) > 0 {
			fmt.Fprintf(w, "      %sContexts:%s        %d\n", ColorCyan, ColorReset, len(k.Contexts))
		}
		if len(k.Namespaces) > 0 {
			fmt.Fprintf(w, "      %sNamespaces:%s      %d\n", ColorCyan, ColorReset, len(k.Namespaces))
		}
		if len(k.HelmRepos) > 0 {
			fmt.Fprintf(w, "      %sHelm Repos:%s      %d\n", ColorCyan, ColorReset, len(k.HelmRepos))
		}
	}
	backupIcon := ColorGreen + "✓ Backed up" + ColorReset
	if !i.BackedUp {
		backupIcon = ColorYellow + "✗ Not backed up" + ColorReset
	}
	fmt.Fprintf(w, "    %sBackup:%s %s\n", ColorCyan, ColorReset, backupIcon)
}

func renderPackageInfo(w io.Writer, p report.PackageInfo) {
	hasAny := p.Apt != nil || p.Snap != nil || p.Flatpak != nil
	if !hasAny {
		return
	}
	sectionHeading(w, "📦", "Packages")

	for _, pm := range []*report.PackageManagerInfo{p.Apt, p.Snap, p.Flatpak} {
		if pm == nil {
			continue
		}
		fmt.Fprintf(w, "\n    %s%s%s\n", ColorBold, pm.Name, ColorReset)
		fmt.Fprintf(w, "      %sRecovery Value:%s %s\n", ColorCyan, ColorReset, recoveryBadge(1))
		if pm.Version != "" {
			fmt.Fprintf(w, "      %sVersion:%s  %s\n", ColorCyan, ColorReset, pm.Version)
		}
		fmt.Fprintf(w, "      %sPackages:%s %d\n", ColorCyan, ColorReset, pm.Count)
	}
}

func renderSecurityInfo(w io.Writer, s *report.SecurityInfo) {
	sectionHeading(w, "🔒", "Certificates")

	fmt.Fprintf(w, "\n    %sRecovery Value:%s %s\n", ColorCyan, ColorReset, recoveryBadge(3))
	fmt.Fprintf(w, "    %sCertificate Stores:%s %d\n", ColorCyan, ColorReset, s.CertStores)
	fmt.Fprintf(w, "    %sValid:%s              %d\n", ColorCyan, ColorReset, s.ValidCerts)
	if s.Expired > 0 {
		fmt.Fprintf(w, "    %s%sExpired:%s%s          %d\n", ColorRed, ColorBold, ColorReset, ColorRed, s.Expired)
	} else {
		fmt.Fprintf(w, "    %sExpired:%s           %d\n", ColorCyan, ColorReset, s.Expired)
	}
	fmt.Fprintf(w, "    %sExpiring:%s          %d\n", ColorCyan, ColorReset, s.Expiring)
	fmt.Fprintf(w, "    %sCustom CA Bundles:%s %d\n", ColorCyan, ColorReset, s.CABundles)
}

func renderProjectsInfo(w io.Writer, p *report.ProjectsInfo) {
	if p.TotalRepos == 0 {
		sectionHeading(w, "📁", "Repositories")
		fmt.Fprintf(w, "\n    %sNo Git repositories discovered.%s\n", ColorCyan, ColorReset)
		return
	}
	sectionHeading(w, "📁", "Repositories")

	fmt.Fprintf(w, "\n    %sRecovery Value:%s %s\n", ColorCyan, ColorReset, recoveryBadge(4))
	fmt.Fprintf(w, "    %sTotal:%s           %d\n", ColorCyan, ColorReset, p.TotalRepos)
	if p.GitHubRepos > 0 {
		fmt.Fprintf(w, "    %sGitHub:%s          %d\n", ColorCyan, ColorReset, p.GitHubRepos)
	}
	if p.GitLabRepos > 0 {
		fmt.Fprintf(w, "    %sGitLab:%s          %d\n", ColorCyan, ColorReset, p.GitLabRepos)
	}
	if p.LocalOnly > 0 {
		fmt.Fprintf(w, "    %sLocal Only:%s      %d\n", ColorCyan, ColorReset, p.LocalOnly)
	}
	if p.DirtyRepos > 0 {
		fmt.Fprintf(w, "    %sUncommitted:%s     %d\n", ColorCyan, ColorReset, p.DirtyRepos)
	}
	if p.NoRemote > 0 {
		fmt.Fprintf(w, "    %sWithout Remote:%s %d\n", ColorCyan, ColorReset, p.NoRemote)
	}
}

func renderVirtInfo(w io.Writer, v *report.VirtualizationInfo) {
	sectionHeading(w, "💻", "Virtualization")

	fmt.Fprintf(w, "\n    %sRecovery Value:%s %s\n", ColorCyan, ColorReset, recoveryBadge(v.RecoveryLevel))

	if len(v.Platforms) > 0 {
		for _, p := range v.Platforms {
			fmt.Fprintf(w, "    %sPlatform:%s %s\n", ColorBold, ColorReset, p)
			fmt.Fprintf(w, "      %sInstalled:%s %s✓%s\n", ColorCyan, ColorReset, ColorGreen, ColorReset)
		}
	}
	backupIcon := ColorGreen + "✓ Backed up" + ColorReset
	if !v.BackedUp {
		backupIcon = ColorYellow + "✗ Not backed up" + ColorReset
	}
	fmt.Fprintf(w, "      %sBackup:%s %s\n", ColorCyan, ColorReset, backupIcon)
}

func renderRecoveryGaps(w io.Writer, gaps []report.RecoveryGap) {
	sectionHeading(w, "⚠", "Recovery Gaps")

	for _, g := range gaps {
		fmt.Fprintf(w, "\n    %s%s%s\n", ColorBold, g.Name, ColorReset)
		fmt.Fprintf(w, "      %sCategory:%s %s\n", ColorCyan, ColorReset, g.Category)
		fmt.Fprintf(w, "      %sIssue:%s    %s%s%s\n", ColorCyan, ColorReset, ColorYellow, g.Issue, ColorReset)
	}
}

func renderBackupSummary(w io.Writer, b report.BackupSummary) {
	sectionHeading(w, "💾", "Backup Summary")

	if b.LatestSnapshot != "" {
		fmt.Fprintf(w, "\n    %sLatest Snapshot%s\n", ColorBold, ColorReset)
		fmt.Fprintf(w, "    %s\n", b.LatestSnapshot)
	}
	if b.CreatedAt != "" {
		fmt.Fprintf(w, "\n    %sCreated%s\n", ColorBold, ColorReset)
		fmt.Fprintf(w, "    %s\n", b.CreatedAt)
	}
	fmt.Fprintf(w, "\n    %sSnapshots:%s  %d\n", ColorCyan, ColorReset, b.SnapshotCount)
	if b.TotalSize != "" {
		fmt.Fprintf(w, "    %sTotal Size:%s %s\n", ColorCyan, ColorReset, b.TotalSize)
	}
	if b.RecoverableCount > 0 && b.TotalCount > 0 {
		fmt.Fprintf(w, "    %sRecoverable:%s %d / %d\n", ColorCyan, ColorReset, b.RecoverableCount, b.TotalCount)
	}
	if b.RestoreTime != "" {
		fmt.Fprintf(w, "    %sRestore Time:%s %s\n", ColorCyan, ColorReset, b.RestoreTime)
	}
	encColor := ColorGreen
	encIcon := "✓"
	if b.Encryption == "Disabled" {
		encColor = ColorYellow
		encIcon = "✗"
	}
	fmt.Fprintf(w, "\n    %sEncryption:%s %s%s %s%s\n", ColorCyan, ColorReset, encColor, encIcon, b.Encryption, ColorReset)
	fmt.Fprintf(w, "    %sStorage:%s    %s\n", ColorCyan, ColorReset, b.StorageProvider)
}

func renderAssetStats(w io.Writer, s report.AssetStats) {
	sectionHeading(w, "📊", "Statistics")

	hasAny := s.Languages > 0 || s.Browsers > 0 || s.Editors > 0 || s.Databases > 0 ||
		s.Containers > 0 || s.DockerVolumes > 0 || s.ComposeProjects > 0 ||
		s.Repositories > 0 || s.Certificates > 0 || s.SSHKeys > 0 || s.GPGKeys > 0 || s.CloudProviders > 0
	if !hasAny {
		fmt.Fprintf(w, "\n    %s•%s No assets detected\n", ColorCyan, ColorReset)
		return
	}
	fmt.Fprintln(w)
	renderStatsLine(w, "Languages", s.Languages)
	renderStatsLine(w, "Browsers", s.Browsers)
	renderStatsLine(w, "Editors", s.Editors)
	renderStatsLine(w, "Databases", s.Databases)
	renderStatsLine(w, "Containers", s.Containers)
	renderStatsLine(w, "Docker Volumes", s.DockerVolumes)
	renderStatsLine(w, "Compose Projects", s.ComposeProjects)
	renderStatsLine(w, "Repositories", s.Repositories)
	renderStatsLine(w, "Certificates", s.Certificates)
	renderStatsLine(w, "SSH Keys", s.SSHKeys)
	renderStatsLine(w, "GPG Keys", s.GPGKeys)
	renderStatsLine(w, "Cloud Providers", s.CloudProviders)
}

func renderStatsLine(w io.Writer, label string, value int) {
	if value == 0 {
		return
	}
	fmt.Fprintf(w, "    %s\n", ColorCyan+dotLeader(label, fmt.Sprintf("%d", value), 22)+ColorReset)
}

func renderNotDetected(w io.Writer, mods []string) {
	sectionHeading(w, "📭", "Not Installed")

	for _, m := range mods {
		fmt.Fprintf(w, "\n    %s•%s %s\n", ColorCyan, ColorReset, m)
	}
}

func renderCoverageInfo(w io.Writer, c report.CoverageInfo) {
	sectionHeading(w, "📡", "Inventory Coverage")

	pctColor := colorForScore(c.CoveragePercent)
	fmt.Fprintf(w, "\n    %sDetected Modules:%s %s%d%s\n", ColorCyan, ColorReset, ColorGreen, c.DetectedModules, ColorReset)
	fmt.Fprintf(w, "    %sTotal Modules:%s   %s%d%s\n", ColorCyan, ColorReset, ColorBold, c.TotalModules, ColorReset)
	if c.MissingModules > 0 {
		fmt.Fprintf(w, "    %sNot Installed:%s   %s%d%s\n", ColorCyan, ColorReset, ColorYellow, c.MissingModules, ColorReset)
	}
	fmt.Fprintf(w, "    %sCoverage:%s        %s%d%%%s\n", ColorCyan, ColorReset, pctColor, c.CoveragePercent, ColorReset)
}

func renderRecoveryVerdict(w io.Writer, v report.RecoveryVerdict) {
	sectionHeading(w, "📌", "Recovery Verdict")

	fmt.Fprintf(w, "\n    %s\n", v.Summary)
	fmt.Fprintf(w, "\n    %sCurrent confidence:%s %d%%\n", ColorCyan, ColorReset, v.Confidence)
	fmt.Fprintf(w, "    %sEstimated after recommendations:%s %d%%\n", ColorCyan, ColorReset, v.TargetConfidence)

	if len(v.CriticalActions) > 0 {
		fmt.Fprintf(w, "\n    %sCritical actions remaining:%s\n", ColorBold, ColorReset)
		for _, action := range v.CriticalActions {
			fmt.Fprintf(w, "    %s•%s %s\n", ColorYellow, ColorReset, action)
		}
	}
}

func renderReportMetadata(w io.Writer, m report.ReportMetadata) {
	sectionHeading(w, "🏷", "Report Metadata")

	fmt.Fprintf(w, "\n    %sGenerated By:%s %s\n", ColorCyan, ColorReset, m.GeneratedBy)
	fmt.Fprintf(w, "    %sVersion:%s      %s\n", ColorCyan, ColorReset, m.Version)
	fmt.Fprintf(w, "    %sGenerated At:%s %s\n", ColorCyan, ColorReset, m.GeneratedAt)
	if m.MachineID != "" {
		fmt.Fprintf(w, "    %sMachine ID:%s   %s\n", ColorCyan, ColorReset, m.MachineID)
	}
	if m.Checksum != "" {
		fmt.Fprintf(w, "    %sChecksum:%s     %s\n", ColorCyan, ColorReset, m.Checksum)
	}
	fmt.Fprintf(w, "    %sFormat:%s       %s\n", ColorCyan, ColorReset, m.Format)
}

func commaList(items []string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items, ", ")
}

func renderConfidence(w io.Writer, c doctor.Confidence) {
	gradeColor := ColorGreen
	if c.Score < 50 {
		gradeColor = ColorRed
	} else if c.Score < 70 {
		gradeColor = ColorYellow
	} else if c.Score < 85 {
		gradeColor = ColorCyan
	}

	fmt.Fprintf(w, "\n  %sRecovery Confidence%s\n", ColorBold+ColorBlue, ColorReset)
	fmt.Fprintln(w)

	bar := scoreBar(c.Score)
	fmt.Fprintf(w, "    %s%s%s\n", ColorBold, c.Grade, ColorReset)
	fmt.Fprintln(w, "    "+bar)
	fmt.Fprintf(w, "    %s%s%s\n", gradeColor, c.Message, ColorReset)

	if len(c.Reasons) > 0 {
		fmt.Fprintln(w)
		for _, reason := range c.Reasons {
			rColor := ColorYellow
			if strings.HasPrefix(reason, "✓") {
				rColor = ColorGreen
			}
			fmt.Fprintf(w, "      %s%s%s\n", rColor, reason, ColorReset)
		}
	}
}

func renderRisks(w io.Writer, risks []doctor.Risk) {
	fmt.Fprintf(w, "  %sRecovery Risks%s\n", ColorBold+ColorBlue, ColorReset)
	fmt.Fprintln(w)

	if len(risks) == 0 {
		fmt.Fprintf(w, "    %s✓%s No risks detected\n", ColorGreen, ColorReset)
		return
	}

	for _, risk := range risks {
		severityColor := severityColor(risk.Severity)
		icon := "⚠"
		if risk.Severity == "Critical" {
			icon = "✗"
		} else if risk.Severity == "Info" {
			icon = "ℹ"
		}

		fmt.Fprintf(w, "    %s%s%s [%s%s%s] %s\n",
			severityColor, icon, ColorReset,
			severityColor, strings.ToUpper(risk.Severity), ColorReset,
			risk.Message)

		if risk.Impact != "" {
			fmt.Fprintf(w, "      %sImpact:%s %s\n", ColorCyan, ColorReset, risk.Impact)
		}
		if risk.Command != "" {
			fmt.Fprintf(w, "      %s▸%s %s\n", ColorCyan, ColorReset, risk.Command)
		}
		if risk.Effort != "" && risk.Effort != "-" {
			fmt.Fprintf(w, "      %sEffort:%s %s\n", ColorCyan, ColorReset, risk.Effort)
		}
		fmt.Fprintln(w)
	}
}

func renderCoverage(w io.Writer, c doctor.BackupCoverage) {
	fmt.Fprintf(w, "  %sBackup Coverage%s\n", ColorBold+ColorBlue, ColorReset)
	fmt.Fprintln(w)

	coveragePct := c.CoveragePercent
	pctColor := colorForScore(coveragePct)
	bar := miniBarColor(c.ProtectedCount, c.ProtectedCount+c.UnprotectedCount, 10, pctColor)
	fmt.Fprintf(w, "    %sProtected Assets:%s %d    %sUnprotected:%s %d    %sCoverage:%s %s%d%%%s\n",
		ColorGreen, ColorReset, c.ProtectedCount,
		ColorYellow, ColorReset, c.UnprotectedCount,
		pctColor, ColorReset,
		pctColor, coveragePct, ColorReset)
	fmt.Fprintln(w, "    "+bar)

	if len(c.Protected) > 0 {
		fmt.Fprintf(w, "    %sProtected:%s %s\n", ColorGreen, ColorReset, strings.Join(c.Protected, ", "))
	}
	if len(c.Unprotected) > 0 {
		fmt.Fprintf(w, "    %sNot Protected:%s %s\n", ColorYellow, ColorReset, strings.Join(c.Unprotected, ", "))
	}
}

func renderTimeline(w io.Writer, tl doctor.RecoveryTimeline) {
	fmt.Fprintf(w, "  %sEstimated Recovery Timeline%s\n", ColorBold+ColorBlue, ColorReset)
	fmt.Fprintln(w)

	for _, e := range tl.Entries {
		dots := 20 - len(e.Category)
		if dots < 1 {
			dots = 1
		}
		fmt.Fprintf(w, "    %s %s %s\n",
			e.Category,
			strings.Repeat(".", dots),
			e.Duration)
	}
	fmt.Fprintf(w, "\n    %sEstimated Full Restore:%s %s\n", ColorBold, ColorReset, tl.Total)
	if tl.EstimatedBackupSize != "" {
		fmt.Fprintf(w, "    %sEstimated Data Size:%s %s\n", ColorBold, ColorReset, tl.EstimatedBackupSize)
	}
	if tl.ManualSteps > 0 {
		fmt.Fprintf(w, "    %sManual Steps Required:%s %d\n", ColorBold, ColorReset, tl.ManualSteps)
	}
}

func renderActionPlan(w io.Writer, actions []doctor.Action, confidence doctor.Confidence) {
	fmt.Fprintf(w, "  %sAction Plan%s\n", ColorBold+ColorBlue, ColorReset)
	fmt.Fprintln(w)

	if len(actions) == 0 {
		fmt.Fprintf(w, "    %s✓%s No actions needed - everything is backed up\n", ColorGreen, ColorReset)
		return
	}

	for _, a := range actions {
		pColor := severityColor(a.Impact)
		difficultyStars := difficultyStars(a.Difficulty)
		fmt.Fprintf(w, "    %sPriority %d%s", ColorBold, a.Priority, ColorReset)
		if a.Difficulty != "" {
			fmt.Fprintf(w, "  %s%s%s", difficultyColor(a.Difficulty), difficultyStars, ColorReset)
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "      %s•%s %s\n", pColor, ColorReset, a.Message)
		fmt.Fprintf(w, "      %sImpact:%s %s    %sEffort:%s %s\n",
			ColorCyan, ColorReset, a.Impact,
			ColorCyan, ColorReset, a.Effort)
		if a.Command != "" {
			fmt.Fprintf(w, "      %s▸%s %s\n", ColorCyan, ColorReset, a.Command)
		}
		fmt.Fprintln(w)
	}

	improvement := confidence.Score + 20
	if improvement > 95 {
		improvement = 95
	}
	fmt.Fprintf(w, "    %sCompleting these actions would raise recovery confidence to ~%d%%%s\n",
		ColorBold, improvement, ColorReset)
}

func renderReadiness(w io.Writer, readiness []doctor.CategoryReadiness) {
	fmt.Fprintf(w, "  %sRecovery Readiness by Category%s\n", ColorBold+ColorBlue, ColorReset)
	fmt.Fprintln(w)

	for _, cat := range readiness {
		pctColor := colorForScore(cat.Percent)
		bar := miniBarColor(cat.Score, cat.Max, 10, pctColor)
		statusColor := statusColor(cat.Status)
		fmt.Fprintf(w, "    %-18s%s  %s%d%%%s  %s(%s)%s\n",
			cat.Name+":", bar,
			pctColor, cat.Percent, ColorReset,
			statusColor, cat.Status, ColorReset)
		fmt.Fprintf(w, "      %sReason:%s %s\n", ColorCyan, ColorReset, cat.Reason)
		if len(cat.Details) > 0 {
			for _, detail := range cat.Details {
				fmt.Fprintf(w, "      %s•%s %s\n", ColorCyan, ColorReset, detail)
			}
		}
	}
}

func renderMachineStatus(w io.Writer, ms doctor.MachineStatus) {
	fmt.Fprintf(w, "  %sMachine Recovery Status%s\n", ColorBold+ColorBlue, ColorReset)
	fmt.Fprintln(w)

	overallColor := ColorGreen
	if ms.Overall == "Significantly At Risk" {
		overallColor = ColorRed
	} else if ms.Overall == "Mostly Recoverable" {
		overallColor = ColorYellow
	}
	fmt.Fprintf(w, "    %sOverall:%s %s%s%s\n", ColorBold, ColorReset, overallColor, ms.Overall, ColorReset)
	fmt.Fprintln(w)

	for _, e := range ms.Categories {
		sc := statusColor(e.Status)
		icon := "✓"
		if e.Status == "At Risk" {
			icon = "✗"
		} else if e.Status == "Partially Recoverable" {
			icon = "~"
		}
		fmt.Fprintf(w, "    %s%s%s %-20s %s%s%s\n",
			sc, icon, ColorReset,
			e.Category,
			sc, e.Status, ColorReset)
	}
}

func renderDoctorSummary(w io.Writer, summary string) {
	fmt.Fprintf(w, "  %sSummary%s\n", ColorBold+ColorBlue, ColorReset)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %s\n", summary)
	fmt.Fprintln(w)
}

func renderDisasterPreview(w io.Writer, dp doctor.DisasterPreview) {
	fmt.Fprintf(w, "  %sDisaster Recovery Preview%s\n", ColorBold+ColorBlue, ColorReset)
	fmt.Fprintln(w)

	if len(dp.WouldLose) == 0 && len(dp.WouldKeep) == 0 {
		fmt.Fprintf(w, "    %s•%s No assets detected — workstation appears clean\n", ColorCyan, ColorReset)
		return
	}

	if len(dp.WouldLose) > 0 {
		fmt.Fprintf(w, "    %s✗ Would Lose:%s\n", ColorRed, ColorReset)
		for _, item := range dp.WouldLose {
			fmt.Fprintf(w, "      %s•%s %s\n", ColorRed, ColorReset, item)
		}
	}

	if len(dp.WouldKeep) > 0 {
		if len(dp.WouldLose) > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "    %s✓ Would Keep:%s\n", ColorGreen, ColorReset)
		for _, item := range dp.WouldKeep {
			fmt.Fprintf(w, "      %s•%s %s\n", ColorGreen, ColorReset, item)
		}
	}
}

func difficultyStars(d string) string {
	switch d {
	case "Easy":
		return "★☆☆"
	case "Medium":
		return "★★☆"
	case "Advanced":
		return "★★★"
	default:
		return ""
	}
}

func difficultyColor(d string) string {
	switch d {
	case "Easy":
		return ColorGreen
	case "Medium":
		return ColorYellow
	case "Advanced":
		return ColorRed
	default:
		return ColorCyan
	}
}

func severityColor(severity string) string {
	switch severity {
	case "Critical":
		return ColorRed
	case "High":
		return ColorYellow
	case "Medium":
		return ColorGreen
	case "Low":
		return ColorCyan
	default:
		return ColorBlue
	}
}

func statusColor(status string) string {
	switch status {
	case "Recoverable":
		return ColorGreen
	case "Partially Recoverable":
		return ColorYellow
	case "At Risk":
		return ColorRed
	default:
		return ColorCyan
	}
}
