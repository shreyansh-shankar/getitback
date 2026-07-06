package cli

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/storage"
	"github.com/spf13/cobra"
)

func newVerifyCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify backup integrity",
		Long:  "Verify the integrity of all snapshots in a backup.",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			cmd.Print(output.SectionHeader("Verify Backup"))

			cmd.Printf("  %sID:%s       %s\n",
				output.ColorBlue, output.ColorReset, backupID)
			cmd.Printf("  %sCreated:%s   %s\n",
				output.ColorBlue, output.ColorReset, manifest.CreatedAt.Format("Jan 2, 2006 15:04 UTC"))
			cmd.Printf("  %sHost:%s     %s\n\n",
				output.ColorBlue, output.ColorReset, manifest.Hostname)

			valid := 0
			invalid := 0

			for _, snap := range manifest.Snapshots {
				state, err := verifySnapshot(snap, cfg)
				if err != nil {
					cmd.Printf("  %s✗%s %s  %s\n",
						output.ColorRed, output.ColorReset,
						snap.Module,
						output.ColorRed+err.Error()+output.ColorReset)
					invalid++
					continue
				}

				checksumStr := ""
				if snap.Checksum != "" {
					if state.checksumMatch {
						checksumStr = fmt.Sprintf("  %s✓%s", output.ColorGreen, output.ColorReset)
					} else {
						checksumStr = fmt.Sprintf("  %s✗%s checksum mismatch",
							output.ColorRed, output.ColorReset)
						invalid++
						continue
					}
				}

				meta := formatBytes(state.size)
				if snap.Encrypted {
					meta += " encrypted"
				}

				cmd.Printf("  %s✓%s %s%s%s  (%s)%s\n",
					output.ColorGreen, output.ColorReset,
					output.ColorBold, snap.Module, output.ColorReset,
					meta, checksumStr)
				valid++
			}

			cmd.Println()
			if invalid == 0 {
				cmd.Printf("  %s✓%s %sAll %d snapshots verified successfully%s\n",
					output.ColorGreen, output.ColorReset,
					output.ColorBold, valid, output.ColorReset)
			} else {
				cmd.Printf("  %s✗%s %s%d valid, %d invalid%s\n",
					output.ColorRed, output.ColorReset,
					output.ColorBold, valid, invalid, output.ColorReset)
				return fmt.Errorf("%d snapshots failed verification", invalid)
			}
			return nil
		},
	}

	cmd.Flags().String("id", "", "Backup ID to verify (default: latest)")
	return cmd
}

type snapState struct {
	exists         bool
	valid          bool
	size           int64
	checksumMatch  bool
	actualChecksum string
}

func verifySnapshot(snap module.Snapshot, cfg *config.Config) (snapState, error) {
	var state snapState

	path := snap.Path
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return state, fmt.Errorf("snapshot file not found: %s", path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return state, fmt.Errorf("stat: %w", err)
	}
	state.exists = true
	state.size = info.Size()

	if snap.Checksum != "" {
		checksum, err := computeChecksum(path)
		if err != nil {
			return state, fmt.Errorf("checksum: %w", err)
		}
		state.actualChecksum = checksum
		state.checksumMatch = checksum == snap.Checksum
	}

	state.valid = true
	return state, nil
}

func computeChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
