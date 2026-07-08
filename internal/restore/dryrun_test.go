package restore

import (
	"context"
	"io"
	"testing"

	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/storage"
)

// mockRuntime tracks which mutation methods were called.
type mockMutationTracker struct {
	installCalled  bool
	removeCalled   bool
	startCalled    bool
	stopCalled     bool
	enableCalled   bool
	disableCalled  bool
	writeFileCalled bool
	copyFileCalled bool
	copyDirCalled  bool
	extractCalled  bool
}

var tracker mockMutationTracker

func resetTracker() {
	tracker = mockMutationTracker{}
}

// verifyNoMutations fails the test if any mutation was recorded.
func verifyNoMutations(t *testing.T, phase string) {
	t.Helper()
	if tracker.installCalled {
		t.Errorf("dry-run %s: PackageManager.Install() was called", phase)
	}
	if tracker.removeCalled {
		t.Errorf("dry-run %s: PackageManager.Remove() was called", phase)
	}
	if tracker.startCalled {
		t.Errorf("dry-run %s: ServiceManager.Start() was called", phase)
	}
	if tracker.stopCalled {
		t.Errorf("dry-run %s: ServiceManager.Stop() was called", phase)
	}
	if tracker.enableCalled {
		t.Errorf("dry-run %s: ServiceManager.Enable() was called", phase)
	}
	if tracker.disableCalled {
		t.Errorf("dry-run %s: ServiceManager.Disable() was called", phase)
	}
	if tracker.writeFileCalled {
		t.Errorf("dry-run %s: FileSystem.WriteFile() was called", phase)
	}
	if tracker.copyFileCalled {
		t.Errorf("dry-run %s: FileSystem.CopyFile() was called", phase)
	}
	if tracker.copyDirCalled {
		t.Errorf("dry-run %s: FileSystem.CopyDir() was called", phase)
	}
	if tracker.extractCalled {
		t.Errorf("dry-run %s: Archive.Extract() was called", phase)
	}
}

type mockModule struct {
	module.Module
	name string
}

func (m *mockModule) Name() string        { return m.name }
func (m *mockModule) Description() string { return "mock " + m.name }
func (m *mockModule) Detect() (bool, error) { return true, nil }
func (m *mockModule) Doctor(ctx context.Context) (*module.DoctorResult, error) {
	return &module.DoctorResult{Module: m.name, Status: module.DoctorStatusOK}, nil
}

type noopDeps struct{ mockModule }
type noopActions struct{ mockModule }

func (m *noopDeps) Dependencies(ctx context.Context) []module.Dependency {
	return []module.Dependency{
		{Type: module.DepSystemPkg, Package: "test-pkg"},
	}
}

// TestDryRunPhaseMethods verifies every phase returns without mutation in dry-run.
func TestDryRunPhaseMethods(t *testing.T) {
	mgr := module.NewManager()
	mgr.Register(&noopDeps{mockModule{name: "test-module"}})

	plan := &RestorePlan{
		Selected: []string{"test-module"},
		Deps: []module.Dependency{
			{Type: module.DepSystemPkg, Package: "test-pkg"},
		},
		Manifest: &storage.Manifest{
			BackupID: "test-backup",
			Snapshots: []module.Snapshot{
				{Module: "test-module", Path: "/tmp/test-snap.tar.zst"},
			},
		},
		DryRunInfo: &DryRunInfo{
			Packages: []string{"test-pkg"},
		},
	}

	tests := []struct {
		name  string
		phase func(*PhaseExecutor, context.Context)
	}{
		{"InstallPhase", func(pe *PhaseExecutor, ctx context.Context) { pe.ExecuteInstallPhase(ctx) }},
		{"RestorePhase", func(pe *PhaseExecutor, ctx context.Context) { pe.ExecuteRestorePhase(ctx) }},
		{"ConfigurePhase", func(pe *PhaseExecutor, ctx context.Context) { pe.ExecuteConfigurePhase(ctx) }},
		{"ServicePhase", func(pe *PhaseExecutor, ctx context.Context) { pe.ExecuteServicePhase(ctx) }},
		{"ValidatePhase", func(pe *PhaseExecutor, ctx context.Context) { pe.ExecuteValidatePhase(ctx) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetTracker()
			pe := &PhaseExecutor{
				manager:  mgr,
				plan:     plan,
				progress: NewProgressReporter(io.Discard, 1),
				rt:       nil,
				dryRun:   true,
				workDir:  "/tmp",
			}
			ctx := context.Background()
			tt.phase(pe, ctx)
			verifyNoMutations(t, tt.name)
		})
	}
}

// TestDryRunEngineExecute verifies the engine skips all mutations in dry-run mode.
func TestDryRunEngineExecute(t *testing.T) {
	resetTracker()

	mgr := module.NewManager()
	mgr.Register(&noopDeps{mockModule{name: "test-module"}})

	plan := &RestorePlan{
		Selected: []string{"test-module"},
		Deps: []module.Dependency{
			{Type: module.DepSystemPkg, Package: "test-pkg"},
		},
		Manifest: &storage.Manifest{
			BackupID:   "test-backup",
			Hostname:   "test-host",
			Snapshots:  []module.Snapshot{{Module: "test-module", Path: "/tmp/test.tar.zst"}},
		},
		DryRunInfo: &DryRunInfo{
			Packages: []string{"test-pkg"},
		},
	}

	engine := &Engine{
		manager:   mgr,
		backupDir: "/tmp",
		manifest:  plan.Manifest,
		plan:      plan,
		progress:  NewProgressReporter(io.Discard, 1),
		dryRun:    true,
	}

	_, err := engine.Execute(context.Background(), io.Discard)
	if err != nil {
		t.Fatalf("Engine.Execute dry-run failed: %v", err)
	}

	verifyNoMutations(t, "engine.Execute")
}
