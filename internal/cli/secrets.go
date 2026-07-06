package cli

import (
	"fmt"
	"sort"

	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/spf13/cobra"
)

func newSecretsCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "List all detected secret files",
		Long:  "Show all files classified as secrets across all modules (filenames are hidden in default inventory).",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			results := manager.Inventory(ctx)

			cmd.Print(output.SectionHeader("Secret Files"))

			var count int
			var items []struct {
				module string
				path   string
				size   int64
			}

			for _, res := range results {
				if !res.Detected {
					continue
				}
				for _, r := range res.Resources {
					if r.Type == module.ResourceTypeSecret {
						count++
						items = append(items, struct {
							module string
							path   string
							size   int64
						}{module: res.Module, path: r.Path, size: r.Size})
					}
				}
			}

			if count == 0 {
				cmd.Printf("  %s✓%s No secret files detected\n",
					output.ColorGreen, output.ColorReset)
				return nil
			}

			sort.Slice(items, func(i, j int) bool {
				return items[i].path < items[j].path
			})

			for _, it := range items {
				size := ""
				if it.size > 0 {
					size = fmt.Sprintf("  (%s)", formatBytes(it.size))
				}
				cmd.Printf("  %s•%s [%s] %s%s\n",
					output.ColorRed, output.ColorReset,
					output.ColorYellow+it.module+output.ColorReset,
					it.path, size)
			}

			cmd.Println()
			cmd.Printf("  %s%d%s secret file(s) found\n",
				output.ColorRed, count, output.ColorReset)
			cmd.Printf("  %s⚠%s Review each file and ensure it is encrypted or removed from plaintext storage\n",
				output.ColorYellow, output.ColorReset)

			return nil
		},
	}

	return cmd
}
