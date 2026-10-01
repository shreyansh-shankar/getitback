# Changelog

All notable changes to `getitback` will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- GitHub Actions CI/CD pipeline (lint, test, build, release)
- Multi-platform release builds (Linux/macOS amd64/arm64)
- Security scanning workflow (govulncheck + CodeQL)
- `CONTRIBUTING.md` with full module development guide
- `SECURITY.md` with vulnerability reporting policy
- GitHub issue templates (bug report, feature request, new module)
- `.gitignore` for Go projects
- Comprehensive `README.md` with badges, module catalog, and architecture docs

---

## [0.4.0] - 2026-10-01

### Added
- Restore engine with dry-run capabilities (`--dry-run` flag)
- Detailed execution plan display before restore
- `--workdir` flag with fallback chain (`GETITBACK_WORKDIR` → `$HOME/.cache/getitback` → `/tmp`)
- Action model for service management and runtime operations
- Report types for comprehensive recovery reporting

### Changed
- Restore now requires root privileges and handles `SUDO_USER` env for correct home resolution

---

## [0.3.0] - 2026-09-15

### Added
- 35 built-in modules covering all major developer toolchain categories
- `getitback modules` command to browse all modules with capabilities
- `getitback secrets` command for encryption key management
- `getitback report` command for detailed recovery reporting
- Recovery score (A–F grading) in backup summary
- Per-category backup progress bar
- Module grouping by category in backup/restore UI

### Changed
- Backup output reorganised into 6 clearly-labelled stages
- Snapshot metadata enriched with duration, compression ratio, recovery value

---

## [0.2.0] - 2026-08-20

### Added
- `getitback verify` — multi-level snapshot integrity verification (SHA-256, archive readable, file count, original size)
- `getitback doctor` — per-module health checks and recovery diagnostics
- `getitback status` — backup history with age, size, and health summary
- `getitback inventory` — structured inventory of detected software and resources
- Encryption support via `filippo.io/age` (X25519)
- Restore presets: `everything`, `dev`, `browsers`, `secrets`, `infra`
- Interactive module selection in restore

---

## [0.1.0] - 2026-07-01

### Added
- Initial release
- `getitback backup` and `getitback restore` commands
- `tar+zstd` compression for all snapshots
- JSON manifest with SHA-256 checksums
- Viper-based YAML configuration (`~/.getitback/config.yaml`)
- Module interface (`Detect`, `Inventory`, `Backup`, `Restore`, `Verify`, `Doctor`)
- Module manager with `Register`/`Get`/`All`/`Inventory` helpers
- First-party modules: `ssh`, `gpg`, `git`, `dotfiles`, `shell`, `vscode`, `neovim`

[Unreleased]: https://github.com/shreyansh-shankar/getitback/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/shreyansh-shankar/getitback/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/shreyansh-shankar/getitback/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/shreyansh-shankar/getitback/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/shreyansh-shankar/getitback/releases/tag/v0.1.0
