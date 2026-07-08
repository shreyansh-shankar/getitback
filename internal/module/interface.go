package module

import "context"

type Module interface {
	Name() string
	Description() string
	Detect() (bool, error)
	Inventory(ctx context.Context) (*InventoryResult, error)
	Backup(ctx context.Context, opts BackupOptions) (*BackupResult, error)
	Restore(ctx context.Context, snap Snapshot, opts RestoreOptions) error
	Verify(ctx context.Context, snap Snapshot) (*VerifyResult, error)
	Doctor(ctx context.Context) (*DoctorResult, error)
}

// Optional restore phase interfaces.
// Modules implement these to participate in the automated restore pipeline.

type RestorePhase string

const (
	PhaseInstall   RestorePhase = "install"
	PhaseRestore   RestorePhase = "restore"
	PhaseConfigure RestorePhase = "configure"
	PhaseValidate  RestorePhase = "validate"
)

type DependencyProvider interface {
	Dependencies(ctx context.Context) []Dependency
}

type Installer interface {
	Install(ctx context.Context, opts RestoreOptions) error
}

type Configurer interface {
	Configure(ctx context.Context, opts RestoreOptions) error
}

type Validator interface {
	Validate(ctx context.Context, snap Snapshot) (*ValidateResult, error)
}
