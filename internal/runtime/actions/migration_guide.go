// Package actions — Migration Guide
//
// This file documents how to migrate a module from imperative Restore()
// to the declarative Action system. It is a reference guide, not code.
//
// ── Overview ──
//
// The Action system is opt-in. Modules that implement the Provider
// interface (Actions method) will have their actions executed by the
// ActionExecutor. Modules that don't implement it fall back to the
// existing Restore() method. Both approaches coexist.
//
// ── Step 1: Import the actions package ──
//
//   import "github.com/shreyansh-shankar/getitback/internal/runtime/actions"
//
// ── Step 2: Add the Actions method ──
//
//   func (m *MyModule) Actions(ctx context.Context, snap module.Snapshot,
//       opts module.RestoreOptions) ([]actions.Action, error) {
//
//       return []actions.Action{
//           &actions.ExtractArchive{...},
//           &actions.CopyFile{...},
//           &actions.CopyDirectory{...},
//       }, nil
//   }
//
// ── Step 3: Compile-time check ──
//
//   var _ actions.Provider = (*MyModule)(nil)
//
// ── Step 4: The existing Restore() method stays as a fallback ──
//
// For backward compatibility, keep the existing Restore() method.
// When the restore engine detects the Actions() method, it uses the
// action executor; otherwise it calls Restore() directly.
//
// ── Concrete Example: Docker Module ──
//
// Before (Restore method in module.go):
//
//   func (m *DockerModule) Restore(ctx context.Context, snap module.Snapshot,
//       opts module.RestoreOptions) error {
//       tmpDir, _ := os.MkdirTemp(...)
//       archive.Extract(snap.Path, tmpDir)
//       // find manifest, walk configs, copy contexts
//       return nil
//   }
//
// After (Actions method in actions.go):
//
//   func (m *DockerModule) Actions(ctx context.Context, snap module.Snapshot,
//       opts module.RestoreOptions) ([]actions.Action, error) {
//       rt, _ := opts.Runtime.(*runtime.Runtime)
//       tmpDir, _ := os.MkdirTemp("", "getitback-actions-docker-*")
//       home := homeDir(rt)
//
//       return []actions.Action{
//           &actions.ExtractArchive{
//               Source:      snap.Path,
//               Destination: tmpDir,
//           },
//           &restoreDockerAction{
//               tmpDir: tmpDir,
//               home:   home,
//           },
//       }, nil
//   }
//
// The restoreDockerAction is a custom action that handles the complex
// post-extraction logic (reading manifest, walking configs). Standard
// actions (ExtractArchive, CopyFile, CopyDirectory) handle the parts
// that fit the declarative model.
//
// ── Standard Actions Available ──
//
//   System:
//     InstallPackage{...}
//     RemovePackage{...}
//
//   Services:
//     StartService{ServiceName}
//     StopService{ServiceName}
//     EnableService{ServiceName}
//     DisableService{ServiceName}
//     RestartService{ServiceName}
//     WaitForService{ServiceName, Timeout, PollInterval}
//
//   Filesystem:
//     ExtractArchive{Source, Destination, StripComponents}
//     CopyFile{Source, Destination, Mode}
//     CopyDirectory{Source, Destination}
//     CreateDirectory{Path, Mode}
//     RemoveDirectory{Path}
//     WriteFile{Path, Data, Mode}
//     CreateSymlink{Target, Link}
//     SetPermission{Path, Mode}
//     Chown{Path, Owner, Group}
//
//   Command:
//     RunCommand{Command, Args, WorkDir, Env}
//     RunBashScript{Script}
//
//   Network:
//     DownloadFile{URL, Destination, Checksum}
//     VerifyChecksum{Path, Checksum}
//
//   Environment:
//     SetEnvironmentVariable{Key, Value}
//
//   Docker:
//     ImportDockerImage{ImagePath, ImageName}
//     RestoreDockerVolume{VolumeName, Archive}
//     DockerComposeUp{ProjectDir, Services}
//
//   Validation:
//     ValidateCondition{CheckName, Condition}
//
//   User Interaction:
//     ManualStep{Message, Help}
//
// ── Custom Actions ──
//
// For logic that doesn't fit standard actions, embed BaseAction and
// implement the Action interface:
//
//   type myCustomAction struct {
//       actions.BaseAction
//       // your fields
//   }
//
//   func (a *myCustomAction) Name() string { return "my_custom" }
//   func (a *myCustomAction) Description() string { return "..." }
//   func (a *myCustomAction) Execute(ctx *runtime.RestoreContext) error {
//       // your logic here
//   }
//
// ── Retries ──
//
// To make an action retryable, implement RetryableAction:
//
//   type myAction struct {
//       BaseAction
//   }
//   func (a *myAction) RetryPolicy() actions.RetryPolicy {
//       return actions.RetryPolicy{MaxAttempts: 3, Backoff: 2 * time.Second}
//   }
//
// ── Dry Run ──
//
// In dry-run mode (--dry-run flag), the action executor prints each
// action's Description() and the total EstimatedDuration(). No
// Execute() methods are called. Custom actions automatically get
// dry-run support through the interface — no extra code needed.
//
// ── Rollback ──
//
// Override Rollback() on your action to provide compensation logic:
//
//   func (a *myAction) Rollback(ctx *runtime.RestoreContext) error {
//       return ctx.Runtime.FS.RemoveAll(a.CreatedDir)
//   }
//
// Rollback is best-effort. The executor calls it on all completed
// actions (in reverse order) when an action fails. If rollback
// itself fails, the error is recorded but execution continues.
//
// ── Validation ──
//
// Override Validate() to check preconditions before execution:
//
//   func (a *myAction) Validate(ctx *runtime.RestoreContext) error {
//       if !ctx.Runtime.FS.Exists(a.Source) {
//           return fmt.Errorf("source %s not found", a.Source)
//       }
//       return nil
//   }
//
// ── Phases and Actions ──
//
// The ActionProvider replaces the Restore phase only. Modules can
// still implement Installer, Configurer, and Validator separately
// for the other phases:
//
//   type MyModule struct{}
//   func (m *MyModule) Install(ctx, opts) error { ... }
//   func (m *MyModule) Configure(ctx, opts) error { ... }
//   func (m *MyModule) Validate(ctx, snap) (*ValidateResult, error) { ... }
//   func (m *MyModule) Actions(ctx, snap, opts) ([]Action, error) { ... }
//
// Or, for modules that want full action coverage, the Actions method
// can return actions for all phases, and the module can skip the
// Installer/Configurer/Validator interfaces.
//
// ── Testing ──
//
// Actions are data structures, making them easy to test:
//
//   func TestMyActions(t *testing.T) {
//       mod := &MyModule{}
//       actions, err := mod.Actions(ctx, snap, opts)
//       assert.NoError(t, err)
//       assert.Len(t, actions, 3)
//       assert.Equal(t, "my_custom", actions[0].Name())
//   }
//
// For execution testing, create a mock RestoreContext:
//
//   import "github.com/shreyansh-shankar/getitback/internal/runtime"
//   ctx := runtime.NewRestoreContext(...)
//   err := myAction.Execute(&ctx)
package actions
