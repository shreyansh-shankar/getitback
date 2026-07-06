package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/storage"
	"github.com/spf13/cobra"
)

func newStatusCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current backup coverage",
		Long:  "Display the current backup coverage status across all modules.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			detected := manager.Detect(ctx)

			backups, err := storage.ListBackups(cfg.Storage.Path)
			if err != nil {
				return fmt.Errorf("list backups: %w", err)
			}

			cmd.Print(output.SectionHeader("Backup Status"))

			coverage := make(map[string]bool)
			if len(backups) > 0 {
				backupDir := filepath.Join(cfg.Storage.Path, backups[0].ID)
				manifest, err := storage.ReadManifest(backupDir)
				if err == nil {
					for _, snap := range manifest.Snapshots {
						coverage[snap.Module] = true
					}
				}
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

			fmt.Fprintf(tw, "  %sModule\t%sDetected\t%sBacked Up%s\n",
				output.ColorBold+output.ColorBlue, output.ColorBold+output.ColorBlue, output.ColorBold+output.ColorBlue, output.ColorReset)
			fmt.Fprintf(tw, "  %s------\t-------\t---------%s\n",
				output.ColorCyan, output.ColorReset)

			grouped := make(map[string][]module.Module)
			groupOrder := []string{}

			modules := manager.All()
			sort.Slice(modules, func(i, j int) bool {
				return modules[i].Name() < modules[j].Name()
			})

			for _, mod := range modules {
				g := moduleGroups[mod.Name()]
				if g == "" {
					g = "Other"
				}
				if _, ok := grouped[g]; !ok {
					groupOrder = append(groupOrder, g)
				}
				grouped[g] = append(grouped[g], mod)
			}
			sort.Strings(groupOrder)

			coveredCount := 0
			detectedCount := 0

			for _, g := range groupOrder {
				mods := grouped[g]
				for _, mod := range mods {
					name := mod.Name()
					d := detected[name]

					detectedStr := fmt.Sprintf("%sno%s", output.ColorRed, output.ColorReset)
					if d.Detected {
						detectedStr = fmt.Sprintf("%syes%s", output.ColorGreen, output.ColorReset)
						detectedCount++
					}

					backedStr := fmt.Sprintf("%sno%s", output.ColorRed, output.ColorReset)
					if coverage[name] {
						backedStr = fmt.Sprintf("%syes%s", output.ColorGreen, output.ColorReset)
						coveredCount++
					}

					fmt.Fprintf(tw, "  %s\t%s\t%s\n", name, detectedStr, backedStr)
				}
			}

			tw.Flush()
			cmd.Println()

			cmd.Printf("  %sModules:%s %d total, %s%d%s detected, %s%d%s backed up\n",
				output.ColorBold, output.ColorReset,
				len(modules),
				output.ColorGreen, detectedCount, output.ColorReset,
				output.ColorGreen, coveredCount, output.ColorReset)

			if len(backups) > 0 {
				cmd.Printf("  %sLatest backup:%s %s (%s, %s)\n",
					output.ColorBold, output.ColorReset,
					backups[0].ID,
					backups[0].CreatedAt.Format("Jan 2, 2006 15:04"),
					formatBytes(backups[0].Size))
			} else {
				cmd.Printf("  %sNo backups found.%s Run: getitback backup\n",
					output.ColorYellow, output.ColorReset)
			}

			return nil
		},
	}
}
