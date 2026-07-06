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
