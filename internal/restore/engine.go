package restore

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/output"
	"github.com/shreyansh-shankar/getitback/internal/runtime"
	"github.com/shreyansh-shankar/getitback/internal/storage"
)

type Engine struct {
	manager   *module.Manager
	backupDir string
	manifest  *storage.Manifest
	plan      *RestorePlan
	progress  *ProgressReporter
	dryRun    bool
	workDir   string
	results   []ModuleResult
}

func NewEngine(manager *module.Manager, backupDir string) *Engine {
	return &Engine{
		manager:   manager,
		backupDir: backupDir,
	}
}

func (e *Engine) SetManifest(manifest *storage.Manifest) {
	e.manifest = manifest
}

func (e *Engine) SetPlan(plan *RestorePlan) {
	e.plan = plan
}

func (e *Engine) SetDryRun(dryRun bool) {
	e.dryRun = dryRun
}

func (e *Engine) SetWorkDir(workDir string) {
	e.workDir = workDir
}

func (e *Engine) Execute(ctx context.Context, w io.Writer) (*RecoveryReport, error) {
	start := time.Now()

	totalModules := len(e.plan.Selected)
	e.progress = NewProgressReporter(w, totalModules)

	rt := runtime.New(w, nil)
	rt.Progress = e.progress
	executor := NewPhaseExecutor(e.manager, e.plan, e.progress, rt, e.dryRun, e.workDir)

	// Stage 1: Loading
	e.progress.Stage("1 / 10", "Loading Backup")
	e.progress.SummaryLine("ID", e.manifest.BackupID)
	if e.manifest.BackupID == "" {
		e.progress.SummaryLine("Created", e.manifest.CreatedAt.Format("Jan 2, 2006 15:04 UTC"))
	}
	e.progress.SummaryLine("Host", e.manifest.Hostname)
	e.progress.SummaryLine("Modules", fmt.Sprintf("%d available", len(e.plan.Selected)))

	// Stage 2: Plan
	e.progress.Stage("2 / 10", "Restore Plan")
	e.progress.InfoLine("Modules to restore: %d", len(e.plan.Selected))
	for _, name := range e.plan.Selected {
		mod, ok := e.manager.Get(name)
		desc := ""
		if ok {
			desc = mod.Description()
		}
		e.progress.ModuleSuccess("plan", name, desc)
	}
	if len(e.plan.Deps) > 0 {
		sysPkgs := 0
		downloads := 0
		manual := 0
		for _, dep := range e.plan.Deps {
			switch dep.Type {
			case module.DepSystemPkg:
				sysPkgs++
			case module.DepDownload:
				downloads++
			case module.DepManual:
				manual++
			}
		}
		if sysPkgs > 0 {
			e.progress.InfoLine("System packages: %d", sysPkgs)
		}
		if downloads > 0 {
			e.progress.InfoLine("Downloads: %d", downloads)
		}
		if manual > 0 {
			e.progress.InfoLine("Manual steps: %d", manual)
		}
	}

	// Show dry-run details when in dry-run mode
	if e.dryRun && e.plan.DryRunInfo != nil {
		e.progress.InfoLine("Dry-run details:")
		fmt.Fprint(w, formatDryRunInfo(e.plan.DryRunInfo))
	}

	// Stage 3: Dependency Resolution
	e.progress.Stage("3 / 10", "Dependency Resolution")
	for _, dep := range e.plan.Deps {
		switch dep.Type {
		case module.DepModule:
			e.progress.ModuleSuccess("deps", dep.Module, "dependency")
		case module.DepSystemPkg:
			status := "required"
			if dep.Optional {
				status = "optional"
			}
			e.progress.ModuleSuccess("deps", dep.Package, fmt.Sprintf("system package (%s)", status))
		case module.DepManual:
			e.progress.ModuleWarning("deps", dep.Message, "manual step required")
		}
	}

	// Stages 4-8: Execute phases (or print plan in dry-run mode)
	if e.dryRun {
		e.results = nil
		e.printExecutionPlan(ctx)
	} else {
		executor.ExecuteInstallPhase(ctx)
		executor.ExecuteRestorePhase(ctx)
		executor.ExecuteConfigurePhase(ctx)
		executor.ExecuteServicePhase(ctx)
		executor.ExecuteValidatePhase(ctx)
	}

	results := e.results
	if !e.dryRun {
		results = executor.Results()
	}

	// Count results
	successCount := 0
	warningCount := 0
	failedCount := 0
	skippedCount := 0
	var reportWarnings []string

	for _, r := range results {
		switch r.Status {
		case "success":
			successCount++
		case "warning":
			warningCount++
		case "failed":
			failedCount++
		case "skipped":
			skippedCount++
		}
	}

	// Collect warnings from modules (only in real mode)
	var stageScores []StageScore
	confidence := 0.0
	automationPct := 100.0
	var remainingEstimate string

	if !e.dryRun {
		for _, name := range e.plan.Selected {
			mod, ok := e.manager.Get(name)
			if !ok {
				continue
			}
			doc, err := mod.Doctor(ctx)
			if err == nil && doc != nil && len(doc.Issues) > 0 {
				for _, issue := range doc.Issues {
					reportWarnings = append(reportWarnings, fmt.Sprintf("%s: %s", name, issue.Message))
				}
			}
		}

		stageScores = computeStageScores(results)
		confidence = computeAvgConfidence(results)
		automationPct = computeAutomationPct(results)
		if automationPct < 100 {
			remainingEstimate = estimateRemaining(results)
		}
	}

	// Stage 9: Recovery Report
	report := &RecoveryReport{
		ID:              e.manifest.BackupID,
		Duration:        time.Since(start).Truncate(100 * time.Millisecond),
		TotalModules:    totalModules,
		SuccessCount:    successCount,
		WarningCount:    warningCount,
		FailedCount:     failedCount,
		SkippedCount:    skippedCount,
		StageScores:     stageScores,
		Results:         results,
		Warnings:        append(reportWarnings, e.plan.ManualSteps...),
		ManualSteps:     e.plan.ManualSteps,
		RecoveryScore:   computeRecoveryScore(totalModules, successCount, failedCount, results),
		Confidence:      confidence,
		AutomationPct:   automationPct,
		EstimatedRemain: remainingEstimate,
	}

	renderRecoveryReport(e.progress, report)

	// Stage 10: Completion — Restore Summary
	e.progress.Stage("10 / 10", "Completion")
	renderRestoreSummary(e.progress, report, time.Since(start))

	return report, nil
}

