# Architecture

This document describes the internal design of `getitback`.

---

## Overview

`getitback` is structured as a **modular CLI application**. Each piece of developer tooling is represented as a *module* that implements a standard interface. The CLI orchestrates modules through a *manager* and executes multi-stage pipelines for backup and restore.

```
┌─────────────────────────────────────────────────────────────────┐
│                          getitback CLI                          │
│                                                                 │
│  backup  │  restore  │  verify  │  doctor  │  status  │  ...   │
└──────────────────────────────┬──────────────────────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │   module.Manager    │
                    │                     │
                    │  Register / Get /   │
                    │  All / Inventory    │
                    └──────────┬──────────┘
                               │
         ┌─────────────────────┼─────────────────────┐
         │                     │                     │
   ┌─────▼─────┐         ┌─────▼─────┐        ┌─────▼─────┐
   │  ssh mod  │         │ postgres  │        │  docker   │
   │  Detect   │         │  Detect   │        │  Detect   │
   │  Backup   │   ...   │  Backup   │  ...   │  Backup   │
   │  Restore  │         │  Restore  │        │  Restore  │
   │  Verify   │         │  Verify   │        │  Verify   │
   │  Doctor   │         │  Doctor   │        │  Doctor   │
   └───────────┘         └───────────┘        └───────────┘
```

---

## Package Map

| Package | Responsibility |
|---------|---------------|
| `cmd/getitback` | Entry point; registers modules, calls `cli.Execute` |
| `internal/cli` | Cobra commands; orchestrates backup/restore pipelines |
| `internal/module` | Module interface, types, manager, registry, module info |
| `internal/modules/*` | 35+ module implementations |
| `internal/archive` | Streaming `tar+zstd` read/write |
| `internal/crypto` | `age` X25519 encrypt/decrypt |
| `internal/storage` | Manifest, SHA256SUMS, per-module metadata I/O |
| `internal/config` | Viper-based YAML config |
| `internal/restore` | Multi-stage restore engine and planner |
| `internal/assessment` | Recovery score computation |
| `internal/runtime` | Cross-platform command executor |
| `internal/output` | Terminal/JSON/YAML/Markdown renderers |
| `internal/report` | Recovery report types |
| `internal/doctor` | Doctor aggregation logic |

---

## Module Interface

```go
type Module interface {
    Name()        string
    Description() string
    Detect()      (bool, error)
    Inventory(ctx context.Context) (*InventoryResult, error)
    Backup(ctx context.Context, opts BackupOptions) (*BackupResult, error)
    Restore(ctx context.Context, snap Snapshot, opts RestoreOptions) error
    Verify(ctx context.Context, snap Snapshot) (*VerifyResult, error)
    Doctor(ctx context.Context) (*DoctorResult, error)
}
```

### Optional restore-phase interfaces

Modules that implement these interfaces participate in the automated restore pipeline:

| Interface | Method | Phase |
|-----------|--------|-------|
| `DependencyProvider` | `Dependencies(ctx) []Dependency` | Pre-install |
| `Installer` | `Install(ctx, opts) error` | Install |
| `Configurer` | `Configure(ctx, opts) error` | Configure |
| `Validator` | `Validate(ctx, snap) (*ValidateResult, error)` | Validate |

---

## Backup Pipeline

```
Stage 1: Initialize
  └─ Create backup directory: $BACKUP_ROOT/<backupID>/
  └─ Create snapshots/, metadata/ subdirs
  └─ Load encryption key if enabled

Stage 2: Inventory
  └─ Run module.Manager.Inventory(ctx)
  └─ Write inventory.json

Stage 3: Plan
  └─ Filter modules (all, or --module <name>)
  └─ Display plan: total, detected, skip

Stage 4: Backup (per module, grouped by category)
  └─ Skip modules where Detect() → false
  └─ Call module.Backup(ctx, opts)
  └─ Archive: tar+zstd → snapshots/<module>.tar.zst
  └─ Record: size, checksum, duration, status
  └─ Write per-module metadata → metadata/<module>.json

Stage 5: Finalize
  └─ Encrypt snapshots if enabled (age X25519)
  └─ Write manifest.json
  └─ Write SHA256SUMS
  └─ Write backup.log

Stage 6: Summary
  └─ Display: counts, size, recovery score, critical assets
  └─ Recommend next command (verify / doctor)
```

### Backup directory layout

```
~/.getitback/backups/20260101T120000Z/
├── manifest.json          # Full snapshot index + inventory
├── inventory.json         # Detected modules at backup time
├── SHA256SUMS             # Checksums for all snapshot files
├── backup.log             # Execution log
├── snapshots/
│   ├── ssh.tar.zst[.age]
│   ├── git.tar.zst[.age]
│   ├── postgres.tar.zst[.age]
│   └── ...
└── metadata/
    ├── ssh.json
    ├── git.json
    └── ...
```

---

## Restore Pipeline

```
Stage 1:  Load Manifest      — read manifest.json, validate backup
Stage 2:  Restore Plan       — select modules (preset / interactive / --module)
Stage 3:  Dependency Resolve — topological sort on DependencyProvider modules
Stage 4:  Install            — run Installer.Install() per module
Stage 5:  Restore Data       — extract archives, call module.Restore()
Stage 6:  Post-Restore Hooks — Configurer.Configure() per module
Stage 7:  Service Startup    — start required services
Stage 8:  Validation         — Validator.Validate() per module
Stage 9:  Recovery Report    — structured report (JSON / terminal)
Stage 10: Completion         — summary with any manual steps required
```

---

## Encryption

`getitback` uses [age](https://age-encryption.org/) with X25519 (native X25519 key exchange):

1. On first use: `getitback secrets generate` creates an age identity in `~/.getitback/key.txt`
2. During backup: each snapshot is passed through `age.Encrypt` using the X25519 recipient derived from the identity. The encrypted file gets a `.age` suffix; the plaintext is removed.
3. During restore: `age.Decrypt` uses the identity from `key.txt` to decrypt each snapshot before extraction.

**The key file is never included in the backup.** Losing it means losing access to encrypted backups.

---

## Storage Format

### `manifest.json`

```json
{
  "version": "1",
  "backupVersion": "1",
  "backupID": "20260101T120000Z",
  "createdAt": "2026-01-01T12:00:00Z",
  "hostname": "mybox",
  "platform": "linux",
  "architecture": "amd64",
  "compression": "zstd",
  "encryption": "age",
  "backupSize": 12345678,
  "snapshots": [
    {
      "module": "ssh",
      "path": ".../snapshots/ssh.tar.zst.age",
      "size": 12416,
      "checksum": "sha256:...",
      "encrypted": true,
      "compression": "zstd",
      "duration": "120ms",
      "status": "success",
      "recoveryValue": "Critical"
    }
  ],
  "inventory": [ ... ]
}
```

---

## Recovery Score

The recovery score is computed by `internal/assessment` based on the detected inventory and backup coverage. It produces a 0–100 score graded A–F:

| Grade | Score |
|-------|-------|
| A | 90–100 |
| B | 80–89 |
| C | 60–79 |
| D | 40–59 |
| F | 0–39 |

Weights are assigned per category (Identity is weighted highest, Browsers lowest).
