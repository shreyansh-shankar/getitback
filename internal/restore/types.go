package restore

import (
	"time"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/storage"
)

type ExecutionGroup struct {
	Phase    module.RestorePhase
	Modules  []string
	Parallel bool
}

type RestorePlan struct {
	BackupDir    string
	Manifest     *storage.Manifest
	SnapshotsDir string
	Selected     []string
	Execution    []ExecutionGroup
	Deps         []module.Dependency
	ManualSteps  []string
	DryRunInfo   *DryRunInfo
}

type DryRunInfo struct {
	Packages      []string          `json:"packages,omitempty"`
	Downloads     []string          `json:"downloads,omitempty"`
	Services      []string          `json:"services,omitempty"`
	Archives      []string          `json:"archives,omitempty"`
	Files         []string          `json:"files,omitempty"`
	OwnershipChgs []string          `json:"ownership_changes,omitempty"`
	DiskUsage     string            `json:"disk_usage,omitempty"`
	DownloadSize  string            `json:"download_size,omitempty"`
	EstimatedTime string            `json:"estimated_time,omitempty"`
	ModuleDetails map[string]string `json:"module_details,omitempty"`
}

type StageScore struct {
	Stage   string `json:"stage"`
	Score   int    `json:"score"`   // 0-100
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
	Skipped int    `json:"skipped"`
}

type ModuleResult struct {
	Module     string               `json:"module"`
	Phase      string               `json:"phase"`
	Status     string               `json:"status"`
	Error      string               `json:"error,omitempty"`
	Details    []string             `json:"details,omitempty"`
	Validation *module.ValidateResult `json:"validation,omitempty"`
}

type RecoveryReport struct {
	ID               string         `json:"id"`
	Duration         time.Duration  `json:"duration"`
	TotalModules     int            `json:"total_modules"`
	SuccessCount     int            `json:"success_count"`
	WarningCount     int            `json:"warning_count"`
	FailedCount      int            `json:"failed_count"`
	SkippedCount     int            `json:"skipped_count"`
	StageScores      []StageScore   `json:"stage_scores,omitempty"`
	Results          []ModuleResult `json:"results"`
	Warnings         []string       `json:"warnings,omitempty"`
	ManualSteps      []string       `json:"manual_steps,omitempty"`
	RecoveryScore    int            `json:"recovery_score"`
	Confidence       float64        `json:"confidence"`
	AutomationPct    float64        `json:"automation_pct"`
	EstimatedRemain  string         `json:"estimated_remaining,omitempty"`
}
