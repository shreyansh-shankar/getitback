<div align="center">

<img src="https://raw.githubusercontent.com/shreyansh-shankar/getitback/main/docs/assets/logo.svg" alt="getitback logo" width="120" />

# getitback

**Developer workstation disaster recovery — in one command.**

[![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?style=flat-square&logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue?style=flat-square)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/shreyansh-shankar/getitback?style=flat-square)](https://goreportcard.com/report/github.com/shreyansh-shankar/getitback)
[![Release](https://img.shields.io/github/v/release/shreyansh-shankar/getitback?style=flat-square)](https://github.com/shreyansh-shankar/getitback/releases)
[![CI](https://img.shields.io/github/actions/workflow/status/shreyansh-shankar/getitback/ci.yml?branch=main&label=CI&style=flat-square)](https://github.com/shreyansh-shankar/getitback/actions)
[![Modules](https://img.shields.io/badge/modules-35+-success?style=flat-square)](#-supported-modules)

</div>

---

`getitback` is a **zero-dependency CLI tool** that backs up, restores, and verifies your entire development environment — including SSH keys, shell config, editor settings, browser profiles, databases, cloud credentials, container configs, and more.

Stop spending days rebuilding your machine. **Get it back in minutes.**

```
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Stage 1 / 6 · Initializing
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  ID ...................... 20260101T120000Z
  Destination ............. /home/user/.getitback/backups/...
  Encryption .............. Enabled

  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  Stage 4 / 6 · Backing Up
  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Identity
    ✓ ssh .............. 12.4 KB
    ✓ gpg .............. 4.2 KB

  Development
    ✓ git .............. 3.1 KB
    ✓ golang ........... 2.8 KB
    ✓ node ............. 1.9 KB
    ✓ python ........... 8.6 KB
    ✓ rust ............. 1.2 KB

  Databases
    ✓ postgres ......... 48.2 KB
    ✓ redis ............ 2.1 KB
    ✓ mongodb .......... 14.7 KB

  Recovery Readiness
    before ............. 42/100 (D)
    after .............. 94/100 (A)
```

---

## ✨ Features

- 🔐 **End-to-end encryption** — [age](https://age-encryption.org/)-based encryption with X25519 keys
- 📦 **35+ built-in modules** — covers the full developer toolchain out of the box
- 🔍 **Smart detection** — only backs up what's actually installed
- 🩺 **Doctor mode** — diagnoses your recovery readiness before disaster strikes
- 🔁 **Automated restore** — dependency resolution, software installation, and data restoration in one pass
- ✅ **Snapshot verification** — SHA-256 checksums, archive integrity, and file-count validation
- 📊 **Recovery score** — A–F graded assessment of your backup coverage
- 🗂️ **Multiple output formats** — terminal, JSON, YAML, Markdown
- 🧩 **Pluggable architecture** — implement the `Module` interface to add custom modules
- 🚀 **Zero runtime dependencies** — single static binary, no agents, no daemons

---

## 🚀 Quick Start

### Install

**From source (recommended):**
```bash
git clone https://github.com/shreyansh-shankar/getitback.git
cd getitback
make install       # installs to /usr/local/bin
```

**Binary release:**
```bash
curl -sSL https://github.com/shreyansh-shankar/getitback/releases/latest/download/getitback-linux-amd64.tar.gz | tar -xz
sudo mv getitback /usr/local/bin/
```

### Backup your machine
```bash
getitback backup
```

### Restore on a new machine
```bash
sudo getitback restore /path/to/backup
```

### Check recovery readiness
```bash
getitback doctor
```

---

## 📖 Commands

| Command | Description |
|---------|-------------|
| `getitback backup` | Create a complete, encrypted machine snapshot |
| `getitback restore <path>` | Restore from a backup (auto-discovers latest if no path given) |
| `getitback verify` | Verify snapshot integrity (checksums + archive validation) |
| `getitback doctor` | Diagnose recovery readiness and surface issues |
| `getitback status` | Show all backups with size, age, and recovery score |
| `getitback inventory` | List all detected software and resources |
| `getitback modules` | Browse all available modules and their capabilities |
| `getitback report` | Generate a detailed recovery report |
| `getitback secrets` | Manage encryption keys |

### Common flags

```bash
# Backup a single module
getitback backup --module ssh

# Dry-run restore (see what would happen)
sudo getitback restore --dry-run

# Restore a specific module from a specific backup
sudo getitback restore --id 20260101T120000Z --module vscode

# Use a preset
sudo getitback restore --preset dev        # Development tools only
sudo getitback restore --preset secrets    # Identity & credentials
sudo getitback restore --preset browsers   # Browser profiles
sudo getitback restore --preset infra      # Docker, K8s, cloud

# JSON output
getitback status --output json
getitback inventory --output json
```

---

## 🧩 Supported Modules

<details>
<summary><strong>Identity & Security (4 modules)</strong></summary>

| Module | What it backs up | Recovery Value |
|--------|-----------------|----------------|
| `ssh` | Keys (ed25519/RSA/ECDSA), config, known_hosts, authorized_keys | Critical |
| `gpg` | Public/private keyrings, trust database, gpg-agent config | Critical |
| `certs` | Custom CA certificates, certificate bundles | High |
| `system` | Hostname, locale, timezone, environment variables | Medium |

</details>

<details>
<summary><strong>Configuration (2 modules)</strong></summary>

| Module | What it backs up | Recovery Value |
|--------|-----------------|----------------|
| `dotfiles` | `.bashrc`, `.zshrc`, `.profile`, `.env` files, XDG config | Critical |
| `shell` | Shell history, aliases, functions, completions | High |

</details>

<details>
<summary><strong>Editors & IDEs (2 modules)</strong></summary>

| Module | What it backs up | Recovery Value |
|--------|-----------------|----------------|
| `vscode` | Extensions list, `settings.json`, keybindings, snippets | High |
| `neovim` | Full config directory, plugin manifests | High |

</details>

<details>
<summary><strong>Development Runtimes (6 modules)</strong></summary>

| Module | What it backs up | Recovery Value |
|--------|-----------------|----------------|
| `git` | Global config, credentials, ignore patterns | High |
| `golang` | `GOPATH`, installed tools, `go env` | Medium |
| `node` | Global packages (npm/yarn/pnpm), `.npmrc`, `.nvmrc` | Medium |
| `python` | pip packages, pyenv versions, virtualenv list | Medium |
| `rust` | Installed toolchains, targets, cargo packages | Medium |
| `java` | SDKMAN candidates, JAVA_HOME config | Medium |

</details>

<details>
<summary><strong>Databases (5 modules)</strong></summary>

| Module | What it backs up | Recovery Value |
|--------|-----------------|----------------|
| `postgres` | `pg_dumpall` of all databases, `pg_hba.conf`, role config | High |
| `mongodb` | `mongodump` of all databases and collections | High |
| `mysql` | Full dump via `mysqldump`, user grants | High |
| `redis` | `dump.rdb`, `redis.conf` | Medium |
| `sqlite` | All `.db`/`.sqlite` files discovered in the home directory | Medium |

</details>

<details>
<summary><strong>Browsers (7 modules)</strong></summary>

| Module | What it backs up | Recovery Value |
|--------|-----------------|----------------|
| `firefox` | Profiles, bookmarks, extensions, cookies | Medium |
| `chrome` | Bookmarks, extensions list, preferences | Medium |
| `chromium` | Bookmarks, extensions list, preferences | Medium |
| `brave` | Bookmarks, extensions, Brave-specific settings | Medium |
| `vivaldi` | Full profile backup | Medium |
| `edge` | Bookmarks, extensions, preferences | Low |
| `opera` | Bookmarks, extensions, preferences | Low |

</details>

<details>
<summary><strong>Packages (3 modules)</strong></summary>

| Module | What it backs up | Recovery Value |
|--------|-----------------|----------------|
| `apt` | `dpkg --get-selections`, manually installed packages list | High |
| `snap` | Installed snap list | Medium |
| `flatpak` | Installed flatpak list | Medium |

</details>

<details>
<summary><strong>Containers & Cloud (4 modules)</strong></summary>

| Module | What it backs up | Recovery Value |
|--------|-----------------|----------------|
| `docker` | `daemon.json`, compose files, image/container lists | High |
| `kubernetes` | `~/.kube/config`, context list | High |
| `cloud` | AWS/GCP/Azure CLI configs and credentials | High |
| `virtualization` | libvirt VM definitions, VirtualBox VMs list | Medium |

</details>

<details>
<summary><strong>Projects (1 module)</strong></summary>

| Module | What it backs up | Recovery Value |
|--------|-----------------|----------------|
| `repos` | Git repository inventory with remote URLs for re-cloning | High |

</details>

---

## 🔐 Encryption

`getitback` uses [age](https://age-encryption.org/) (X25519 key exchange) for at-rest encryption of all snapshots.

```bash
# Generate a new key pair
getitback secrets generate

# Enable encryption in config
# ~/.getitback/config.yaml:
encryption:
  enabled: true
  key_path: ~/.getitback/key.txt
```

> **Important:** Back up your `key.txt` separately (e.g., print it, store it in a password manager). Without it, your encrypted snapshots cannot be decrypted.

---

## ⚙️ Configuration

Config lives at `~/.getitback/config.yaml` and is auto-created on first run.

```yaml
storage:
  path: /home/user/.getitback/backups   # where backups are stored

encryption:
  enabled: false                         # set to true to enable
  key_path: /home/user/.getitback/key.txt
```

---

## 🏗️ Architecture

```
getitback/
├── cmd/getitback/        # Binary entry point — registers all modules
├── internal/
│   ├── cli/              # Cobra commands (backup, restore, verify, doctor …)
│   ├── module/           # Module interface, types, manager, registry, module info
│   ├── modules/          # 35+ module implementations
│   │   ├── ssh/
│   │   ├── git/
│   │   ├── postgres/
│   │   └── ...
│   ├── archive/          # tar+zstd streaming read/write
│   ├── crypto/           # age encryption/decryption
│   ├── storage/          # Manifest, checksums, per-module metadata
│   ├── config/           # Config loading (viper)
│   ├── restore/          # Multi-stage restore engine & planner
│   ├── assessment/       # Recovery score computation
│   ├── runtime/          # Cross-platform command executor
│   ├── report/           # Report types
│   ├── doctor/           # Doctor logic
│   └── output/           # Terminal/JSON/YAML/Markdown renderers
└── test/
    └── integration/      # Integration test suite
```

### Module Interface

Every module implements a single interface:

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

Optional interfaces extend the restore pipeline:

```go
type DependencyProvider interface { Dependencies(ctx context.Context) []Dependency }
type Installer          interface { Install(ctx context.Context, opts RestoreOptions) error }
type Configurer         interface { Configure(ctx context.Context, opts RestoreOptions) error }
type Validator          interface { Validate(ctx context.Context, snap Snapshot) (*ValidateResult, error) }
```

### Backup Pipeline

```
Detect → Inventory → Plan → Backup (tar+zstd) → Encrypt (age) → Manifest + SHA256SUMS
```

### Restore Pipeline

```
Load Manifest → Plan → Dependency Resolution → Install → Restore → Configure → Validate → Report
```

---

## 🛠️ Development

### Prerequisites

- Go 1.24+
- `make`

### Build

```bash
make build        # build binary to ./getitback
make install      # install to /usr/local/bin
make clean        # remove build artifacts
```

### Run tests

```bash
go test ./...
go test ./internal/archive/...
go test ./internal/module/...
go test ./internal/config/...
```

### Adding a new module

1. Create `internal/modules/<name>/module.go`
2. Implement the `module.Module` interface
3. Optionally implement `DependencyProvider`, `Installer`, `Configurer`, `Validator`
4. Register in `cmd/getitback/main.go`: `manager.Register(<name>.NewModule())`
5. Add module info to `internal/module/info.go`
6. Add to `internal/cli/modulegroups.go` for category grouping

See [`internal/modules/ssh`](internal/modules/ssh) for a minimal reference implementation.

---

## 🤝 Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.

**Good first issues:**
- Adding a new module (see guide above)
- Improving test coverage
- macOS support improvements
- Windows support (WSL2)

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

<div align="center">
Made with ❤️ for developers who've lost their machine and had to start from scratch.
</div>
