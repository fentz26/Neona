# Neona

[![GitHub release (latest by date)](https://img.shields.io/github/v/release/Neona-AI/Neona)](https://github.com/Neona-AI/Neona/releases)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Go Version](https://img.shields.io/badge/go-1.21+-blue.svg)](https://golang.org/dl/)

**AI-Native Task Orchestration for Developers**

Neona is a lightweight control plane that coordinates AI agents and tools to execute complex, multi-step development tasks. It provides task management, shared policy enforcement, audit trails, and a beautiful terminal interface—all while keeping you in control of what gets executed.

Think of it as a **command center** for your AI coding assistants, ensuring they work together under consistent rules while maintaining full transparency and security.

Created by **[Fentzzz](https://github.com/fentz26)**

---

## ✨ Key Features

- **Task Orchestration** - Create, claim, and execute development tasks with lease-based concurrency control
- **AI Agent Coordination** - Connect multiple AI tools (Cursor, Claude CLI, etc.) under unified policies
- **Policy Enforcement** - Define shared rules, constraints, and safety checks in `.ai/policy.yaml`
- **Audit Trails** - Comprehensive PDR (Process Data Record) logging for compliance and debugging
- **Memory System** - Contextual knowledge base that persists across tasks and sessions
- **Beautiful TUI** - Rich terminal interface built with Textual (Python) for interactive task management
- **Secure by Default** - Command allowlisting, no secret access, minimal privilege execution
- **Auto-Updates** - Automatic version checking and self-updating capabilities
- **SQLite Backend** - Lightweight, serverless persistence with no external dependencies
- **HTTP API** - RESTful API for programmatic access and custom integrations

## 🚀 Quick Start

### Installation

```bash
# Install Neona via one-line script
curl -fsSL https://cli.neona.app/install.sh | bash
```

### Usage

Navigate to your project folder and run:

```bash
neona
```

That's it! Neona's TUI will launch, automatically starting the background daemon if needed.

## 🏗️ Architecture

Neona follows a client-daemon architecture with three main components:

```text
┌──────────────────────────────────────────────────────────────┐
│                     User Interface Layer                      │
├──────────────────────────────────────────────────────────────┤
│  neona CLI (Go)                │  neona-tui (Python/Textual) │
│  • task add/list/show          │  • Rich visual interface    │
│  • claim/release/run           │  • Interactive commands     │
│  • memory add/query            │  • Real-time updates        │
│  • daemon start/stop           │  • Status bar with health   │
└─────────────────┬──────────────┴────────────────┬────────────┘
                  │                               │
                  └──────────┬ HTTP API ──────────┘
                             │ 127.0.0.1:7466
              ┌──────────────▼──────────────┐
              │      Neona Daemon (Go)      │
              ├─────────────────────────────┤
              │  Control Plane Service      │
              │  ├── Task Management        │
              │  │   (CRUD + claim/release) │
              │  ├── Lease Manager          │
              │  │   (TTL + heartbeat)      │
              │  ├── Run Executor           │
              │  │   (via Connectors)       │
              │  ├── Memory Service         │
              │  │   (add/query/search)     │
              │  └── PDR Writer             │
              │      (audit trail)          │
              ├─────────────────────────────┤
              │  Connectors                 │
              │  └── LocalExec (allowlist)  │
              ├─────────────────────────────┤
              │  SQLite Store               │
              │  ~/.neona/neona.db          │
              │  • tasks, leases, locks     │
              │  • runs, pdr, memory_items  │
              └─────────────────────────────┘
```

### Component Responsibilities

**CLI (`neona`)** - Command-line interface for all operations, written in Go with Cobra framework

**TUI (`neona-tui`)** - Beautiful terminal UI built with Python Textual, provides rich visual experience

**Daemon (`neonad`)** - Background service that manages state, enforces policies, and executes tasks

**Connectors** - Pluggable execution backends (currently LocalExec with command allowlisting)

**Store** - SQLite database for persistent state, no external dependencies required

## 🎯 Use Cases

**Multi-Agent Development** - Coordinate multiple AI coding assistants working on the same codebase without conflicts

**Policy-Driven Automation** - Ensure all AI agents follow your organization's coding standards, security policies, and best practices

**Audit & Compliance** - Track every command executed by AI agents with full audit trails (PDR)

**Task Management** - Break down complex features into manageable tasks that can be claimed and executed systematically

**Knowledge Sharing** - Build a persistent memory system that AI agents can query for project context and decisions

## ✅ What Neona Is

- **Task Orchestrator** - Manages concurrent task execution with lease-based locking
- **Policy Enforcer** - Ensures all agents follow defined rules from `.ai/policy.yaml`
- **Audit System** - Tracks all operations in PDR (Process Data Record) format
- **Memory Database** - Maintains contextual knowledge across tasks and sessions
- **Integration Hub** - Connects multiple AI tools under a unified control plane

## ❌ What Neona Is NOT

- **Not a Code Editor** - Works with existing editors/IDEs, doesn't replace them
- **Not an Autonomous Agent** - Human oversight required; agents execute defined tasks only
- **Not a Secret Manager** - Policy explicitly disables access to credentials and sensitive data
- **Not a CI/CD System** - Complements your existing pipeline, doesn't replace it
- **Not a Task Creator** - You define tasks; Neona orchestrates their execution

## 💻 CLI Commands

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

### TUI (Terminal User Interface)

```bash
neona tui
# Or launch directly if neona-tui is installed
neona-tui
```

The TUI provides a rich, interactive experience built with [Textual](https://textual.textualize.io/) (Python):

**Features:**
- Real-time task list with status filtering
- Detailed task view with run logs and memory
- Status bar showing daemon health, version, and statistics
- Command bar with contextual help
- Beautiful color scheme and responsive layout

**Keyboard Controls:**

- `↑/↓` or `j/k` - Navigate tasks
- `Enter` - View task details
- `Tab` - Cycle status filters (All/Pending/Claimed/Running/Completed/Failed)
- `Esc` - Go back to main view
- `:` - Open command mode
- `r` - Refresh task list
- `q` or `Ctrl+C` - Quit

**Available Commands:**

| Command | Description | Example |
|---------|-------------|---------|
| `add <title>` | Create new task | `:add Fix authentication bug` |
| `claim` | Claim selected task | `:claim` |
| `release` | Release claimed task | `:release` |
| `run <cmd>` | Execute command on task | `:run git status` |
| `note <text>` | Add memory note | `:note Updated API endpoint` |
| `query <term>` | Search memory | `:query authentication` |
| `refresh` | Reload task list | `:refresh` |

## 🔌 HTTP API Reference

The daemon exposes a RESTful API on `127.0.0.1:7466` by default.

### Task Endpoints

| Endpoint | Method | Description | Parameters |
|----------|--------|-------------|------------|
| `/tasks` | POST | Create a new task | `title`, `description` |
| `/tasks` | GET | List all tasks | `?status=pending\|claimed\|running\|completed\|failed` |
| `/tasks/{id}` | GET | Get task details | - |
| `/tasks/{id}/claim` | POST | Claim task with lease | `holder_id`, `ttl_sec` (default: 300) |
| `/tasks/{id}/release` | POST | Release task lease | `holder_id` |
| `/tasks/{id}/run` | POST | Execute command on task | `holder_id`, `command`, `args[]` |
| `/tasks/{id}/logs` | GET | Get execution logs | - |
| `/tasks/{id}/memory` | GET | Get task-specific memory | - |

### Memory Endpoints

| Endpoint | Method | Description | Parameters |
|----------|--------|-------------|------------|
| `/memory` | POST | Add memory item | `content`, `task_id` (optional), `tags[]` (optional) |
| `/memory` | GET | Query memory items | `?q=search term` |

### System Endpoints

| Endpoint | Method | Description | Response |
|----------|--------|-------------|----------|
| `/health` | GET | Daemon health check | Version, database status, uptime |
| `/workers` | GET | Worker pool statistics | Active workers, queue depth |

### Authentication

Currently local-only (127.0.0.1). Future versions will support API tokens for remote access.

## 🛡️ Security & Safety

Neona is designed with security as a first-class concern:

### Command Allowlisting

The `LocalExec` connector uses a strict allowlist of permitted commands:

**Allowed Commands:**
- `go test ./...` - Run Go tests
- `git diff` - View changes
- `git status` - Check repository status

All other commands are **rejected by default**. This prevents accidental or malicious code execution.

### Policy Enforcement

The `.ai/policy.yaml` file defines system-wide constraints:

```yaml
task_execution:
  claim_required: true          # Must claim before executing
  direct_main_write: false      # No direct commits to main
  autonomous_task_creation: false  # No self-generated tasks

safety:
  secrets_access: false         # No credential access
  speculative_changes: false    # Stay within task scope
  minimal_diff: true            # Keep changes focused

completion:
  evidence_required: true       # Proof of completion needed
  test_evidence: true           # Test results required
  diff_required: true           # Show what changed
```

### Audit Trail (PDR)

Every operation is logged in Process Data Record (PDR) format:
- Who claimed/executed the task
- What commands were run
- When each action occurred
- Results and exit codes

PDR logs are stored in SQLite and can be exported for compliance audits.

## 📂 Project Structure

```text
neona/
├── cmd/neona/              # Go CLI application
│   ├── main.go             # Entry point with Cobra root
│   ├── daemon.go           # Daemon start/stop commands
│   ├── task.go             # Task CRUD operations
│   ├── memory.go           # Memory management
│   ├── tui_cmd.go          # TUI launcher (calls Python)
│   ├── update_cmd.go       # Auto-update functionality
│   └── api_client.go       # HTTP client for daemon API
│
├── internal/               # Go internal packages
│   ├── models/             # Domain types (Task, Lease, Run, etc.)
│   ├── store/              # SQLite database layer
│   ├── audit/              # PDR (Process Data Record) writer
│   ├── connectors/         # Execution backends
│   │   └── localexec/      # LocalExec with allowlisting
│   ├── controlplane/       # HTTP server + business logic
│   ├── scheduler/          # Task scheduling & workers
│   ├── mcp/                # MCP (Model Context Protocol) support
│   └── update/             # Self-update system
│
├── neona-tui/              # Python TUI (Textual-based)
│   ├── neona_tui/
│   │   ├── app.py          # Main Textual application
│   │   └── api_client.py   # HTTP client for Go daemon
│   ├── pyproject.toml      # Python dependencies
│   └── README.md           # TUI-specific docs
│
├── .ai/                    # AI agent configuration
│   ├── policy.yaml         # Global policy constraints
│   ├── knowledge/          # Knowledge base (Markdown)
│   └── prompts/            # System and role prompts
│
├── scripts/
│   └── install.sh          # One-line installation script
│
└── docs/
    └── REPO_CHARTER.md     # Repository charter & governance
```

## ⚙️ Configuration

### Directory Structure

Neona uses the `.ai/` directory as the **single source of truth** for all AI agents:

```text
.ai/
├── policy.yaml              # Enforcement rules and constraints
├── knowledge/               # Project-specific knowledge base
│   ├── neona_overview.md    # System architecture docs
│   └── repo_map.md          # Codebase navigation
└── prompts/                 # AI agent instructions
    ├── system.md            # Base system prompt
    └── roles/               # Role-specific prompts
        ├── planner.md       # Task planning agent
        ├── worker.md        # Execution agent
        └── reviewer.md      # Code review agent
```

### Database Location

**Default:** `~/.neona/neona.db`

Override with environment variable or CLI flag:

```bash
# Environment variable
export NEONA_DB_PATH=/custom/path/neona.db

# CLI flag
neona daemon --db /custom/path/neona.db
```

### Daemon Configuration

**Default listen address:** `127.0.0.1:7466`

Override with:

```bash
neona daemon --listen 127.0.0.1:8080
# or
export NEONA_LISTEN=127.0.0.1:8080
```

### TUI Configuration

The Go CLI discovers the Python TUI using:
1. `NEONA_TUI_PATH` environment variable
2. `neona-tui` in system PATH
3. Development fallback (when run from repo root): `neona-tui/neona_tui/app.py`

## 🛠️ Development

### Prerequisites

- **Go 1.21+** - For the CLI and daemon
- **Python 3.9+** - For the TUI (optional)
- **Git** - For version control

### Building from Source

```bash
# Clone the repository
git clone https://github.com/Neona-AI/Neona.git
cd Neona

# Install Go dependencies
go mod download

# Build the CLI
go build -o neona ./cmd/neona

# Run the daemon
./neona daemon

# In another terminal, use the CLI
./neona task add --title "Test task" --desc "Testing"
./neona task list
```

### Running Tests

```bash
# Run all Go tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/store/
go test ./internal/connectors/localexec/
```

### Python TUI Development

```bash
cd neona-tui

# Install in development mode
pip install -e ".[dev]"

# Run with live reload
textual run --dev neona_tui/app.py

# Ensure daemon is running first
# In another terminal: go run ./cmd/neona daemon
```

### Development Tips

**Hot Reload for Go**
```bash
# Install air for auto-recompilation
go install github.com/cosmtrek/air@latest

# Run with hot reload
air
```

**Testing with Custom DB**
```bash
# Use temporary database for testing
./neona daemon --db /tmp/neona-test.db --listen :8080
```

**Debugging API Calls**
```bash
# Check daemon health
curl http://localhost:7466/health

# List tasks
curl http://localhost:7466/tasks

# Create a task
curl -X POST http://localhost:7466/tasks \
  -H "Content-Type: application/json" \
  -d '{"title":"Debug task","description":"Testing API"}'
```

### Project Standards

- Follow [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- Use `gofmt` for formatting (enforced by CI)
- Write tests for new features and bug fixes
- Update documentation for user-facing changes
- Keep commits atomic and well-described

## 🤝 Contributing

Contributions are welcome! Neona is in active development and we'd love your help.

**Ways to Contribute:**
- 🐛 Report bugs and issues
- 💡 Suggest new features or improvements
- 📝 Improve documentation
- 🔧 Submit pull requests
- ⭐ Star the repository to show support

**Before Contributing:**
1. Check existing [issues](https://github.com/Neona-AI/Neona/issues) and [PRs](https://github.com/Neona-AI/Neona/pulls)
2. Read the [Repository Charter](docs/REPO_CHARTER.md) for project governance
3. Follow the coding standards outlined in the Development section
4. Write tests for new features
5. Ensure all tests pass before submitting

**Pull Request Process:**
1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to your branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request with a clear description

## 🗺️ Roadmap

**Current Version: 0.0.1** (Early Development)

### Upcoming Features

- [ ] **MCP (Model Context Protocol)** integration for Claude Desktop and other MCP clients
- [ ] **Remote execution** support for distributed task processing
- [ ] **Advanced connectors** (Docker, Kubernetes, SSH)
- [ ] **Web UI** for browser-based task management
- [ ] **Task templates** for common development workflows
- [ ] **Webhook notifications** for task status changes
- [ ] **Multi-tenant support** for team deployments
- [ ] **Enhanced security** with API tokens and RBAC
- [ ] **Plugin system** for custom connectors and policies
- [ ] **Cloud sync** for distributed teams

See the [Issues](https://github.com/Neona-AI/Neona/issues) for detailed feature requests and discussions.

## 📜 License

**Copyright (c) 2026 [Fentzzz](https://github.com/fentz26)**

Licensed under the **GNU Affero General Public License v3 (AGPL-3.0)**.

This means you can:
- ✅ Use Neona for personal and commercial projects
- ✅ Modify and distribute the source code
- ✅ Use it in SaaS/hosted services

But you must:
- 📝 Disclose your source code modifications
- 📄 Include the original license and copyright
- 🔓 Share your modifications under AGPL-3.0

See the [LICENSE](LICENSE) file for full details.

### Enterprise & Cloud

This is the **open source core** of Neona. For enterprise features like:
- Multi-tenant cloud orchestration
- Managed teams and workspaces
- SSO/SAML authentication
- Premium support and SLAs
- Custom connectors and integrations

Check out **[Neona Cloud](https://github.com/Neona-AI/Neona-Cloud)** (coming soon).

## 📬 Support & Community

- **Issues:** [GitHub Issues](https://github.com/Neona-AI/Neona/issues)
- **Discussions:** [GitHub Discussions](https://github.com/Neona-AI/Neona/discussions)
- **Documentation:** [Wiki](https://github.com/Neona-AI/Neona/wiki) (coming soon)
- **Email:** support@neona.app (coming soon)

---

<div align="center">

**Built with ❤️ by [Fentzzz](https://github.com/fentz26)**

If you find Neona useful, please consider starring ⭐ the repository!

[⬆ Back to Top](#neona)

</div>
