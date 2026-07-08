package actions

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/runtime"
)

// RunCommand executes an arbitrary shell command.
type RunCommand struct {
	BaseAction
	Command string
	Args    []string
	WorkDir string
	Env     map[string]string
}

func (a *RunCommand) Name() string { return "run_command" }

func (a *RunCommand) Description() string {
	return fmt.Sprintf("Run %s", a.Command)
}

func (a *RunCommand) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.Exec.Run(a.Command, a.Args...)
}

func (a *RunCommand) EstimatedDuration() time.Duration { return 10 * time.Second }

// RunBashScript executes a bash script.
type RunBashScript struct {
	BaseAction
	Script string
}

func (a *RunBashScript) Name() string { return "run_bash_script" }

func (a *RunBashScript) Description() string {
	short := a.Script
	if len(short) > 60 {
		short = short[:57] + "..."
	}
	return fmt.Sprintf("Execute: %s", short)
}

func (a *RunBashScript) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.Exec.RunBash(a.Script)
}

// DownloadFile downloads a URL to a destination path.
type DownloadFile struct {
	BaseAction
	URL         string
	Destination string
	Checksum    string // optional SHA256
}

func (a *DownloadFile) Name() string { return "download_file" }

func (a *DownloadFile) Description() string {
	return fmt.Sprintf("Download %s", a.URL)
}

func (a *DownloadFile) Execute(ctx *runtime.RestoreContext) error {
	if a.Checksum != "" {
		return ctx.Runtime.Download.GetAndVerify(a.URL, a.Destination, a.Checksum)
	}
	return ctx.Runtime.Download.Get(a.URL, a.Destination)
}

func (a *DownloadFile) EstimatedDuration() time.Duration { return 30 * time.Second }

// VerifyChecksum verifies a file's SHA256 checksum.
type VerifyChecksum struct {
	BaseAction
	Path     string
	Checksum string
}

func (a *VerifyChecksum) Name() string { return "verify_checksum" }

func (a *VerifyChecksum) Description() string {
	return fmt.Sprintf("Verify checksum of %s", filepath.Base(a.Path))
}

func (a *VerifyChecksum) Execute(ctx *runtime.RestoreContext) error {
	ok, err := ctx.Runtime.Download.VerifyChecksum(a.Path, a.Checksum)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("checksum mismatch for %s", a.Path)
	}
	return nil
}

func (a *VerifyChecksum) Validate(ctx *runtime.RestoreContext) error {
	return a.Execute(ctx)
}

// SetEnvironmentVariable sets an environment variable for the current session.
type SetEnvironmentVariable struct {
	BaseAction
	Key   string
	Value string
}

func (a *SetEnvironmentVariable) Name() string { return "set_env" }

func (a *SetEnvironmentVariable) Description() string {
	return fmt.Sprintf("Set %s=%s", a.Key, a.Value)
}

func (a *SetEnvironmentVariable) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.Env.Set(a.Key, a.Value)
}

// WaitForService polls until a service becomes active.
type WaitForService struct {
	ServiceAction
	Timeout      time.Duration
	PollInterval time.Duration
}

func (a *WaitForService) Name() string { return "wait_for_service" }

func (a *WaitForService) Description() string {
	return fmt.Sprintf("Wait for service %s (timeout %s)", a.ServiceName, a.Timeout)
}

func (a *WaitForService) Execute(ctx *runtime.RestoreContext) error {
	deadline := time.Now().Add(a.Timeout)
	for time.Now().Before(deadline) {
		active, err := ctx.Runtime.Service.Status(a.ServiceName)
		if err == nil && active {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(a.PollInterval):
		}
	}
	return fmt.Errorf("service %s not active within %s", a.ServiceName, a.Timeout)
}

func (a *WaitForService) EstimatedDuration() time.Duration { return a.Timeout }

// ValidateCondition checks that a condition function returns nil.
type ValidateCondition struct {
	BaseAction
	CheckName string
	Condition func(ctx *runtime.RestoreContext) error
}

func (a *ValidateCondition) Name() string { return "validate_" + a.CheckName }

func (a *ValidateCondition) Description() string {
	return fmt.Sprintf("Validate %s", a.CheckName)
}

func (a *ValidateCondition) Execute(ctx *runtime.RestoreContext) error {
	return a.Condition(ctx)
}

func (a *ValidateCondition) Validate(ctx *runtime.RestoreContext) error {
	return a.Condition(ctx)
}

// ManualStep pauses and instructs the user to perform a manual action.
type ManualStep struct {
	BaseAction
	Message string
	Help    string
}

func (a *ManualStep) Name() string { return "manual_step" }

func (a *ManualStep) Description() string {
	return a.Message
}

func (a *ManualStep) Execute(ctx *runtime.RestoreContext) error {
	return fmt.Errorf("manual step required: %s", a.Message)
}
