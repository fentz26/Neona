# TUI Architecture & AI-Driven Interface

## Overview

Neona provides a **full TUI (Terminal User Interface)** similar to OpenCode, Letta-Code, and Claude CLI. Users interact primarily through an interactive terminal interface where they can:
- Chat with AI to create rules, tasks, and policies
- Route requests to custom AI APIs (like 9router)
- Manage workflows through conversational interface
- View and edit configurations interactively

## Primary Interface: TUI Mode

### Entry Point

```bash
# Default: Start TUI in current directory
neona

# Start TUI in specific directory
neona /path/to/project

# Start TUI with specific AI provider
neona --provider claude-001
```

### TUI Components

**Pattern**: Inspired by OpenCode and Letta-Code TUI implementations.

```
┌─────────────────────────────────────────────────────────────┐
│  Neona TUI                                                   │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Conversation History                               │   │
│  │                                                     │   │
│  │  User: Create a task to review PR #123             │   │
│  │                                                     │   │
│  │  Neona: I'll create a task for reviewing PR #123.   │   │
│  │          Task created: task-001                   │   │
│  │          Status: queued                           │   │
│  │          Dependencies: [none]                       │   │
│  │                                                     │   │
│  │  User: Make a policy that prevents direct main     │   │
│  │        branch writes                               │   │
│  │                                                     │   │
│  │  Neona: Creating policy rule...                   │   │
│  │          Policy created in .ai/policy.yaml        │   │
│  │          Rule: direct_main_write: false            │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                               │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Sidebar: Tasks | Policies | AIs | Workflows      │   │
│  │                                                     │   │
│  │  Tasks (3)                                         │   │
│  │  ├─ task-001: Review PR [queued]                  │   │
│  │  ├─ task-002: Run tests [running]                  │   │
│  │  └─ task-003: Update docs [blocked]               │   │
│  │                                                     │   │
│  │  Policies (2)                                       │   │
│  │  ├─ Global (.ai/policy.yaml)                       │   │
│  │  └─ claude-001 (.ai/policy/claude-001.yaml)        │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                               │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Prompt: [Type message or /command]               │   │
│  │                                                     │   │
│  │  > Create a workflow for code review               │   │
│  │                                                     │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                               │
│  [Ctrl+X] Commands  [Tab] Switch panels  [Esc] Exit         │
└─────────────────────────────────────────────────────────────┘
```

---

## AI-Driven Creation

### Conversational Interface

Users can create rules, tasks, and policies through natural language conversation with AI.

#### Example: Creating Tasks via AI

```
User: Create a task to review PR #123

Neona (AI): I'll create a task for reviewing PR #123.
            Task details:
            - ID: task-001
            - Description: Review PR #123
            - Type: code-review
            - Priority: high
            - Labels: [code-review, urgent]
            
            Should I assign this to a specific AI? [Y/n]
            
User: Yes, assign to claude-001

Neona: Task created and assigned to claude-001
       Status: queued
       You can view it with: /task show task-001
```

#### Example: Creating Policies via AI

```
User: Make a rule that prevents any AI from writing directly to main branch

Neona (AI): I'll create a policy rule to prevent direct main branch writes.
            Policy rule:
            - Scope: Global
            - Rule: direct_main_write: false
            - Applies to: All AIs
            
            This will be added to .ai/policy.yaml
            Should I apply this now? [Y/n]
            
User: Yes

Neona: Policy rule created and applied
       Location: .ai/policy.yaml
       All AIs will now require PR/diff for main branch changes
```

#### Example: Creating Workflows via AI

```
User: Create a workflow for code review that runs tests first, then lint, then review

Neona (AI): I'll create a workflow for code review with the steps you specified.
            Workflow: code-review
            Steps:
            1. Run tests (task-001)
            2. Run linter (task-002) [depends on: task-001]
            3. Code review (task-003) [depends on: task-001, task-002]
            
            Should I save this workflow? [Y/n]
            
User: Yes

Neona: Workflow saved as .ai/workflows/code-review.yaml
       You can execute it with: /workflow run code-review
```

---

## Custom AI API Routing

**Pattern**: Inspired by 9router's provider abstraction and routing system.

### Provider Registration

Users can register custom AI APIs through TUI or CLI:

