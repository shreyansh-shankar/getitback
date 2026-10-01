# Module Development Guide

This guide walks you through creating a new `getitback` module from scratch.

---

## What is a Module?

A module is a Go struct that implements the `module.Module` interface. It represents one tool or service in a developer's environment (e.g., VS Code, PostgreSQL, Firefox).

Each module is responsible for:
- **Detecting** whether the tool is installed
- **Inventorying** what data it manages
- **Backing up** that data as a compressed archive
- **Restoring** data from an archive
- **Verifying** archive integrity
- **Doctor**-ing its own health

---

## Quick Start

### 1. Create the module file

```bash
mkdir -p internal/modules/mytool
touch internal/modules/mytool/module.go
```

### 2. Implement the interface

```go
package mytool

import (
    "context"
    "os"
    "path/filepath"

    "github.com/shreyansh-shankar/getitback/internal/archive"
    "github.com/shreyansh-shankar/getitback/internal/module"
)

type Module struct{}

func NewModule() *Module { return &Module{} }

func (m *Module) Name() string        { return "mytool" }
func (m *Module) Description() string { return "MyTool configuration and data" }

// Detect returns true if the tool appears to be installed.
func (m *Module) Detect() (bool, error) {
    home, err := os.UserHomeDir()
    if err != nil {
        return false, err
    }
    _, err = os.Stat(filepath.Join(home, ".config", "mytool"))
    return err == nil, nil
}

// Inventory lists all files/resources managed by this module.
func (m *Module) Inventory(ctx context.Context) (*module.InventoryResult, error) {
    ok, err := m.Detect()
    if err != nil || !ok {
        return &module.InventoryResult{Module: m.Name(), Detected: false}, nil
    }

    home, _ := os.UserHomeDir()
    configDir := filepath.Join(home, ".config", "mytool")

    result := &module.InventoryResult{
        Module:   m.Name(),
        Detected: true,
    }

    filepath.WalkDir(configDir, func(path string, d os.DirEntry, err error) error {
        if err != nil || d.IsDir() {
            return nil
        }
        info, _ := d.Info()
        result.Resources = append(result.Resources, module.Resource{
            Name:     d.Name(),
            Path:     path,
            Size:     info.Size(),
            Modified: info.ModTime(),
            Type:     module.ResourceTypeConfig,
        })
        return nil
    })

    return result, nil
}

// Backup creates a tar.zst archive of all module data.
func (m *Module) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
    ok, err := m.Detect()
    if err != nil || !ok {
        return &module.BackupResult{Module: m.Name()}, nil
    }

    home, _ := os.UserHomeDir()
    configDir := filepath.Join(home, ".config", "mytool")
    archivePath := filepath.Join(opts.SnapshotsDir, m.Name()+".tar.zst")

    snap, err := archive.CreateFromDir(configDir, archivePath)
    if err != nil {
        return nil, err
    }
    snap.Module = m.Name()

    return &module.BackupResult{
        Module:    m.Name(),
        Snapshots: []module.Snapshot{*snap},
        Contents:  []string{"~/.config/mytool"},
    }, nil
}

// Restore extracts the archive and places files at their correct paths.
func (m *Module) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
    home, err := os.UserHomeDir()
    if err != nil {
        return err
    }
    return archive.ExtractToDir(snap.Path, filepath.Join(home, ".config", "mytool"))
}

// Verify checks that the snapshot archive is intact.
func (m *Module) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
    result := &module.VerifyResult{
        Module:   m.Name(),
        Snapshot: snap,
    }
    if err := archive.Verify(snap.Path); err != nil {
        result.Valid = false
        result.Errors = []string{err.Error()}
        return result, nil
    }
    result.Valid = true
    return result, nil
}

// Doctor checks whether the tool is correctly set up.
func (m *Module) Doctor(ctx context.Context) (*module.DoctorResult, error) {
    ok, _ := m.Detect()
    if !ok {
        return &module.DoctorResult{
            Module: m.Name(),
            Status: module.DoctorStatusOK,
        }, nil
    }

    // Add real health checks here
    return &module.DoctorResult{
        Module: m.Name(),
        Status: module.DoctorStatusOK,
    }, nil
}
```

