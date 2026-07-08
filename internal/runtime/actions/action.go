package actions

import (
	"time"

	"github.com/shreyansh-shankar/getitback/internal/runtime"
)

type Action interface {
	Name() string
	Description() string
	Execute(ctx *runtime.RestoreContext) error
	Rollback(ctx *runtime.RestoreContext) error
	Validate(ctx *runtime.RestoreContext) error
	EstimatedDuration() time.Duration
}

type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
}

type RetryableAction interface {
	Action
	RetryPolicy() RetryPolicy
}

type ParallelSafeAction interface {
	Action
	ParallelSafe()
}

type RetryPolicyDefaults struct{}

func (RetryPolicyDefaults) RetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 1, Backoff: time.Second}
}

type BaseAction struct{}

func (BaseAction) EstimatedDuration() time.Duration { return 5 * time.Second }

func (BaseAction) Rollback(ctx *runtime.RestoreContext) error { return nil }

func (BaseAction) Validate(ctx *runtime.RestoreContext) error { return nil }