```bash
# Via TUI command
/connect provider

# Via CLI
neona provider add custom-api \
  --base-url https://api.example.com/v1 \
  --auth-type bearer \
  --api-key $API_KEY \
  --format openai
```

### Provider Configuration

```yaml
# .neona/providers/custom-api.yaml
provider:
  name: custom-api
  base_url: https://api.example.com/v1
  format: openai  # openai, anthropic, custom
  auth:
    type: bearer  # bearer, oauth, api-key
    api_key: ${CUSTOM_API_KEY}
  models:
    - id: gpt-4-custom
      name: Custom GPT-4
      context: 8192
  priority: 1
  fallback: true
```

### Routing Logic

Neona routes requests to appropriate providers based on:
- Model selection
- Provider priority
- Availability (health checks)
- Cost/rate limits
- User preferences

```go
// Provider routing (inspired by 9router)
type ProviderRouter struct {
    providers map[string]Provider
    priorities map[string]int
}

func (r *ProviderRouter) Route(model string, request Request) (*Provider, error) {
    // 1. Find providers that support the model
    candidates := r.findProvidersForModel(model)
    
    // 2. Check availability
    available := r.filterAvailable(candidates)
    
    // 3. Select by priority
    selected := r.selectByPriority(available)
    
    // 4. Apply fallback if needed
    if selected == nil {
        selected = r.selectFallback(candidates)
    }
    
    return selected, nil
}
```

### Format Translation

Neona translates between different API formats (like 9router):

```go
// Format translation (inspired by 9router translator)
type FormatTranslator struct {
    sourceFormat string  // openai, anthropic, custom
    targetFormat string
}

func (t *FormatTranslator) TranslateRequest(req Request) (Request, error) {
    switch t.sourceFormat {
    case "openai":
        return t.translateFromOpenAI(req)
    case "anthropic":
        return t.translateFromAnthropic(req)
    default:
        return t.translateCustom(req)
    }
}
```

---

## TUI Commands

### Slash Commands (Input Field Commands)

Users can type commands directly in the TUI input field using the `/` prefix:

#### Task Management
```
/task              # Show task management menu
/task create       # Create new task (opens dialog)
/task list         # List all tasks
/task show <id>    # Show task details
/task claim <id>   # Claim a task
/task execute <id> # Execute a task
/task cancel <id>  # Cancel a task
/task complete <id> # Mark task as complete
```

#### Planning
```
/plan              # Show planning interface
/plan create       # Create new plan
/plan list         # List all plans
/plan show <id>    # Show plan details
/plan execute <id> # Execute a plan
```

#### Policy Management
```
/policy            # Show policy management
/policy create     # Create new policy rule
/policy list       # List all policies
/policy show       # Show policy hierarchy
/policy validate   # Validate policy configuration
/policy override <ai-id> <rule> <value>  # Override policy for AI
```

#### MCP Integration
```
/mcp               # Show MCP server management
/mcp list          # List connected MCP servers
/mcp connect <url> # Connect to MCP server
/mcp disconnect <id> # Disconnect MCP server
/mcp test <id>     # Test MCP server connection
```

#### Plugin System
```
/plugin            # Show plugin management
/plugin list       # List installed plugins
/plugin install <name>  # Install plugin
/plugin uninstall <name>  # Uninstall plugin
/plugin enable <name>    # Enable plugin
/plugin disable <name>   # Disable plugin
```

#### Agent Management
```
/agent             # Show agent/AI management
/agent list        # List connected agents/AIs
/agent connect     # Connect new AI agent
/agent disconnect <id>  # Disconnect agent
/agent label <id> <tags...>  # Label agent
/agent status <id> # Show agent status
```

#### Rule Management
```
/rule              # Show rule management
/rule create       # Create new rule
/rule list         # List all rules
/rule show <id>    # Show rule details
/rule test <id>    # Test rule
```

#### Listing & Status
```
/list              # List all entities (tasks, agents, policies, etc.)
/status            # Show system status dashboard
                  # - Active tasks
                  # - Connected agents
                  # - System health
                  # - Recent activity
```

#### Configuration
```
/config            # Show configuration management
/config show       # Show current configuration
/config edit       # Edit configuration (opens editor)
/config reset      # Reset configuration to defaults
/config export     # Export configuration
/config import     # Import configuration
```

