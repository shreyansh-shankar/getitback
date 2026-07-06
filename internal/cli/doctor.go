package cli

import (
	"os"
	"path/filepath"

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
		Short: "Identify missing backups, tools, and potential issues",
		Long:  "Scan the machine and backup storage to identify gaps and potential recovery issues.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			issues := 0

			cmd.Print(output.SectionHeader("Doctor"))

			detected := manager.Detect(ctx)
			doctorResults := manager.Doctor(ctx)

			var detectedModules []string
			for name, d := range detected {
				if d.Detected {
					detectedModules = append(detectedModules, name)
				}
			}

			// Per-module diagnostics
			hasModuleIssues := false
			for _, name := range detectedModules {
				result, ok := doctorResults[name]
				if !ok || result == nil || len(result.Issues) == 0 {
					continue
				}
				if !hasModuleIssues {
					cmd.Printf("  %sModule Diagnostics%s\n\n",
						output.ColorBold+output.ColorYellow, output.ColorReset)
					hasModuleIssues = true
				}
				for _, issue := range result.Issues {
					color := output.ColorYellow
					icon := string(output.IconWarn)
					if issue.Severity == "error" {
						color = output.ColorRed
						icon = string(output.IconCross)
					}
					cmd.Printf("    %s%s %s%s: %s\n",
						color, icon, name, output.ColorReset, issue.Message)
					if issue.Help != "" {
						cmd.Printf("      %s▸%s %s\n", output.ColorCyan, output.ColorReset, issue.Help)
					}
					issues++
				}
			}

			if !hasModuleIssues && len(detectedModules) > 0 {
				cmd.Printf("  %s✓%s All modules healthy\n", output.ColorGreen, output.ColorReset)
			}

			// Check for backups
			cmd.Println()
			cmd.Printf("  %sBackup Status%s\n\n",
				output.ColorBold+output.ColorBlue, output.ColorReset)

			backups, err := storage.ListBackups(cfg.Storage.Path)
			if err != nil {
				cmd.Printf("    %s✗%s cannot access backup storage: %v\n",
					output.ColorRed, output.ColorReset, err)
				issues++
			}

			if len(backups) == 0 {
				cmd.Printf("    %s✗%s No backups found\n",
					output.ColorYellow, output.ColorReset)
				cmd.Printf("      %s▸%s Run: getitback backup\n",
					output.ColorCyan, output.ColorReset)
				issues++
			} else {
				latest := backups[0]
				cmd.Printf("    %s✓%s Latest: %s (%s, %d snapshots, %s)\n",
					output.ColorGreen, output.ColorReset,
					latest.ID,
					latest.CreatedAt.Format("Jan 2, 2006 15:04 UTC"),
					latest.SnapshotCount,
					formatBytes(latest.Size))

				backupDir := filepath.Join(cfg.Storage.Path, latest.ID)
				manifest, err := storage.ReadManifest(backupDir)
				if err == nil {
					backedModules := make(map[string]bool)
					for _, snap := range manifest.Snapshots {
						backedModules[snap.Module] = true
					}

					for _, name := range detectedModules {
						if !backedModules[name] {
							cmd.Printf("    %s⚠%s Gap: %s installed but not backed up\n",
								output.ColorYellow, output.ColorReset, name)
							issues++
						}
					}

					for mod := range backedModules {
						if d, ok := detected[mod]; !ok || !d.Detected {
							cmd.Printf("    %sℹ%s %s was backed up but no longer detected\n",
								output.ColorBlue, output.ColorReset, mod)
						}
					}
				}
			}

			// Check encryption
			cmd.Println()
			cmd.Printf("  %sEncryption%s\n\n",
				output.ColorBold+output.ColorBlue, output.ColorReset)

			if cfg.Encryption.Enabled {
				cmd.Printf("    %s✓%s Enabled\n", output.ColorGreen, output.ColorReset)
				if pathExists(cfg.Encryption.KeyPath) {
					cmd.Printf("    %s✓%s Key: %s\n",
						output.ColorGreen, output.ColorReset, cfg.Encryption.KeyPath)
				} else {
					cmd.Printf("    %s✗%s Key missing: %s\n",
						output.ColorRed, output.ColorReset, cfg.Encryption.KeyPath)
					issues++
				}
			} else {
				cmd.Printf("    %sℹ%s Disabled\n", output.ColorBlue, output.ColorReset)
				cmd.Printf("      %s▸%s Enable with: encryption.enabled: true in ~/.getitback/config.yaml\n",
					output.ColorCyan, output.ColorReset)
			}

			// Recommendations
			cmd.Println()
			cmd.Printf("  %sRecommendations%s\n\n",
				output.ColorBold+output.ColorBlue, output.ColorReset)

			results := manager.Inventory(ctx)
			coverage := assessment.ComputeCoverage(results)
			recs := assessment.GenerateRecommendations(results, coverage, moduleGroups)

			if len(recs) == 0 {
				cmd.Printf("    %s✓%s No recommendations — everything looks good\n",
					output.ColorGreen, output.ColorReset)
			} else {
				for _, r := range recs {
					pColor := priorityColor(r.Priority)
					cmd.Printf("    %s•%s [%s%s%s] %s\n",
						output.ColorCyan, output.ColorReset,
						pColor, r.Priority, output.ColorReset,
						r.Message)
					if r.Help != "" {
						cmd.Printf("      %s▸%s %s\n", output.ColorCyan, output.ColorReset, r.Help)
					}
				}
			}

			// Summary
			cmd.Println()
			if issues == 0 {
				cmd.Printf("  %s✓%s %sAll checks passed%s\n\n",
					output.ColorGreen, output.ColorReset,
					output.ColorBold, output.ColorReset)
			} else {
				cmd.Printf("  %s%s%d issue(s) found%s\n\n",
					output.ColorBold, output.ColorYellow, issues, output.ColorReset)
			}

			return nil
		},
	}
}

func pathExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
