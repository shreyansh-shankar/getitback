package actions

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/runtime"
)

type ExecutorOption func(*ActionExecutor)

func WithDryRun(dryRun bool) ExecutorOption {
	return func(e *ActionExecutor) {
		e.dryRun = dryRun
	}
}

type ActionExecutor struct {
	ctx      *runtime.RestoreContext
	actions  []Action
	dryRun   bool
	metrics  *MetricsCollector
	mu       sync.Mutex
}

func NewExecutor(ctx *runtime.RestoreContext, actions []Action, opts ...ExecutorOption) *ActionExecutor {
	e := &ActionExecutor{
		ctx:     ctx,
		actions: actions,
		metrics: NewMetricsCollector(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *ActionExecutor) Execute() (*TransactionReport, error) {
	if e.dryRun {
		return e.dryRunExecute()
	}

	report := &TransactionReport{
		ActionCount: len(e.actions),
	}
	start := time.Now()
	defer func() {
		report.Duration = time.Since(start).Truncate(time.Millisecond)
	}()

	var completed []Action

	for _, action := range e.actions {
		select {
		case <-e.ctx.Done():
			report.RootCause = e.ctx.Err()
			return report, e.ctx.Err()
		default:
		}

		// Validate before execution
		if err := action.Validate(e.ctx); err != nil {
			report.Failed++
			report.FailedActions = append(report.FailedActions, TransactionAction{
				Name:   action.Name(),
				Status: StatusFailed,
				Error:  fmt.Sprintf("validation: %s", err),
			})
			report.RootCause = fmt.Errorf("action %s failed validation: %w", action.Name(), err)
			e.rollbackCompleted(completed, report)
			return report, report.RootCause
		}

		// Execute with retries
		result := e.executeWithRetries(action)
		e.metrics.Record(result)

		switch result.Status {
		case StatusSuccess:
			report.Succeeded++
			completed = append(completed, action)
		case StatusFailed:
			report.Failed++
			report.FailedActions = append(report.FailedActions, TransactionAction{
				Name:   action.Name(),
				Status: StatusFailed,
				Error:  result.Error,
			})
			report.RootCause = fmt.Errorf("action %s failed: %s", action.Name(), result.Error)
			e.rollbackCompleted(completed, report)
			return report, report.RootCause
		case StatusSkipped:
			report.Skipped++
			report.SkippedActions = append(report.SkippedActions, TransactionAction{
				Name:   action.Name(),
				Status: StatusSkipped,
			})
		}
	}

	return report, nil
}

func (e *ActionExecutor) executeWithRetries(action Action) ActionMetrics {
	start := time.Now()
	base := ActionMetrics{Name: action.Name()}

	maxAttempts := 1
	backoff := time.Second
	if r, ok := action.(RetryableAction); ok {
		p := r.RetryPolicy()
		maxAttempts = p.MaxAttempts
		backoff = p.Backoff
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(backoff)
			backoff *= 2
		}

		select {
		case <-e.ctx.Done():
			base.Status = StatusSkipped
			base.Duration = time.Since(start).Truncate(time.Millisecond)
			return base
		default:
		}

		if err := action.Execute(e.ctx); err != nil {
			lastErr = err
			base.Retries = attempt + 1
			continue
		}

		base.Status = StatusSuccess
		base.Duration = time.Since(start).Truncate(time.Millisecond)
		return base
	}

	if lastErr != nil {
		base.Status = StatusFailed
		base.Error = lastErr.Error()
	}
	base.Duration = time.Since(start).Truncate(time.Millisecond)
	return base
}

func (e *ActionExecutor) rollbackCompleted(completed []Action, report *TransactionReport) {
	for i := len(completed) - 1; i >= 0; i-- {
		action := completed[i]
		if err := action.Rollback(e.ctx); err != nil {
			name := action.Name()
			report.RolledBackActions = append(report.RolledBackActions, TransactionAction{
				Name:   name,
				Status: StatusFailed,
				Error:  fmt.Sprintf("rollback error: %s", err),
			})
		} else {
			report.RolledBack++
		}
	}
}

func (e *ActionExecutor) dryRunExecute() (*TransactionReport, error) {
	report := &TransactionReport{ActionCount: len(e.actions)}
	var totalDuration time.Duration

	for _, action := range e.actions {
		d := action.EstimatedDuration()
		totalDuration += d

		e.metrics.Record(ActionMetrics{
			Name:   action.Name(),
			Status: StatusDryRun,
		})
	}

	report.Duration = totalDuration
	return report, nil
}

func (e *ActionExecutor) DryRunPlan() string {
	var b strings.Builder
	var total time.Duration

	b.WriteString("Restore Plan:\n")
	for i, action := range e.actions {
		d := action.EstimatedDuration()
		total += d
		b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, action.Description()))
	}
	b.WriteString(fmt.Sprintf("\nEstimated total time: %s\n", total.Round(time.Second)))
	return b.String()
}

func (e *ActionExecutor) Metrics() *MetricsCollector {
	return e.metrics
}

func (e *ActionExecutor) SortedByDuration() []Action {
	sorted := make([]Action, len(e.actions))
	copy(sorted, e.actions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].EstimatedDuration() < sorted[j].EstimatedDuration()
	})
	return sorted
}
