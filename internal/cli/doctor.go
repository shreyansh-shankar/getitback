package cli

import (
	"os"
	"path/filepath"

	"github.com/shreyansh-shankar/getitback/internal/doctor"
	"github.com/shreyansh-shankar/getitback/internal/assessment"
	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/storage"
	"github.com/spf13/cobra"
)

func newDoctorCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Recovery assessment and action plan",
		Long:  "Evaluate workstation recoverability, identify risks, and generate a prioritized action plan.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			cmd.Println(output.SectionHeader("Doctor"))

			cmd.Println("  Analyzing workstation recoverability...")
			results := manager.Inventory(ctx)

			coverage := assessment.ComputeCoverage(results)
			score := assessment.ComputeScore(results, coverage, moduleGroups)

			backedUp := make(map[string]bool)
			backups, err := storage.ListBackups(cfg.Storage.Path)
			if err == nil && len(backups) > 0 {
				latest := backups[0]
				backupDir := filepath.Join(cfg.Storage.Path, latest.ID)
				manifest, err := storage.ReadManifest(backupDir)
				if err == nil {
					for _, snap := range manifest.Snapshots {
						backedUp[snap.Module] = true
					}
				}
			}

			report := doctor.NewReport(results, &score, backedUp, cfg.Encryption.Enabled)

			format := outputFormat(cmd)
			renderer := output.NewRenderer(format)

			return renderer.RenderDoctor(os.Stdout, report)
		},
	}
}
