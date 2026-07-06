package output

import (
	"fmt"
	"io"

	"github.com/shreyansh-shankar/getitback/internal/doctor"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/report"
	"strings"
)

type MarkdownRenderer struct{}

func (r *MarkdownRenderer) RenderInventory(w io.Writer, results []*module.InventoryResult, _ RenderOptions) error {
	for _, res := range results {
		if !res.Detected {
			continue
		}

		fmt.Fprintf(w, "## %s\n\n", res.Module)

		if res.Version != "" {
			fmt.Fprintf(w, "**Version:** %s\n\n", res.Version)
		}

		for k, v := range res.Metadata {
			fmt.Fprintf(w, "- **%s:** %v\n", k, v)
		}

		for _, resource := range res.Resources {
			size := ""
			if resource.Size > 0 {
				size = " (" + formatSize(resource.Size) + ")"
			}
			fmt.Fprintf(w, "- %s%s\n", resource.Path, size)
		}

		if len(res.Errors) > 0 {
			fmt.Fprintln(w)
			for _, err := range res.Errors {
				fmt.Fprintf(w, "> **Error:** %s\n", err)
			}
		}

		fmt.Fprintln(w)
	}
	return nil
}

func (r *MarkdownRenderer) RenderReport(w io.Writer, rep *report.Report) error {
	// Header
	fmt.Fprintf(w, "# Machine Audit Report\n\n")
	fmt.Fprintf(w, "**Hostname:** %s\n\n", rep.Header.Hostname)
	fmt.Fprintf(w, "**Generated:** %s\n\n", rep.Header.GeneratedAt)
	fmt.Fprintf(w, "**Operating System:** %s\n\n", rep.Header.OperatingSystem)
	fmt.Fprintf(w, "**GetItBack Version:** %s\n\n", rep.Header.GetItBackVersion)
	if rep.Header.RecoverySnapshot != "" {
		fmt.Fprintf(w, "**Recovery Snapshot:** %s\n\n", rep.Header.RecoverySnapshot)
	}

	// Summary
	fmt.Fprintf(w, "## Summary\n\n")
	s := rep.Summary
	if s.OperatingSystem != "" {
		fmt.Fprintf(w, "- **Operating System:** %s\n", s.OperatingSystem)
	}
	if s.PrimaryShell != "" {
		fmt.Fprintf(w, "- **Primary Shell:** %s\n", s.PrimaryShell)
	}
	if s.PrimaryEditor != "" {
		fmt.Fprintf(w, "- **Primary Editor:** %s\n", s.PrimaryEditor)
	}
	if s.PrimaryBrowser != "" {
		fmt.Fprintf(w, "- **Primary Browser:** %s\n", s.PrimaryBrowser)
	}
	if len(s.Languages) > 0 {
		fmt.Fprintf(w, "- **Languages:** %s\n", strings.Join(s.Languages, ", "))
	}
	if len(s.Databases) > 0 {
		fmt.Fprintf(w, "- **Databases:** %s\n", strings.Join(s.Databases, ", "))
	}
	if s.Containers > 0 {
		fmt.Fprintf(w, "- **Containers:** %d\n", s.Containers)
	}
	if s.DockerVolumes > 0 {
		fmt.Fprintf(w, "- **Docker Volumes:** %d\n", s.DockerVolumes)
	}
	if s.Repositories > 0 {
		fmt.Fprintf(w, "- **Repositories:** %d\n", s.Repositories)
	}
	if len(s.CloudProviders) > 0 {
		fmt.Fprintf(w, "- **Cloud Providers:** %s\n", strings.Join(s.CloudProviders, ", "))
	}
	if s.BackupSnapshots > 0 {
		fmt.Fprintf(w, "- **Backups Available:** %d snapshots\n", s.BackupSnapshots)
	}
	fmt.Fprintln(w)

	// Machine Profile
	fmt.Fprintf(w, "## Machine Profile\n\n")
	m := rep.Machine
	if m.Architecture != "" {
		fmt.Fprintf(w, "- **Architecture:** %s\n", m.Architecture)
	}
	if m.CPU != "" {
		fmt.Fprintf(w, "- **CPU:** %s\n", m.CPU)
	}
	if m.RAM != "" {
		fmt.Fprintf(w, "- **RAM:** %s\n", m.RAM)
	}
	if m.Storage != "" {
		s := m.Storage
		if m.StorageUsed != "" && m.StorageAvail != "" {
			s += fmt.Sprintf(" (%s used, %s free)", m.StorageUsed, m.StorageAvail)
		}
		fmt.Fprintf(w, "- **Storage:** %s\n", s)
	}
	if m.Kernel != "" {
		fmt.Fprintf(w, "- **Kernel:** %s\n", m.Kernel)
	}
	if m.Desktop != "" {
		fmt.Fprintf(w, "- **Desktop:** %s\n", m.Desktop)
	}
	if m.Session != "" {
		fmt.Fprintf(w, "- **Session:** %s\n", m.Session)
	}
	if m.Timezone != "" {
		fmt.Fprintf(w, "- **Timezone:** %s\n", m.Timezone)
	}
	if m.Locale != "" {
		fmt.Fprintf(w, "- **Locale:** %s\n", m.Locale)
	}
	fmt.Fprintln(w)

	// Development Stack
	fmt.Fprintf(w, "## Development Stack\n\n")

	if len(rep.Development.Languages) > 0 {
		fmt.Fprintf(w, "### Languages\n\n")
		for _, l := range rep.Development.Languages {
			v := ""
			if l.Version != "" {
				v = " (%s)" + l.Version
			}
			fmt.Fprintf(w, "- **%s**%s\n", l.Name, v)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "### Version Control\n\n")
	for _, vc := range rep.Development.VersionControl {
		status := "✓"
		if !vc.Configured {
			status = "∘"
		}
		v := ""
		if vc.Version != "" {
			v = " (" + vc.Version + ")"
		}
		fmt.Fprintf(w, "- %s **%s**%s\n", status, vc.Name, v)
	}
	fmt.Fprintln(w)

	if len(rep.Development.PackageManagers) > 0 {
		fmt.Fprintf(w, "### Package Managers\n\n")
		for _, p := range rep.Development.PackageManagers {
			d := ""
			if p.Count > 0 {
				d = fmt.Sprintf(" (%d packages)", p.Count)
			}
			fmt.Fprintf(w, "- **%s**%s\n", p.Name, d)
		}
		fmt.Fprintln(w)
	}

	// Identity
	if rep.Identity.SSH != nil || rep.Identity.GPG != nil {
		fmt.Fprintf(w, "## Identity\n\n")
		if rep.Identity.SSH != nil {
			fmt.Fprintf(w, "### SSH\n\n")
			fmt.Fprintf(w, "- **Identities:** %d\n", rep.Identity.SSH.IdentityCount)
			fmt.Fprintf(w, "- **Keys:** %d\n", rep.Identity.SSH.Keys)
		}
		if rep.Identity.GPG != nil {
			fmt.Fprintf(w, "### GPG\n\n")
			fmt.Fprintf(w, "- **Keys:** %d\n", rep.Identity.GPG.KeyCount)
		}
		fmt.Fprintln(w)
	}

	// Browsers
	if len(rep.Browsers) > 0 {
		fmt.Fprintf(w, "## Browsers\n\n")
		for _, b := range rep.Browsers {
			fmt.Fprintf(w, "### %s\n\n", b.Name)
			if b.Version != "" {
				fmt.Fprintf(w, "- **Version:** %s\n", b.Version)
			}
			if b.ProfileCount > 0 {
				fmt.Fprintf(w, "- **Profiles:** %d\n", b.ProfileCount)
				if b.DefaultProfile != "" {
					fmt.Fprintf(w, "- **Default:** %s\n", b.DefaultProfile)
				}
			} else {
				fmt.Fprintf(w, "- No profiles created.\n")
			}
			if b.Storage != "" {
				fmt.Fprintf(w, "- **Storage:** %s\n", b.Storage)
			}
			if b.InstallMethod != "" {
				fmt.Fprintf(w, "- **Installed Via:** %s\n", b.InstallMethod)
			}
			fmt.Fprintln(w)
		}
	}

	// Editors
	if len(rep.Editors) > 0 {
		fmt.Fprintf(w, "## Editors\n\n")
		for _, e := range rep.Editors {
			fmt.Fprintf(w, "### %s\n\n", e.Name)
			if e.Version != "" {
				fmt.Fprintf(w, "- **Version:** %s\n", e.Version)
			}
			if e.Extensions > 0 {
				fmt.Fprintf(w, "- **Extensions:** %d\n", e.Extensions)
			}
			if e.Settings > 0 {
				fmt.Fprintf(w, "- **Settings:** %d\n", e.Settings)
			}
			if e.Themes > 0 {
				fmt.Fprintf(w, "- **Themes:** %d\n", e.Themes)
			}
			if e.Snippets > 0 {
				fmt.Fprintf(w, "- **Snippets:** %d\n", e.Snippets)
			}
			fmt.Fprintln(w)
		}
	}

	// Databases
	if len(rep.Databases) > 0 {
		fmt.Fprintf(w, "## Databases\n\n")
		for _, d := range rep.Databases {
			fmt.Fprintf(w, "### %s\n\n", d.Name)
			if d.Version != "" {
				fmt.Fprintf(w, "- **Version:** %s\n", d.Version)
			}
			fmt.Fprintf(w, "- **Installed:** Yes\n")
			if d.ConfigFile != "" {
				fmt.Fprintf(w, "- **Configuration:** %s\n", d.ConfigFile)
			}
			if d.Storage != "" {
				fmt.Fprintf(w, "- **Storage:** %s\n", d.Storage)
			}
			if d.DataDir != "" {
				fmt.Fprintf(w, "- **Data Directory:** %s\n", d.DataDir)
			}
			fmt.Fprintln(w)
		}
	}

	// Docker
	if rep.Containers != nil {
		fmt.Fprintf(w, "## Docker\n\n")
		c := rep.Containers
		if c.Version != "" {
			fmt.Fprintf(w, "- **Version:** %s\n", c.Version)
		}
		fmt.Fprintf(w, "- **Containers:** %d\n", c.Containers)
		fmt.Fprintf(w, "  - Running: %d\n", c.Running)
		fmt.Fprintf(w, "  - Stopped: %d\n", c.Stopped)
		fmt.Fprintf(w, "- **Images:** %d\n", c.Images)
		fmt.Fprintf(w, "- **Volumes:** %d\n", c.Volumes)
		fmt.Fprintf(w, "- **Networks:** %d\n", c.Networks)
		if c.CustomNetworks > 0 {
			fmt.Fprintf(w, "  - Custom: %d\n", c.CustomNetworks)
		}
		fmt.Fprintf(w, "- **Compose Projects:** %d\n", c.ComposeProjects)
		if c.DanglingImages > 0 {
			fmt.Fprintf(w, "- **Dangling Images:** %d\n", c.DanglingImages)
		}
		if c.ImageStorage != "" {
			fmt.Fprintf(w, "- **Image Storage:** %s\n", c.ImageStorage)
		}
		if c.VolumeStorage != "" {
			fmt.Fprintf(w, "- **Volume Storage:** %s\n", c.VolumeStorage)
		}
		if c.RootDir != "" {
			fmt.Fprintf(w, "- **Root Directory:** %s\n", c.RootDir)
		}
		fmt.Fprintf(w, "- **Rootless:** %v\n", c.Rootless)
		fmt.Fprintln(w)
	}

	// Cloud
	if rep.Cloud != nil {
		fmt.Fprintf(w, "## Cloud Accounts\n\n")
		for _, p := range rep.Cloud.Providers {
			fmt.Fprintf(w, "### %s\n\n", p.Name)
			fmt.Fprintf(w, "- **CLI Installed:** Yes\n")
			if p.AccountID != "" {
				fmt.Fprintf(w, "- **Account:** %s\n", p.AccountID)
			}
			fmt.Fprintf(w, "- **Authenticated:** %v\n", p.Authenticated)
			fmt.Fprintf(w, "- **Credentials:** %v\n", p.Credentials)
			fmt.Fprintln(w)
		}
	}

	// Infrastructure
	if rep.Infra != nil {
		fmt.Fprintf(w, "## Infrastructure\n\n")
		if len(rep.Infra.Tools) > 0 {
			fmt.Fprintf(w, "**Tools:** %s\n\n", strings.Join(rep.Infra.Tools, ", "))
		}
		if rep.Infra.Kubernetes != nil {
			k := rep.Infra.Kubernetes
			fmt.Fprintf(w, "### Kubernetes\n\n")
			if k.Version != "" {
				fmt.Fprintf(w, "- **Version:** %s\n", k.Version)
			}
			if k.CurrentContext != "" {
				fmt.Fprintf(w, "- **Current Context:** %s\n", k.CurrentContext)
			}
			if len(k.Contexts) > 0 {
				fmt.Fprintf(w, "- **Contexts:** %s\n", strings.Join(k.Contexts, ", "))
			}
			if len(k.Namespaces) > 0 {
				fmt.Fprintf(w, "- **Namespaces:** %s\n", strings.Join(k.Namespaces, ", "))
			}
			if len(k.HelmRepos) > 0 {
				fmt.Fprintf(w, "- **Helm Repos:** %s\n", strings.Join(k.HelmRepos, ", "))
			}
		}
		fmt.Fprintln(w)
	}

	// Packages
	if rep.Packages.Apt != nil || rep.Packages.Snap != nil || rep.Packages.Flatpak != nil {
		fmt.Fprintf(w, "## Packages\n\n")
		for _, pm := range []*report.PackageManagerInfo{rep.Packages.Apt, rep.Packages.Snap, rep.Packages.Flatpak} {
			if pm == nil {
				continue
			}
			fmt.Fprintf(w, "### %s\n\n", pm.Name)
			if pm.Version != "" {
				fmt.Fprintf(w, "- **Version:** %s\n", pm.Version)
			}
			fmt.Fprintf(w, "- **Packages:** %d\n", pm.Count)
			fmt.Fprintln(w)
		}
	}

	// Security
	if rep.Security != nil {
		fmt.Fprintf(w, "## Certificates\n\n")
		fmt.Fprintf(w, "- **Certificate Stores:** %d\n", rep.Security.CertStores)
		fmt.Fprintf(w, "- **Valid:** %d\n", rep.Security.ValidCerts)
		fmt.Fprintf(w, "- **Expiring:** %d\n", rep.Security.Expiring)
		fmt.Fprintf(w, "- **Expired:** %d\n", rep.Security.Expired)
		fmt.Fprintf(w, "- **Custom CA Bundles:** %d\n", rep.Security.CABundles)
		fmt.Fprintln(w)
	}

	// Projects
	if rep.Projects != nil {
		fmt.Fprintf(w, "## Repositories\n\n")
		fmt.Fprintf(w, "- **Total:** %d\n", rep.Projects.TotalRepos)
		if rep.Projects.DirtyRepos > 0 {
			fmt.Fprintf(w, "- **Uncommitted Changes:** %d\n", rep.Projects.DirtyRepos)
		}
		if rep.Projects.NoRemote > 0 {
			fmt.Fprintf(w, "- **Without Remote:** %d\n", rep.Projects.NoRemote)
		}
		if rep.Projects.GitHubRepos > 0 {
			fmt.Fprintf(w, "- **GitHub:** %d\n", rep.Projects.GitHubRepos)
		}
		if rep.Projects.GitLabRepos > 0 {
			fmt.Fprintf(w, "- **GitLab:** %d\n", rep.Projects.GitLabRepos)
		}
		if rep.Projects.LocalOnly > 0 {
			fmt.Fprintf(w, "- **Local Only:** %d\n", rep.Projects.LocalOnly)
		}
		fmt.Fprintln(w)
	}

	// Virtualization
	if rep.Virtualization != nil {
		fmt.Fprintf(w, "## Virtualization\n\n")
		fmt.Fprintf(w, "- **Platforms:** %s\n", strings.Join(rep.Virtualization.Platforms, ", "))
		fmt.Fprintln(w)
	}

	// Backups
	fmt.Fprintf(w, "## Backup Summary\n\n")
	b := rep.Backups
	if b.LatestSnapshot != "" {
		fmt.Fprintf(w, "- **Latest Snapshot:** %s\n", b.LatestSnapshot)
	}
	if b.CreatedAt != "" {
		fmt.Fprintf(w, "- **Created:** %s\n", b.CreatedAt)
	}
	fmt.Fprintf(w, "- **Snapshots:** %d\n", b.SnapshotCount)
	if b.TotalSize != "" {
		fmt.Fprintf(w, "- **Total Backup Size:** %s\n", b.TotalSize)
	}
	fmt.Fprintf(w, "- **Encryption:** %s\n", b.Encryption)
	fmt.Fprintf(w, "- **Storage Provider:** %s\n", b.StorageProvider)
	fmt.Fprintln(w)

	// Statistics
	fmt.Fprintf(w, "## Statistics\n\n")
	stats := rep.Statistics
	writeMDStat(w, "Languages", stats.Languages)
	writeMDStat(w, "Browsers", stats.Browsers)
	writeMDStat(w, "Editors", stats.Editors)
	writeMDStat(w, "Databases", stats.Databases)
	writeMDStat(w, "Containers", stats.Containers)
	writeMDStat(w, "Docker Volumes", stats.DockerVolumes)
	writeMDStat(w, "Compose Projects", stats.ComposeProjects)
	writeMDStat(w, "Repositories", stats.Repositories)
	writeMDStat(w, "Certificates", stats.Certificates)
	writeMDStat(w, "SSH Keys", stats.SSHKeys)
	writeMDStat(w, "GPG Keys", stats.GPGKeys)
	writeMDStat(w, "Cloud Providers", stats.CloudProviders)
	fmt.Fprintln(w)

	// Not Detected
	if len(rep.NotDetected) > 0 {
		fmt.Fprintf(w, "## Not Detected\n\n")
		for _, m := range rep.NotDetected {
			fmt.Fprintf(w, "- %s\n", m)
		}
		fmt.Fprintln(w)
	}

	// Coverage
	fmt.Fprintf(w, "## Inventory Coverage\n\n")
	c := rep.Coverage
	fmt.Fprintf(w, "- **Detected Modules:** %d\n", c.DetectedModules)
	fmt.Fprintf(w, "- **Total Modules:** %d\n", c.TotalModules)
	if c.MissingModules > 0 {
		fmt.Fprintf(w, "- **Missing:** %d\n", c.MissingModules)
	}
	fmt.Fprintf(w, "- **Coverage:** %d%%\n", c.CoveragePercent)
	fmt.Fprintln(w)

	// Metadata
	fmt.Fprintf(w, "## Report Metadata\n\n")
	md := rep.Meta
	fmt.Fprintf(w, "- **Generated By:** %s\n", md.GeneratedBy)
	fmt.Fprintf(w, "- **Version:** %s\n", md.Version)
	fmt.Fprintf(w, "- **Generated At:** %s\n", md.GeneratedAt)
	if md.MachineID != "" {
		fmt.Fprintf(w, "- **Machine ID:** %s\n", md.MachineID)
	}
	if md.Checksum != "" {
		fmt.Fprintf(w, "- **Checksum:** %s\n", md.Checksum)
	}
	fmt.Fprintf(w, "- **Report Format:** %s\n", md.Format)
	fmt.Fprintln(w)

	return nil
}

func writeMDStat(w io.Writer, label string, value int) {
	if value == 0 {
		return
	}
	fmt.Fprintf(w, "- **%s:** %d\n", label, value)
}

func (r *MarkdownRenderer) RenderDoctor(w io.Writer, report *doctor.Report) error {
	fmt.Fprintf(w, "# Recovery Assessment\n\n")
	fmt.Fprintf(w, "**Recovery Confidence:** %d%% (%s)\n\n", report.Confidence.Score, report.Confidence.Grade)
	fmt.Fprintf(w, "%s\n\n", report.Confidence.Message)

	fmt.Fprintf(w, "## Risks\n\n")
	for _, risk := range report.Risks {
		fmt.Fprintf(w, "- **[%s]** %s\n", risk.Severity, risk.Message)
		if risk.Impact != "" {
			fmt.Fprintf(w, "  - Impact: %s\n", risk.Impact)
		}
		if risk.Command != "" {
			fmt.Fprintf(w, "  - Fix: `%s`\n", risk.Command)
		}
	}

	fmt.Fprintf(w, "\n## Backup Coverage\n\n")
	fmt.Fprintf(w, "Protected: %d, Unprotected: %d, Coverage: %d%%\n",
		report.Coverage.ProtectedCount, report.Coverage.UnprotectedCount, report.Coverage.CoveragePercent)

	fmt.Fprintf(w, "\n## Estimated Restore Time\n\n")
	fmt.Fprintf(w, "%s\n", report.Timeline.Total)

	fmt.Fprintf(w, "\n## Summary\n\n")
	fmt.Fprintf(w, "%s\n", report.Summary)
	return nil
}
