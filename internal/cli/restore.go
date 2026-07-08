package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/restore"
	"github.com/shreyansh-shankar/getitback/internal/runtime/executor"
	"github.com/shreyansh-shankar/getitback/internal/storage"
	"github.com/spf13/cobra"
)

func newRestoreCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <backup-dir>",
		Short: "Restore a machine from a backup",
		Long: `Restore your development environment from a backup.

Fully automated machine recovery with dependency resolution,
software installation, data restoration, and validation.

Stages:
  1. Loading Backup
  2. Restore Plan
  3. Dependency Resolution
  4. Installing Required Software
  5. Restoring Data
  6. Post-Restore Hooks
  7. Service Startup
  8. Validation
  9. Recovery Report
  10. Completion`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			w := cmd.OutOrStdout()

			// Require root privileges for restore
			if !executor.IsRoot() {
				fmt.Fprintf(w, "\n  %sRestore requires administrator privileges.%s\n\n", output.ColorRed, output.ColorReset)
				fmt.Fprintf(w, "  Please run:\n\n")
				fmt.Fprintf(w, "    %ssudo getitback restore%s\n\n", output.ColorBold, output.ColorReset)
				return fmt.Errorf("restore requires root privileges")
			}

			// When running under sudo, set HOME to the original user's home so
			// modules restore data to the correct location.
			if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
				origHome := filepath.Join("/home", sudoUser)
				if info, err := os.Stat(origHome); err == nil && info.IsDir() {
					os.Setenv("HOME", origHome)
				}
			}

			// Determine backup directory
			var backupDir string
			if len(args) == 1 {
				backupDir = args[0]
			} else if id, _ := cmd.Flags().GetString("id"); id != "" {
				backupDir = filepath.Join(cfg.Storage.Path, id)
			} else {
				latest, err := storage.LatestBackup(cfg.Storage.Path)
				if err != nil {
					return fmt.Errorf("list backups: %w", err)
				}
				if latest == "" {
					return fmt.Errorf("no backups found in %s", cfg.Storage.Path)
				}
				backupDir = filepath.Join(cfg.Storage.Path, latest)
			}

			backupDir, err := filepath.Abs(backupDir)
			if err != nil {
				return fmt.Errorf("resolve path: %w", err)
			}

			info, err := os.Stat(filepath.Join(backupDir, "manifest.json"))
			if err != nil {
				return fmt.Errorf("backup not found at %s: %w", backupDir, err)
			}
			_ = info

			planner := restore.NewPlanner(manager, backupDir)
			manifest, err := planner.LoadManifest()
			if err != nil {
				return fmt.Errorf("load manifest: %w", err)
			}

			available := planner.AvailableModules(manifest)
			if len(available) == 0 {
				return fmt.Errorf("no modules in backup match registered modules")
			}

			moduleFilter, _ := cmd.Flags().GetString("module")
			preset, _ := cmd.Flags().GetString("preset")

			var selected []string

			switch {
			case moduleFilter != "":
				found := false
				for _, a := range available {
					if a == moduleFilter {
						selected = []string{a}
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("module %q not found in backup", moduleFilter)
				}

			case preset != "":
				selected = filterByPreset(available, preset)

			default:
				selected = interactiveSelection(w, available, manifest)
			}

			if len(selected) == 0 {
				return fmt.Errorf("no modules selected for restore")
			}

			// Build and execute plan
			plan, err := planner.BuildPlan(ctx, manifest, selected)
			if err != nil {
				return fmt.Errorf("build restore plan: %w", err)
			}

			dryRun, _ := cmd.Flags().GetBool("dry-run")

			// Resolve working directory with fallback chain:
			// 1. --workdir CLI flag
			// 2. GETITBACK_WORKDIR env var
			// 3. $HOME/.cache/getitback
			// 4. /tmp (last resort)
			workDir, _ := cmd.Flags().GetString("workdir")
			if workDir == "" {
				workDir = os.Getenv("GETITBACK_WORKDIR")
			}
			if workDir == "" {
				home := os.Getenv("HOME")
				if home != "" {
					cacheDir := filepath.Join(home, ".cache", "getitback")
					if info, err := os.Stat(cacheDir); err == nil && info.IsDir() {
						workDir = cacheDir
					}
				}
			}
			if workDir == "" {
				workDir = "/tmp"
			}

			engine := restore.NewEngine(manager, backupDir)
			engine.SetManifest(manifest)
			engine.SetPlan(plan)
			engine.SetDryRun(dryRun)
			engine.SetWorkDir(workDir)

			report, err := engine.Execute(ctx, w)
			if err != nil {
				return err
			}

			if jsonFlag, _ := cmd.Flags().GetBool("json"); jsonFlag {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				enc.Encode(report)
			} else if reportPath, _ := cmd.Flags().GetString("report"); reportPath != "" {
				f, err := os.Create(reportPath)
				if err != nil {
					return fmt.Errorf("create report file: %w", err)
				}
				defer f.Close()
				enc := json.NewEncoder(f)
				enc.SetIndent("", "  ")
				enc.Encode(report)
			}

			return nil
		},
	}

	cmd.Flags().String("id", "", "Backup ID to restore from (alternative to path argument)")
	cmd.Flags().String("module", "", "Restore a single module only")
	cmd.Flags().String("preset", "", "Preset: everything, dev, browsers, secrets, identity")
	cmd.Flags().Bool("dry-run", false, "Print restore plan without making changes")
	cmd.Flags().Bool("json", false, "Output recovery report as JSON")
	cmd.Flags().String("report", "", "Save recovery report to file")
	cmd.Flags().String("workdir", "", "Working directory for temporary extraction (default: GETITBACK_WORKDIR, or $HOME/.cache/getitback, or /tmp)")
	return cmd
}

