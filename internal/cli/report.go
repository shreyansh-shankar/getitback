package cli

import (
	"os"
	"path/filepath"

	"github.com/shreyansh-shankar/getitback/internal/assessment"
	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/report"
	"github.com/shreyansh-shankar/getitback/internal/storage"
	"github.com/spf13/cobra"
)

func newReportCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Generate a complete machine audit report",
		Long: `Generate a professional, human-readable audit report of your development workstation.

The report includes machine profile, development stack, browsers, databases,
containers, cloud accounts, backup summary, and asset statistics.

Useful for:
  - Archiving before reinstalling an OS
  - Sharing with teammates
  - Attaching to GitHub issues
  - Comparing across machines`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			hostname, _ := os.Hostname()
			cmd.Printf("  Generating report for %s...\n", hostname)
			results := manager.Inventory(ctx)

			backups, err := storage.ListBackups(cfg.Storage.Path)
			if err != nil {
				backups = nil
			}

			backedUp := make(map[string]bool)
			if len(backups) > 0 {
				backupDir := filepath.Join(cfg.Storage.Path, backups[0].ID)
				manifest, err := storage.ReadManifest(backupDir)
				if err == nil {
					for _, snap := range manifest.Snapshots {
						backedUp[snap.Module] = true
					}
				}
			}

			coverage := assessment.ComputeCoverage(results)
			score := assessment.ComputeScore(results, coverage, moduleGroups)

			r := report.NewReport(results, backups, cfg.Encryption.Enabled, backedUp, &score)

			format := outputFormat(cmd)
			renderer := output.NewRenderer(format)

			return renderer.RenderReport(os.Stdout, r)
		},
	}
}
