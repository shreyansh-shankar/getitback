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
}

type Snapshot struct {
	Module    string `json:"module" yaml:"module"`
	Path      string `json:"path" yaml:"path"`
	Size      int64  `json:"size" yaml:"size"`
	Checksum  string `json:"checksum" yaml:"checksum"`
	Encrypted bool   `json:"encrypted" yaml:"encrypted"`
}

type RestoreOptions struct {
	SnapshotsDir string
	Decrypt      bool
	KeyPath      string
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
	Total         int `json:"total" yaml:"total"`
	Identity      int `json:"identity" yaml:"identity"`
	Configuration int `json:"configuration" yaml:"configuration"`
	Development   int `json:"development" yaml:"development"`
	Editors       int `json:"editors" yaml:"editors"`
	Browsers      int `json:"browsers" yaml:"browsers"`
	Packages      int `json:"packages" yaml:"packages"`
	Databases     int `json:"databases" yaml:"databases"`
	Containers    int `json:"containers" yaml:"containers"`
	Cloud         int `json:"cloud" yaml:"cloud"`
	Infrastructure int `json:"infrastructure" yaml:"infrastructure"`
	Projects      int `json:"projects" yaml:"projects"`
	Virtualization int `json:"virtualization" yaml:"virtualization"`
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
