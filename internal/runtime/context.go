package runtime

import (
	"context"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/storage"
)

type RestoreContext struct {
	context.Context
	Runtime      *Runtime
	Manifest     *storage.Manifest
	Snapshot     module.Snapshot
	SelectedMods []string
	Options      module.RestoreOptions
}

func NewRestoreContext(ctx context.Context, rt *Runtime, manifest *storage.Manifest, snap module.Snapshot, selected []string, opts module.RestoreOptions) RestoreContext {
	return RestoreContext{
		Context:      ctx,
		Runtime:      rt,
		Manifest:     manifest,
		Snapshot:     snap,
		SelectedMods: selected,
		Options:      opts,
	}
}

func (c RestoreContext) WithSnapshot(snap module.Snapshot) RestoreContext {
	c.Snapshot = snap
	return c
}
