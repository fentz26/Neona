# Neona Vertical Slice Overview

This document describes how to run the Neona vertical slice implementation.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        neona CLI                             │
│  (daemon | task add/list/show/claim/release/run | tui)      │
└─────────────────────────┬───────────────────────────────────┘
                          │ HTTP
┌─────────────────────────▼───────────────────────────────────┐
│                     neonad (daemon)                          │
│                   127.0.0.1:7466                             │
├──────────────────────────────────────────────────────────────┤
│  Control Plane Service                                       │
│  ├── Task Management (CRUD + claim/release)                 │
│  ├── Lease Manager (TTL + heartbeat)                        │
│  ├── Run Executor (via Connector)                           │
│  ├── Memory Service (add/query)                             │
│  └── PDR Writer (audit trail)                               │
├──────────────────────────────────────────────────────────────┤
│  Connectors                                                  │
│  └── LocalExec (allowlist: go test, git diff, git status)  │
├──────────────────────────────────────────────────────────────┤
│  SQLite Store (~/.neona/neona.db)                           │
│  Tables: tasks, leases, locks, runs, pdr, memory_items      │
└──────────────────────────────────────────────────────────────┘
```

## Quick Start

### 1. Install Dependencies

```bash
cd /path/to/neona
go mod tidy
```

### 2. Start the Daemon

```bash
# Terminal 1
go run ./cmd/neona daemon
# Output: Starting Neona daemon on 127.0.0.1:7466
```

### 3. Create and Manage Tasks (CLI)

```bash
# Terminal 2

# Create a task
go run ./cmd/neona task add --title "Run tests" --desc "Execute go test"

# List tasks
go run ./cmd/neona task list

# Show task details
go run ./cmd/neona task show <task-id>

# Claim a task
go run ./cmd/neona task claim <task-id>

# Run a command (must be claimed first)
go run ./cmd/neona task run <task-id> --cmd "git status"

# View run logs
go run ./cmd/neona task log <task-id>

# Release the task
go run ./cmd/neona task release <task-id>
```

### 4. Use the TUI

```bash
go run ./cmd/neona tui
```

**TUI Controls:**

- `j/k` or `↑/↓` - Navigate task list
- `Enter` - View task details
- `Tab` - Cycle status filter
- `Esc` - Go back
- `:` - Open command bar
- `q` - Quit

**TUI Commands:**

- `add <title>` - Create a task
- `claim` - Claim selected task
- `release` - Release selected task
- `run <cmd>` - Run command (e.g., `run git status`)
- `note <content>` - Add memory note
- `query <term>` - Search memory

### 5. Memory Operations

```bash
# Add a memory item
go run ./cmd/neona memory add --content "Important note" --task <task-id>

# Query memory
go run ./cmd/neona memory query --q "important"
```

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

## Allowed Commands (Connector)

The LocalExec connector only allows:

- `go test ./...`
- `git diff`
- `git status`

All other commands are rejected for safety.

## Database Location

Default: `~/.neona/neona.db`

Override with `--db` flag:

```bash
go run ./cmd/neona daemon --db /custom/path/neona.db
```

## Running Tests

```bash
go test ./...
```