#### Model & Provider Management
```
/model             # Show model selection
/model list        # List available models
/model select <model>  # Select model for current session
/provider          # Show provider management
/provider add      # Add custom API provider
/provider list     # List all providers
/provider test <id>  # Test provider connection
```

#### Conversation Management
```
/clear             # Clear conversation history
/compact           # Compact/summarize conversation
/context           # Show current context (files, tasks, policies)
/rewind            # Rewind to previous conversation state
```

#### Cost & Usage
```
/cost              # Show cost/usage information
                  # - Token usage
                  # - API costs
                  # - Usage statistics
```

#### System Health
```
/doctor            # Run system diagnostics
                  # - Check connections
                  # - Validate configuration
                  # - Test AI providers
                  # - Check state store
```

#### Help & Documentation
```
/help              # Show help menu
/help <command>    # Show help for specific command
```

#### Authentication
```
/login             # Login to Neona (if using cloud features)
/logout            # Logout from Neona
```

#### Development & Debugging
```
/debug             # Toggle debug mode
/debug logs        # Show debug logs
/debug state       # Show internal state
/audit             # Show audit trail
/audit <filter>    # Show filtered audit entries
```

#### Git & Version Control
```
/commit            # Commit changes (if integrated with git)
/commit <message>  # Commit with message
```

#### Exit
```
/exit              # Exit TUI
/quit              # Exit TUI (alias)
```

### Keybindings (like OpenCode)

```
Ctrl+X C    # Compact session
Ctrl+X D    # Toggle details
Ctrl+X E    # Open external editor
Ctrl+X M    # Switch model/provider
Ctrl+X N    # New conversation
Ctrl+X L    # List sessions
Ctrl+X B    # Toggle sidebar
Ctrl+X T    # Show tasks panel
Ctrl+X P    # Show policies panel
Ctrl+X A    # Show agents panel
Ctrl+X H    # Show help
Tab         # Switch panels / autocomplete
Esc         # Exit/cancel / close dialog
Ctrl+C      # Cancel current operation
Ctrl+L      # Clear screen (keep context)
```

### Command Autocomplete

As users type commands, the TUI provides autocomplete suggestions:
- Type `/` to see all available commands
- Type `/task` to see task-related commands
- Type `/agent` to see agent-related commands
- Type `@` to see available agents/sub-agents
- Type `!` to see available plugins
- Type `!plugin-` to see plugin-specific commands
- Use `Tab` to complete commands and see argument options

#### Autocomplete Examples

```
User types: "@"
Autocomplete shows:
  @claude-001
  @letta-agent
  @sub-agent-1

User types: "!"
Autocomplete shows:
  !github-plugin
  !debug-plugin
  !workflow-plugin

User types: "!github-"
Autocomplete shows:
  !github-plugin

User types: "!github-plugin /"
Autocomplete shows plugin commands:
  !github-plugin /pr
  !github-plugin /repo
  !github-plugin /issue
```

### Tagging System

#### At-Sign Tagging (`@`) - For Agents/Sub-Agents

Users can tag agents or sub-agents using `@` prefix:

```
@agent-name <message>       # Send message to specific agent
@sub-agent-name <command>   # Execute command on sub-agent
@agent-name /task create    # Create task assigned to agent
```

#### Exclamation Mark Tagging (`!`) - For Plugins

Users can tag plugins using `!` prefix:

```
!plugin-name <command>      # Execute plugin command
!plugin-name /config        # Plugin-specific configuration
!plugin-name /command args  # Execute plugin command with args
```

#### Examples

```
!github-plugin /pr list     # Use github-plugin to list PRs
@claude-001 Create a task to review PR #123
!debug-plugin /debug logs   # Use debug-plugin for logs
!github-plugin /pr create   # Create PR using github-plugin
@letta-agent /task claim task-001  # Assign task to agent
```

#### Tag Resolution

