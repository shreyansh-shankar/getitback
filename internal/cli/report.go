package cli

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/shreyansh-shankar/getitback/internal/config"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/storage"
	"github.com/spf13/cobra"
)

type machineReport struct {
	GeneratedAt time.Time              `json:"generated_at" yaml:"generated_at"`
	Hostname    string                 `json:"hostname" yaml:"hostname"`
	System      map[string]any         `json:"system" yaml:"system"`
	Modules     []moduleReport         `json:"modules" yaml:"modules"`
	Backups     storage.BackupEntry    `json:"latest_backup,omitempty" yaml:"latest_backup,omitempty"`
}

type moduleReport struct {
	Name        string                   `json:"name" yaml:"name"`
	Detected    bool                     `json:"detected" yaml:"detected"`
	Version     string                   `json:"version,omitempty" yaml:"version,omitempty"`
	Description string                   `json:"description" yaml:"description"`
	Inventory   *module.InventoryResult  `json:"inventory,omitempty" yaml:"inventory,omitempty"`
}

func newReportCmd(cfg *config.Config, manager *module.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Generate a human-readable machine report",
		Long:  "Generate a detailed report of your development machine configuration.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			hostname, _ := os.Hostname()

			detected := manager.Detect(ctx)
			var reports []moduleReport

			for _, mod := range manager.All() {
				d := detected[mod.Name()]
				mr := moduleReport{
					Name:        mod.Name(),
					Detected:    d.Detected,
					Description: mod.Description(),
				}

				if d.Detected {
					inv, err := mod.Inventory(ctx)
					if err == nil && inv != nil {
						mr.Version = inv.Version
						mr.Inventory = inv
					}
				}
				reports = append(reports, mr)
			}

			sort.Slice(reports, func(i, j int) bool {
				return reports[i].Name < reports[j].Name
			})

			report := machineReport{
				GeneratedAt: time.Now(),
				Hostname:    hostname,
				System:      collectSystemInfo(),
				Modules:     reports,
			}

			if backups, err := storage.ListBackups(cfg.Storage.Path); err == nil && len(backups) > 0 {
				report.Backups = backups[0]
			}

			format := outputFormat(cmd)
			renderReport(cmd, format, &report)

			return nil
		},
	}
}

func collectSystemInfo() map[string]any {
	info := make(map[string]any)

	info["hostname"], _ = os.Hostname()

	if kernel, err := exec.Command("uname", "-r").Output(); err == nil {
		info["kernel"] = strings.TrimSpace(string(kernel))
	}

	if cpu, err := exec.Command("nproc").Output(); err == nil {
		info["cpu_cores"] = strings.TrimSpace(string(cpu))
	}

	if mem, err := exec.Command("free", "-h").Output(); err == nil {
		lines := strings.Split(string(mem), "\n")
		if len(lines) > 1 {
			fields := strings.Fields(lines[1])
			if len(fields) > 1 {
				info["memory"] = fields[1]
			}
		}
	}

	if disk, err := exec.Command("df", "-h", "/").Output(); err == nil {
		lines := strings.Split(string(disk), "\n")
		if len(lines) > 1 {
			fields := strings.Fields(lines[1])
			if len(fields) > 3 {
				info["disk_total"] = fields[1]
				info["disk_used"] = fields[2]
				info["disk_avail"] = fields[3]
			}
		}
	}

	f, err := os.Open("/etc/os-release")
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				info["os"] = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			}
		}
	}

	if info["os"] == nil {
		info["os"] = "unknown"
	}

	return info
}

func renderReport(cmd *cobra.Command, format output.Format, report *machineReport) {
	switch format {
	case output.FormatJSON:
		renderJSONReport(cmd, report)
	case output.FormatYAML:
		renderYAMLReport(cmd, report)
	case output.FormatMarkdown:
		renderMarkdownReport(cmd, report)
	default:
		renderTerminalReport(cmd, report)
	}
}

