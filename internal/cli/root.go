package cli

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/spf13/cobra"
)

func Execute(manager *module.Manager) {
	cfg, err := config.Load()
	if err != nil {
		cobra.CheckErr(err)
	}

	rootCmd := &cobra.Command{
		Use:   "getitback",
		Short: "Developer workstation recovery platform",
		Long: `getitback is a disaster recovery and machine bootstrap tool for developers.

It discovers, backs up, restores, and verifies your entire development environment.`,
	}

	rootCmd.PersistentFlags().StringP("output", "o", "terminal",
		"Output format (terminal, json, yaml, markdown)")

	rootCmd.AddCommand(newInventoryCmd(cfg, manager))
	rootCmd.AddCommand(newBackupCmd(cfg, manager))
	rootCmd.AddCommand(newRestoreCmd(cfg, manager))
	rootCmd.AddCommand(newVerifyCmd(cfg, manager))
	rootCmd.AddCommand(newDoctorCmd(cfg, manager))
	rootCmd.AddCommand(newReportCmd(cfg, manager))
	rootCmd.AddCommand(newStatusCmd(cfg, manager))
	rootCmd.AddCommand(newModulesCmd(cfg, manager))
	rootCmd.AddCommand(newSecretsCmd(cfg, manager))

	cobra.CheckErr(rootCmd.Execute())
}

func outputFormat(cmd *cobra.Command) output.Format {
	s, _ := cmd.Flags().GetString("output")
	return output.ParseFormat(s)
}

func fileChecksum(path string) (string, error) {
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

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
