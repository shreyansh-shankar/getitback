package restore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/runtime/actions"
)

type PhaseExecutor struct {
	manager  *module.Manager
	plan     *RestorePlan
	progress *ProgressReporter
	results  []ModuleResult
	rt       *runtime.Runtime
	dryRun   bool
	workDir  string
}

func NewPhaseExecutor(manager *module.Manager, plan *RestorePlan, progress *ProgressReporter, rt *runtime.Runtime, dryRun bool, workDir string) *PhaseExecutor {
	return &PhaseExecutor{
		manager:  manager,
		plan:     plan,
		progress: progress,
		rt:       rt,
		dryRun:   dryRun,
		workDir:  workDir,
	}
}

func (e *PhaseExecutor) baseOpts() module.RestoreOptions {
	return module.RestoreOptions{
		SnapshotsDir:    e.plan.SnapshotsDir,
		BackupDir:       e.plan.BackupDir,
		WorkDir:         e.workDir,
		Runtime:         e.rt,
		OverwritePolicy: module.OverwritePolicyBackup,
	}
}

func (e *PhaseExecutor) ExecuteInstallPhase(ctx context.Context) {
	if e.dryRun {
		return
	}
	e.progress.Stage("4 / 10", "Installing Required Software")

	var pkgs []string
	var downloads []module.Dependency

	for _, dep := range e.plan.Deps {
		switch dep.Type {
		case module.DepSystemPkg:
			pkg := dep.Package
			if pkg == "" {
				continue
			}
			if !e.rt.Pkg.IsInstalled(pkg) {
				pkgs = append(pkgs, pkg)
				e.progress.DetailLine("  %s needs installation", pkg)
			}
		case module.DepDownload:
			downloads = append(downloads, dep)
		}
	}

	if len(pkgs) > 0 {
		e.progress.InfoLine("Installing %d system package(s)...", len(pkgs))
		pkgStr := strings.Join(pkgs, ", ")
		e.progress.DetailLine("  Packages: %s", pkgStr)
		if err := e.rt.Pkg.Install(pkgs...); err != nil {
			e.progress.InfoLine("Package installation failed: %v", err)
		} else {
			for _, pkg := range pkgs {
				e.progress.ModuleSuccess("install", pkg, "via apt")
			}
		}
	}

	for _, dep := range downloads {
		e.progress.ModuleSuccess("install", dep.Hint, "")
	}

	for _, name := range e.plan.Selected {
		mod, ok := e.manager.Get(name)
		if !ok {
			continue
		}
		inst, ok := mod.(module.Installer)
		if !ok {
			continue
		}
		e.progress.InfoLine("Setting up %s...", name)
		opts := e.baseOpts()
		if err := inst.Install(ctx, opts); err != nil {
			e.progress.ModuleFailure("install", name, err)
			e.results = append(e.results, ModuleResult{Module: name, Phase: "install", Status: "failed", Error: err.Error()})
		} else {
			e.progress.ModuleSuccess("install", name, "")
			e.results = append(e.results, ModuleResult{Module: name, Phase: "install", Status: "success"})
		}
	}
}

func (e *PhaseExecutor) ExecuteRestorePhase(ctx context.Context) {
	if e.dryRun {
		return
	}
	e.progress.Stage("5 / 10", "Restoring Data")

	for _, name := range e.plan.Selected {
		mod, ok := e.manager.Get(name)
		if !ok {
			e.progress.ModuleSkip("restore", name, "module not registered")
			e.results = append(e.results, ModuleResult{Module: name, Phase: "restore", Status: "skipped", Error: "module not registered"})
			continue
		}

		var snap *module.Snapshot
		for i := range e.plan.Manifest.Snapshots {
			if e.plan.Manifest.Snapshots[i].Module == name {
				snap = &e.plan.Manifest.Snapshots[i]
				break
			}
		}
		if snap == nil {
			e.progress.ModuleSkip("restore", name, "no snapshot in backup")
			e.results = append(e.results, ModuleResult{Module: name, Phase: "restore", Status: "skipped", Error: "no snapshot"})
			continue
		}

		e.progress.DetailLine("  %s...", name)
		opts := e.baseOpts()

		if p, ok := mod.(actions.Provider); ok {
			e.executeWithActions(ctx, p, name, *snap, opts)
		} else if err := mod.Restore(ctx, *snap, opts); err != nil {
			e.progress.ModuleFailure("restore", name, err)
			e.results = append(e.results, ModuleResult{Module: name, Phase: "restore", Status: "failed", Error: err.Error()})
		} else {
			e.progress.ModuleSuccess("restore", name, "")
			e.results = append(e.results, ModuleResult{Module: name, Phase: "restore", Status: "success"})
		}
	}
}

