# Neona Repository Charter

## Scope

### What Belongs

- CLI binary (`neona`)
- Daemon (`neonad`) with HTTP + MCP transport
- SQLite store (tasks, leases, locks, runs, PDR, memory)
- Scheduler + Worker pool (local execution)
- Connectors (LocalExec, Claude CLI, future: HTTP)
- Policy enforcement (.ai/policy.yaml)
- TUI (terminal interface)

### What Does NOT Belong

- Web UI (→ Neona-X)
- Cloud/SaaS infrastructure (→ Neona-Cloud)
- Marketing/documentation site (→ Neona-Website)
- React/Next.js/frontend frameworks

## Deployment Model

| Aspect | Value |
|--------|-------|
| Distribution | Single static binary (Go) |
| Platforms | linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 |
| Install | `curl -fsSL https://neona.app/install.sh \| bash` |
| Data | `~/.neona/neona.db` (SQLite) |
| Listen | `127.0.0.1:7466` (configurable) |

## Security Model

- **Local-first:** No external network calls by default
- **Secrets:** NEVER stored in Neona DB; use env vars or external vault
- **Shell execution:** Allowlist-only via connector config
- **Workspace isolation:** git worktree per task/run
- **Policy enforcement:** Fail-closed on policy violations
- **Audit:** All decisions logged to PDR

## Release Strategy

- **Versioning:** SemVer (major.minor.patch)
- **Tags:** `v0.x.y` for pre-1.0, `vX.Y.Z` after
- **CI:** GitHub Actions (test, build, release)
- **Changelog:** CHANGELOG.md with Keep a Changelog format
- **Breaking changes:** Major version bump, migration guide

## Dependency Rules

- Go stdlib preferred
- Minimal external deps: sqlite driver, cobra, bubbletea
- NO CGO (pure Go for cross-compilation)
- NO UI framework dependencies
