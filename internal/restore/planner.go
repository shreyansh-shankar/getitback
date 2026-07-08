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

	return plan, nil
}