func interactiveSelection(w interface{ Write([]byte) (int, error) }, available []string, manifest *storage.Manifest) []string {
	// Group available modules by category
	type catMod struct {
		Name     string
		Category string
		Size     int64
	}
	var allMods []catMod
	for _, name := range available {
		cat := moduleGroups[name]
		if cat == "" {
			cat = "Other"
		}
		var size int64
		for _, snap := range manifest.Snapshots {
			if snap.Module == name {
				size = snap.Size
				break
			}
		}
		allMods = append(allMods, catMod{Name: name, Category: cat, Size: size})
	}

	grouped := make(map[string][]catMod)
	for _, m := range allMods {
		grouped[m.Category] = append(grouped[m.Category], m)
	}

	// Print categories as numbered groups
	fmt.Fprintf(w, "\n")
	stageHeader(w, "Select", "Modules to Restore")

	type groupInfo struct {
		name    string
		indices []int // flat indices into allMods
	}
	var groups []groupInfo
	flatIdx := 1
	optionCount := 1
	for _, cat := range catOrder {
		mods, ok := grouped[cat]
		if !ok {
			continue
		}
		sort.Slice(mods, func(i, j int) bool {
			return mods[i].Name < mods[j].Name
		})
		var indices []int
		for range mods {
			indices = append(indices, flatIdx)
			flatIdx++
		}
		options := groupInfo{name: cat, indices: indices}
		groups = append(groups, options)
		optionCount++
	}

	// Print preset options
	fmt.Fprintf(w, "\n  Presets:\n")
	fmt.Fprintf(w, "    %d. Everything (%d modules)\n", 1, len(available))
	optionNum := 2
	presetMap := make(map[int]string)
	for _, g := range groups {
		fmt.Fprintf(w, "    %d. %s (%d modules)\n", optionNum, g.name, len(g.indices))
		presetMap[optionNum] = g.name
		optionNum++
	}
	customNum := optionNum
	fmt.Fprintf(w, "    %d. Custom selection\n", customNum)

	fmt.Fprintf(w, "\n  Select mode: ")

	var modeInput string
	fmt.Scanln(&modeInput)
	modeNum, err := strconv.Atoi(strings.TrimSpace(modeInput))
	if err != nil {
		modeNum = 1
	}

	if modeNum == 1 {
		return available
	}

	if preset, ok := presetMap[modeNum]; ok {
		return filterByPreset(available, preset)
	}

	// Custom selection — show all modules with checkboxes
	fmt.Fprintf(w, "\n  Toggle modules (comma-separated numbers, e.g. 1,3,5-8):\n")
	modNum := 1
	modIndex := make(map[int]string)
	for _, g := range groups {
		fmt.Fprintf(w, "\n  %s%s%s\n", output.ColorBold+output.ColorBlue, g.name, output.ColorReset)
		for _, mods := range grouped[g.name] {
			sizeStr := ""
			if mods.Size > 0 {
				sizeStr = "  " + formatBytes(mods.Size)
			}
			fmt.Fprintf(w, "    %2d. [ ] %s%s\n", modNum, mods.Name, sizeStr)
			modIndex[modNum] = mods.Name
			modNum++
		}
	}

	fmt.Fprintf(w, "\n  Enter selection: ")
	var selInput string
	fmt.Scanln(&selInput)

	return parseSelection(selInput, modIndex)
}

func parseSelection(input string, modIndex map[int]string) []string {
	selected := make(map[string]bool)
	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			rng := strings.SplitN(part, "-", 2)
			start, err1 := strconv.Atoi(strings.TrimSpace(rng[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(rng[1]))
			if err1 == nil && err2 == nil && start <= end {
				for i := start; i <= end; i++ {
					if name, ok := modIndex[i]; ok {
						selected[name] = true
					}
				}
			}
		} else {
			num, err := strconv.Atoi(part)
			if err == nil {
				if name, ok := modIndex[num]; ok {
					selected[name] = true
				}
			}
		}
	}
	var result []string
	for _, name := range modIndex {
		if selected[name] {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

func filterByPreset(available []string, preset string) []string {
	preset = strings.ToLower(preset)

	if preset == "everything" {
		return available
	}

	selected := make(map[string]bool)

	// Preset maps category name -> modules
	for _, name := range available {
		cat := moduleGroups[name]
		if cat == "" {
			cat = "Other"
		}
		catLower := strings.ToLower(cat)

		switch preset {
		case strings.ToLower(cat):
			selected[name] = true
		case "dev", "development":
			if catLower == "development" || catLower == "editors" {
				selected[name] = true
			}
		case "browsers":
			if catLower == "browsers" {
				selected[name] = true
			}
		case "secrets", "identity":
			if catLower == "identity" || catLower == "security" {
				selected[name] = true
			}
		case "infra", "infrastructure":
			if catLower == "containers" || catLower == "cloud" || catLower == "infrastructure" {
				selected[name] = true
			}
		}
	}

	var result []string
	for _, name := range available {
		if selected[name] {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}
