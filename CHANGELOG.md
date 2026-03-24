# Changelog

All notable changes to this project are documented in this file.

## [1.0.12] - 2026-03-24

### Fixed
- Init and healing services now follow the intended default hotfix behavior.
- Keepalive/systemd locking flow was corrected to avoid lock contention during service healing.

## [1.0.1] - 2026-03-20

### Added
- Tag-driven release automation for CLI artifacts (`linux`/`darwin`, `amd64`/`arm64`) with checksums and GitHub Release uploads.
- Tag-driven GHCR publication for `ghcr.io/hrafngud/gungnr-api` and `ghcr.io/hrafngud/gungnr-web`.
- NetBird control plane end-to-end: planner/apply/reapply/status/ACL APIs, CLI commands, and frontend management view.
- Workbench compose-authority baseline with import, preview, apply, restore, resolver, and project-level dependency graph.
- Workbench optional-service catalog baseline with deterministic add/remove support for Redis, Nginx, Prometheus, and MinIO.

### Changed
- Keepalive now prioritizes system-level `systemd` supervision with explicit fallback to user-level `systemd` and cron.
- Project Detail UI was restructured into a denser project-centric workspace with selected-service inspection/editing and improved compose workflow visibility.
- Host runtime telemetry now includes expanded identity and usage signals, including per-project CPU/RAM/Disk indicators.

### Fixed
- Resource-specific not-found behavior now preserves endpoint fallback codes instead of collapsing to generic `CORE-404`.
- Quick-service host-port reporting now keeps requested/effective port data consistent across queue payloads, logs, and audit metadata.

## [1.0.0]
- Initial public release.
