package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/crypto"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/storage"
	"github.com/spf13/cobra"
)

func newRestoreCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore from an existing backup",
		Long:  "Restore your development environment from a previous backup.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			backupID, _ := cmd.Flags().GetString("id")

			if backupID == "" {
				latest, err := storage.LatestBackup(cfg.Storage.Path)
				if err != nil {
					return fmt.Errorf("list backups: %w", err)
				}
				if latest == "" {
					return fmt.Errorf("no backups found in %s", cfg.Storage.Path)
				}
				backupID = latest
			}

			backupDir := filepath.Join(cfg.Storage.Path, backupID)

			manifest, err := storage.ReadManifest(backupDir)
			if err != nil {
				return fmt.Errorf("read manifest: %w", err)
			}

			snapshotsDir := filepath.Join(backupDir, "snapshots")

			cmd.Print(output.SectionHeader("Restore"))

			cmd.Printf("  %sID:%s       %s\n", output.ColorBlue, output.ColorReset, backupID)
			cmd.Printf("  %sCreated:%s   %s\n", output.ColorBlue, output.ColorReset, manifest.CreatedAt.Format("Jan 2, 2006 15:04 UTC"))
			cmd.Printf("  %sHost:%s     %s\n\n", output.ColorBlue, output.ColorReset, manifest.Hostname)

			successCount := 0
			skipCount := 0
			failCount := 0

			for _, snap := range manifest.Snapshots {
				mod, ok := manager.Get(snap.Module)
				if !ok {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s▸%s %s  %s⚠ module not found%s\n",
						output.ColorCyan, output.ColorReset, snap.Module,
						output.ColorYellow, output.ColorReset)
					skipCount++
					continue
				}

				fmt.Fprintf(cmd.OutOrStdout(), "  %s▸%s %s...",
					output.ColorCyan, output.ColorReset, snap.Module)

				restorePath := snap.Path
				cleanup := false

				if snap.Encrypted {
					decryptedPath := snap.Path + ".decrypted"
					if err := decryptSnapshot(snap.Path, decryptedPath, cfg); err != nil {
						fmt.Fprintf(cmd.OutOrStdout(), " %s✗%s decrypt failed: %v\n",
							output.ColorRed, output.ColorReset, err)
						failCount++
						continue
					}
					restorePath = decryptedPath
					cleanup = true
				}

				opts := module.RestoreOptions{
					SnapshotsDir: snapshotsDir,
				}

				if err := mod.Restore(ctx, snap, opts); err != nil {
					if cleanup {
						os.Remove(restorePath)
					}
					fmt.Fprintf(cmd.OutOrStdout(), " %s✗%s %v\n",
						output.ColorRed, output.ColorReset, err)
					failCount++
					continue
				}

				if cleanup {
					os.Remove(restorePath)
				}

				fmt.Fprintf(cmd.OutOrStdout(), " %s✓%s\n",
					output.ColorGreen, output.ColorReset)
				successCount++
			}

			cmd.Println()
			cmd.Printf("  %sRestore complete:%s %d restored, %d skipped, %d failed\n",
				output.ColorBold+output.ColorGreen, output.ColorReset,
				successCount, skipCount, failCount)

			if failCount > 0 {
				return fmt.Errorf("%d modules failed to restore", failCount)
			}
			return nil
		},
	}

	cmd.Flags().String("id", "", "Backup ID to restore from (default: latest)")
	return cmd
}

func decryptSnapshot(snapPath, destPath string, cfg *config.Config) error {
	keyData, err := os.ReadFile(cfg.Encryption.KeyPath)
	if err != nil {
		return fmt.Errorf("read encryption key: %w", err)
	}
	return crypto.DecryptFile(snapPath, destPath, string(keyData))
}