```go
// Tag parser (internal/tui/parser/tag.go)
type TagParser struct {
    plugins map[string]Plugin
    agents  map[string]Agent
}

func (p *TagParser) Parse(input string) (*TaggedInput, error) {
    // Check for ! tag (plugins)
    if strings.HasPrefix(input, "!") {
        parts := strings.SplitN(input, " ", 2)
        tag := strings.TrimPrefix(parts[0], "!")
        content := ""
        if len(parts) > 1 {
            content = parts[1]
        }
        
        // Resolve plugin
        if plugin, ok := p.plugins[tag]; ok {
            return &TaggedInput{
                Type: "plugin",
                Target: tag,
                Plugin: plugin,
                Content: content,
            }, nil
        }
        
        return nil, fmt.Errorf("unknown plugin: !%s", tag)
    }
    
    // Check for @ tag (agents/sub-agents)
    if strings.HasPrefix(input, "@") {
        parts := strings.SplitN(input, " ", 2)
        tag := strings.TrimPrefix(parts[0], "@")
        content := ""
        if len(parts) > 1 {
            content = parts[1]
        }
        
        // Resolve agent
        if agent, ok := p.agents[tag]; ok {
            return &TaggedInput{
                Type: "agent",
                Target: tag,
                Agent: agent,
                Content: content,
            }, nil
        }
        
        return nil, fmt.Errorf("unknown agent: @%s", tag)
    }
    
    // No tag, treat as normal input
    return &TaggedInput{Type: "normal", Content: input}, nil
}
```

#### Tagged Command Flow

```
User Input: "!github-plugin /pr list"
    │
    ▼
Tag Parser (extract "!github-plugin")
    │
    ▼
Plugin Registry (resolve "github-plugin")
    │
    ▼
Command Parser (parse "/pr list")
    │
    ▼
Plugin Command Handler
    │
    ▼
Plugin.Execute("/pr", ["list"])
    │
    ▼
Result Displayed in TUI
```

#### Agent Tagged Command Flow

```
User Input: "@claude-001 Create a task to review PR #123"
    │
    ▼
Tag Parser (extract "@claude-001")
    │
    ▼
Agent Registry (resolve "claude-001")
    │
    ▼
Message Parser (parse message content)
    │
    ▼
Agent Command Handler
    │
    ▼
Agent.SendMessage("Create a task to review PR #123")
    │
    ▼
Agent processes and creates task
    │
    ▼
Result Displayed in TUI
```

### Command Router

Commands are parsed and routed to appropriate handlers:

```go
// Command router (internal/tui/handlers/router.go)
type CommandRouter struct {
    handlers map[string]CommandHandler
    tagParser *TagParser
}

func (r *CommandRouter) Route(input string) error {
    // Check for ! or @ tag first
    tagged, err := r.tagParser.Parse(input)
    if err != nil {
        return err
    }
    
    // Handle tagged input
    if tagged.Type == "plugin" {
        return r.handlePluginCommand(tagged)
    }
    
    if tagged.Type == "agent" {
        return r.handleAgentCommand(tagged)
    }
    
    // Parse regular command
    parts := strings.Fields(input)
    if len(parts) == 0 || !strings.HasPrefix(parts[0], "/") {
        // Not a command, treat as regular message
        return r.handleMessage(input)
    }
    
    cmd := strings.TrimPrefix(parts[0], "/")
    args := parts[1:]
    
    // Route to handler
    handler, ok := r.handlers[cmd]
    if !ok {
        return fmt.Errorf("unknown command: %s", cmd)
    }
    
    return handler.Execute(args...)
}

func (r *CommandRouter) handlePluginCommand(tagged *TaggedInput) error {
    // Extract command from content
    parts := strings.Fields(tagged.Content)
    if len(parts) == 0 {
        return fmt.Errorf("no command specified for plugin")
    }
    
    cmd := parts[0]
    args := parts[1:]
    
    // Check if it's a slash command
    if strings.HasPrefix(cmd, "/") {
        cmd = strings.TrimPrefix(cmd, "/")
    }
    
    // Execute plugin command
    return tagged.Plugin.Execute(cmd, args...)
}

func (r *CommandRouter) handleAgentCommand(tagged *TaggedInput) error {
    // If content starts with "/", parse as command
    if strings.HasPrefix(tagged.Content, "/") {
        parts := strings.Fields(tagged.Content)
        cmd := strings.TrimPrefix(parts[0], "/")
        args := parts[1:]
        return tagged.Agent.ExecuteCommand(cmd, args...)
    }
    
    // Otherwise, send as message
    return tagged.Agent.SendMessage(tagged.Content)
}
```

### Command Execution Flow

