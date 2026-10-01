# Contributing to getitback

Thank you for taking the time to contribute! 🎉

This document covers everything you need to know to get started.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How Can I Contribute?](#how-can-i-contribute)
  - [Reporting Bugs](#reporting-bugs)
  - [Suggesting Features](#suggesting-features)
  - [Adding a New Module](#adding-a-new-module)
  - [Improving Tests](#improving-tests)
  - [Documentation](#documentation)
- [Development Setup](#development-setup)
- [Pull Request Process](#pull-request-process)
- [Commit Convention](#commit-convention)
- [Code Style](#code-style)

---

## Code of Conduct

By participating in this project you agree to abide by the [Contributor Covenant](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). Be respectful, constructive, and kind.

---

## How Can I Contribute?

### Reporting Bugs

Before opening a bug report, please search existing issues.

When opening a bug report, include:

- **OS and version** (e.g., Ubuntu 24.04, macOS 14.4)
- **getitback version** (`getitback --version`)
- **Steps to reproduce** — be specific and minimal
- **Actual vs. expected behaviour**
- **Relevant output** — redact any sensitive info (keys, passwords)

### Suggesting Features

Open a [Feature Request](https://github.com/shreyansh-shankar/getitback/issues/new?template=feature_request.md) with:

- The use case or problem you're solving
- Your proposed solution
- Alternatives you've considered

### Adding a New Module

New module PRs are the most impactful contribution. Modules extend `getitback` to back up new tools or environments.

#### Step-by-step guide

1. **Create the module directory:**
   ```
   internal/modules/<name>/
   └── module.go
   ```

2. **Implement the `module.Module` interface:**

   ```go
   package <name>

   import (
       "context"
       "github.com/shreyansh-shankar/getitback/internal/module"
   )

   type Module struct{}

   func NewModule() *Module { return &Module{} }

   func (m *Module) Name()        string { return "<name>" }
   func (m *Module) Description() string { return "Short description" }

   func (m *Module) Detect() (bool, error) {
       // Return true if this tool is installed on the current machine.
       // Use exec.LookPath or check for config dirs.
   }

   func (m *Module) Inventory(ctx context.Context) (*module.InventoryResult, error) {
       // Discover all resources this module manages.
   }

   func (m *Module) Backup(ctx context.Context, opts module.BackupOptions) (*module.BackupResult, error) {
       // Snapshot resources into opts.SnapshotsDir as a .tar.zst archive.
       // Use internal/archive helpers.
   }

   func (m *Module) Restore(ctx context.Context, snap module.Snapshot, opts module.RestoreOptions) error {
       // Extract the snapshot archive and place files at their correct paths.
   }

   func (m *Module) Verify(ctx context.Context, snap module.Snapshot) (*module.VerifyResult, error) {
       // Validate that the snapshot is intact (checksum, archive readable, etc.)
   }

   func (m *Module) Doctor(ctx context.Context) (*module.DoctorResult, error) {
       // Check health: is the tool installed? Are critical files present?
   }
   ```

3. **Optionally implement restore-phase interfaces** (`DependencyProvider`, `Installer`, `Configurer`, `Validator`) for automated restore support.

4. **Register the module** in `cmd/getitback/main.go`:
   ```go
   import "github.com/shreyansh-shankar/getitback/internal/modules/<name>"
   // ...
   manager.Register(<name>.NewModule())
   ```

5. **Add module metadata** to `internal/module/info.go`:
   ```go
   "<name>": {
       Name: "<Display Name>",
       Description: "...",
       Category: "Development",   // or Identity, Editors, Databases, etc.
       Maturity: MaturityBeta,
       Platforms: []string{"Linux", "macOS"},
       DataCollected: []string{"..."},
       RecoveryValue: "High",
   },
   ```

6. **Add to the module group map** in `internal/cli/modulegroups.go`:
   ```go
   "<name>": "Development",
   ```

7. **Write tests** in `internal/modules/<name>/module_test.go`.

8. **Document** what is collected in the module's `DataCollected` field and in the README module table.

#### Reference implementations

- Simple (no dependencies): [`internal/modules/git`](internal/modules/git)
- With database dump: [`internal/modules/postgres`](internal/modules/postgres)
- With browser profile: [`internal/modules/firefox`](internal/modules/firefox)

### Improving Tests

- Unit tests belong in `*_test.go` files alongside the code they test.
- Integration tests go in `test/integration/`.
- Aim for table-driven tests using `t.Run`.
- Mock external commands using an injectable executor — see `internal/runtime/executor`.

### Documentation

- Fix typos, clarify confusing sections, add examples — all welcome!
- Docs live in `README.md`, `CONTRIBUTING.md`, and inline Go doc comments.
- For larger doc changes, open an issue first to align on scope.

---

## Development Setup

```bash
# Clone
git clone https://github.com/shreyansh-shankar/getitback.git
cd getitback

# Build
make build

# Run tests
go test ./...

# Run a specific package
go test ./internal/archive/... -v

# Install locally for manual testing
make install
```

**Prerequisites:** Go 1.24+, `make`, `git`

---

## Pull Request Process

1. **Fork** the repo and create your branch from `main`:
   ```bash
   git checkout -b feat/my-new-module
   ```

2. **Write your code** following the style guide below.

3. **Add tests** — PRs without tests for new logic will not be merged.

4. **Run checks locally:**
   ```bash
   go vet ./...
   go test ./...
   go mod tidy
   ```

5. **Push and open a PR** against `main`. Use the PR template.

6. **Address review feedback** — maintainers aim to review within 48 hours.

7. Once approved, a maintainer will **squash-merge** your PR.

---

## Commit Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short description>

[optional body]

[optional footer]
```

**Types:** `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `perf`, `ci`

**Examples:**
```
feat(modules): add helix editor module
fix(restore): handle missing workdir gracefully
docs(readme): add Windows WSL2 install instructions
test(archive): add table-driven tests for OpenReader
```

---

## Code Style

- Follow standard Go conventions (`go fmt`, `go vet`)
- Keep functions small and focused — prefer composition over long functions
- Prefer explicit error handling over panics
- Use `context.Context` for all potentially long-running operations
- Export only what needs to be exported (keep packages focused)
- Add Go doc comments to all exported types and functions

---

Thank you for helping make `getitback` better! 🚀
