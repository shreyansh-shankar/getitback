package cli

import (
	"sort"

	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/spf13/cobra"
)

var moduleGroups = map[string]string{
	"system": "System",
	"git":    "Development", "golang": "Development",
	"nodejs": "Development", "python": "Development", "rust": "Development",
	"java": "Development", "docker": "Containers",
	"ssh": "Identity", "gpg": "Identity",
	"vscode": "Editors", "neovim": "Editors",
	"firefox": "Browsers", "chromium": "Browsers",
	"chrome": "Browsers", "brave": "Browsers", "vivaldi": "Browsers", "edge": "Browsers", "opera": "Browsers",
	"postgres": "Databases", "mongodb": "Databases", "redis": "Databases", "sqlite": "Databases",
	"mysql": "Databases",
	"shell": "Configuration", "dotfiles": "Configuration",
	"apt": "Packages", "snap": "Packages", "flatpak": "Packages",
	"cloud": "Cloud", "kubernetes": "Infrastructure",
	"virtualization": "Virtualization", "certs": "Security",
	"repos": "Projects",
}

func newModulesCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "modules",
		Short: "List available modules and their status",
		Long:  "List all registered modules and whether each one is detected on this machine.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			detected := manager.Detect(ctx)

			cmd.Print(output.SectionHeader("Modules"))

			grouped := make(map[string][]struct {
				name    string
				desc    string
				ok      bool
			})
			groupOrder := []string{}

			for _, mod := range manager.All() {
				status := detected[mod.Name()]
				g := moduleGroups[mod.Name()]
				if g == "" {
					g = "Other"
				}
				grouped[g] = append(grouped[g], struct {
					name string
					desc string
					ok   bool
				}{mod.Name(), mod.Description(), status.Detected})
			}

			for _, mods := range grouped {
				sort.Slice(mods, func(i, j int) bool { return mods[i].name < mods[j].name })
			}
			for g := range grouped {
				groupOrder = append(groupOrder, g)
			}
			sort.Strings(groupOrder)

			detectedCount := 0
			totalCount := len(manager.All())

			for _, g := range groupOrder {
				mods := grouped[g]
				cmd.Printf("  %s%s%s\n", output.ColorBold+output.ColorBlue, g, output.ColorReset)

				for _, m := range mods {
					if m.ok {
						detectedCount++
						cmd.Printf("    %s✓%s %s%s%s  %s\n",
							output.ColorGreen, output.ColorReset,
							output.ColorBold, m.name, output.ColorReset,
							m.desc)
					} else {
						cmd.Printf("    %s %s  %s\n", m.name, output.ColorCyan+"(not detected)"+output.ColorReset, m.desc)
					}
				}
				cmd.Println()
			}

			cmd.Printf("  %s%d%s modules total, %s%d%s detected\n",
				output.ColorBold, totalCount, output.ColorReset,
				output.ColorGreen, detectedCount, output.ColorReset)

			return nil
		},
	}
}
