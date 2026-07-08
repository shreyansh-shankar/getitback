package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/assessment"
	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/crypto"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/storage"
	"github.com/spf13/cobra"
)

type backupState struct {
	backupID         string
	backupDir        string
	snapshotsDir     string
	cfg              *config.Config
	manager          *module.Manager
	w                interface{ Write([]byte) (int, error) }
	start            time.Time
	allSnapshots     []module.Snapshot
	backedCount      int
	notInstalledCount int
	nothingCount     int
	failCount        int
	catResults       map[string]categoryResult
	moduleContent    map[string][]string
	moduleWarn       map[string][]string
	criticalMods     []string
	highMods         []string
}

type categoryResult struct {
	ok   int
	skip int
	fail int
	size int64
}

type catSize struct {
	name string
	size int64
}

func newBackupCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create a complete machine backup",
		Long: `Backup all discovered configurations, credentials, and data.

Stages:
  1. Initialize   — create backup directory, configure encryption
  2. Inventory    — detect installed software and resources
  3. Plan         — build backup plan with estimated coverage
  4. Backup       — snapshot each detected module
  5. Finalize     — write manifest, apply encryption
  6. Summary      — display results, recovery score, next steps`,
		RunE: func(cmd *cobra.Command, args []string) error {
			state := &backupState{
				cfg:           cfg,
				manager:       manager,
				w:             cmd.OutOrStdout(),
				start:         time.Now(),
				catResults:    make(map[string]categoryResult),
				moduleContent: make(map[string][]string),
				moduleWarn:    make(map[string][]string),
			}

			ctx := cmd.Context()
			moduleFilter, _ := cmd.Flags().GetString("module")

			state.backupID = time.Now().UTC().Format("20060102T150405Z")
			state.backupDir = filepath.Join(cfg.Storage.Path, state.backupID)
			state.snapshotsDir = filepath.Join(state.backupDir, "snapshots")
			metaDir := filepath.Join(state.backupDir, "metadata")
			logBuf := new(strings.Builder)

			logLine := func(format string, args ...any) {
				fmt.Fprintf(logBuf, format+"\n", args...)
			}
			logLine("Backup started: %s", state.backupID)

			// ── Stage 1: Initialize ──
			stageHeader(state.w, "1 / 6", "Initializing")
			logLine("Stage 1: Initializing")

			if err := os.MkdirAll(state.snapshotsDir, 0700); err != nil {
				return fmt.Errorf("create backup dir: %w", err)
			}
			if err := os.MkdirAll(metaDir, 0700); err != nil {
				return fmt.Errorf("create metadata dir: %w", err)
			}
			logLine("Created directories")

			encLabel := "Disabled"
			encColor := output.ColorCyan
			if cfg.Encryption.Enabled {
				encLabel = "Enabled"
				encColor = output.ColorGreen
			}

			fmt.Fprintln(state.w)
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("ID", state.backupID, 22))
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("Destination", state.backupDir, 22))
			fmt.Fprintf(state.w, "  %s%s%s\n",
				encColor,
				output.DotLeader("Encryption", encLabel, 22),
				output.ColorReset)

			// ── Stage 2: Inventory ──
			stageHeader(state.w, "2 / 6", "Inventory")
			logLine("Stage 2: Inventory")

			inventory := manager.Inventory(ctx)
			detectedCount := len(inventory)
			logLine("Detected %d modules", detectedCount)

			fmt.Fprintf(state.w, "\n  %s%d modules detected%s\n",
				output.ColorGreen, detectedCount, output.ColorReset)

			invPath := filepath.Join(state.backupDir, "inventory.json")
			invFile, err := os.Create(invPath)
			if err != nil {
				return fmt.Errorf("create inventory: %w", err)
			}
			enc := json.NewEncoder(invFile)
			enc.SetIndent("", "  ")
			enc.Encode(inventory)
			invFile.Close()

			// ── Stage 3: Plan ──
			stageHeader(state.w, "3 / 6", "Planning")
			logLine("Stage 3: Planning")

			modulesToBackup := manager.All()
			if moduleFilter != "" {
				mod, ok := manager.Get(moduleFilter)
				if !ok {
					return fmt.Errorf("unknown module: %s", moduleFilter)
				}
				modulesToBackup = []module.Module{mod}
			}

			planTotal := len(modulesToBackup)
			fmt.Fprintf(state.w, "\n  %s%d%s total modules\n",
				output.ColorCyan, planTotal, output.ColorReset)
			fmt.Fprintf(state.w, "  %s%d%s detected, will back up\n",
				output.ColorGreen, detectedCount, output.ColorReset)
			if planTotal-detectedCount > 0 {
				fmt.Fprintf(state.w, "  %s%d%s not detected, will skip\n",
					output.ColorCyan, planTotal-detectedCount, output.ColorReset)
			}

			// ── Stage 4: Backup ──
			stageHeader(state.w, "4 / 6", "Backing Up")
			logLine("Stage 4: Backing Up")

			fmt.Fprintf(state.w, "\n  %s✓%s Success   %s⚠%s Partial   %s✗%s Failed   %s○%s Not Installed\n",
				output.ColorGreen, output.ColorReset,
				output.ColorYellow, output.ColorReset,
				output.ColorRed, output.ColorReset,
				output.ColorCyan, output.ColorReset)

			type namedModule struct {
				mod  module.Module
				name string
				cat  string
			}
			grouped := make(map[string][]namedModule)
			for _, mod := range modulesToBackup {
				cat := moduleGroups[mod.Name()]
				if cat == "" {
					cat = "Other"
				}
				grouped[cat] = append(grouped[cat], namedModule{mod: mod, name: mod.Name(), cat: cat})
			}

			reposDetected := false

			for _, cat := range catOrder {
				mods, ok := grouped[cat]
				if !ok {
					continue
				}
				sort.Slice(mods, func(i, j int) bool {
					return mods[i].name < mods[j].name
				})

				state.catResults[cat] = categoryResult{}
				catSize := int64(0)
				catOK := 0
				catSkip := 0
				catFail := 0

				fmt.Fprintf(state.w, "\n  %s%s%s\n", output.ColorBold+output.ColorBlue, cat, output.ColorReset)

				for _, nm := range mods {
					isDetected := false
					for _, inv := range inventory {
						if inv.Module == nm.name {
							isDetected = true
							break
						}
					}

					hadData := false
					iconColor := output.ColorCyan
					icon := string(output.IconCircle)
					detail := "Not installed"

					if !isDetected {
						state.notInstalledCount++
						catSkip++
						logLine("  %s: not installed", nm.name)
						fmt.Fprintf(state.w, "    %s%s%s %s\n",
							iconColor, icon, output.ColorReset,
							output.DotLeader(nm.name, detail, 24))
						continue
					}

					logLine("  %s: starting backup", nm.name)
					moduleStart := time.Now()
					opts := module.BackupOptions{
						SnapshotsDir: state.snapshotsDir,
						Encrypt:      cfg.Encryption.Enabled,
						KeyPath:      cfg.Encryption.KeyPath,
					}
					result, err := nm.mod.Backup(ctx, opts)

					if err != nil {
						iconColor = output.ColorRed
						icon = string(output.IconCross)
						detail = fmt.Sprintf("%s", err)
						catFail++
						state.failCount++
						logLine("  %s: failed — %s", nm.name, err)
					} else if result == nil || len(result.Snapshots) == 0 {
						if result != nil && len(result.Warnings) > 0 {
							state.moduleWarn[nm.name] = result.Warnings
						}
						iconColor = output.ColorGreen
						icon = string(output.IconCheck)
						detail = "Nothing to back up"
						catSkip++
						state.nothingCount++
						logLine("  %s: nothing to back up", nm.name)
					} else {
						hadData = true
						isPartial := result.Partial
						if isPartial {
							iconColor = output.ColorYellow
							icon = string(output.IconWarn)
						} else {
							iconColor = output.ColorGreen
							icon = string(output.IconCheck)
						}
						moduleDur := time.Since(moduleStart).Truncate(time.Millisecond)

						// Enrich snapshot metadata
						statusStr := "success"
						if isPartial {
							statusStr = "partial"
						}
						for i := range result.Snapshots {
							result.Snapshots[i].ArchiveFile = nm.name + ".tar.zst"
							result.Snapshots[i].Compression = storage.CompressionAlgo
							result.Snapshots[i].Duration = moduleDur.String()
							result.Snapshots[i].Status = statusStr
							if info := module.GetModuleInfo(nm.name); info != nil {
								result.Snapshots[i].RecoveryValue = info.RecoveryValue
							}
						}
						state.allSnapshots = append(state.allSnapshots, result.Snapshots...)

						size := result.Snapshots[0].Size
						totalOrig := result.Snapshots[0].OriginalSize
						totalComp := result.Snapshots[0].CompressedSize
						archiveFile := result.Snapshots[0].ArchiveFile
						for i := 1; i < len(result.Snapshots); i++ {
							size += result.Snapshots[i].Size
							totalOrig += result.Snapshots[i].OriginalSize
							totalComp += result.Snapshots[i].CompressedSize
						}
						detail = formatBytes(size)
						catSize += size

						if len(result.Contents) > 0 {
							state.moduleContent[nm.name] = result.Contents
						}
						if len(result.Warnings) > 0 {
							state.moduleWarn[nm.name] = result.Warnings
						}

						if isPartial {
							detail = formatBytes(size) + " (partial)"
						}
						catOK++
						state.backedCount++

						// Write per-module metadata
						meta := storage.ModuleMeta{
							Module:           nm.name,
							ArchiveFile:      archiveFile,
							OriginalSize:     totalOrig,
							CompressedSize:   totalComp,
							Checksum:         result.Snapshots[0].Checksum,
							FileCount:        result.Snapshots[0].FileCount,
							Status:           "success",
							Duration:         moduleDur.String(),
							Created:          time.Now().UTC().Format(time.RFC3339),
							Compression:      storage.CompressionAlgo,
							CompressionLevel: storage.CompressionLevel,
						}
						if info := module.GetModuleInfo(nm.name); info != nil {
							meta.RecoveryValue = info.RecoveryValue
						}
						if isPartial {
							meta.Status = "partial"
						}
						if err := storage.WriteModuleMeta(metaDir, &meta); err != nil {
							logLine("  %s: metadata write error — %s", nm.name, err)
						}
						logLine("  %s: complete (%s in %s)", nm.name, formatBytes(size), moduleDur)
					}

					if nm.name == "repos" && hadData {
						reposDetected = true
					}
				}

				state.catResults[cat] = categoryResult{
					ok: catOK, skip: catSkip, fail: catFail,
					size: catSize,
				}

				// Category-level progress
				catDone := 0
				for _, c := range catOrder {
					if m, ok := grouped[c]; ok && len(m) > 0 {
						catDone++
					}
				}
				catProcessed := 0
				for _, c := range catOrder {
					if _, ok := grouped[c]; ok {
						catProcessed++
						if c == cat {
							break
						}
					}
				}
				if catProcessed > 0 && catDone > 0 {
					filled := catProcessed * 20 / catDone
					bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
					percent := catProcessed * 100 / catDone
					fmt.Fprintf(state.w, "\n  %s[%s]%s %3d%% (%d/%d categories)\n",
						output.ColorGreen, bar, output.ColorReset, percent, catProcessed, catDone)
				}
			}

			// ── Stage 5: Finalize ──
			stageHeader(state.w, "5 / 6", "Finalizing")

			if cfg.Encryption.Enabled && len(state.allSnapshots) > 0 {
				keyData, err := os.ReadFile(cfg.Encryption.KeyPath)
				if err != nil {
					return fmt.Errorf("read encryption key: %w", err)
				}

				identity := string(keyData)
				encrypted := 0

				for i, snap := range state.allSnapshots {
					if snap.Encrypted {
						continue
					}
					encryptedPath := snap.Path + ".age"

					recipientData, err := recipientFromIdentity(identity)
					if err != nil {
						continue
					}

					if err := crypto.EncryptFile(snap.Path, encryptedPath, recipientData); err != nil {
						continue
					}

					rawPath := snap.Path
					state.allSnapshots[i].Path = encryptedPath
					state.allSnapshots[i].Encrypted = true

					checksum, _ := fileChecksum(encryptedPath)
					state.allSnapshots[i].Checksum = checksum

					info, _ := os.Stat(encryptedPath)
					if info != nil {
						state.allSnapshots[i].Size = info.Size()
					}

					os.Remove(rawPath)
					encrypted++
				}

				if encrypted > 0 {
					fmt.Fprintf(state.w, "\n  %s•%s %s\n",
						output.ColorGreen, output.ColorReset,
						output.DotLeader("encrypted snapshots", fmt.Sprintf("%d", encrypted), 22))
				}
			}

			hostname, _ := os.Hostname()
			machineID := machineID()

			// Compute total size before building manifest
			manifestTotalSize := int64(0)
			for _, snap := range state.allSnapshots {
				manifestTotalSize += snap.Size
			}

			manifest := storage.Manifest{
				Version:          storage.ManifestVersion,
				ManifestVersion:  storage.BackupFormatVersion,
				BackupVersion:    storage.BackupFormatVersion,
				BackupID:         state.backupID,
				CreatedAt:        time.Now(),
				Hostname:         hostname,
				MachineID:        machineID,
				Platform:         runtime.GOOS,
				Architecture:     runtime.GOARCH,
				OS:               fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH),
				Compression:      storage.CompressionAlgo,
				BackupSize:       manifestTotalSize,
				InventoryVersion: storage.InventoryVersion,
				Snapshots:        state.allSnapshots,
				Inventory:        inventory,
			}
			if cfg.Encryption.Enabled {
				manifest.Encryption = "age"
			}

			if err := storage.WriteManifest(state.backupDir, &manifest); err != nil {
				return fmt.Errorf("write manifest: %w", err)
			}
			logLine("Manifest written (format %s)", storage.BackupFormatVersion)

			fmt.Fprintf(state.w, "\n  %s•%s %s\n",
				output.ColorGreen, output.ColorReset,
				output.DotLeader("manifest written", "version "+storage.ManifestVersion, 22))

			// Write SHA256SUMS
			if err := storage.WriteChecksums(state.backupDir, state.allSnapshots); err != nil {
				logLine("SHA256SUMS write error: %s", err)
			} else {
				logLine("SHA256SUMS written")
				fmt.Fprintf(state.w, "  %s•%s %s\n",
					output.ColorGreen, output.ColorReset,
					output.DotLeader("checksums written", fmt.Sprintf("%d snapshots", len(state.allSnapshots)), 22))
			}

			// Write backup metadata
			totalOrigSize := int64(0)
			totalCompSize := int64(0)
			for _, snap := range state.allSnapshots {
				totalOrigSize += snap.OriginalSize
				totalCompSize += snap.Size
			}
			ratio := 0.0
			if totalCompSize > 0 && totalOrigSize > 0 {
				ratio = float64(totalOrigSize) / float64(totalCompSize)
			}
			bMeta := storage.BackupMeta{
				BackupID:           state.backupID,
				CreatedAt:          time.Now().UTC().Format(time.RFC3339),
				Hostname:           hostname,
				MachineID:          machineID,
				Platform:           runtime.GOOS,
				Architecture:       runtime.GOARCH,
				ModuleCount:        state.backedCount,
				SnapshotCount:      len(state.allSnapshots),
				TotalOriginalSize:  totalOrigSize,
				TotalCompressedSize: totalCompSize,
				Compression:        storage.CompressionAlgo,
				CompressionRatio:   ratio,
				Encryption:         encLabel,
				Duration:           time.Since(state.start).Truncate(100 * time.Millisecond).String(),
			}
			if err := storage.WriteBackupMeta(metaDir, &bMeta); err != nil {
				logLine("backup metadata write error: %s", err)
			} else {
				logLine("Backup metadata written")
			}

			// Write backup log
			logLine("Backup completed: %s", state.backupID)
			if err := os.WriteFile(filepath.Join(state.backupDir, "backup.log"), []byte(logBuf.String()), 0600); err != nil {
				logLine("backup log write error: %s", err)
			}

			// ── Stage 6: Summary ──
			stageHeader(state.w, "6 / 6", "Summary")
			elapsed := time.Since(state.start).Truncate(100 * time.Millisecond)

			totalSize := int64(0)
			for _, snap := range state.allSnapshots {
				totalSize += snap.Size
			}

			statusColor := output.ColorGreen
			statusTitle := "Backup Complete"
			if state.failCount > 0 {
				statusColor = output.ColorYellow
				statusTitle = "Backup Complete with Errors"
			}

			fmt.Fprintf(state.w, "\n  %s%s%s\n", statusColor, statusTitle, output.ColorReset)
			fmt.Fprintln(state.w)

			pad := 18
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("Backup ID", state.backupID, pad))
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("Duration", elapsed.String(), pad))
			fmt.Fprintf(state.w, "  %s%s%s\n",
				output.ColorCyan,
				output.DotLeader("Encryption", encLabel, pad),
				output.ColorReset)

			// Modules summary (split into meaningful categories)
			fmt.Fprintln(state.w)
			fmt.Fprintf(state.w, "  %sModules%s\n", output.ColorBold, output.ColorReset)
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorGreen,
				output.DotLeader(fmt.Sprintf("%s protected", string(output.IconCheck)),
					fmt.Sprintf("%d", state.backedCount), pad))
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader(fmt.Sprintf("%s not installed", string(output.IconCircle)),
					fmt.Sprintf("%d", state.notInstalledCount), pad))
			if state.nothingCount > 0 {
				fmt.Fprintf(state.w, "  %s%s\n",
					output.ColorGreen,
					output.DotLeader(fmt.Sprintf("%s nothing to back up", string(output.IconCheck)),
						fmt.Sprintf("%d", state.nothingCount), pad))
			}
			if state.failCount > 0 {
				fmt.Fprintf(state.w, "  %s%s%s\n",
					output.ColorRed,
					output.DotLeader(fmt.Sprintf("%s failed", string(output.IconCross)),
						fmt.Sprintf("%d", state.failCount), pad),
					output.ColorReset)
			}

			// Snapshots
			fmt.Fprintln(state.w)
			fmt.Fprintf(state.w, "  %sSnapshots%s\n", output.ColorBold, output.ColorReset)
			fmt.Fprintf(state.w, "  %s%s\n",
				output.ColorCyan,
				output.DotLeader("created", fmt.Sprintf("%d", len(state.allSnapshots)), pad))
			fmt.Fprintf(state.w, "  %s%s%s\n",
				output.ColorCyan,
				output.DotLeader("total size", formatBytes(totalSize), pad),
				output.ColorReset)

			// Backup contents by category (sorted by size descending)
			fmt.Fprintln(state.w)
			fmt.Fprintf(state.w, "  %sBackup Contents%s\n", output.ColorBold, output.ColorReset)
			var catSizes []catSize
			for _, cat := range catOrder {
				res, ok := state.catResults[cat]
				if !ok || (res.ok == 0 && res.size == 0) {
					continue
				}
				if res.size > 0 {
					catSizes = append(catSizes, catSize{name: cat, size: res.size})
				}
			}
			sort.Slice(catSizes, func(i, j int) bool {
				return catSizes[i].size > catSizes[j].size
			})
			for _, cs := range catSizes {
				fmt.Fprintf(state.w, "  %s%s\n",
					output.ColorCyan,
					output.DotLeader(cs.name, formatBytes(cs.size), pad))
			}
			fmt.Fprint(state.w, output.ColorReset)

			// Recovery readiness (capped at 100, based on actual backup coverage)
			fmt.Fprintln(state.w)
			beforeScore := assessment.ComputeScore(inventory, assessment.ComputeCoverage(inventory), moduleGroups)
			beforeTotal := beforeScore.Total
			if beforeTotal > 100 {
				beforeTotal = 100
			}

			afterTotal := beforeTotal
			if state.backedCount > 0 {
				bonus := state.backedCount * 2
				if bonus > 30 {
					bonus = 30
				}
				afterTotal = beforeTotal + bonus
				if afterTotal > 100 {
					afterTotal = 100
				}
			}

			if beforeTotal > 0 {
				fmt.Fprintf(state.w, "  %sRecovery Readiness%s\n", output.ColorBold, output.ColorReset)
				fmt.Fprintf(state.w, "  %s%s\n",
					output.ColorCyan,
					output.DotLeader("before",
						fmt.Sprintf("%d/100 (%s)", beforeTotal, scoreLetter(beforeTotal)), pad))

				afterLetter := scoreLetter(afterTotal)
				afterColor := output.ColorGreen
				if afterLetter == "C" || afterLetter == "D" {
					afterColor = output.ColorYellow
				} else if afterLetter == "F" {
					afterColor = output.ColorRed
				}
				fmt.Fprintf(state.w, "  %s%s%s%s\n",
					output.ColorCyan,
					output.DotLeader("after", "", pad),
					afterColor,
					fmt.Sprintf(" %d/100 (%s)", afterTotal, afterLetter))
				fmt.Fprint(state.w, output.ColorReset)
				fmt.Fprintln(state.w)
			}

			// Critical Assets Protected
			for _, snap := range state.allSnapshots {
				info := module.GetModuleInfo(snap.Module)
				if info != nil {
					switch info.RecoveryValue {
					case "Critical":
						state.criticalMods = append(state.criticalMods, snap.Module)
					case "High":
						state.highMods = append(state.highMods, snap.Module)
					}
				}
			}

			if len(state.criticalMods) > 0 {
				fmt.Fprintln(state.w)
				fmt.Fprintf(state.w, "  %sCritical Assets Protected%s\n", output.ColorBold, output.ColorReset)
				deduped := uniqueStrings(state.criticalMods)
				for _, mod := range deduped {
					displayName := moduleDisplayName(mod)
					fmt.Fprintf(state.w, "  %s✓%s %s\n",
						output.ColorGreen, output.ColorReset, displayName)
				}
			}

			if len(state.highMods) > 0 {
				fmt.Fprintln(state.w)
				fmt.Fprintf(state.w, "  %sHigh Value Assets Protected%s\n", output.ColorBold, output.ColorReset)
				deduped := uniqueStrings(state.highMods)
				for _, mod := range deduped {
					displayName := moduleDisplayName(mod)
					fmt.Fprintf(state.w, "  %s✓%s %s\n",
						output.ColorGreen, output.ColorReset, displayName)
				}
			}

			// Warnings
			warnShown := false
			for modName, warns := range state.moduleWarn {
				if len(warns) == 0 {
					continue
				}
				if !warnShown {
					fmt.Fprintln(state.w)
					fmt.Fprintf(state.w, "  %sWarnings%s\n", output.ColorYellow, output.ColorReset)
					warnShown = true
				}
				for _, w := range warns {
					fmt.Fprintf(state.w, "  %s%s%s %s: %s\n",
						output.ColorYellow, string(output.IconWarn), output.ColorReset,
						modName, w)
				}
			}

			// Repo-specific message
			if !reposDetected {
				home, _ := os.UserHomeDir()
				fmt.Fprintln(state.w)
				fmt.Fprintf(state.w, "  %sRepositories%s\n", output.ColorBold, output.ColorReset)
				fmt.Fprintf(state.w, "  No Git repositories discovered.\n")
				fmt.Fprintf(state.w, "\n  Search root:\n")
				fmt.Fprintf(state.w, "  %s\n", home)
				fmt.Fprintf(state.w, "\n  %sHint:%s Projects stored on external drives\n",
					output.ColorCyan, output.ColorReset)
				fmt.Fprintf(state.w, "  or outside the search paths will not appear.\n")
			}

			// Integrity summary
			fmt.Fprintln(state.w)
			fmt.Fprintf(state.w, "  %sIntegrity%s\n", output.ColorBold, output.ColorReset)
			fmt.Fprintf(state.w, "  %s✓%s Manifest written\n",
				output.ColorGreen, output.ColorReset)
			fmt.Fprintf(state.w, "  %s✓%s Inventory stored\n",
				output.ColorGreen, output.ColorReset)
			if len(state.allSnapshots) > 0 {
				fmt.Fprintf(state.w, "  %s✓%s Checksums generated\n",
					output.ColorGreen, output.ColorReset)
				fmt.Fprintf(state.w, "  %s✓%s Snapshots indexed\n",
					output.ColorGreen, output.ColorReset)
			}

			// Location
			fmt.Fprintln(state.w)
			fmt.Fprintf(state.w, "  %sLocation%s\n", output.ColorBold, output.ColorReset)
			fmt.Fprintf(state.w, "  %s%s%s\n",
				output.ColorCyan,
				output.DotLeader("path", state.backupDir, pad),
				output.ColorReset)

			// Next recommended command
			fmt.Fprintln(state.w)
			fmt.Fprintf(state.w, "  %sNext Recommended Command%s\n", output.ColorBold, output.ColorReset)
			if state.failCount > 0 {
				fmt.Fprintf(state.w, "  %sgetitback doctor%s — check your recovery score and address issues\n",
					output.ColorGreen, output.ColorReset)
			} else if len(state.allSnapshots) > 0 {
				fmt.Fprintf(state.w, "  %sgetitback verify%s — verify snapshot integrity\n",
					output.ColorGreen, output.ColorReset)
			}
			fmt.Fprintln(state.w)

			if state.failCount > 0 {
				return fmt.Errorf("%d modules failed to back up", state.failCount)
			}

			return nil
		},
	}

	cmd.Flags().String("module", "", "Backup a specific module only")
	return cmd
}

