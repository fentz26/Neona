# Neona

[![GitHub release (latest by date)](https://img.shields.io/github/v/release/fentz26/Neona)](https://github.com/fentz26/Neona/releases)

A CLI-centric AI Control Plane that coordinates multiple AI tools (Cursor, AntiGravity, Zencoder, Claude CLI) to execute multi-step tasks under shared rules, knowledge, and policy.

## Quick Start

### Installation

```bash
# Install Neona via one-line script
curl -fsSL https://neona.app/install.sh | bash
```

### Usage

Navigate to your project folder and run:

```bash
neona
```

That's it! Neona's TUI will launch, automatically starting the background daemon if needed.

> **Note:** Neona requires `go` to be installed on your system.

## Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                        neona CLI                            │
│      daemon | task | memory | tui                           │
└─────────────────────────┬───────────────────────────────────┘
                          │ HTTP (127.0.0.1:7466)
┌─────────────────────────▼───────────────────────────────────┐
│                     neonad (daemon)                         │
├─────────────────────────────────────────────────────────────┤
│  Control Plane Service                                      │
│  ├── Task Management (CRUD + claim/release)                 │
│  ├── Lease Manager (TTL + heartbeat)                        │
│  ├── Run Executor (via Connector)                           │
│  ├── Memory Service (add/query)                             │
│  └── PDR Writer (audit trail)                               │
├─────────────────────────────────────────────────────────────┤
│  SQLite Store (~/.neona/neona.db)                           │
│  Tables: tasks, leases, locks, runs, pdr, memory_items      │
└─────────────────────────────────────────────────────────────┘
```

## What Neona Is

Neona is an orchestration system that:

- Coordinates task-based execution across AI agents
- Enforces shared policy and knowledge
- Provides audit trails and evidence collection (PDR)
- Manages task leases with TTL and heartbeat
- Connects IDE AIs as external workers

## What Neona Is NOT

- Not a standalone application
- Not a business logic execution engine
- Not a secret management system
- Not an autonomous task creator

## CLI Commands

### Daemon

```bash
neona daemon [--listen 127.0.0.1:7466] [--db ~/.neona/neona.db]
```

### Tasks

```bash
neona task add --title "Title" --desc "Description"
neona task list [--status pending|claimed|running|completed|failed]
neona task show <task-id>
neona task claim <task-id> [--holder <id>] [--ttl 300]
neona task release <task-id>
neona task run <task-id> --cmd "git status"
neona task log <task-id>
```

### Memory

```bash
neona memory add --content "Note content" [--task <task-id>] [--tags "tag1,tag2"]
neona memory query --q "search term"
```

### TUI

```bash
neona tui
```

**TUI Controls:**

- `j/k` or `↑/↓` - Navigate
- `Enter` - Select/View details
- `Tab` - Cycle status filter
- `Esc` - Go back
- `:` - Command mode
- `q` - Quit

**TUI Commands:** `add`, `claim`, `release`, `run`, `note`, `query`

## API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/tasks` | POST | Create task |
| `/tasks` | GET | List tasks (`?status=` filter) |
| `/tasks/{id}` | GET | Get task |
| `/tasks/{id}/claim` | POST | Claim with lease |
| `/tasks/{id}/release` | POST | Release lease |
| `/tasks/{id}/run` | POST | Execute command |
| `/tasks/{id}/logs` | GET | Get run logs |
| `/tasks/{id}/memory` | GET | Get task memory |
| `/memory` | POST | Add memory item |
| `/memory` | GET | Query memory (`?q=` search) |
| `/health` | GET | Health check |

## Connector Allowlist

The LocalExec connector only permits safe commands:

- `go test ./...`
- `git diff`
- `git status`

## Project Structure

```text
neona/
├── cmd/neona/           # CLI entry points
│   ├── main.go          # Cobra root command
│   ├── daemon.go        # neona daemon
│   ├── task.go          # neona task *
│   ├── memory.go        # neona memory *
│   └── tui_cmd.go       # neona tui
├── internal/
│   ├── models/          # Domain types
│   ├── store/           # SQLite persistence
│   ├── audit/           # PDR writer
│   ├── connectors/      # Connector interface + localexec
│   ├── controlplane/    # HTTP API + service layer
│   └── tui/             # Bubble Tea TUI
└── .ai/
    └── knowledge/       # neona_overview.md
```

## Configuration

Policy, prompts, and knowledge are stored in `.ai/` directory and serve as the single source of truth for all agents.

Default database: `~/.neona/neona.db`

## Development

```bash
# Run tests
go test ./...

# Build binary
go build -o neona ./cmd/neona

# Run daemon with custom DB
./neona daemon --db /tmp/test.db --listen :8080
```

## License

MIT
