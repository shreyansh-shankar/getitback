package restore

import (
	"context"
	"fmt"
	"io"
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
}

func NewEngine(manager *module.Manager, backupDir string) *Engine {
	return &Engine{
		manager:  manager,
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

func (e *Engine) Execute(ctx context.Context, w io.Writer) (*RecoveryReport, error) {
	start := time.Now()

	totalModules := len(e.plan.Selected)
	e.progress = NewProgressReporter(w, totalModules)

	rt := runtime.New(w, nil)
	executor := NewPhaseExecutor(e.manager, e.plan, e.progress, rt, e.dryRun)

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

	// Stages 4-8: Execute phases
	executor.ExecuteInstallPhase(ctx)
	executor.ExecuteRestorePhase(ctx)
	executor.ExecuteConfigurePhase(ctx)
	executor.ExecuteServicePhase(ctx)
	executor.ExecuteValidatePhase(ctx)

	results := executor.Results()

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

	// Collect warnings from modules
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

	// Compute rich recovery score from validation data
	confidence := computeAvgConfidence(results)
	automationPct := computeAutomationPct(results)
	var remainingEstimate string
	if automationPct < 100 {
		remainingEstimate = estimateRemaining(results)
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
		Results:         results,
		Warnings:        append(reportWarnings, e.plan.ManualSteps...),
		ManualSteps:     e.plan.ManualSteps,
		RecoveryScore:   computeRecoveryScore(totalModules, successCount, failedCount, results),
		Confidence:      confidence,
		AutomationPct:   automationPct,
		EstimatedRemain: remainingEstimate,
	}

	renderRecoveryReport(e.progress, report)

	// Stage 10: Completion
	e.progress.Stage("10 / 10", "Completion")
	elapsed := time.Since(start).Truncate(100 * time.Millisecond)
	if failedCount == 0 {
		fmt.Fprintf(w, "\n  %s✓%s Recovery complete in %s — %d modules restored successfully.\n",
			output.ColorGreen, output.ColorReset, elapsed, successCount)
	} else {
		fmt.Fprintf(w, "\n  %s⚠%s Recovery finished with %d failures in %s.\n",
			output.ColorYellow, output.ColorReset, failedCount, elapsed)
	}
	fmt.Fprintln(w)

	return report, nil
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

	// Module-specific manual steps from validation
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
	// Factor in confidence from validate results
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
