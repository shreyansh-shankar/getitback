package actions

import "time"

type ActionMetrics struct {
	Name       string
	Duration   time.Duration
	Retries    int
	Status     ActionStatus
	Error      string
	RolledBack bool
	Validation string
}

type ActionStatus string

const (
	StatusPending    ActionStatus = "pending"
	StatusRunning    ActionStatus = "running"
	StatusSuccess    ActionStatus = "success"
	StatusFailed     ActionStatus = "failed"
	StatusSkipped    ActionStatus = "skipped"
	StatusRolledBack ActionStatus = "rolled_back"
	StatusDryRun     ActionStatus = "dry_run"
)

type MetricsCollector struct {
	results []ActionMetrics
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

func (m *MetricsCollector) Record(metrics ActionMetrics) {
	m.results = append(m.results, metrics)
}

func (m *MetricsCollector) Results() []ActionMetrics {
	return m.results
}

func (m *MetricsCollector) Summary() MetricsSummary {
	summary := MetricsSummary{}
	for _, r := range m.results {
		summary.Total++
		summary.TotalDuration += r.Duration
		switch r.Status {
		case StatusSuccess:
			summary.Succeeded++
		case StatusFailed:
			summary.Failed++
		case StatusSkipped:
			summary.Skipped++
		case StatusRolledBack:
			summary.RolledBack++
		case StatusDryRun:
			summary.DryRun++
		}
		if r.RolledBack {
			summary.Rollbacks++
		}
		summary.TotalRetries += r.Retries
	}
	return summary
}

type MetricsSummary struct {
	Total         int
	Succeeded     int
	Failed        int
	Skipped       int
	RolledBack    int
	DryRun        int
	Rollbacks     int
	TotalRetries  int
	TotalDuration time.Duration
}
