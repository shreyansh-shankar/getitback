package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/spf13/cobra"
)

var modulesCategoryOrder = []string{
	"Identity",
	"Configuration",
	"Development",
	"Editors",
	"Browsers",
	"Containers",
	"Databases",
	"Cloud",
	"Infrastructure",
	"Projects",
	"Security",
	"Virtualization",
	"Packages",
	"System",
}

func newModulesCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "modules [module]",
		Short: "List available modules and their capabilities",
		Long: `List all modules supported by GetItBack with detection status and capabilities.

Use "getitback modules <name>" to inspect a specific module in detail.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if len(args) == 1 {
				return printModuleDetail(cmd, manager, ctx, args[0])
			}

			detected := manager.Detect(ctx)

			w := os.Stdout
			printModulesHeader(w)
			printModulesOverview(w, manager, detected)
			fmt.Fprintln(w)
			printModuleListing(w, manager, detected)
			fmt.Fprintln(w)
			printModulesFooter(w, manager, detected)

			return nil
		},
	}

	return cmd
}

func printModulesHeader(w *os.File) {
	fmt.Fprintf(w, "\n  %s%s%s\n", output.ColorBold+output.ColorCyan, strings.Repeat("━", 50), output.ColorReset)
	fmt.Fprintf(w, "  %s  Modules%s\n", output.ColorBold+output.ColorCyan, output.ColorReset)
	fmt.Fprintf(w, "  %s%s%s\n", output.ColorBold+output.ColorCyan, strings.Repeat("━", 50), output.ColorReset)
}

func printModulesOverview(w *os.File, manager *module.Manager, detected map[string]module.DetectResult) {
	allMods := manager.All()
	total := len(allMods)
	detectedCount := 0
	cats := make(map[string]bool)

	for _, mod := range allMods {
		if detected[mod.Name()].Detected {
			detectedCount++
		}
		info := module.GetModuleInfo(mod.Name())
		if info != nil {
			cats[info.Category] = true
		}
	}

	catCount := len(cats)
	pct := 0
	if total > 0 {
		pct = detectedCount * 100 / total
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %sOverview%s\n", output.ColorBold, output.ColorReset)
	fmt.Fprintln(w)

	pad := 18
	fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Supported Modules", fmt.Sprintf("%d", total), pad)+output.ColorReset)
	fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Detected Modules", output.ColorGreen+fmt.Sprintf("%d", detectedCount)+output.ColorReset, pad)+output.ColorReset)
	fmt.Fprintf(w, "    %s\n", output.ColorCyan+output.DotLeader("Categories", fmt.Sprintf("%d", catCount), pad)+output.ColorReset)

	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %sDetection Coverage%s\n", output.ColorBold, output.ColorReset)
	fmt.Fprintln(w)
	bar := detectionBar(pct)
	fmt.Fprintf(w, "    %s\n", output.ColorCyan+bar+output.ColorReset)
	pctColor := output.ColorGreen
	if pct < 40 {
		pctColor = output.ColorRed
	} else if pct < 70 {
		pctColor = output.ColorYellow
	}
	fmt.Fprintf(w, "    %s%d%%%s\n", pctColor, pct, output.ColorReset)
}

func detectionBar(pct int) string {
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

func printModuleListing(w *os.File, manager *module.Manager, detected map[string]module.DetectResult) {
	grouped := make(map[string][]module.Module)
	for _, mod := range manager.All() {
		info := module.GetModuleInfo(mod.Name())
		cat := "Other"
		if info != nil {
			cat = info.Category
		}
		grouped[cat] = append(grouped[cat], mod)
	}

	for _, cat := range modulesCategoryOrder {
		mods, ok := grouped[cat]
		if !ok || len(mods) == 0 {
			continue
		}

		sort.Slice(mods, func(i, j int) bool {
			return mods[i].Name() < mods[j].Name()
		})

		fmt.Fprintf(w, "    %s%s%s\n", output.ColorBold+output.ColorBlue, cat, output.ColorReset)

		for _, mod := range mods {
			det := detected[mod.Name()]
			info := module.GetModuleInfo(mod.Name())

			if det.Detected {
				fmt.Fprintf(w, "      %s✓%s %s%s%s\n",
					output.ColorGreen, output.ColorReset,
					output.ColorBold, mod.Name(), output.ColorReset)
				fmt.Fprintf(w, "        %sDescription:%s %s\n", output.ColorCyan, output.ColorReset, mod.Description())

				caps := infoCapabilities(info)
				fmt.Fprintf(w, "        %sCapabilities:%s %s\n", output.ColorCyan, output.ColorReset, caps)

				maturity := "Stable"
				if info != nil && info.Maturity != "" {
					maturity = string(info.Maturity)
				}
				fmt.Fprintf(w, "        %sMaturity:%s    %s\n", output.ColorCyan, output.ColorReset, maturity)
			} else {
				fmt.Fprintf(w, "      %s○%s %s%s%s\n",
					output.ColorCyan, output.ColorReset,
					output.ColorBold, mod.Name(), output.ColorReset)
				fmt.Fprintf(w, "        %sDescription:%s %s\n", output.ColorCyan, output.ColorReset, mod.Description())
				fmt.Fprintf(w, "        %sStatus:%s      %sNot Installed%s\n", output.ColorCyan, output.ColorReset, output.ColorYellow, output.ColorReset)
			}
		}
		fmt.Fprintln(w)
	}
}

func infoCapabilities(info *module.ModuleInfo) string {
	if info == nil || len(info.Capabilities) == 0 {
		return strings.Join(capNames(module.DefaultCapabilities()), ", ")
	}
	return strings.Join(capNames(info.Capabilities), ", ")
}

func capNames(caps []module.Capability) []string {
	names := make([]string, len(caps))
	for i, c := range caps {
		names[i] = string(c)
	}
	return names
}

func printModulesFooter(w *os.File, manager *module.Manager, detected map[string]module.DetectResult) {
	total := len(manager.All())
	detectedCount := 0
	for _, d := range detected {
		if d.Detected {
			detectedCount++
		}
	}

	fmt.Fprintf(w, "    %sDetected Modules:%s %s%d%s\n", output.ColorCyan, output.ColorReset, output.ColorGreen, detectedCount, output.ColorReset)
	fmt.Fprintf(w, "    %sSupported Modules:%s %d\n", output.ColorCyan, output.ColorReset, total)
	fmt.Fprintln(w)
	fmt.Fprintf(w, "    %sRun:%s getitback modules <name> to inspect a module\n", output.ColorCyan, output.ColorReset)
}

func printModuleDetail(cmd *cobra.Command, manager *module.Manager, ctx context.Context, moduleName string) error {
	// Find the module
	var mod module.Module
	for _, m := range manager.All() {
		if m.Name() == moduleName {
			mod = m
			break
		}
	}
	if mod == nil {
		return fmt.Errorf("unknown module: %s", moduleName)
	}

	// Run detection
	detected := manager.Detect(ctx)
	det := detected[moduleName]
	info := module.GetModuleInfo(moduleName)

	w := os.Stdout
	displayName := moduleName
	if info != nil {
		displayName = info.Name
	}

	fmt.Fprintf(w, "\n  %s%s%s\n", output.ColorBold+output.ColorCyan, strings.Repeat("━", 50), output.ColorReset)
	fmt.Fprintf(w, "  %s  %s Module%s\n", output.ColorBold+output.ColorCyan, displayName, output.ColorReset)
	fmt.Fprintf(w, "  %s%s%s\n", output.ColorBold+output.ColorCyan, strings.Repeat("━", 50), output.ColorReset)
	fmt.Fprintln(w)

	// Description
	fmt.Fprintf(w, "    %sDescription%s\n", output.ColorBold, output.ColorReset)
	fmt.Fprintf(w, "    %s\n", mod.Description())
	fmt.Fprintln(w)

	// Status
	fmt.Fprintf(w, "    %sStatus%s\n", output.ColorBold, output.ColorReset)
	if det.Detected {
		fmt.Fprintf(w, "      %s✓%s %sDetected%s\n", output.ColorGreen, output.ColorReset, output.ColorGreen, output.ColorReset)
	} else {
		fmt.Fprintf(w, "      %s○%s %sNot Installed%s\n", output.ColorCyan, output.ColorReset, output.ColorYellow, output.ColorReset)
	}
	fmt.Fprintln(w)

	// Capabilities
	fmt.Fprintf(w, "    %sCapabilities%s\n", output.ColorBold, output.ColorReset)
	caps := module.DefaultCapabilities()
	if info != nil && len(info.Capabilities) > 0 {
		caps = info.Capabilities
	}
	for _, cap := range caps {
		fmt.Fprintf(w, "      %s✓%s %s\n", output.ColorGreen, output.ColorReset, cap)
	}
	fmt.Fprintln(w)

	// Platform
	fmt.Fprintf(w, "    %sPlatform%s\n", output.ColorBold, output.ColorReset)
	platforms := []string{"Linux", "macOS", "Windows"}
	if info != nil && len(info.Platforms) > 0 {
		for _, p := range platforms {
			supported := false
			for _, sp := range info.Platforms {
				if sp == p {
					supported = true
					break
				}
			}
			if supported {
				fmt.Fprintf(w, "      %s✓%s %s\n", output.ColorGreen, output.ColorReset, p)
			} else {
				fmt.Fprintf(w, "      %s✗%s %s\n", output.ColorRed, output.ColorReset, p)
			}
		}
	} else {
		fmt.Fprintf(w, "      %s✓%s Linux\n", output.ColorGreen, output.ColorReset)
	}
	fmt.Fprintln(w)

	// Data collected
	if info != nil && len(info.DataCollected) > 0 {
		fmt.Fprintf(w, "    %sData Collected%s\n", output.ColorBold, output.ColorReset)
		for _, item := range info.DataCollected {
			fmt.Fprintf(w, "      %s•%s %s\n", output.ColorCyan, output.ColorReset, item)
		}
		fmt.Fprintln(w)
	}

	// Dependencies
	if info != nil && len(info.Dependencies) > 0 {
		fmt.Fprintf(w, "    %sDependencies%s\n", output.ColorBold, output.ColorReset)
		for _, dep := range info.Dependencies {
			fmt.Fprintf(w, "      %s•%s %s\n", output.ColorCyan, output.ColorReset, dep)
		}
		fmt.Fprintln(w)
	}

	// Recovery Importance
	if info != nil && info.RecoveryValue != "" {
		fmt.Fprintf(w, "    %sRecovery Importance%s\n", output.ColorBold, output.ColorReset)
		impColor := output.ColorGreen
		switch info.RecoveryValue {
		case "Critical":
			impColor = output.ColorRed
		case "High":
			impColor = output.ColorYellow
		case "Medium":
			impColor = output.ColorCyan
		case "Low":
			impColor = output.ColorBlue
		}
		fmt.Fprintf(w, "      %s%s%s\n", impColor, info.RecoveryValue, output.ColorReset)
		fmt.Fprintln(w)
	}

	// Current machine data (if detected)
	if det.Detected {
		fmt.Fprintf(w, "    %sCurrent Machine%s\n", output.ColorBold, output.ColorReset)
		fmt.Fprintf(w, "      %sRun inventory to see current data for this module%s\n", output.ColorCyan, output.ColorReset)
		fmt.Fprintln(w)
	}

	// Commands
	fmt.Fprintf(w, "    %sCommands%s\n", output.ColorBold, output.ColorReset)
	fmt.Fprintf(w, "      %sgetitback backup --module %s%s\n", output.ColorGreen, moduleName, output.ColorReset)
	fmt.Fprintf(w, "      %sgetitback restore --module %s%s\n", output.ColorGreen, moduleName, output.ColorReset)

	return nil
}
