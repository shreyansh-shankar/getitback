package actions

import (
	"fmt"
	"strings"
	"time"
)

type TransactionReport struct {
	ActionCount      int
	Succeeded        int
	Failed           int
	Skipped          int
	RolledBack       int
	Duration         time.Duration
	FailedActions    []TransactionAction
	RolledBackActions []TransactionAction
	SkippedActions   []TransactionAction
	RootCause        error
}

type TransactionAction struct {
	Name        string
	Description string
	Status      ActionStatus
	Duration    time.Duration
	Error       string
	Retries     int
}

func (r *TransactionReport) Success() bool {
	return r.Failed == 0 && r.RootCause == nil
}

func (r *TransactionReport) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Transaction Report: %d actions, %d succeeded, %d failed, %d rolled back",
		r.ActionCount, r.Succeeded, r.Failed, r.RolledBack))
	if r.RootCause != nil {
		b.WriteString(fmt.Sprintf("\n  Root cause: %s", r.RootCause))
	}
	for _, a := range r.FailedActions {
		b.WriteString(fmt.Sprintf("\n  ✗ %s: %s", a.Name, a.Error))
	}
	for _, a := range r.RolledBackActions {
		b.WriteString(fmt.Sprintf("\n  ⟲ %s rolled back", a.Name))
	}
	return b.String()
}