func (e *Engine) printExecutionPlan(ctx context.Context) {
	e.progress.Stage("4 / 8", "Packages")
	for _, dep := range e.plan.Deps {
		switch dep.Type {
		case module.DepSystemPkg:
			e.progress.ModuleSuccess("plan", dep.Package, "")
		case module.DepDownload:
			hint := dep.Hint
			if hint == "" {
				hint = dep.Package
			}
			if hint == "" {
				hint = "download"
			}
			e.progress.ModuleSuccess("plan", hint, "")
		}
	}

	e.progress.Stage("5 / 8", "Archives")
	for _, name := range e.plan.Selected {
		for _, snap := range e.plan.Manifest.Snapshots {
			if snap.Module == name {
				size := ""
				if snap.Size > 0 {
					size = fmt.Sprintf(" (%.1f MB)", float64(snap.Size)/1024/1024)
				}
				e.progress.ModuleSuccess("plan", snap.Module+size, "")
				break
			}
		}
	}

	e.progress.Stage("6 / 8", "Services")
	servicesDisplayed := false
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
		if ok {
			e.progress.ModuleSuccess("plan", svc, "")
			servicesDisplayed = true
		}
	}
	if !servicesDisplayed {
		e.progress.InfoLine("No managed services.")
	}

	// Stage 7: Summary
	e.progress.Stage("7 / 8", "Estimated Resources")
	if e.plan.DryRunInfo != nil {
		if e.plan.DryRunInfo.DiskUsage != "" {
			e.progress.InfoLine("Disk required: %s", e.plan.DryRunInfo.DiskUsage)
		}
		if e.plan.DryRunInfo.EstimatedTime != "" {
			e.progress.InfoLine("Estimated time: %s", e.plan.DryRunInfo.EstimatedTime)
		}
	}

	// Compute estimated time
	var totalEst time.Duration
	for _, dep := range e.plan.Deps {
		switch dep.Type {
		case module.DepSystemPkg:
			totalEst += 30 * time.Second
		case module.DepDownload:
			totalEst += 60 * time.Second
		}
	}
	for _, name := range e.plan.Selected {
		for _, snap := range e.plan.Manifest.Snapshots {
			if snap.Module == name {
				totalEst += 10 * time.Second
				if snap.Size > 10*1024*1024 {
					totalEst += time.Duration(snap.Size/10*1024*1024) * time.Second
				}
				break
			}
		}
	}
	if totalEst > 0 {
		e.progress.InfoLine("Estimated restore time: %s", totalEst.Round(time.Second))
	}

	e.progress.Stage("8 / 8", "Dry-Run Complete")
	fmt.Fprintf(e.progress.w, "\n  %s✓%s No changes have been made.\n", output.ColorGreen, output.ColorReset)
	fmt.Fprintln(e.progress.w)

	// Populate results with dry_run status so the summary works
	for _, name := range e.plan.Selected {
		e.results = append(e.results, ModuleResult{Module: name, Phase: "restore", Status: "dry_run"})
	}
}

