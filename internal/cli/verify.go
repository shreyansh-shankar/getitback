package cli

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/archive"
	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/storage"
	"github.com/spf13/cobra"
)

type verifyState struct {
	backupID      string
	backupDir     string
	cfg           *config.Config
	manager       *module.Manager
	w             interface{ Write([]byte) (int, error) }
	start         time.Time
	manifest      *storage.Manifest
	quick         bool

	verifiedMods    int
	failedMods      int
	verifiedArchives int
	totalSize       int64
	totalOrigSize   int64
	totalFileCount  int
	sha256Match     int
	sha256Fail      int
	readableFail    int
	missingArchives  int
	fileCountMatch  int
	fileCountFail   int
	origSizeMatch   int
	origSizeFail    int

	level2Verified bool
	hasWarnings    bool
	warnings       []string

	failDetails     map[string][]string
	passDetails     map[string][]string

	hasSHA256SUMS    bool
	manifestsValid   bool
	inventoryValid   bool
	metadataValid    bool
	sumsMatch        bool
	sumsOrphans      int
	sumsMissing      int
}

type snapVerifyResult struct {
	module        string
	status        string
	icon          string
	iconColor     string
	size          int64
	origSize      int64
	fileCount     int
	checksumMatch bool
	readable      bool
	encrypted     bool
	fileCountOK   bool
	origSizeOK    bool
	details       []string
	checkLabels   []string
}

func newVerifyCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify backup integrity and recovery readiness",
		Long: `Verify the integrity of a backup.

Stages:
  1. Loading Snapshot      — locate backup, load manifest and inventory
  2. Validating Metadata   — validate manifest, inventory, and format
  3. Verifying Archives    — verify each module archive and checksum
  4. Verifying Integrity   — cross-check manifest vs actual files
  5. Verification Summary  — display results and recovery readiness`,
		RunE: func(cmd *cobra.Command, args []string) error {
			state := &verifyState{
				cfg:         cfg,
				manager:     manager,
				w:           cmd.OutOrStdout(),
				start:       time.Now(),
				quick:       cmd.Flag("quick").Changed,
				failDetails: make(map[string][]string),
				passDetails: make(map[string][]string),
			}

			backupID, _ := cmd.Flags().GetString("id")

			stageHeader(state.w, "1 / 5", "Loading Snapshot")

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
			state.backupID = backupID
			state.backupDir = filepath.Join(cfg.Storage.Path, backupID)

			manifest, err := storage.ReadManifest(state.backupDir)
			if err != nil {
				return fmt.Errorf("read manifest: %w", err)
			}
			state.manifest = manifest

			fmt.Fprintln(state.w)
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("Backup", state.backupID, 22))
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("Created", manifest.CreatedAt.Format("Jan 2, 2006 15:04 UTC"), 22))
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("Host", manifest.Hostname, 22))
			if manifest.BackupVersion != "" {
				fmt.Fprintf(state.w, "  %s%s\n",
					output.ColorCyan,
					output.DotLeader("Format", manifest.BackupVersion, 22))
			}
			if manifest.Compression != "" {
				fmt.Fprintf(state.w, "  %s%s\n",
					output.ColorCyan,
					output.DotLeader("Compression", manifest.Compression, 22))
			}
			if manifest.Encryption != "" && manifest.Encryption != "disabled" {
				fmt.Fprintf(state.w, "  %s%s\n",
					output.ColorCyan,
					output.DotLeader("Encryption", manifest.Encryption, 22))
			}
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("Modules", fmt.Sprintf("%d", len(manifest.Snapshots)), 22))
			fmt.Fprintf(state.w, "  %s%s%s\n",
				output.ColorCyan,
				output.DotLeader("Location", state.backupDir, 22),
				output.ColorReset)

			stageHeader(state.w, "2 / 5", "Validating Metadata")

			state.manifestsValid = manifest.Version != ""
			metaDir := filepath.Join(state.backupDir, "metadata")

			fmt.Fprintln(state.w)
			if state.manifestsValid {
				fmt.Fprintf(state.w, "  %s✓%s %s\n",
					output.ColorGreen, output.ColorReset,
					output.DotLeader("Manifest", "valid ("+manifest.Version+")", 22))
			} else {
				fmt.Fprintf(state.w, "  %s✗%s %s\n",
					output.ColorRed, output.ColorReset,
					output.DotLeader("Manifest", "invalid", 22))
				state.failDetails["metadata"] = append(state.failDetails["metadata"], "Manifest missing version field")
			}

			invPath := filepath.Join(state.backupDir, "inventory.json")
			if invData, err := os.ReadFile(invPath); err == nil && len(invData) > 0 {
				state.inventoryValid = true
				fmt.Fprintf(state.w, "  %s✓%s %s\n",
					output.ColorGreen, output.ColorReset,
					output.DotLeader("Inventory", fmt.Sprintf("valid (%d bytes)", len(invData)), 22))
			} else {
				fmt.Fprintf(state.w, "  %s○%s %s\n",
					output.ColorCyan, output.ColorReset,
					output.DotLeader("Inventory", "not found", 22))
			}

			if metaInfo, err := os.Stat(metaDir); err == nil && metaInfo.IsDir() {
				entries, _ := os.ReadDir(metaDir)
				metaCount := 0
				for _, e := range entries {
					if strings.HasSuffix(e.Name(), ".json") {
						metaCount++
					}
				}
				state.metadataValid = metaCount > 0
				if metaCount > 0 {
					fmt.Fprintf(state.w, "  %s✓%s %s\n",
						output.ColorGreen, output.ColorReset,
						output.DotLeader("Module metadata", fmt.Sprintf("%d files", metaCount), 22))
				}
			} else {
				fmt.Fprintf(state.w, "  %s○%s %s\n",
					output.ColorCyan, output.ColorReset,
					output.DotLeader("Module metadata", "not present", 22))
			}

			sumsPath := filepath.Join(state.backupDir, "SHA256SUMS")
			sumsMap := make(map[string]string)
			if sumsData, err := os.ReadFile(sumsPath); err == nil && len(sumsData) > 0 {
				state.hasSHA256SUMS = true
				lines := strings.Split(strings.TrimSpace(string(sumsData)), "\n")
				for _, line := range lines {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						sumsMap[parts[1]] = parts[0]
					}
				}
				fmt.Fprintf(state.w, "  %s✓%s %s\n",
					output.ColorGreen, output.ColorReset,
					output.DotLeader("SHA256SUMS", fmt.Sprintf("%d entries", len(lines)), 22))
			} else {
				fmt.Fprintf(state.w, "  %s○%s %s\n",
					output.ColorCyan, output.ColorReset,
					output.DotLeader("SHA256SUMS", "not present", 22))
			}

			// Cross-check manifest vs SHA256SUMS
			if state.hasSHA256SUMS && len(manifest.Snapshots) > 0 {
				sumsOrphans := 0
				sumsMissing := 0
				for _, snap := range manifest.Snapshots {
					if snap.ArchiveFile != "" {
						if _, ok := sumsMap[snap.ArchiveFile]; !ok {
							sumsMissing++
						}
					}
				}
				for archiveFile := range sumsMap {
					found := false
					for _, snap := range manifest.Snapshots {
						if snap.ArchiveFile == archiveFile {
							found = true
							break
						}
					}
					if !found {
						sumsOrphans++
					}
				}
				state.sumsOrphans = sumsOrphans
				state.sumsMissing = sumsMissing
				if sumsMissing == 0 && sumsOrphans == 0 {
					state.sumsMatch = true
					fmt.Fprintf(state.w, "  %s✓%s %s\n",
						output.ColorGreen, output.ColorReset,
						output.DotLeader("Manifest × SHA256SUMS", "consistent", 22))
				} else {
					fmt.Fprintf(state.w, "  %s⚠%s %s\n",
						output.ColorYellow, output.ColorReset,
						output.DotLeader("Manifest × SHA256SUMS", "inconsistencies", 22))
					if sumsMissing > 0 {
						fmt.Fprintf(state.w, "    %s%s%d snapshots missing from SHA256SUMS%s\n",
							output.ColorYellow, string(output.IconWarn), sumsMissing, output.ColorReset)
					}
					if sumsOrphans > 0 {
						fmt.Fprintf(state.w, "    %s%s%d orphaned SHA256SUMS entries%s\n",
							output.ColorYellow, string(output.IconWarn), sumsOrphans, output.ColorReset)
					}
				}
			}

			// Check format compat
			compat := true
			if manifest.BackupVersion != "" && manifest.BackupVersion != storage.BackupFormatVersion {
				if state.quick {
					fmt.Fprintf(state.w, "  %s%s%s %s\n",
						output.ColorYellow, string(output.IconWarn), output.ColorReset,
						output.DotLeader("Format", fmt.Sprintf("backup %s, current %s", manifest.BackupVersion, storage.BackupFormatVersion), 22))
				}
				if manifest.BackupVersion < storage.BackupFormatVersion {
					state.warnings = append(state.warnings, fmt.Sprintf("Backup format %s is older than current %s — upgrade recommended", manifest.BackupVersion, storage.BackupFormatVersion))
					state.hasWarnings = true
				}
			}
			_ = compat

			fmt.Fprint(state.w, output.ColorReset)

			stageHeader(state.w, "3 / 5", "Verifying Archives")

			fmt.Fprintf(state.w, "\n  %s✓%s Valid   %s✗%s Corrupt   %s○%s Missing\n",
				output.ColorGreen, output.ColorReset,
				output.ColorRed, output.ColorReset,
				output.ColorCyan, output.ColorReset)

			if state.quick {
				fmt.Fprintf(state.w, "  %sQuick mode: checksums only (skip archive readability)%s\n",
					output.ColorCyan, output.ColorReset)
			}

			type namedSnap struct {
				snap module.Snapshot
			}
			grouped := make(map[string][]namedSnap)
			for _, snap := range manifest.Snapshots {
				cat := moduleGroups[snap.Module]
				if cat == "" {
					cat = "Other"
				}
				grouped[cat] = append(grouped[cat], namedSnap{snap: snap})
			}

			runningArchives := 0
			runningSize := int64(0)

			for _, cat := range catOrder {
				snaps, ok := grouped[cat]
				if !ok {
					continue
				}
				sort.Slice(snaps, func(i, j int) bool {
					return snaps[i].snap.Module < snaps[j].snap.Module
				})

				fmt.Fprintf(state.w, "\n  %s%s%s\n", output.ColorBold+output.ColorBlue, cat, output.ColorReset)

				for _, ns := range snaps {
					result := verifyOneSnapshot(ns.snap, cfg, state.quick)
					runningArchives++
					runningSize += result.size

					state.verifiedArchives++
					state.totalSize += result.size
					state.totalOrigSize += result.origSize
					state.totalFileCount += result.fileCount
					if result.status == "ok" {
						state.verifiedMods++
					} else {
						state.failedMods++
					}
					if result.checksumMatch {
						state.sha256Match++
					} else {
						state.sha256Fail++
					}
					if !result.readable {
						state.readableFail++
					}
					if !state.quick {
						if result.fileCountOK {
							state.fileCountMatch++
						} else {
							state.fileCountFail++
						}
						if result.origSizeOK {
							state.origSizeMatch++
						} else {
							state.origSizeFail++
						}
					}

					state.failDetails[ns.snap.Module] = append(state.failDetails[ns.snap.Module], result.details...)
					if result.checksumMatch && result.readable {
						state.passDetails[ns.snap.Module] = append(state.passDetails[ns.snap.Module], "Archive valid")
					}

					meta := ""
					if result.size > 0 {
						meta = formatBytes(result.size)
					}
					if result.encrypted {
						meta += " encrypted"
					}

					// Build check detail line
					checkParts := []string{}
					if result.checksumMatch {
						checkParts = append(checkParts, fmt.Sprintf("%sSHA256 ✓%s", output.ColorGreen, output.ColorReset))
					} else {
						checkParts = append(checkParts, fmt.Sprintf("%sSHA256 ✗%s", output.ColorRed, output.ColorReset))
					}
					if !state.quick {
						if result.readable {
							checkParts = append(checkParts, fmt.Sprintf("%s%s ✓%s", output.ColorGreen, "zstd", output.ColorReset))
							checkParts = append(checkParts, fmt.Sprintf("%s%s ✓%s", output.ColorGreen, "tar", output.ColorReset))
						} else {
							checkParts = append(checkParts, fmt.Sprintf("%s%s ✗%s", output.ColorRed, "format", output.ColorReset))
						}
					}
					if !result.fileCountOK {
						checkParts = append(checkParts, fmt.Sprintf("%sfiles ✗%s", output.ColorRed, output.ColorReset))
					}
					if !result.origSizeOK {
						checkParts = append(checkParts, fmt.Sprintf("%ssize ✗%s", output.ColorRed, output.ColorReset))
					}

					checkDetail := ""
					if len(checkParts) > 0 {
						checkDetail = "  · " + strings.Join(checkParts, " · ")
					}

					fmt.Fprintf(state.w, "    %s%s%s %s%s\n",
						result.iconColor, result.icon, output.ColorReset,
						output.DotLeader(ns.snap.Module, meta, 24), checkDetail)

					if result.status == "fail" {
						for _, d := range result.details {
							fmt.Fprintf(state.w, "      %s·%s %s\n",
								output.ColorRed, output.ColorReset, d)
						}
						if len(result.details) > 0 {
							fmt.Fprintf(state.w, "      %sRecommendation:%s Recreate this backup.\n",
								output.ColorYellow, output.ColorReset)
						}
					}
				}

				// Category progress
				fmt.Fprintf(state.w, "  %s(%d/%d archives · %s verified)%s\n",
					output.ColorCyan,
					runningArchives, len(manifest.Snapshots),
					formatBytes(runningSize),
					output.ColorReset)
			}

			stageHeader(state.w, "4 / 5", "Verifying Integrity")

			fmt.Fprintln(state.w)

			missingCount := 0
			for _, snap := range manifest.Snapshots {
				if _, err := os.Stat(snap.Path); os.IsNotExist(err) {
					missingCount++
					state.failDetails[snap.Module] = append(state.failDetails[snap.Module], "Snapshot file not found on disk")
				}
			}
			state.missingArchives = missingCount

			if missingCount == 0 {
				fmt.Fprintf(state.w, "  %s✓%s %s\n",
					output.ColorGreen, output.ColorReset,
					output.DotLeader("All archives present", fmt.Sprintf("%d/%d", len(manifest.Snapshots)-missingCount, len(manifest.Snapshots)), 22))
			} else {
				fmt.Fprintf(state.w, "  %s✗%s %s\n",
					output.ColorRed, output.ColorReset,
					output.DotLeader("Missing archives", fmt.Sprintf("%d", missingCount), 22))
			}

			snapDir := filepath.Join(state.backupDir, "snapshots")
			snapDirInfo, err := os.Stat(snapDir)
			if err == nil && snapDirInfo.IsDir() {
				fmt.Fprintf(state.w, "  %s✓%s %s\n",
					output.ColorGreen, output.ColorReset,
					output.DotLeader("Snapshot directory", "present", 22))
			} else {
				fmt.Fprintf(state.w, "  %s✗%s %s\n",
					output.ColorRed, output.ColorReset,
					output.DotLeader("Snapshot directory", "missing", 22))
			}

			totalSize := int64(0)
			for _, snap := range manifest.Snapshots {
				totalSize += snap.Size
			}
			fmt.Fprintf(state.w, "  %s✓%s %s\n",
				output.ColorGreen, output.ColorReset,
				output.DotLeader("Data accounted", formatBytes(totalSize), 22))

			// Level 2: content depth verification
			if !state.quick && state.fileCountMatch+state.fileCountFail+state.origSizeMatch+state.origSizeFail > 0 {
				level2OK := state.fileCountFail == 0 && state.origSizeFail == 0
				state.level2Verified = true
				if level2OK {
					fmt.Fprintf(state.w, "  %s✓%s %s\n",
						output.ColorGreen, output.ColorReset,
						output.DotLeader("Content verification", "file count & size match", 22))
				} else {
					fmt.Fprintf(state.w, "  %s✗%s %s\n",
						output.ColorRed, output.ColorReset,
						output.DotLeader("Content verification", "mismatch detected", 22))
					if state.fileCountFail > 0 {
						fmt.Fprintf(state.w, "    %s%s%d archives have unexpected file counts%s\n",
							output.ColorYellow, string(output.IconWarn), state.fileCountFail, output.ColorReset)
					}
					if state.origSizeFail > 0 {
						fmt.Fprintf(state.w, "    %s%s%d archives have unexpected sizes%s\n",
							output.ColorYellow, string(output.IconWarn), state.origSizeFail, output.ColorReset)
					}
				}
			}

			// Warnings
			{
				// Snapshot age
				age := time.Since(manifest.CreatedAt)
				if age > 30*24*time.Hour {
					days := int(age.Hours() / 24)
					state.warnings = append(state.warnings, fmt.Sprintf("This snapshot is %d days old — consider creating a fresh backup", days))
					state.hasWarnings = true
				}

				// Machine ID mismatch
				currentMachineID := machineID()
				if manifest.MachineID != "" && currentMachineID != "" && manifest.MachineID != currentMachineID {
					state.warnings = append(state.warnings, fmt.Sprintf("This backup was created on a different machine (ID: %s)", manifest.MachineID[:8]))
					state.hasWarnings = true
				}

				// Encryption disabled
				enc := strings.ToLower(manifest.Encryption)
				if enc == "" || enc == "disabled" || enc == "none" {
					state.warnings = append(state.warnings, "This backup is not encrypted — sensitive data may be at risk")
					state.hasWarnings = true
				}

				// Partial modules
				partialCount := 0
				for _, snap := range manifest.Snapshots {
					if snap.Status == "partial" {
						partialCount++
					}
				}
				if partialCount > 0 {
					state.warnings = append(state.warnings, fmt.Sprintf("%d module(s) have partial backup status — some files may be missing", partialCount))
					state.hasWarnings = true
				}

				if state.hasWarnings {
					fmt.Fprintf(state.w, "\n  %s%s Warnings%s\n",
						output.ColorYellow, string(output.IconWarn), output.ColorReset)
					for _, w := range state.warnings {
						fmt.Fprintf(state.w, "    %s•%s %s\n", output.ColorYellow, output.ColorReset, w)
					}
				} else {
					fmt.Fprintf(state.w, "\n  %s✓%s %s\n",
						output.ColorGreen, output.ColorReset,
						output.DotLeader("Health", "no warnings", 22))
				}
			}

			fmt.Fprint(state.w, output.ColorReset)

			stageHeader(state.w, "5 / 5", "Verification Summary")
			elapsed := time.Since(state.start).Truncate(100 * time.Millisecond)

			speed := float64(0)
			if elapsed.Seconds() > 0 {
				speed = float64(state.totalSize) / (elapsed.Seconds() * 1024 * 1024)
			}

			statusColor := output.ColorGreen
			statusTitle := "Verification Complete"
			if state.failedMods > 0 || missingCount > 0 {
				statusColor = output.ColorYellow
				statusTitle = "Verification Complete with Issues"
			}

			fmt.Fprintf(state.w, "\n  %s%s%s\n", statusColor, statusTitle, output.ColorReset)
			fmt.Fprintln(state.w)

			pad := 18

			// Integrity section
			fmt.Fprintf(state.w, "\n  %sIntegrity%s\n", output.ColorBold, output.ColorReset)
			if state.manifestsValid {
				fmt.Fprintf(state.w, "  %s✓%s %s\n", output.ColorGreen, output.ColorReset,
					output.DotLeader("Manifest", "valid", pad))
			} else {
				fmt.Fprintf(state.w, "  %s✗%s %s\n", output.ColorRed, output.ColorReset,
					output.DotLeader("Manifest", "invalid", pad))
			}
			if state.inventoryValid {
				fmt.Fprintf(state.w, "  %s✓%s %s\n", output.ColorGreen, output.ColorReset,
					output.DotLeader("Inventory", "valid", pad))
			} else {
				fmt.Fprintf(state.w, "  %s○%s %s\n", output.ColorCyan, output.ColorReset,
					output.DotLeader("Inventory", "not found", pad))
			}
			if state.hasSHA256SUMS {
				fmt.Fprintf(state.w, "  %s✓%s %s\n", output.ColorGreen, output.ColorReset,
					output.DotLeader("SHA256SUMS", "verified", pad))
			} else {
				fmt.Fprintf(state.w, "  %s○%s %s\n", output.ColorCyan, output.ColorReset,
					output.DotLeader("SHA256SUMS", "not present", pad))
			}

			sha256ok := state.sha256Match
			sha256total := state.verifiedArchives
			if sha256total > 0 && sha256ok == sha256total {
				fmt.Fprintf(state.w, "  %s✓%s %s\n", output.ColorGreen, output.ColorReset,
					output.DotLeader("SHA256 checksums", "all match", pad))
			} else if sha256ok > 0 {
				fmt.Fprintf(state.w, "  %s⚠%s %s\n", output.ColorYellow, output.ColorReset,
					output.DotLeader("SHA256 checksums", fmt.Sprintf("%d/%d match", sha256ok, sha256total), pad))
			}

			if missingCount == 0 {
				fmt.Fprintf(state.w, "  %s✓%s %s\n", output.ColorGreen, output.ColorReset,
					output.DotLeader("Archives present", fmt.Sprintf("%d/%d", len(manifest.Snapshots)-missingCount, len(manifest.Snapshots)), pad))
			} else {
				fmt.Fprintf(state.w, "  %s✗%s %s\n", output.ColorRed, output.ColorReset,
					output.DotLeader("Archives present", fmt.Sprintf("%d missing", missingCount), pad))
			}

			if state.level2Verified {
				label := "verified"
				if state.fileCountFail > 0 || state.origSizeFail > 0 {
					label = "mismatch"
				}
				fmt.Fprintf(state.w, "  %s✓%s %s\n", output.ColorGreen, output.ColorReset,
					output.DotLeader("Content depth", label, pad))
			}

			// Modules section
			fmt.Fprintf(state.w, "\n  %sModules%s\n", output.ColorBold, output.ColorReset)
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorGreen,
				output.DotLeader(fmt.Sprintf("%s verified", string(output.IconCheck)),
					fmt.Sprintf("%d", state.verifiedMods), pad))
			if state.failedMods > 0 {
				fmt.Fprintf(state.w, "  %s%s\n",
					output.ColorRed,
					output.DotLeader(fmt.Sprintf("%s failed", string(output.IconCross)),
						fmt.Sprintf("%d", state.failedMods), pad))
			}

			// Data stats
			fmt.Fprintf(state.w, "\n  %sData Verified%s\n", output.ColorBold, output.ColorReset)
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("archives", formatBytes(state.totalSize), pad))
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("compressed", formatBytes(state.totalSize), pad))
			if state.totalOrigSize > 0 {
				fmt.Fprintf(state.w, "  %s%s\n",
					output.ColorCyan,
					output.DotLeader("original", formatBytes(state.totalOrigSize), pad))
				ratio := float64(state.totalOrigSize) / float64(state.totalSize)
				fmt.Fprintf(state.w, "  %s%s\n",
					output.ColorCyan,
					output.DotLeader("ratio", fmt.Sprintf("%.2fx", ratio), pad))
			}
			if state.totalFileCount > 0 {
				fmt.Fprintf(state.w, "  %s%s\n",
					output.ColorCyan,
					output.DotLeader("files", fmt.Sprintf("%d", state.totalFileCount), pad))
			}
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("modules", fmt.Sprintf("%d", state.verifiedArchives), pad))

			// Duration and speed
			fmt.Fprintf(state.w, "\n  %sDuration%s\n", output.ColorBold, output.ColorReset)
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("time", elapsed.String(), pad))
			if speed > 0 {
				fmt.Fprintf(state.w, "  %s%s\n",
					output.ColorCyan,
					output.DotLeader("throughput", fmt.Sprintf("%.0f MB/s", speed), pad))
			}
			fmt.Fprintf(state.w, "  %s%s%s\n",
				output.ColorCyan,
				output.DotLeader("mode", map[bool]string{true: "quick", false: "full"}[state.quick], pad),
				output.ColorReset)

			// Recovery readiness
			fmt.Fprintln(state.w)
			fmt.Fprintf(state.w, "  %sRecovery Readiness%s\n", output.ColorBold, output.ColorReset)

			hasErrors := state.failedMods > 0 || missingCount > 0

			if !hasErrors && state.level2Verified {
				fmt.Fprintf(state.w, "\n  %s%s%s\n",
					output.ColorGreen, "★★★★★ Excellent", output.ColorReset)
				fmt.Fprintf(state.w, "  %sAll archives verified — checksums match, content depth confirmed, archive format valid.%s\n",
					output.ColorGreen, output.ColorReset)
				fmt.Fprintf(state.w, "  %sThis backup is fully restorable.%s\n",
					output.ColorGreen, output.ColorReset)
			} else if !hasErrors && state.verifiedArchives > 0 {
				fmt.Fprintf(state.w, "\n  %s%s%s\n",
					output.ColorGreen, "★★★★☆ Good", output.ColorReset)
				if state.quick {
					fmt.Fprintf(state.w, "  %sAll checksums match. Run without %s--quick%s for archive format validation.%s\n",
						output.ColorGreen, output.ColorCyan, output.ColorGreen, output.ColorReset)
				} else {
					fmt.Fprintf(state.w, "  %sAll archives intact — checksums match, archive format valid.%s\n",
						output.ColorGreen, output.ColorReset)
					fmt.Fprintf(state.w, "  %sRun %sgetitback verify --deep%s for full content verification.%s\n",
						output.ColorGreen, output.ColorCyan, output.ColorGreen, output.ColorReset)
				}
			} else if state.failedMods <= 3 {
				fmt.Fprintf(state.w, "\n  %s%s%s\n",
					output.ColorYellow, "★★★☆☆ Fair", output.ColorReset)
				fmt.Fprintf(state.w, "  %sSome modules have issues but most data is recoverable.%s\n",
					output.ColorYellow, output.ColorReset)
				fmt.Fprintf(state.w, "\n  Issues:\n")
				for mod, details := range state.failDetails {
					for _, d := range details {
						fmt.Fprintf(state.w, "    %s✗%s %s: %s\n",
							output.ColorRed, output.ColorReset, mod, d)
					}
				}
			} else {
				fmt.Fprintf(state.w, "\n  %s%s%s\n",
					output.ColorRed, "★★☆☆☆ Critical", output.ColorReset)
				fmt.Fprintf(state.w, "  %sThis backup has significant issues and may not be fully restorable.%s\n",
					output.ColorRed, output.ColorReset)
				fmt.Fprintf(state.w, "\n  Issues:\n")
				for mod, details := range state.failDetails {
					for _, d := range details {
						fmt.Fprintf(state.w, "    %s✗%s %s: %s\n",
							output.ColorRed, output.ColorReset, mod, d)
					}
				}
			}

			if state.hasWarnings && hasErrors {
				fmt.Fprintf(state.w, "\n  %sWarnings:%s\n", output.ColorYellow, output.ColorReset)
				for _, w := range state.warnings {
					fmt.Fprintf(state.w, "    %s•%s %s\n", output.ColorYellow, output.ColorReset, w)
				}
			}

			// Next recommended command
			fmt.Fprintln(state.w)
			fmt.Fprintf(state.w, "  %sNext Recommended Command%s\n", output.ColorBold, output.ColorReset)
			switch {
			case state.failedMods > 0:
				fmt.Fprintf(state.w, "  %sgetitback backup%s — create a fresh backup to replace damaged archives\n",
					output.ColorGreen, output.ColorReset)
			case state.hasWarnings:
				fmt.Fprintf(state.w, "  %sgetitback doctor%s — review warnings and improve recovery readiness\n",
					output.ColorGreen, output.ColorReset)
			default:
				fmt.Fprintf(state.w, "  %sgetitback doctor%s — review your overall recovery readiness\n",
					output.ColorGreen, output.ColorReset)
				if state.level2Verified {
					fmt.Fprintf(state.w, "  %s  (use --deep for full restore simulation)%s\n",
						output.ColorCyan, output.ColorReset)
				}
			}
			fmt.Fprintln(state.w)

			if state.failedMods > 0 {
				return fmt.Errorf("%d modules failed verification", state.failedMods)
			}

			return nil
		},
	}

	cmd.Flags().String("id", "", "Backup ID to verify (default: latest)")
	cmd.Flags().Bool("quick", false, "Checksums only, skip archive readability check")
	return cmd
}