```
User Input: "/task create"
    │
    ▼
Command Parser (parse "/task" and args)
    │
    ▼
Command Router (route to TaskHandler)
    │
    ▼
TaskHandler.Execute(args)
    │
    ▼
Show Task Creation Dialog (TUI)
    │
    ▼
User Fills Form / AI Assists
    │
    ▼
Task Created → TaskManager
    │
    ▼
Update UI (Sidebar, Status)
```

---

## AI Provider Integration

### Provider Types

1. **Built-in Providers** (OpenAI, Anthropic, etc.)
2. **Custom API Providers** (user-defined endpoints)
3. **Local CLI Tools** (Claude Code, Letta Code, OpenCode)
4. **Proxy Providers** (9router-style routing)

### Provider Interface

```go
type Provider interface {
    Name() string
    Models() []Model
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    Stream(ctx context.Context, req ChatRequest) (<-chan ChatChunk, error)
    HealthCheck(ctx context.Context) error
}
```

### Provider Registration Flow

```
User: /provider add custom-api

TUI: Provider Configuration
     Name: [custom-api]
     Base URL: [https://api.example.com/v1]
     Format: [openai] (openai/anthropic/custom)
     Auth Type: [bearer] (bearer/oauth/api-key)
     API Key: [********]
     
     Testing connection...
     ✓ Connection successful
     ✓ Found 3 models
     
     Save provider? [Y/n]
```

---

## AI-Driven Rule Creation

### Natural Language to Policy

```go
// AI interprets user intent and creates policy
type PolicyCreator struct {
    ai Provider
}

func (c *PolicyCreator) CreateFromPrompt(prompt string) (*Policy, error) {
    // 1. Send prompt to AI with policy schema
    response := c.ai.Chat(ctx, ChatRequest{
        Messages: []Message{
            {Role: "system", Content: policyCreationPrompt},
            {Role: "user", Content: prompt},
        },
    })
    
    // 2. Parse AI response into policy structure
    policy := parsePolicyFromResponse(response)
    
    // 3. Validate policy
    if err := validatePolicy(policy); err != nil {
        return nil, err
    }
    
    // 4. Save policy
    return savePolicy(policy), nil
}
```

### Policy Creation Prompt Template

```
You are a policy creation assistant for Neona.

User requests will be in natural language. Your job is to:
1. Understand the user's intent
2. Convert it to a valid policy rule
3. Return the policy in YAML format

Policy Schema:
- global: Global rules (applies to all AIs)
- per_ai: AI-specific overrides
- per_project: Project-specific overrides

Example:
User: "Prevent direct main branch writes"
You: 
global:
  task_execution:
    direct_main_write: false
```

---

## Implementation Architecture

### TUI Framework (Go)