func renderRestoreSummary(p *ProgressReporter, report *RecoveryReport, elapsed time.Duration) {
	elapsed = elapsed.Truncate(100 * time.Millisecond)

	fmt.Fprintf(p.w, "\n  %sRecovery Complete%s\n", output.ColorBold+output.ColorGreen, output.ColorReset)
	fmt.Fprintln(p.w)

	// Group phase results
	type phaseSummary struct {
		success int
		failed  int
		skipped int
	}
	phaseResults := make(map[string]*phaseSummary)
	for _, r := range report.Results {
		ps, ok := phaseResults[r.Phase]
		if !ok {
			ps = &phaseSummary{}
			phaseResults[r.Phase] = ps
		}
		switch r.Status {
		case "success":
			ps.success++
		case "failed":
			ps.failed++
		case "skipped":
			ps.skipped++
		}
	}

	// Display phases in order
	phaseOrder := []string{"install", "restore", "configure", "service", "validate"}
	for _, phase := range phaseOrder {
		ps, ok := phaseResults[phase]
		if !ok {
			continue
		}
		if ps.success > 0 && ps.failed == 0 {
			fmt.Fprintf(p.w, "  %s✓%s %s\n", output.ColorGreen, output.ColorReset, phaseLabel(phase))
		} else if ps.failed > 0 && ps.success == 0 {
			fmt.Fprintf(p.w, "  %s✗%s %s\n", output.ColorRed, output.ColorReset, phaseLabel(phase))
		} else if ps.failed > 0 {
			fmt.Fprintf(p.w, "  %s⚠%s %s (%d ok, %d failed, %d skipped)\n",
				output.ColorYellow, output.ColorReset, phaseLabel(phase),
				ps.success, ps.failed, ps.skipped)
		} else {
			fmt.Fprintf(p.w, "  %s○%s %s (skipped)\n", output.ColorCyan, output.ColorReset, phaseLabel(phase))
		}
	}

	fmt.Fprintln(p.w)

	// Manual actions
	if len(report.ManualSteps) > 0 {
		fmt.Fprintf(p.w, "  %sManual actions remaining:%s\n", output.ColorYellow, output.ColorReset)
		for _, step := range report.ManualSteps {
			fmt.Fprintf(p.w, "    ▸ %s\n", step)
		}
		fmt.Fprintln(p.w)
	} else {
		fmt.Fprintf(p.w, "  %sManual actions:%s (none)\n", output.ColorGreen, output.ColorReset)
		fmt.Fprintln(p.w)
	}

	// Recovery score and confidence
	scoreColor := output.ColorGreen
	if report.RecoveryScore < 70 {
		scoreColor = output.ColorYellow
	}
	if report.RecoveryScore < 50 {
		scoreColor = output.ColorRed
	}
	fmt.Fprintf(p.w, "  %sMachine recovery confidence:%s %s%d%%%s\n",
		output.ColorBold, output.ColorReset,
		scoreColor, report.RecoveryScore, output.ColorReset)

	fmt.Fprintf(p.w, "\n  Recovery completed in %s.\n", elapsed)
	fmt.Fprintln(p.w)
}

func phaseLabel(phase string) string {
	switch phase {
	case "install":
		return "Packages installed"
	case "restore":
		return "Configurations restored"
	case "configure":
		return "Post-restore configuration"
	case "service":
		return "Services started"
	case "validate":
		return "Validation"
	default:
		return phase
	}
}