func (e *PhaseExecutor) executeWithActions(ctx context.Context, p actions.Provider, name string, snap module.Snapshot, opts module.RestoreOptions) {
	actList, err := p.Actions(ctx, snap, opts)
	if err != nil {
		e.progress.ModuleFailure("restore", name, fmt.Errorf("build actions: %w", err))
		e.results = append(e.results, ModuleResult{Module: name, Phase: "restore", Status: "failed", Error: err.Error()})
		return
	}

	if len(actList) == 0 {
		e.progress.ModuleSkip("restore", name, "no actions returned")
		e.results = append(e.results, ModuleResult{Module: name, Phase: "restore", Status: "skipped"})
		return
	}

	if e.dryRun {
		e.executeDryRun(name, actList)
		return
	}

	restoreCtx := runtime.NewRestoreContext(ctx, e.rt, e.plan.Manifest, snap, e.plan.Selected, opts)
	executor := actions.NewExecutor(&restoreCtx, actList)
	report, err := executor.Execute()

	metrics := executor.Metrics().Results()
	duration := report.Duration
	detail := fmt.Sprintf("%d actions in %s", len(actList), duration)
	if report.Failed > 0 {
		var sb strings.Builder
		for _, fa := range report.FailedActions {
			if sb.Len() > 0 {
				sb.WriteString("; ")
			}
			sb.WriteString(fa.Name)
			if fa.Error != "" {
				sb.WriteString(": " + fa.Error)
			}
		}
		e.progress.ModuleFailure("restore", name, fmt.Errorf("%s", sb.String()))
		e.results = append(e.results, ModuleResult{
			Module: name, Phase: "restore", Status: "failed",
			Error: sb.String(), Details: metricsDetails(metrics),
		})
	} else if report.Skipped > 0 {
		e.progress.ModuleSkip("restore", name, detail)
		e.results = append(e.results, ModuleResult{
			Module: name, Phase: "restore", Status: "skipped",
			Details: append([]string{detail}, metricsDetails(metrics)...),
		})
	} else {
		e.progress.ModuleSuccess("restore", name, detail)
		e.results = append(e.results, ModuleResult{
			Module: name, Phase: "restore", Status: "success",
			Details: append([]string{detail}, metricsDetails(metrics)...),
		})
	}
	_ = err
}

func (e *PhaseExecutor) executeDryRun(name string, actList []actions.Action) {
	e.progress.InfoLine("%s:", name)
	for _, a := range actList {
		e.progress.DetailLine("  %s", a.Description())
	}
	var total time.Duration
	for _, a := range actList {
		total += a.EstimatedDuration()
	}
	e.progress.DetailLine("  Estimated time: %s", total.Round(time.Second))
	e.results = append(e.results, ModuleResult{
		Module: name, Phase: "restore", Status: "dry_run",
	})
}

func metricsDetails(metrics []actions.ActionMetrics) []string {
	var details []string
	for _, m := range metrics {
		if m.Status == actions.StatusFailed {
			details = append(details, m.Name+": "+m.Error)
		}
	}
	return details
}

func (e *PhaseExecutor) ExecuteConfigurePhase(ctx context.Context) {
	if e.dryRun {
		return
	}
	e.progress.Stage("6 / 10", "Post-Restore Configuration")

	for _, name := range e.plan.Selected {
		mod, ok := e.manager.Get(name)
		if !ok {
			continue
		}
		conf, ok := mod.(module.Configurer)
		if !ok {
			continue
		}

		opts := e.baseOpts()
		if err := conf.Configure(ctx, opts); err != nil {
			e.progress.ModuleFailure("configure", name, err)
			e.results = append(e.results, ModuleResult{Module: name, Phase: "configure", Status: "failed", Error: err.Error()})
		} else {
			e.progress.ModuleSuccess("configure", name, "")
			e.results = append(e.results, ModuleResult{Module: name, Phase: "configure", Status: "success"})
		}
	}
}

