package actions

import (
	"fmt"
	"time"

	"github.com/shreyansh-shankar/getitback/internal/runtime"
)

// ServiceAction is a base for all service management actions.
type ServiceAction struct {
	BaseAction
	ServiceName string
}

// StartService starts a system service.
type StartService struct {
	ServiceAction
}

func (a *StartService) Name() string { return "start_service" }

func (a *StartService) Description() string {
	return fmt.Sprintf("Start service %s", a.ServiceName)
}

func (a *StartService) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.Service.Start(a.ServiceName)
}

func (a *StartService) EstimatedDuration() time.Duration { return 10 * time.Second }

// StopService stops a system service.
type StopService struct {
	ServiceAction
}

func (a *StopService) Name() string { return "stop_service" }

func (a *StopService) Description() string {
	return fmt.Sprintf("Stop service %s", a.ServiceName)
}

func (a *StopService) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.Service.Stop(a.ServiceName)
}

// EnableService enables a system service to start on boot.
type EnableService struct {
	ServiceAction
}

func (a *EnableService) Name() string { return "enable_service" }

func (a *EnableService) Description() string {
	return fmt.Sprintf("Enable service %s", a.ServiceName)
}

func (a *EnableService) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.Service.Enable(a.ServiceName)
}

// DisableService disables a system service.
type DisableService struct {
	ServiceAction
}

func (a *DisableService) Name() string { return "disable_service" }

func (a *DisableService) Description() string {
	return fmt.Sprintf("Disable service %s", a.ServiceName)
}

func (a *DisableService) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.Service.Disable(a.ServiceName)
}

// RestartService restarts a system service.
type RestartService struct {
	ServiceAction
}

func (a *RestartService) Name() string { return "restart_service" }

func (a *RestartService) Description() string {
	return fmt.Sprintf("Restart service %s", a.ServiceName)
}

func (a *RestartService) Execute(ctx *runtime.RestoreContext) error {
	return ctx.Runtime.Service.Restart(a.ServiceName)
}

func (a *RestartService) EstimatedDuration() time.Duration { return 10 * time.Second }