func computeStageScores(results []ModuleResult) []StageScore {
	phaseCounts := make(map[string]map[string]int)
	for _, r := range results {
		if phaseCounts[r.Phase] == nil {
			phaseCounts[r.Phase] = make(map[string]int)
		}
		phaseCounts[r.Phase][r.Status]++
	}

	var scores []StageScore
	for phase, counts := range phaseCounts {
		total := counts["success"] + counts["failed"] + counts["warning"]
		score := 100
		if total > 0 {
			score = (counts["success"] * 100) / total
		}
		scores = append(scores, StageScore{
			Stage:   phase,
			Score:   score,
			Success: counts["success"],
			Failed:  counts["failed"] + counts["warning"],
			Skipped: counts["skipped"],
		})
	}

	sort.Slice(scores, func(i, j int) bool {
		order := map[string]int{"install": 0, "restore": 1, "configure": 2, "service": 3, "validate": 4}
		return order[scores[i].Stage] < order[scores[j].Stage]
	})

	return scores
}

func renderRecoveryReport(p *ProgressReporter, report *RecoveryReport) {
	p.Stage("9 / 10", "Recovery Report")

	scoreColor := output.ColorGreen
	if report.RecoveryScore < 70 {
		scoreColor = output.ColorYellow
	}
	if report.RecoveryScore < 50 {
		scoreColor = output.ColorRed
	}
	fmt.Fprintf(p.w, "\n  %sRecovery Score: %s%d%%%s",
		output.ColorBold, scoreColor, report.RecoveryScore, output.ColorReset)
	if report.Confidence > 0 {
		fmt.Fprintf(p.w, "  (Avg confidence: %.0f%%)", report.Confidence)
	}
	fmt.Fprintln(p.w)

	if report.AutomationPct < 100 {
		fmt.Fprintf(p.w, "  %sAutomation: %.0f%%%s",
			output.ColorCyan, report.AutomationPct, output.ColorReset)
		if report.EstimatedRemain != "" {
			fmt.Fprintf(p.w, "  — %s", report.EstimatedRemain)
		}
		fmt.Fprintln(p.w)
	}

	// Per-stage scores
	if len(report.StageScores) > 0 {
		fmt.Fprintf(p.w, "\n  %sStage Scores%s\n", output.ColorBold, output.ColorReset)
		for _, s := range report.StageScores {
			sColor := output.ColorGreen
			if s.Score < 70 {
				sColor = output.ColorYellow
			}
			if s.Score < 50 {
				sColor = output.ColorRed
			}
			fmt.Fprintf(p.w, "    %s%s:%s %s%3d%%%s  (%d ok, %d failed)\n",
				output.ColorCyan, s.Stage, output.ColorReset,
				sColor, s.Score, output.ColorReset,
				s.Success, s.Failed)
		}
	}

	if report.SuccessCount > 0 {
		fmt.Fprintf(p.w, "\n  %sSuccessfully Restored (%d)%s\n",
			output.ColorGreen, report.SuccessCount, output.ColorReset)
		for _, r := range report.Results {
			if r.Status == "success" {
				detail := ""
				if len(r.Details) > 0 {
					detail = " — " + r.Details[0]
				}
				conf := r.Validation
				extra := ""
				if conf != nil && len(conf.Recovered) > 0 {
					extra = fmt.Sprintf(" [%d assets]", len(conf.Recovered))
				}
				fmt.Fprintf(p.w, "    %s✓%s %s%s%s\n",
					output.ColorGreen, output.ColorReset, r.Module, detail, extra)
			}
		}
	}

	// Warnings with validation details
	if len(report.Warnings) > 0 || hasValidationWarnings(report.Results) {
		fmt.Fprintf(p.w, "\n  %sWarnings%s\n",
			output.ColorYellow, output.ColorReset)
		for _, w := range report.Warnings {
			fmt.Fprintf(p.w, "    %s⚠%s %s\n",
				output.ColorYellow, output.ColorReset, w)
		}
		for _, r := range report.Results {
			if r.Validation == nil || len(r.Validation.Warnings) == 0 {
				continue
			}
			for _, w := range r.Validation.Warnings {
				fmt.Fprintf(p.w, "    %s⚠%s %s: %s\n",
					output.ColorYellow, output.ColorReset, r.Module, w)
			}
		}
	}

	// Recovered assets by module
	hasRecovered := false
	for _, r := range report.Results {
		if r.Validation != nil && len(r.Validation.Recovered) > 0 {
			if !hasRecovered {
				fmt.Fprintf(p.w, "\n  %sAssets Restored%s\n", output.ColorGreen, output.ColorReset)
				hasRecovered = true
			}
			for _, a := range r.Validation.Recovered {
				fmt.Fprintf(p.w, "    %s✓%s %s: %s\n",
					output.ColorGreen, output.ColorReset, r.Module, a)
			}
		}
	}

	// Missing assets
	hasMissing := false
	for _, r := range report.Results {
		if r.Validation != nil && len(r.Validation.Missing) > 0 {
			if !hasMissing {
				fmt.Fprintf(p.w, "\n  %sMissing Assets%s\n", output.ColorYellow, output.ColorReset)
				hasMissing = true
			}
			for _, a := range r.Validation.Missing {
				fmt.Fprintf(p.w, "    %s✗%s %s: %s\n",
					output.ColorYellow, output.ColorReset, r.Module, a)
			}
		}
	}

	if len(report.ManualSteps) > 0 {
		fmt.Fprintf(p.w, "\n  %sManual Steps Required (%d)%s\n",
			output.ColorBold+output.ColorCyan, len(report.ManualSteps), output.ColorReset)
		for _, step := range report.ManualSteps {
			fmt.Fprintf(p.w, "    %s▸%s %s\n",
				output.ColorCyan, output.ColorReset, step)
		}
	}

	for _, r := range report.Results {
		if r.Validation == nil || len(r.Validation.ManualSteps) == 0 {
			continue
		}
		for _, ms := range r.Validation.ManualSteps {
			fmt.Fprintf(p.w, "    %s▸%s %s: %s\n",
				output.ColorCyan, output.ColorReset, r.Module, ms)
		}
	}

	if report.FailedCount > 0 {
		fmt.Fprintf(p.w, "\n  %sFailed (%d)%s\n",
			output.ColorRed, report.FailedCount, output.ColorReset)
		for _, r := range report.Results {
			if r.Status == "failed" {
				fmt.Fprintf(p.w, "    %s✗%s %s — %s\n",
					output.ColorRed, output.ColorReset, r.Module, r.Error)
			}
		}
	}

	fmt.Fprintln(p.w)
}

