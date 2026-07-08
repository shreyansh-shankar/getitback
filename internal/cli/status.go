package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/assessment"
	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/storage"
	"github.com/spf13/cobra"
)

var statusCategoryOrder = []string{
	"Identity",
	"Configuration",
	"Development",
	"Editors",
	"Browsers",
	"Databases",
	"Containers",
	"Cloud",
	"Infrastructure",
	"Projects",
	"Virtualization",
	"Packages",
	"Security",
	"System",
}

var statusImportance = map[string]int{
	"ssh":            5,
	"gpg":            5,
	"postgres":       5,
	"mysql":          5,
	"mongodb":        5,
	"docker":         5,
	"cloud":          5,
	"kubernetes":     5,
	"firefox":        4,
	"chrome":         4,
	"repos":          4,
	"certs":          4,
	"dotfiles":       4,
	"vscode":         4,
	"redis":          4,
	"sqlite":         3,
	"chromium":       3,
	"brave":          3,
	"neovim":         3,
	"git":            3,
	"golang":         2,
	"nodejs":         2,
	"python":         2,
	"rust":           2,
	"java":           2,
	"shell":          2,
	"virtualization": 2,
	"vivaldi":        2,
	"edge":           2,
	"opera":          2,
	"apt":            1,
	"snap":           1,
	"flatpak":        1,
	"system":         1,
}

var statusModuleDisplay = map[string]string{
	"ssh": "SSH", "gpg": "GPG", "docker": "Docker",
	"firefox": "Firefox", "chrome": "Chrome", "chromium": "Chromium",
	"brave": "Brave", "vivaldi": "Vivaldi", "edge": "Edge", "opera": "Opera",
	"postgres": "PostgreSQL", "mysql": "MySQL", "mongodb": "MongoDB",
	"redis": "Redis", "sqlite": "SQLite",
	"golang": "Go", "nodejs": "Node.js", "python": "Python",
	"rust": "Rust", "java": "Java",
	"vscode": "VS Code", "neovim": "Neovim",
	"dotfiles": "Dotfiles", "shell": "Shell", "git": "Git",
	"cloud": "Cloud Credentials", "kubernetes": "Kubernetes",
	"repos": "Repositories", "certs": "Certificates",
	"virtualization": "Virtualization",
	"apt": "APT", "snap": "Snap", "flatpak": "Flatpak",
	"system": "System",
}

func newStatusCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show backup health dashboard",
		Long: `Display a concise backup health dashboard for your workstation.

Shows recovery confidence, critical gaps, coverage, and a module summary
at a glance. Designed to answer "am I protected right now?" in under 5 seconds.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			results := manager.Inventory(ctx)

			backups, err := storage.ListBackups(cfg.Storage.Path)
			if err != nil {
				backups = nil
			}

			backedUp := make(map[string]bool)
			var latestSnapshot string
			var backupTime time.Time
			var totalBackupSize int64

			if len(backups) > 0 {
				backupDir := filepath.Join(cfg.Storage.Path, backups[0].ID)
				manifest, err := storage.ReadManifest(backupDir)
				if err == nil {
					for _, snap := range manifest.Snapshots {
						backedUp[snap.Module] = true
					}
				}
				latestSnapshot = backups[0].ID
				backupTime = backups[0].CreatedAt
				totalBackupSize = backups[0].Size
			}

			coverage := assessment.ComputeCoverage(results)
			score := assessment.ComputeScore(results, coverage, moduleGroups)

			w := os.Stdout

			printStatusHeader(w)
			printRecoveryOverview(w, results, backedUp, &score, len(backups), backupTime, cfg.Encryption.Enabled)
			fmt.Fprintln(w)
			printCriticalGaps(w, results, backedUp)
			fmt.Fprintln(w)
			printBackupCoverage(w, results, backedUp)
			fmt.Fprintln(w)
			printModuleSummary(w, results, backedUp)
			fmt.Fprintln(w)
			printRecentBackup(w, latestSnapshot, len(backups), totalBackupSize)
			fmt.Fprintln(w)
			printNextAction(w, results, backedUp)

			return nil
		},
	}
}

func printStatusHeader(w *os.File) {
	fmt.Fprintf(w, "\n  %s%s%s\n", output.ColorBold+output.ColorCyan, strings.Repeat("━", 50), output.ColorReset)
	fmt.Fprintf(w, "  %s  Backup Status%s\n", output.ColorBold+output.ColorCyan, output.ColorReset)
	fmt.Fprintf(w, "  %s%s%s\n", output.ColorBold+output.ColorCyan, strings.Repeat("━", 50), output.ColorReset)
}

func printRecoveryOverview(w *os.File, results []*module.InventoryResult, backedUp map[string]bool, score *module.RecoveryScore, backupCount int, backupTime time.Time, encryptionEnabled bool) {
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
	health := "Critical"
	healthColor := output.ColorRed
	switch {
	case pct >= 80:
		grade = "GOOD"
		health = "Healthy"
		healthColor = output.ColorGreen
	case pct >= 50:
		grade = "FAIR"
		health = "Needs Attention"
		healthColor = output.ColorYellow
	}

	detectedCount := 0
	for _, res := range results {
		if res.Detected {
			detectedCount++
		}
	}

	protectedCount := 0
	for _, res := range results {
		if res.Detected && backedUp[res.Module] {
			protectedCount++
		}
	}

	pad := 16
	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %sRecovery Overview%s\n", output.ColorBold, output.ColorReset)
	fmt.Fprintln(w)

	gradeColor := output.ColorGreen
	if pct < 50 {
		gradeColor = output.ColorRed
	} else if pct < 70 {
		gradeColor = output.ColorYellow
	} else if pct < 85 {
		gradeColor = output.ColorCyan
	}
	fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Recovery Confidence", gradeColor+fmt.Sprintf("%d%%", pct)+" ("+grade+")"+output.ColorReset, pad)+output.ColorReset)

	fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Protected Modules", output.ColorGreen+fmt.Sprintf("%d", protectedCount)+output.ColorReset+" / "+fmt.Sprintf("%d", detectedCount), pad)+output.ColorReset)

	if backupCount > 0 {
		fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Latest Backup", backupTime.Format("Jan 2, 2006 15:04 MST"), pad)+output.ColorReset)
	} else {
		fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Latest Backup", output.ColorYellow+"No backups"+output.ColorReset, pad)+output.ColorReset)
	}

	encText := "Disabled"
	encColor := output.ColorYellow
	if encryptionEnabled {
		encText = "Enabled"
		encColor = output.ColorGreen
	}
	fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Encryption", encColor+encText+output.ColorReset, pad)+output.ColorReset)

	fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Backup Health", healthColor+health+output.ColorReset, pad)+output.ColorReset)

	// Score bar
	bar := scoreBarCompact(pct)
	fmt.Fprintf(w, "\n    %s\n", output.ColorCyan+bar+output.ColorReset)
}

func scoreBarCompact(score int) string {
	const width = 25
	filled := score * width / 100
	c := output.ColorGreen
	if score < 40 {
		c = output.ColorRed
	} else if score < 70 {
		c = output.ColorYellow
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
	return c + bar.String() + output.ColorReset + "  " + c + fmt.Sprintf("%d%%", score) + output.ColorReset
}

func printCriticalGaps(w *os.File, results []*module.InventoryResult, backedUp map[string]bool) {
	type gap struct {
		module     string
		importance int
		display    string
	}

	var gaps []gap
	for _, res := range results {
		if !res.Detected {
			continue
		}
		if backedUp[res.Module] {
			continue
		}
		imp := statusImportance[res.Module]
		display := res.Module
		if d, ok := statusModuleDisplay[res.Module]; ok {
			display = d
		}
		gaps = append(gaps, gap{module: res.Module, importance: imp, display: display})
	}

	if len(gaps) == 0 {
		return
	}

	sort.Slice(gaps, func(i, j int) bool {
		if gaps[i].importance != gaps[j].importance {
			return gaps[i].importance > gaps[j].importance
		}
		return gaps[i].module < gaps[j].module
	})

	if len(gaps) > 8 {
		gaps = gaps[:8]
	}

	fmt.Fprintf(w, "    %sCritical Gaps%s\n", output.ColorBold, output.ColorReset)
	fmt.Fprintln(w)
	for _, g := range gaps {
		fmt.Fprintf(w, "      %s✗%s %s\n", output.ColorRed, output.ColorReset, g.display)
	}
	if len(gaps) >= 8 {
		fmt.Fprintf(w, "      %s... and more%s\n", output.ColorCyan, output.ColorReset)
	}
}

func printBackupCoverage(w *os.File, results []*module.InventoryResult, backedUp map[string]bool) {
	protected := 0
	unprotected := 0
	for _, res := range results {
		if !res.Detected {
			continue
		}
		if statusImportance[res.Module] < 2 {
			continue
		}
		if backedUp[res.Module] {
			protected++
		} else {
			unprotected++
		}
	}

	total := protected + unprotected
	pct := 0
	if total > 0 {
		pct = protected * 100 / total
	}

	fmt.Fprintf(w, "    %sBackup Coverage%s\n", output.ColorBold, output.ColorReset)
	fmt.Fprintln(w)

	bar := coverageBar(pct)
	fmt.Fprintf(w, "    %s\n", output.ColorCyan+bar+output.ColorReset)
	pctColor := output.ColorGreen
	if pct < 40 {
		pctColor = output.ColorRed
	} else if pct < 70 {
		pctColor = output.ColorYellow
	}
	fmt.Fprintf(w, "    %s%d%%%s\n", pctColor, pct, output.ColorReset)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %sProtected:%s %d    %sMissing:%s %d\n",
		output.ColorGreen, output.ColorReset, protected,
		output.ColorYellow, output.ColorReset, unprotected)
}

func coverageBar(pct int) string {
	const width = 20
	filled := pct * width / 100
	c := output.ColorGreen
	if pct < 40 {
		c = output.ColorRed
	} else if pct < 70 {
		c = output.ColorYellow
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
	return c + bar.String() + output.ColorReset
}

func printModuleSummary(w *os.File, results []*module.InventoryResult, backedUp map[string]bool) {
	// Group detected modules by category
	type modInfo struct {
		name    string
		display string
		backed  bool
	}
	grouped := make(map[string][]modInfo)

	for _, res := range results {
		if !res.Detected {
			continue
		}
		cat := moduleGroups[res.Module]
		if cat == "" {
			cat = "Other"
		}
		display := res.Module
		if d, ok := statusModuleDisplay[res.Module]; ok {
			display = d
		}
		grouped[cat] = append(grouped[cat], modInfo{
			name:    res.Module,
			display: display,
			backed:  backedUp[res.Module],
		})
	}

	if len(grouped) == 0 {
		return
	}

	fmt.Fprintf(w, "    %sModule Summary%s\n", output.ColorBold, output.ColorReset)
	fmt.Fprintln(w)

	for _, cat := range statusCategoryOrder {
		mods, ok := grouped[cat]
		if !ok || len(mods) == 0 {
			continue
		}

		sort.Slice(mods, func(i, j int) bool {
			ii := statusImportance[mods[i].name]
			ji := statusImportance[mods[j].name]
			if ii != ji {
				return ii > ji
			}
			return mods[i].name < mods[j].name
		})

		fmt.Fprintf(w, "    %s%s%s\n", output.ColorBold+output.ColorBlue, cat, output.ColorReset)
		for _, m := range mods {
			icon := output.ColorGreen + "✓" + output.ColorReset
			if !m.backed {
				icon = output.ColorRed + "✗" + output.ColorReset
			}
			fmt.Fprintf(w, "      %s %s\n", icon, m.display)
		}
	}
}

func printRecentBackup(w *os.File, latestSnapshot string, snapshotCount int, totalSize int64) {
	fmt.Fprintf(w, "    %sRecent Backup%s\n", output.ColorBold, output.ColorReset)
	fmt.Fprintln(w)

	if latestSnapshot == "" {
		fmt.Fprintf(w, "      %sNo backups have been created yet.%s\n", output.ColorYellow, output.ColorReset)
		fmt.Fprintf(w, "      Run: getitback backup\n")
		return
	}

	pad := 14
	backupID := latestSnapshot
	if len(backupID) > 40 {
		backupID = backupID[:37] + "..."
	}
	fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Latest Snapshot", backupID, pad)+output.ColorReset)
	fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Snapshots", fmt.Sprintf("%d", snapshotCount), pad)+output.ColorReset)
	fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Backup Size", formatBytes(totalSize), pad)+output.ColorReset)
}

func printNextAction(w *os.File, results []*module.InventoryResult, backedUp map[string]bool) {
	var bestMod string
	bestImp := 0

	for _, res := range results {
		if !res.Detected {
			continue
		}
		if backedUp[res.Module] {
			continue
		}
		imp := statusImportance[res.Module]
		if imp > bestImp {
			bestImp = imp
			bestMod = res.Module
		}
	}

	fmt.Fprintf(w, "    %sRecommended Next Action%s\n", output.ColorBold, output.ColorReset)
	fmt.Fprintln(w)

	if bestMod == "" {
		fmt.Fprintf(w, "      %s✓%s All detected modules are backed up.\n", output.ColorGreen, output.ColorReset)
		return
	}

	display := bestMod
	if d, ok := statusModuleDisplay[bestMod]; ok {
		display = d
	}

	fmt.Fprintf(w, "      %sRun:%s\n", output.ColorCyan, output.ColorReset)
	fmt.Fprintf(w, "      %s$ getitback backup --module %s%s\n", output.ColorGreen, bestMod, output.ColorReset)
	fmt.Fprintf(w, "\n      %s%s is the highest-priority asset not yet backed up.%s\n", output.ColorCyan, display, output.ColorReset)
}
