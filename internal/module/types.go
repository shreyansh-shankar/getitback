package module

import "time"

type InventoryResult struct {
	Module    string         `json:"module" yaml:"module"`
	Detected  bool           `json:"detected" yaml:"detected"`
	Version   string         `json:"version,omitempty" yaml:"version,omitempty"`
	Resources []Resource     `json:"resources,omitempty" yaml:"resources,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Warnings  []string       `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Errors    []string       `json:"errors,omitempty" yaml:"errors,omitempty"`
}

const (
	ResourceTypeConfig = "config"
	ResourceTypeSecret = "secret"
	ResourceTypeData   = "data"
	ResourceTypeTemp   = "temp"
)

type Resource struct {
	Name     string    `json:"name" yaml:"name"`
	Path     string    `json:"path" yaml:"path"`
	Size     int64     `json:"size,omitempty" yaml:"size,omitempty"`
	Modified time.Time `json:"modified,omitempty" yaml:"modified,omitempty"`
	Type     string    `json:"type" yaml:"type"`
}

type BackupOptions struct {
	SnapshotsDir string
	Encrypt      bool
	KeyPath      string
}

type BackupResult struct {
	Module    string     `json:"module" yaml:"module"`
	Snapshots []Snapshot `json:"snapshots" yaml:"snapshots"`
	Contents  []string   `json:"contents,omitempty" yaml:"contents,omitempty"`
	Warnings  []string   `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Partial   bool       `json:"partial,omitempty" yaml:"partial,omitempty"`
}

type Snapshot struct {
	Module     string `json:"module" yaml:"module"`
	Path       string `json:"path" yaml:"path"`
	Size       int64  `json:"size" yaml:"size"`
	Checksum   string `json:"checksum" yaml:"checksum"`
	Encrypted  bool   `json:"encrypted" yaml:"encrypted"`

	OriginalSize   int64  `json:"originalSize,omitempty" yaml:"originalSize,omitempty"`
	CompressedSize int64  `json:"compressedSize,omitempty" yaml:"compressedSize,omitempty"`
	Compression    string `json:"compression,omitempty" yaml:"compression,omitempty"`
	Duration       string `json:"duration,omitempty" yaml:"duration,omitempty"`
	Status         string `json:"status,omitempty" yaml:"status,omitempty"`
	ArchiveFile    string `json:"archiveFile,omitempty" yaml:"archiveFile,omitempty"`
	RecoveryValue  string `json:"recoveryValue,omitempty" yaml:"recoveryValue,omitempty"`
	FileCount      int    `json:"fileCount,omitempty" yaml:"fileCount,omitempty"`
}

type DependencyType string

const (
	DepModule    DependencyType = "module"      // depends on another registered module
	DepSystemPkg DependencyType = "system_pkg"  // system package (apt, yum, etc.)
	DepDownload  DependencyType = "download"    // URL to download
	DepCommand   DependencyType = "command"     // arbitrary shell command
	DepManual    DependencyType = "manual"      // user must do something manually
)

type Dependency struct {
	Type     DependencyType `json:"type"`
	Module   string         `json:"module,omitempty"`   // for DepModule
	Package  string         `json:"package,omitempty"`  // for DepSystemPkg
	URL      string         `json:"url,omitempty"`      // for DepDownload
	Command  string         `json:"command,omitempty"`  // for DepCommand
	Message  string         `json:"message,omitempty"`  // for DepManual
	Optional bool           `json:"optional,omitempty"`
	Hint     string         `json:"hint,omitempty"`
}

type ValidateCategory string

const (
	ValidateCategoryArchive       ValidateCategory = "archive"       // archive extracted successfully
	ValidateCategoryConfig        ValidateCategory = "config"        // configuration files valid
	ValidateCategorySoftware      ValidateCategory = "software"      // software installed
	ValidateCategoryService       ValidateCategory = "service"       // service running
	ValidateCategoryUsable        ValidateCategory = "usable"        // application usable
)

type ValidateResult struct {
	Module      string           `json:"module"`
	Success     bool             `json:"success"`
	Category    ValidateCategory `json:"category,omitempty"`
	Version     string           `json:"version,omitempty"`
	Checks      []string         `json:"checks,omitempty"`
	Warnings    []string         `json:"warnings,omitempty"`
	Errors      []string         `json:"errors,omitempty"`
	ManualSteps []string         `json:"manualSteps,omitempty"`
	Recovered   []string         `json:"recovered,omitempty"`
	Missing     []string         `json:"missing,omitempty"`
	Confidence  int              `json:"confidence"` // 0–100
}

// Runtime is an interface that internal/runtime.Runtime satisfies.
// It allows modules to access cross-platform primitives without
// importing the runtime package directly.
type Runtime interface {
	// Reserve for future use — type assertion at point of use.
}

type OverwritePolicy string

const (
	OverwritePolicyBackup    OverwritePolicy = "backup"     // rename existing with .getitback-bak
	OverwritePolicyReplace   OverwritePolicy = "replace"    // overwrite without backup
	OverwritePolicySkip      OverwritePolicy = "skip"       // skip if exists
	OverwritePolicyMerge     OverwritePolicy = "merge"      // merge (directory merge, skip individual files)
)

type RestoreOptions struct {
	SnapshotsDir    string
	Decrypt         bool
	KeyPath         string
	BackupDir       string
	WorkDir         string           // working directory for temp extraction (falls back: --workdir > GETITBACK_WORKDIR > $HOME/.cache/getitback > /tmp)
	Runtime         Runtime          // optional, injected by restore engine
	OverwritePolicy OverwritePolicy  // how to handle existing files
}

type VerifyResult struct {
	Module   string   `json:"module" yaml:"module"`
	Snapshot Snapshot `json:"snapshot" yaml:"snapshot"`
	Valid    bool     `json:"valid" yaml:"valid"`
	Errors   []string `json:"errors,omitempty" yaml:"errors,omitempty"`
}

type DoctorResult struct {
	Module  string        `json:"module" yaml:"module"`
	Status  DoctorStatus  `json:"status" yaml:"status"`
	Issues  []DoctorIssue `json:"issues,omitempty" yaml:"issues,omitempty"`
}

type DoctorStatus string

const (
	DoctorStatusOK      DoctorStatus = "ok"
	DoctorStatusWarning DoctorStatus = "warning"
	DoctorStatusError   DoctorStatus = "error"
)

type DoctorIssue struct {
	Severity string `json:"severity" yaml:"severity"`
	Message  string `json:"message" yaml:"message"`
	Help     string `json:"help,omitempty" yaml:"help,omitempty"`
}

type Coverage struct {
	Resources int  `json:"resources" yaml:"resources"`
	Configs   int  `json:"configs" yaml:"configs"`
	Secrets   int  `json:"secrets" yaml:"secrets"`
	Data      int  `json:"data" yaml:"data"`
	HasConfig bool `json:"hasConfig" yaml:"hasConfig"`
	HasSecret bool `json:"hasSecret" yaml:"hasSecret"`
	HasData   bool `json:"hasData" yaml:"hasData"`
}

type RecoveryScore struct {
	Total             int     `json:"total" yaml:"total"`
	Identity          int     `json:"identity" yaml:"identity"`
	Configuration     int     `json:"configuration" yaml:"configuration"`
	Development       int     `json:"development" yaml:"development"`
	Editors           int     `json:"editors" yaml:"editors"`
	Browsers          int     `json:"browsers" yaml:"browsers"`
	Packages          int     `json:"packages" yaml:"packages"`
	Databases         int     `json:"databases" yaml:"databases"`
	Containers        int     `json:"containers" yaml:"containers"`
	Cloud             int     `json:"cloud" yaml:"cloud"`
	Infrastructure    int     `json:"infrastructure" yaml:"infrastructure"`
	Projects          int     `json:"projects" yaml:"projects"`
	Virtualization    int     `json:"virtualization" yaml:"virtualization"`
	Confidence        float64 `json:"confidence" yaml:"confidence"`
	AutomationPercent float64 `json:"automationPercent" yaml:"automationPercent"`
	ManualActions     int     `json:"manualActions" yaml:"manualActions"`
}

type RecommendationPriority string

const (
	RecPriorityCritical RecommendationPriority = "critical"
	RecPriorityHigh     RecommendationPriority = "high"
	RecPriorityMedium   RecommendationPriority = "medium"
	RecPriorityLow      RecommendationPriority = "low"
)

type Recommendation struct {
	Priority RecommendationPriority `json:"priority" yaml:"priority"`
	Category string                 `json:"category" yaml:"category"`
	Message  string                 `json:"message" yaml:"message"`
	Help     string                 `json:"help,omitempty" yaml:"help,omitempty"`
}
