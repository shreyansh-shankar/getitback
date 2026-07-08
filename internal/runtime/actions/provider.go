package actions

import (
	"context"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

type Provider interface {
	Actions(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) ([]Action, error)
}

type ModuleActions interface {
	module.Module
	Provider
}