### 3. Register the module

In `cmd/getitback/main.go`:

```go
import "github.com/shreyansh-shankar/getitback/internal/modules/mytool"
// ...
manager.Register(mytool.NewModule())
```

### 4. Add module info

In `internal/module/info.go`, add to `moduleInfoMap`:

```go
"mytool": {
    Name:          "MyTool",
    Description:   "MyTool configuration and data",
    Category:      "Development",   // Identity | Configuration | Editors | Development | Databases | Browsers | Packages | Containers | Cloud | Projects
    Maturity:      MaturityBeta,    // MaturityStable | MaturityBeta | MaturityExperimental
    Platforms:     []string{"Linux", "macOS"},
    DataCollected: []string{"~/.config/mytool"},
    RecoveryValue: "Medium",        // Critical | High | Medium | Low
},
```

### 5. Add to the group map

In `internal/cli/modulegroups.go`:

```go
"mytool": "Development",
```

### 6. Write tests

```go
// internal/modules/mytool/module_test.go
package mytool_test

import (
    "context"
    "testing"

    "github.com/shreyansh-shankar/getitback/internal/modules/mytool"
)

func TestDetect(t *testing.T) {
    m := mytool.NewModule()
    _, err := m.Detect()
    if err != nil {
        t.Fatalf("Detect() returned unexpected error: %v", err)
    }
}

func TestInventory_NotInstalled(t *testing.T) {
    m := mytool.NewModule()
    ctx := context.Background()
    inv, err := m.Inventory(ctx)
    if err != nil {
        t.Fatalf("Inventory() error: %v", err)
    }
    if inv == nil {
        t.Fatal("Inventory() returned nil")
    }
}
```

---

## Advanced: Restore Pipeline Interfaces

To participate in the automated restore pipeline (dependency resolution, installation, configuration, validation), implement any of these optional interfaces:

### DependencyProvider

```go
func (m *Module) Dependencies(ctx context.Context) []module.Dependency {
    return []module.Dependency{
        {
            Type:    module.DepSystemPkg,
            Package: "mytool",
            Hint:    "Install from https://mytool.io",
        },
    }
}
```

### Installer

```go
func (m *Module) Install(ctx context.Context, opts module.RestoreOptions) error {
    // Install the software (e.g., via apt, snap, curl)
    return nil
}
```

### Configurer

```go
func (m *Module) Configure(ctx context.Context, opts module.RestoreOptions) error {
    // Post-restore configuration (e.g., set permissions, reload config)
    return nil
}
```

### Validator

```go
func (m *Module) Validate(ctx context.Context, snap module.Snapshot) (*module.ValidateResult, error) {
    // Verify the tool is working correctly after restore
    return &module.ValidateResult{
        Module:  m.Name(),
        Success: true,
        Checks:  []string{"config file present", "tool executable found"},
    }, nil
}
```

---

## Reference Implementations

| Module | Features demonstrated |
|--------|-----------------------|
| [`internal/modules/ssh`](../internal/modules/ssh) | File discovery, secure permissions, `Critical` recovery value |
| [`internal/modules/git`](../internal/modules/git) | Simple config backup, version detection |
| [`internal/modules/postgres`](../internal/modules/postgres) | External command (`pg_dump`), multi-snapshot |
| [`internal/modules/firefox`](../internal/modules/firefox) | Browser profile discovery, large data |
| [`internal/modules/vscode`](../internal/modules/vscode) | Extension list export + settings backup |

---

## Checklist before opening a PR

- [ ] Module compiles (`go build ./...`)
- [ ] `Detect()` is cheap (stat check or `exec.LookPath`) — no network calls
- [ ] Backup creates a `.tar.zst` file via `archive.CreateFromDir` or equivalent
- [ ] Restore is idempotent (safe to run twice)
- [ ] `Doctor()` returns `DoctorStatusOK` when the tool is not installed (don't error on optional tools)
- [ ] Module info added to `internal/module/info.go`
- [ ] Module registered in `cmd/getitback/main.go`
- [ ] Category added to `internal/cli/modulegroups.go`
- [ ] At least one unit test written
- [ ] README module table updated