func verifyOneSnapshot(snap module.Snapshot, cfg *config.Config, quick bool) snapVerifyResult {
	res := snapVerifyResult{
		module:    snap.Module,
		status:    "ok",
		icon:      string(output.IconCheck),
		iconColor: output.ColorGreen,
		size:      snap.Size,
		origSize:  snap.OriginalSize,
		fileCount: snap.FileCount,
	}

	path := snap.Path

	if _, err := os.Stat(path); os.IsNotExist(err) {
		res.status = "fail"
		res.icon = string(output.IconCircle)
		res.iconColor = output.ColorCyan
		res.details = append(res.details, "Snapshot file not found")
		return res
	}

	info, err := os.Stat(path)
	if err != nil {
		res.status = "fail"
		res.icon = string(output.IconCross)
		res.iconColor = output.ColorRed
		res.details = append(res.details, fmt.Sprintf("Cannot stat file: %s", err))
		return res
	}
	res.size = info.Size()

	// Checksum
	if snap.Checksum != "" {
		checksum, err := computeChecksum(path)
		if err != nil {
			res.status = "fail"
			res.icon = string(output.IconCross)
			res.iconColor = output.ColorRed
			res.details = append(res.details, fmt.Sprintf("Checksum computation failed: %s", err))
			return res
		}
		res.checksumMatch = checksum == snap.Checksum
		if !res.checksumMatch {
			res.status = "fail"
			res.icon = string(output.IconCross)
			res.iconColor = output.ColorRed
			res.details = append(res.details, fmt.Sprintf("SHA256 mismatch: expected %s, got %s", snap.Checksum[:16], checksum[:16]))
			return res
		}
	} else {
		res.checksumMatch = true
	}

	// Archive readability + content stats
	if !quick {
		vResult, err := archive.VerifyReadable(path)
		if err != nil {
			res.status = "fail"
			res.icon = string(output.IconCross)
			res.iconColor = output.ColorRed
			res.details = append(res.details, fmt.Sprintf("Archive corrupted or unreadable: %s", err))
			return res
		}
		res.readable = true

		// File count cross-check
		if snap.FileCount > 0 {
			res.fileCountOK = vResult.FileCount == snap.FileCount
			if !res.fileCountOK {
				res.status = "fail"
				res.icon = string(output.IconCross)
				res.iconColor = output.ColorRed
				res.details = append(res.details, fmt.Sprintf("File count mismatch: expected %d, archive has %d", snap.FileCount, vResult.FileCount))
			}
		} else {
			res.fileCountOK = true
		}

		// Original size cross-check
		if snap.OriginalSize > 0 {
			res.origSizeOK = vResult.OriginalSize == snap.OriginalSize
			if !res.origSizeOK {
				res.status = "fail"
				res.icon = string(output.IconCross)
				res.iconColor = output.ColorRed
				res.details = append(res.details, fmt.Sprintf("Original size mismatch: expected %s, archive has %s",
					formatBytes(snap.OriginalSize), formatBytes(vResult.OriginalSize)))
			}
		} else {
			res.origSizeOK = true
		}
	} else {
		res.readable = true
		res.fileCountOK = true
		res.origSizeOK = true
	}

	res.encrypted = snap.Encrypted

	if res.status == "ok" {
		res.icon = string(output.IconCheck)
		res.iconColor = output.ColorGreen
	}

	return res
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