**Recommended**: Use `github.com/charmbracelet/bubbletea` (inspired by Letta-Code's React TUI)

```go
// TUI application structure
type NeonaTUI struct {
    model *Model
    tea   *tea.Program
}

type Model struct {
    // Conversation state
    messages []Message
    
    // UI state
    activePanel string  // "chat", "tasks", "policies"
    sidebarOpen bool
    
    // AI provider
    provider Provider
    
    // Task/policy managers
    taskManager   *TaskManager
    policyManager *PolicyManager
}
```

### Component Structure

```
internal/
├── tui/
│   ├── app.go           # Main TUI application
│   ├── components/
│   │   ├── chat.go      # Chat interface
│   │   ├── sidebar.go   # Sidebar navigation
│   │   ├── tasklist.go  # Task list view
│   │   ├── policytree.go # Policy tree view
│   │   ├── prompt.go    # Input prompt with autocomplete
│   │   └── status.go    # Status dashboard
│   ├── commands/
│   │   ├── task.go      # Task commands
│   │   ├── plan.go      # Plan commands
│   │   ├── policy.go    # Policy commands
│   │   ├── rule.go      # Rule commands
│   │   ├── agent.go     # Agent commands
│   │   ├── mcp.go       # MCP commands
│   │   ├── plugin.go    # Plugin commands
│   │   ├── config.go    # Config commands
│   │   ├── model.go     # Model/provider commands
│   │   ├── context.go   # Context commands
│   │   ├── cost.go      # Cost commands
│   │   ├── doctor.go    # Doctor/diagnostics
│   │   ├── audit.go     # Audit commands
│   │   ├── debug.go     # Debug commands
│   │   └── workflow.go  # Workflow commands
│   ├── handlers/
│   │   ├── ai.go        # AI interaction handler
│   │   ├── router.go    # Command router
│   │   └── autocomplete.go # Command autocomplete
│   └── parser/
│       ├── command.go   # Command parser
│       └── args.go      # Argument parser
├── provider/
│   ├── registry.go      # Provider registry
│   ├── router.go        # Request routing
│   ├── translator.go    # Format translation
│   └── providers/
│       ├── openai.go
│       ├── anthropic.go
│       ├── custom.go    # Custom API provider
│       └── cli.go       # CLI tool provider
```

---

## User Workflows

### Workflow 1: Creating Task via AI

```
1. User types: "Create a task to fix the auth bug"
2. TUI sends to AI provider (routed via provider router)
3. AI interprets intent and creates task structure
4. TUI displays task preview
5. User confirms
6. Task saved to state store
7. Task appears in sidebar
```

### Workflow 2: Adding Custom AI Provider

```
1. User types: /provider add
2. TUI shows provider configuration dialog
3. User enters:
   - Name: my-custom-api
   - Base URL: https://api.example.com/v1
   - Format: openai
   - API Key: [entered securely]
4. TUI tests connection
5. Provider registered
6. Available in model selection
```

### Workflow 3: Creating Policy via AI

```
1. User types: "Make a rule that requires approval for main branch changes"
2. TUI sends to AI with policy creation context
3. AI generates policy YAML
4. TUI shows policy preview
5. User reviews and confirms
6. Policy saved to .ai/policy.yaml
7. Policy appears in policy tree
```

### Workflow 4: Using Plugin Commands

```
1. User types: "!github-plugin /pr list"
2. Tag parser extracts "!github-plugin"
3. Plugin registry resolves "github-plugin"
4. Command parser extracts "/pr list"
5. Plugin's PR handler executes
6. Results displayed in TUI
```

### Workflow 5: Tagging Agent with Task

```
1. User types: "@claude-001 Create a task to review PR #123"
2. Tag parser extracts "@claude-001"
3. Agent registry resolves "claude-001"
4. Message sent to agent with task creation intent
5. Agent creates task and assigns to itself
6. Task appears in sidebar with agent label
```

### Workflow 6: Plugin Command Execution

```
1. User types: "!debug-plugin /logs show"
2. Tag parser identifies "!debug-plugin"
3. Plugin command handler routes to plugin
4. Plugin executes "/logs show" command
5. Debug logs displayed in TUI
```

---

## Integration with Existing Systems

### Cobra CLI Framework

```go
// cmd/neona/main.go
package main

import (
    "github.com/spf13/cobra"
    "github.com/charmbracelet/bubbletea"
)

var rootCmd = &cobra.Command{
    Use:   "neona",
    Short: "Neona AI Control Plane",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Start TUI if no subcommand
        if len(args) == 0 {
            return startTUI()
        }
        return nil
    },
}

func startTUI() error {
    model := NewModel()
    program := tea.NewProgram(model)
    return program.Start()
}
```

### Command Structure

```go
// Commands available in both TUI and CLI
var (
    taskCmd = &cobra.Command{
        Use: "task",
        // ...
    }
    
    policyCmd = &cobra.Command{
        Use: "policy",
        // ...
    }
    
    providerCmd = &cobra.Command{
        Use: "provider",
        // ...
    }
)
```

---

## Benefits

1. **Conversational Interface**: Natural language interaction
2. **AI-Powered**: AI helps create rules, tasks, policies
3. **Flexible Routing**: Custom AI APIs supported
4. **Unified Experience**: Single TUI for all operations
5. **CLI Compatible**: Commands work in both TUI and CLI
6. **Extensible**: Easy to add new providers and commands

---

## Implementation Priority

### Phase 1 (MVP)
- ✅ Basic TUI scaffold (bubbletea)
- ✅ Chat interface
- ✅ AI provider routing (one provider)
- ✅ Basic commands (/task, /policy)

### Phase 2 (Enhanced)
- ✅ Multiple AI providers
- ✅ Custom API provider support
- ✅ AI-driven task/policy creation
- ✅ Sidebar navigation

### Phase 3 (Advanced)
- ✅ Format translation (9router-style)
- ✅ Provider fallback
- ✅ Advanced routing logic
- ✅ Workflow creation via AI