func hasValidationWarnings(results []ModuleResult) bool {
	for _, r := range results {
		if r.Validation != nil && len(r.Validation.Warnings) > 0 {
			return true
		}
	}
	return false
}

func computeRecoveryScore(total, success, failed int, results []ModuleResult) int {
	if total == 0 {
		return 0
	}
	baseScore := 100 - (failed * 100 / total)
	var confSum int
	var confCount int
	for _, r := range results {
		if r.Validation != nil {
			confSum += r.Validation.Confidence
			confCount++
		}
	}
	if confCount > 0 {
		avgConf := confSum / confCount
		baseScore = (baseScore*3 + avgConf) / 4
	}
	if baseScore < 0 {
		baseScore = 0
	}
	return baseScore
}

func computeAvgConfidence(results []ModuleResult) float64 {
	var sum int
	var count int
	for _, r := range results {
		if r.Validation != nil {
			sum += r.Validation.Confidence
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return float64(sum) / float64(count)
}

func computeAutomationPct(results []ModuleResult) float64 {
	var totalManual, totalSteps int
	for _, r := range results {
		if r.Validation == nil {
			continue
		}
		totalSteps++
		if len(r.Validation.ManualSteps) > 0 {
			totalManual++
		}
		if len(r.Validation.Missing) > 0 {
			totalManual++
		}
	}
	if totalSteps == 0 {
		return 100
	}
	return float64(totalSteps-totalManual) / float64(totalSteps) * 100
}

func estimateRemaining(results []ModuleResult) string {
	var steps int
	for _, r := range results {
		if r.Validation == nil {
			continue
		}
		steps += len(r.Validation.ManualSteps)
		steps += len(r.Validation.Missing)
	}
	if steps == 0 {
		return ""
	}
	return fmt.Sprintf("%d manual step(s) remaining", steps)
}
