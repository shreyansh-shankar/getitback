package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/crypto"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/storage"
	"github.com/spf13/cobra"
)

func newBackupCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Create a complete developer backup",
		Long:  "Backup all discovered configurations, secrets, and data from your machine.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			backupID := time.Now().UTC().Format("20060102T150405Z")
			backupDir := filepath.Join(cfg.Storage.Path, backupID)
			snapshotsDir := filepath.Join(backupDir, "snapshots")

			if err := os.MkdirAll(snapshotsDir, 0700); err != nil {
				return fmt.Errorf("create backup dir: %w", err)
			}

			cmd.Print(output.SectionHeader("Backup"))
			cmd.Printf("  %sID:%s  %s\n", output.ColorBlue, output.ColorReset, backupID)

			cmd.Println("\n  Running inventory...")
			inventory := manager.Inventory(ctx)

			invPath := filepath.Join(backupDir, "inventory.json")
			invFile, err := os.Create(invPath)
			if err != nil {
				return fmt.Errorf("create inventory file: %w", err)
			}
			enc := json.NewEncoder(invFile)
			enc.SetIndent("", "  ")
			enc.Encode(inventory)
			invFile.Close()

			cmd.Printf("  %sBacking up modules...%s\n", output.ColorBold, output.ColorReset)

			var allSnapshots []module.Snapshot
			backedCount := 0
			skipCount := 0

			for _, mod := range manager.All() {
				ok, err := mod.Detect()
				if err != nil || !ok {
					skipCount++
					continue
				}

				fmt.Fprintf(cmd.OutOrStdout(), "    %s▸%s %s...",
					output.ColorCyan, output.ColorReset, mod.Name())

				opts := module.BackupOptions{
					SnapshotsDir: snapshotsDir,
					Encrypt:      cfg.Encryption.Enabled,
					KeyPath:      cfg.Encryption.KeyPath,
				}
				result, err := mod.Backup(ctx, opts)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), " %s✗%s\n", output.ColorRed, output.ColorReset)
					continue
				}
				if result == nil || len(result.Snapshots) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), " %s-\n", output.ColorCyan)
					skipCount++
					continue
				}
				allSnapshots = append(allSnapshots, result.Snapshots...)
				backedCount++
				fmt.Fprintf(cmd.OutOrStdout(), " %s✓%s\n", output.ColorGreen, output.ColorReset)
			}

			hostname, _ := os.Hostname()

			if cfg.Encryption.Enabled && len(allSnapshots) > 0 {
				cmd.Printf("\n  %sEncrypting snapshots...%s\n", output.ColorBold, output.ColorReset)

				keyData, err := os.ReadFile(cfg.Encryption.KeyPath)
				if err != nil {
					return fmt.Errorf("read encryption key: %w", err)
				}

				identity := string(keyData)

				for i, snap := range allSnapshots {
					if snap.Encrypted {
						continue
					}
					encryptedPath := snap.Path + ".age"

					recipientData, err := recipientFromIdentity(identity)
					if err != nil {
						cmd.Printf("    %s⚠%s Warning: failed to parse key for %s: %v\n",
							output.ColorYellow, output.ColorReset, snap.Module, err)
						continue
					}

					if err := crypto.EncryptFile(snap.Path, encryptedPath, recipientData); err != nil {
						cmd.Printf("    %s⚠%s Warning: encryption failed for %s: %v\n",
							output.ColorYellow, output.ColorReset, snap.Module, err)
						continue
					}

					os.Remove(snap.Path)
					rawPath := snap.Path
					allSnapshots[i].Path = encryptedPath
					allSnapshots[i].Encrypted = true

					checksum, _ := fileChecksum(encryptedPath)
					allSnapshots[i].Checksum = checksum

					info, _ := os.Stat(encryptedPath)
					if info != nil {
						allSnapshots[i].Size = info.Size()
					}

					os.Remove(rawPath)
				}
			}

			manifest := storage.Manifest{
				Version:   storage.ManifestVersion,
				CreatedAt: time.Now(),
				Hostname:  hostname,
				OS:        fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH),
				Snapshots: allSnapshots,
				Inventory: inventory,
			}

			if err := storage.WriteManifest(backupDir, &manifest); err != nil {
				return fmt.Errorf("write manifest: %w", err)
			}

			var totalSize int64
			for _, snap := range allSnapshots {
				totalSize += snap.Size
			}

			cmd.Println()
			cmd.Printf("  %s✓%s %sBackup complete%s\n",
				output.ColorGreen, output.ColorReset,
				output.ColorBold, output.ColorReset)
			cmd.Printf("    %sID:%s        %s\n", output.ColorBlue, output.ColorReset, backupID)
			cmd.Printf("    %sLocation:%s  %s\n", output.ColorBlue, output.ColorReset, backupDir)
			cmd.Printf("    %sSnapshots:%s %d\n", output.ColorBlue, output.ColorReset, backedCount)
			cmd.Printf("    %sSize:%s      %s\n", output.ColorBlue, output.ColorReset, formatBytes(totalSize))
			cmd.Printf("    %sEncrypted:%s %v\n", output.ColorBlue, output.ColorReset, cfg.Encryption.Enabled)

			return nil
		},
	}
}

func recipientFromIdentity(identity string) (string, error) {
	id, err := crypto.ParseIdentity(identity)
	if err != nil {
		return "", err
	}
	return id.Recipient().String(), nil
}