func (e *PhaseExecutor) ExecuteValidatePhase(ctx context.Context) {
	if e.dryRun {
		return
	}
	e.progress.Stage("8 / 10", "Validation")

	for _, name := range e.plan.Selected {
		mod, ok := e.manager.Get(name)
		if !ok {
			continue
		}
		val, ok := mod.(module.Validator)
		if !ok {
			continue
		}

		var snap module.Snapshot
		for _, s := range e.plan.Manifest.Snapshots {
			if s.Module == name {
				snap = s
				break
			}
		}

		result, err := val.Validate(ctx, snap)
		if err != nil {
			e.progress.ModuleFailure("validate", name, err)
			e.results = append(e.results, ModuleResult{Module: name, Phase: "validate", Status: "failed", Error: err.Error()})
			continue
		}
		if result == nil {
			continue
		}

		if result.Success {
			detail := result.Version
			if detail == "" && len(result.Checks) > 0 {
				detail = strings.Join(result.Checks, ", ")
			}
			e.progress.ModuleSuccess("validate", name, detail)
			modResult := ModuleResult{Module: name, Phase: "validate", Status: "success", Details: result.Checks, Validation: result}
			if len(result.Warnings) > 0 {
				modResult.Status = "warning"
				modResult.Details = append(modResult.Details, result.Warnings...)
			}
			e.results = append(e.results, modResult)
		} else {
			errMsg := strings.Join(result.Errors, "; ")
			e.progress.ModuleFailure("validate", name, fmt.Errorf("%s", errMsg))
			e.results = append(e.results, ModuleResult{Module: name, Phase: "validate", Status: "failed", Error: errMsg, Validation: result})
		}
	}
}

// services that may need starting after a restore, mapped by module name
var moduleServices = map[string]string{
	"docker":     "docker",
	"postgres":   "postgresql",
	"mysql":      "mysql",
	"mongodb":    "mongod",
	"redis":      "redis-server",
	"ssh":        "ssh",
	"sshd":       "ssh",
	"system":     "systemd-journald",
	"virt":       "libvirtd",
	"virtualization": "libvirtd",
}

func (e *PhaseExecutor) ExecuteServicePhase(ctx context.Context) {
	if e.dryRun {
		return
	}
	e.progress.Stage("7 / 10", "Service Startup")

	started := 0
	failed := 0
	skipped := 0
	for _, name := range e.plan.Selected {
		svc, ok := moduleServices[name]
		if !ok {
			for key, service := range moduleServices {
				if strings.Contains(name, key) {
					svc = service
					ok = true
					break
				}
			}
		}
		if !ok {
			continue
		}

		// Check if service exists (handles missing systemctl gracefully)
		if !e.rt.Service.Exists(svc) {
			e.progress.ModuleSkip("service", svc, "service unit not found")
			skipped++
			continue
		}

		// Enable the service so it starts on boot
		if err := e.rt.Service.Enable(svc); err != nil {
			e.progress.ModuleWarning("service", svc, fmt.Sprintf("enable failed: %v", err))
		}

		// Start the service
		if err := e.rt.Service.Start(svc); err != nil {
			e.progress.ModuleFailure("service", svc, fmt.Errorf("start failed: %w", err))
			e.results = append(e.results, ModuleResult{Module: svc, Phase: "service", Status: "failed", Error: err.Error()})
			failed++
		} else {
			e.progress.ModuleSuccess("service", svc, "started and enabled")
			e.results = append(e.results, ModuleResult{Module: svc, Phase: "service", Status: "success"})
			started++
		}
	}

	if started == 0 && failed == 0 && skipped == 0 {
		e.progress.InfoLine("No managed services to start.")
	}
}

func (e *PhaseExecutor) Results() []ModuleResult {
	return e.results
}


