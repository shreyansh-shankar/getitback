package restore

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/storage"
)

type Planner struct {
	manager   *module.Manager
	backupDir string
}

func NewPlanner(manager *module.Manager, backupDir string) *Planner {
	return &Planner{manager: manager, backupDir: backupDir}
}

func (p *Planner) LoadManifest() (*storage.Manifest, error) {
	return storage.ReadManifest(p.backupDir)
}

func (p *Planner) AvailableModules(manifest *storage.Manifest) []string {
	registered := make(map[string]bool)
	for _, mod := range p.manager.All() {
		registered[mod.Name()] = true
	}

	var available []string
	for _, snap := range manifest.Snapshots {
		if registered[snap.Module] {
			available = append(available, snap.Module)
		}
	}
	return available
}

func (p *Planner) BuildPlan(ctx context.Context, manifest *storage.Manifest, selected []string) (*RestorePlan, error) {
	resolver := NewDependencyResolver(p.manager, selected)
	order, deps, manualSteps, err := resolver.Resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("dependency resolution: %w", err)
	}

	plan := &RestorePlan{
		BackupDir:    p.backupDir,
		Manifest:     manifest,
		SnapshotsDir: filepath.Join(p.backupDir, "snapshots"),
		Selected:     order,
		Deps:         deps,
		ManualSteps:  manualSteps,
	}

	plan.Execution = []ExecutionGroup{
		{Phase: module.PhaseInstall, Modules: order, Parallel: false},
		{Phase: module.PhaseRestore, Modules: order, Parallel: false},
		{Phase: module.PhaseConfigure, Modules: order, Parallel: false},
		{Phase: module.PhaseValidate, Modules: order, Parallel: false},
	}

	// Populate dry-run info
	plan.DryRunInfo = p.buildDryRunInfo(manifest, order, deps)

	return plan, nil
}

func (p *Planner) buildDryRunInfo(manifest *storage.Manifest, selected []string, deps []module.Dependency) *DryRunInfo {
	info := &DryRunInfo{
		ModuleDetails: make(map[string]string),
	}

	var totalSize int64
	for _, snap := range manifest.Snapshots {
		for _, name := range selected {
			if snap.Module == name {
				totalSize += snap.Size
				desc := ""
				if mod, ok := p.manager.Get(name); ok {
					desc = mod.Description()
				}
				info.ModuleDetails[name] = desc
				break
			}
		}
	}

	for _, dep := range deps {
		switch dep.Type {
		case module.DepSystemPkg:
			info.Packages = append(info.Packages, dep.Package)
		case module.DepDownload:
			hint := dep.Hint
			if hint == "" {
				hint = dep.Package
			}
			if hint == "" {
				hint = "download"
			}
			info.Downloads = append(info.Downloads, hint)
		}
	}

	// Get services for selected modules
	for _, name := range selected {
		if svc, ok := moduleServices[name]; ok {
			info.Services = append(info.Services, svc)
		} else {
			for key, service := range moduleServices {
				if filepath.Base(name) == key {
					info.Services = append(info.Services, service)
				}
			}
		}
	}

	// Count files in snapshots
	var fileCount int
	for _, snap := range manifest.Snapshots {
		for _, name := range selected {
			if snap.Module == name {
				fileCount += snap.FileCount
				break
			}
		}
	}

	info.DiskUsage = fmt.Sprintf("%.1f MB across %d files", float64(totalSize)/1024/1024, fileCount)

	return info
}

func formatDryRunInfo(info *DryRunInfo) string {
	var result string

	if len(info.Packages) > 0 {
		result += fmt.Sprintf("  Packages to install: %d\n", len(info.Packages))
		for _, pkg := range info.Packages {
			result += fmt.Sprintf("    - %s\n", pkg)
		}
	}

	if len(info.Downloads) > 0 {
		result += fmt.Sprintf("  Downloads: %d\n", len(info.Downloads))
		for _, d := range info.Downloads {
			result += fmt.Sprintf("    - %s\n", d)
		}
	}

	if len(info.Services) > 0 {
		result += fmt.Sprintf("  Services to manage: %d\n", len(info.Services))
		for _, svc := range info.Services {
			result += fmt.Sprintf("    - %s\n", svc)
		}
	}

	if len(info.ModuleDetails) > 0 {
		result += fmt.Sprintf("  Modules to restore: %d\n", len(info.ModuleDetails))
		for name, desc := range info.ModuleDetails {
			d := ""
			if desc != "" {
				d = " — " + desc
			}
			result += fmt.Sprintf("    - %s%s\n", name, d)
		}
	}

	if info.DiskUsage != "" {
		result += fmt.Sprintf("  Estimated disk usage: %s\n", info.DiskUsage)
	}
	if info.DownloadSize != "" && info.DownloadSize != "unknown" {
		result += fmt.Sprintf("  Estimated download size: %s\n", info.DownloadSize)
	}
	if info.EstimatedTime != "" {
		result += fmt.Sprintf("  Estimated restore time: %s\n", info.EstimatedTime)
	}

	return result
}
