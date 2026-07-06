package cli

import (
	"os"

	"github.com/shreyansh-shankar/getitback/internal/assessment"
	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/spf13/cobra"
)

func newInventoryCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inventory",
		Short: "Discover everything on the machine",
		Long:  "Scan the machine and discover all installed tools, configurations, and resources.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			verbose, _ := cmd.Flags().GetBool("verbose")

			if !verbose {
				cmd.Println("  Scanning machine...")
			}
			results := manager.Inventory(ctx)

			format := outputFormat(cmd)
			renderer := output.NewRenderer(format)

			coverage := assessment.ComputeCoverage(results)
			score := assessment.ComputeScore(results, coverage, moduleGroups)
			grade := assessment.ScoreGrade(score.Total)
			recs := assessment.GenerateRecommendations(results, coverage, moduleGroups)

			opts := output.RenderOptions{
				Verbose:    verbose,
				Categories: moduleGroups,
				Coverage:   coverage,
				Score:      &score,
				Grade:      grade,
				Recs:       recs,
			}

			return renderer.RenderInventory(os.Stdout, results, opts)
		},
	}

	cmd.Flags().BoolP("verbose", "v", false, "Show detailed metadata and secret filenames")
	return cmd
}