func stageHeader(w interface{ Write([]byte) (int, error) }, stage, title string) {
	fmt.Fprintf(w, "\n\n  %s%s%s\n",
		output.ColorBold+output.ColorCyan,
		strings.Repeat("━", 50),
		output.ColorReset,
	)
	fmt.Fprintf(w, "  %sStage %s · %s%s\n",
		output.ColorBold+output.ColorCyan, stage, title, output.ColorReset,
	)
	fmt.Fprintf(w, "  %s%s%s\n",
		output.ColorBold+output.ColorCyan,
		strings.Repeat("━", 50),
		output.ColorReset,
	)
}

func scoreLetter(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 60:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

func recipientFromIdentity(identity string) (string, error) {
	id, err := crypto.ParseIdentity(identity)
	if err != nil {
		return "", err
	}
	return id.Recipient().String(), nil
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, v := range s {
		if seen[v] {
			continue
		}
		seen[v] = true
		result = append(result, v)
	}
	return result
}

func moduleDisplayName(name string) string {
	info := module.GetModuleInfo(name)
	if info != nil {
		return info.Name
	}
	return name
}

func machineID() string {
	data, err := os.ReadFile("/etc/machine-id")
	if err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data))
	}
	data, err = os.ReadFile("/var/lib/dbus/machine-id")
	if err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data))
	}
	return ""
}