func renderTerminalReport(cmd *cobra.Command, report *machineReport) {
	cmd.Print(output.SectionHeader("Machine Report: " + report.Hostname))

	// System section
	cmd.Printf("  %sSystem%s\n", output.ColorBold+output.ColorBlue, output.ColorReset)
	for k, v := range report.System {
		cmd.Printf("    %s%s:%s %v\n", output.ColorBlue, k, output.ColorReset, v)
	}
	cmd.Println()

	// Modules sections
	grouped := groupModules(report.Modules)
	for groupName, mods := range grouped {
		cmd.Printf("  %s%s%s\n", output.ColorBold+output.ColorBlue, groupName, output.ColorReset)

		for _, m := range mods {
			if m.Detected {
				version := ""
				if m.Version != "" {
					version = "  " + output.ColorCyan + m.Version + output.ColorReset
				}
				cmd.Printf("    %s✓%s %s%s%s%s\n",
					output.ColorGreen, output.ColorReset,
					output.ColorBold, m.Name, output.ColorReset, version)
				if m.Inventory != nil {
					resourceCount := len(m.Inventory.Resources)
					if resourceCount > 0 {
						cmd.Printf("        %s•%s %d resources\n",
							output.ColorCyan, output.ColorReset, resourceCount)
					}
					if m.Inventory.Metadata != nil {
						for k, v := range m.Inventory.Metadata {
							display := formatMetaValue(v)
							cmd.Printf("        %s•%s %s: %s\n",
								output.ColorCyan, output.ColorReset, k, display)
						}
					}
				}
			} else {
				cmd.Printf("    %s  %s(not detected)%s\n",
					m.Name, output.ColorCyan, output.ColorReset)
			}
		}
		cmd.Println()
	}

	// Backups section
	cmd.Printf("  %sBackups%s\n", output.ColorBold+output.ColorBlue, output.ColorReset)
	if report.Backups.ID != "" {
		cmd.Printf("    %s✓%s Latest: %s\n",
			output.ColorGreen, output.ColorReset, report.Backups.ID)
		cmd.Printf("    %sCreated:%s %s\n",
			output.ColorBlue, output.ColorReset, report.Backups.CreatedAt.Format("Jan 2, 2006 15:04 UTC"))
		cmd.Printf("    %sSnapshots:%s %d\n",
			output.ColorBlue, output.ColorReset, report.Backups.SnapshotCount)
		cmd.Printf("    %sSize:%s %s\n",
			output.ColorBlue, output.ColorReset, formatBytes(report.Backups.Size))
	} else {
		cmd.Printf("    %s⚠%s No backups found\n",
			output.ColorYellow, output.ColorReset)
	}
}

func renderJSONReport(cmd *cobra.Command, report *machineReport) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		cmd.PrintErrf("error: %v\n", err)
		return
	}
	cmd.Println(string(data))
}

func renderYAMLReport(cmd *cobra.Command, report *machineReport) {
	data, err := yaml.Marshal(report)
	if err != nil {
		cmd.PrintErrf("error: %v\n", err)
		return
	}
	cmd.Println(string(data))
}

func renderMarkdownReport(cmd *cobra.Command, report *machineReport) {
	cmd.Printf("# Machine Report: %s\n\n", report.Hostname)
	cmd.Printf("Generated: %s\n\n", report.GeneratedAt.Format("Jan 2, 2006 15:04 UTC"))

	cmd.Printf("## System\n\n")
	for k, v := range report.System {
		cmd.Printf("- **%s:** %v\n", k, v)
	}
	cmd.Println()

	grouped := groupModules(report.Modules)
	for groupName, mods := range grouped {
		cmd.Printf("## %s\n\n", groupName)

		for _, m := range mods {
			if m.Detected {
				version := ""
				if m.Version != "" {
					version = " (%s)" + m.Version
				}
				cmd.Printf("- **%s**%s\n", m.Name, version)
			} else {
				cmd.Printf("- %s (not detected)\n", m.Name)
			}
		}
		cmd.Println()
	}
}

func groupModules(modules []moduleReport) map[string][]moduleReport {
	groups := map[string]string{
		"system": "System",
		"git":    "Development", "ssh": "Development", "golang": "Development",
		"nodejs": "Development", "python": "Development", "rust": "Development",
		"vscode": "Editors", "neovim": "Editors",
		"firefox": "Browsers", "chromium": "Browsers",
		"chrome": "Browsers", "brave": "Browsers", "vivaldi": "Browsers", "edge": "Browsers", "opera": "Browsers",
		"postgres": "Databases", "mongodb": "Databases", "redis": "Databases", "sqlite": "Databases",
		"shell":   "Configuration", "dotfiles": "Configuration",
		"apt": "Packages", "snap": "Packages", "flatpak": "Packages",
		"gpg": "Security",
	}

	result := make(map[string][]moduleReport)
	for _, m := range modules {
		group := groups[m.Name]
		if group == "" {
			group = "Other"
		}
		result[group] = append(result[group], m)
	}

	// Sort within each group
	for _, mods := range result {
		sort.Slice(mods, func(i, j int) bool {
			return mods[i].Name < mods[j].Name
		})
	}

	return result
}


