package module_test

import (
	"context"
	"testing"

	"github.com/shreyansh-shankar/getitback/internal/module"
)

type fakeModule struct {
	name        string
	description string
	detected    bool
	detectErr   error
	inventory   *module.InventoryResult
	inventoryErr error
}

func (m *fakeModule) Name() string                          { return m.name }
func (m *fakeModule) Description() string                   { return m.description }
func (m *fakeModule) Detect() (bool, error)                { return m.detected, m.detectErr }
func (m *fakeModule) Inventory(context.Context) (*module.InventoryResult, error) { return m.inventory, m.inventoryErr }
func (m *fakeModule) Backup(context.Context, module.BackupOptions) (*module.BackupResult, error) { return nil, nil }
func (m *fakeModule) Restore(context.Context, module.Snapshot, module.RestoreOptions) error { return nil }
func (m *fakeModule) Verify(context.Context, module.Snapshot) (*module.VerifyResult, error) { return nil, nil }
func (m *fakeModule) Doctor(context.Context) (*module.DoctorResult, error) { return nil, nil }

func TestManager_Register(t *testing.T) {
	mgr := module.NewManager()
	mgr.Register(&fakeModule{name: "test"})

	if _, ok := mgr.Get("test"); !ok {
		t.Error("expected module 'test' to be registered")
	}
}

func TestManager_RegisterDuplicate(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate registration")
		}
	}()
	mgr := module.NewManager()
	mgr.Register(&fakeModule{name: "dup"})
	mgr.Register(&fakeModule{name: "dup"})
}

func TestManager_Get_NotFound(t *testing.T) {
	mgr := module.NewManager()
	if _, ok := mgr.Get("nonexistent"); ok {
		t.Error("expected false for nonexistent module")
	}
}

func TestManager_All(t *testing.T) {
	mgr := module.NewManager()
	mgr.Register(&fakeModule{name: "z_last"})
	mgr.Register(&fakeModule{name: "a_first"})

	all := mgr.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(all))
	}
	if all[0].Name() != "a_first" {
		t.Errorf("expected first module to be 'a_first', got %s", all[0].Name())
	}
}

func TestManager_Detect(t *testing.T) {
	mgr := module.NewManager()
	mgr.Register(&fakeModule{name: "detected", detected: true})
	mgr.Register(&fakeModule{name: "not_detected", detected: false})

	ctx := context.Background()
	results := mgr.Detect(ctx)

	if !results["detected"].Detected {
		t.Error("expected 'detected' to be detected")
	}
	if results["not_detected"].Detected {
		t.Error("expected 'not_detected' to not be detected")
	}
}

func TestManager_Inventory_SkipsUndetected(t *testing.T) {
	mgr := module.NewManager()
	mgr.Register(&fakeModule{
		name:      "undetected",
		detected:  false,
		inventory: &module.InventoryResult{Module: "undetected", Detected: true},
	})

	ctx := context.Background()
	results := mgr.Inventory(ctx)

	if len(results) != 0 {
		t.Errorf("expected 0 results for undetected module, got %d", len(results))
	}
}
