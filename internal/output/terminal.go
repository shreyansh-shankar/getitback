package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/module"
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
