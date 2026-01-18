// Package tui provides the interactive terminal UI for Neona.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fentz26/neona/internal/agents"
	"github.com/fentz26/neona/internal/auth"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED")
	secondaryColor = lipgloss.Color("#6366F1")
	successColor   = lipgloss.Color("#10B981")
	warningColor   = lipgloss.Color("#F59E0B")
	errorColor     = lipgloss.Color("#EF4444")
	mutedColor     = lipgloss.Color("#6B7280")
	bgColor        = lipgloss.Color("#1F2937")
	fgColor        = lipgloss.Color("#F9FAFB")
	cyanColor      = lipgloss.Color("#06B6D4")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#374151")).
			Foreground(fgColor).
			Padding(0, 1)

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)

	taskItemStyle = lipgloss.NewStyle().
			Padding(0, 2)

	selectedStyle = lipgloss.NewStyle().
			Background(primaryColor).
			Foreground(fgColor).
			Bold(true).
			Padding(0, 2)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor).
			Padding(0, 1)

	agentOnlineStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	agentOfflineStyle = lipgloss.NewStyle().
				Foreground(errorColor)
)

// App is the main TUI application model.
type App struct {
	client       *Client
	tasks        []TaskItem
	selectedIdx  int
	input        textinput.Model
	viewport     viewport.Model
	width        int
	height       int
	mode         string // "list", "detail", "agents", "workers"
	currentTask  *TaskDetail
	runs         []RunDetail
	memory       []MemoryDetail
	message      string
	filter       string
	filterIdx    int
	loading      bool
	agents       []agents.Agent
	agentIdx     int
	daemonOnline bool
	suggestions  *Suggestions
	workersStats *WorkersStats
	authManager  *auth.Manager
	currentUser  *auth.User
}

var filters = []string{"", "pending", "claimed", "running", "completed", "failed"}
var filterNames = []string{"ALL", "PENDING", "CLAIMED", "RUNNING", "DONE", "FAILED"}

// New creates a new TUI application.
func New(apiAddr string) *App {
	ti := textinput.New()
	ti.Placeholder = "Type: add <title> | claim | run <cmd> | release | scan | login"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80

	vp := viewport.New(80, 20)

	// Detect agents on startup
	detector := agents.NewDetector()
	detectedAgents := detector.Scan()

	// Initialize auth manager
	authMgr, _ := auth.NewManager()
	var currentUser *auth.User
	if authMgr != nil && authMgr.IsAuthenticated() {
		currentUser = authMgr.GetUser()
	}

	return &App{
		client:      NewClient(apiAddr),
		input:       ti,
		viewport:    vp,
		mode:        "list",
		agents:      detectedAgents,
		suggestions: NewSuggestions(),
		authManager: authMgr,
		currentUser: currentUser,
	}
}

// Run starts the TUI application.
func (a *App) Run() error {
	p := tea.NewProgram(a, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Init implements tea.Model
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		a.fetchTasks(),
		a.checkDaemon(),
	)
}

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit

		case "esc":
			if a.mode == "detail" || a.mode == "agents" || a.mode == "workers" {
				a.mode = "list"
				a.currentTask = nil
				return a, a.fetchTasks()
			}

		case "up", "k":
			if a.suggestions.IsVisible() {
				a.suggestions.Prev()
			} else if a.mode == "list" && a.selectedIdx > 0 {
				a.selectedIdx--
			} else if a.mode == "agents" && a.agentIdx > 0 {
				a.agentIdx--
			}

		case "down", "j":
			if a.suggestions.IsVisible() {
				a.suggestions.Next()
			} else if a.mode == "list" && a.selectedIdx < len(a.tasks)-1 {
				a.selectedIdx++
			} else if a.mode == "agents" && a.agentIdx < len(a.agents)-1 {
				a.agentIdx++
			}

		case "tab":
			// If suggestions visible, accept selection
			if a.suggestions.IsVisible() {
				if selected := a.suggestions.Selected(); selected != nil {
					a.input.SetValue(selected.Text + " ")
					a.suggestions.Update("")
				}
				return a, nil
			}
			// Cycle through modes: list -> agents -> list
			if a.mode == "list" {
				a.mode = "agents"
			} else {
				a.mode = "list"
				a.filterIdx = (a.filterIdx + 1) % len(filters)
				a.filter = filters[a.filterIdx]
				return a, a.fetchTasks()
			}

		case "enter":
			// If suggestions visible, accept selection
			if a.suggestions.IsVisible() {
				if selected := a.suggestions.Selected(); selected != nil {
					a.input.SetValue(selected.Text + " ")
					a.suggestions.Update("")
				}
				return a, nil
			}
			cmd := strings.TrimSpace(a.input.Value())
			if cmd != "" {
				a.input.SetValue("")
				return a, a.executeCommand(cmd)
			} else if a.mode == "list" && len(a.tasks) > 0 {
				task := a.tasks[a.selectedIdx]
				a.mode = "detail"
				return a, a.fetchTaskDetail(task.ID)
			}

		case "r":
			if a.mode == "list" {
				return a, a.fetchTasks()
			} else if a.mode == "agents" {
				return a, a.scanAgents()
			}

		case "a":
			// Quick switch to agents view
			a.mode = "agents"

		case "w":
			// Quick switch to workers view
			a.mode = "workers"
			return a, tea.Batch(a.fetchWorkers(), a.tickCmd())
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.input.Width = msg.Width - 4
		a.viewport.Width = msg.Width
		a.viewport.Height = msg.Height - 10

	case tasksLoadedMsg:
		a.loading = false
		a.tasks = msg.tasks
		if a.selectedIdx >= len(a.tasks) {
			a.selectedIdx = max(0, len(a.tasks)-1)
		}

	case taskDetailLoadedMsg:
		a.currentTask = msg.task
		a.runs = msg.runs
		a.memory = msg.memory

	case agentsScanMsg:
		a.agents = msg.agents
		a.message = fmt.Sprintf("✓ Found %d agents", len(a.agents))

	case daemonStatusMsg:
		a.daemonOnline = msg.online

	case workersFetchedMsg:
		a.workersStats = msg.stats
		if a.mode == "workers" {
			// Schedule the next tick only after the current fetch is complete.
			cmds = append(cmds, a.tickCmd())
		}

	case tickMsg:
		if a.mode == "workers" {
			return a, a.fetchWorkers()
		}

	case commandResultMsg:
		a.message = msg.message
		return a, a.fetchTasks()

	case errMsg:
		a.message = "Error: " + msg.err.Error()
	}

	// Update input
	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	cmds = append(cmds, cmd)

	// Update suggestions based on input
	a.suggestions.Update(a.input.Value())

	// Populate dynamic suggestions for @
	if strings.HasPrefix(a.input.Value(), "@") {
		var agentNames []string
		for _, ag := range a.agents {
			agentNames = append(agentNames, ag.Name)
		}
		a.suggestions.SetAgents(agentNames)

		var taskIDs []string
		for _, t := range a.tasks {
			taskIDs = append(taskIDs, t.ID)
		}
		a.suggestions.SetTasks(taskIDs)
	}

	return a, tea.Batch(cmds...)
}

// View implements tea.Model
func (a *App) View() string {
	var b strings.Builder

	// Header with daemon status
	daemonStatus := agentOnlineStyle.Render("● DAEMON")
	if !a.daemonOnline {
		daemonStatus = agentOfflineStyle.Render("○ DAEMON")
	}

	// User status
	userStatus := lipgloss.NewStyle().Foreground(mutedColor).Render("○ not signed in")
	if a.currentUser != nil {
		userStatus = lipgloss.NewStyle().Foreground(successColor).Render(fmt.Sprintf("● %s", a.currentUser.Username))
	}

	header := titleStyle.Render("🚀 NEONA Control Plane")
	header += "  " + daemonStatus
	header += "  " + lipgloss.NewStyle().Foreground(cyanColor).Render(fmt.Sprintf("[%d agents]", len(a.agents)))
	header += "  " + userStatus

	b.WriteString(header + "\n")
	b.WriteString(strings.Repeat("─", a.width) + "\n")

	// Main content area
	contentHeight := a.height - 8
	if contentHeight < 5 {
		contentHeight = 5
	}

	switch a.mode {
	case "list":
		filterLabel := fmt.Sprintf(" Filter: [%s]", filterNames[a.filterIdx])
		b.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Render(filterLabel) + "\n")
		b.WriteString(a.renderTaskList(contentHeight - 1))
	case "detail":
		b.WriteString(a.renderTaskDetail(contentHeight))
	case "agents":
		b.WriteString(a.renderAgentsPanel(contentHeight))
	case "workers":
		b.WriteString(a.renderWorkersPanel(contentHeight))
	}

	// Message bar
	if a.message != "" {
		msgStyle := lipgloss.NewStyle().Foreground(successColor)
		if strings.HasPrefix(a.message, "Error") {
			msgStyle = lipgloss.NewStyle().Foreground(errorColor)
		}
		b.WriteString("\n" + msgStyle.Render(a.message))
	} else {
		b.WriteString("\n")
	}

	// Input box
	b.WriteString("\n")
	b.WriteString(inputBoxStyle.Render(a.input.View()))

	// Suggestions dropdown (if visible) - renders BELOW input
	if a.suggestions.IsVisible() {
		b.WriteString("\n")
		b.WriteString(a.suggestions.Render(a.width))
	}
	b.WriteString("\n")

	// Status bar
	var status string
	switch a.mode {
	case "list":
		status = fmt.Sprintf(" Tasks: %d | ↑↓:nav | Tab:agents | a:agents | w:workers | r:refresh | Ctrl+C:quit", len(a.tasks))
	case "agents":
		status = fmt.Sprintf(" Agents: %d | ↑↓:nav | r:rescan | Esc:back | scan:detect", len(a.agents))
	case "workers":
		workerCount := 0
		if a.workersStats != nil {
			workerCount = a.workersStats.ActiveWorkers
		}
		status = fmt.Sprintf(" Workers: %d | Esc:back | w:refresh", workerCount)
	default:
		status = " Esc:back | Enter:command | Ctrl+C:quit"
	}
	b.WriteString(statusBarStyle.Width(a.width).Render(status))

	return b.String()
}

func (a *App) renderTaskList(height int) string {
	if a.loading {
		return "\n  Loading tasks...\n"
	}
	if len(a.tasks) == 0 {
		return "\n  No tasks found. Type: add <title> to create one.\n"
	}

	var lines []string
	for i, task := range a.tasks {
		status := a.formatStatus(task.Status)

		if i == a.selectedIdx {
			line := selectedStyle.Render(fmt.Sprintf("▶ %s  %s", a.formatStatusPlain(task.Status), task.TaskTitle))
			lines = append(lines, line)
		} else {
			line := taskItemStyle.Render(fmt.Sprintf("  %s  %s", status, task.TaskTitle))
			lines = append(lines, line)
		}
	}

	// Limit visible lines
	if len(lines) > height {
		start := a.selectedIdx - height/2
		if start < 0 {
			start = 0
		}
		end := start + height
		if end > len(lines) {
			end = len(lines)
			start = max(0, end-height)
		}
		lines = lines[start:end]
	}

	return strings.Join(lines, "\n")
}

func (a *App) renderAgentsPanel(height int) string {
	var b strings.Builder

	b.WriteString("\n  🤖 Connected Agents\n")
	b.WriteString("  " + strings.Repeat("─", 40) + "\n\n")

	if len(a.agents) == 0 {
		b.WriteString("  No agents detected.\n")
		b.WriteString("  Type: scan to detect installed AI tools\n")
		b.WriteString("  Type: agent add <name> <type> to add manually\n")
		return b.String()
	}

	for i, agent := range a.agents {
		statusIcon := agentOnlineStyle.Render("●")
		if agent.Status != "online" {
			statusIcon = agentOfflineStyle.Render("○")
		}

		name := agent.Name
		typeLabel := lipgloss.NewStyle().Foreground(mutedColor).Render(fmt.Sprintf("(%s)", agent.Type))

		var line string
		if i == a.agentIdx {
			line = selectedStyle.Render(fmt.Sprintf("▶ %s %s %s", statusIcon, name, typeLabel))
		} else {
			line = fmt.Sprintf("    %s %s %s", statusIcon, name, typeLabel)
		}
		b.WriteString(line + "\n")

		// Show path for selected agent
		if i == a.agentIdx && agent.Path != "" {
			pathLine := lipgloss.NewStyle().Foreground(mutedColor).Render(fmt.Sprintf("      Path: %s", agent.Path))
			b.WriteString(pathLine + "\n")
		}
		if i == a.agentIdx && agent.Version != "" {
			verLine := lipgloss.NewStyle().Foreground(mutedColor).Render(fmt.Sprintf("      Version: %s", agent.Version))
			b.WriteString(verLine + "\n")
		}
	}

	b.WriteString("\n  " + helpStyle.Render("Commands: scan | agent add <name> <type>") + "\n")

	return b.String()
}

func (a *App) renderTaskDetail(height int) string {
	if a.currentTask == nil {
		return "\n  Loading...\n"
	}

	var b strings.Builder
	t := a.currentTask

	b.WriteString(fmt.Sprintf("\n  📋 %s\n", lipgloss.NewStyle().Bold(true).Render(t.Title)))
	b.WriteString(fmt.Sprintf("  ID: %s\n", t.ID[:8]))
	b.WriteString(fmt.Sprintf("  Status: %s\n", a.formatStatus(t.Status)))
	if t.Description != "" {
		b.WriteString(fmt.Sprintf("  Description: %s\n", t.Description))
	}
	if t.ClaimedBy != "" {
		b.WriteString(fmt.Sprintf("  Claimed by: %s\n", t.ClaimedBy))
	}

	if len(a.runs) > 0 {
		b.WriteString("\n  📜 Recent Runs:\n")
		for i, run := range a.runs {
			if i >= 3 {
				break
			}
			exitStyle := lipgloss.NewStyle().Foreground(successColor)
			if run.ExitCode != 0 {
				exitStyle = lipgloss.NewStyle().Foreground(errorColor)
			}
			b.WriteString(fmt.Sprintf("    • %s (exit: %s)\n", run.Command, exitStyle.Render(fmt.Sprintf("%d", run.ExitCode))))
		}
	}

	if len(a.memory) > 0 {
		b.WriteString("\n  💾 Memory:\n")
		for i, mem := range a.memory {
			if i >= 3 {
				break
			}
			content := mem.Content
			if len(content) > 50 {
				content = content[:50] + "..."
			}
			b.WriteString(fmt.Sprintf("    • %s\n", content))
		}
	}

	return b.String()
}

func (a *App) formatStatus(status string) string {
	switch status {
	case "pending":
		return lipgloss.NewStyle().Foreground(warningColor).Render("○ PENDING")
	case "claimed":
		return lipgloss.NewStyle().Foreground(secondaryColor).Render("◐ CLAIMED")
	case "running":
		return lipgloss.NewStyle().Foreground(primaryColor).Render("◑ RUNNING")
	case "completed":
		return lipgloss.NewStyle().Foreground(successColor).Render("● DONE")
	case "failed":
		return lipgloss.NewStyle().Foreground(errorColor).Render("✗ FAILED")
	default:
		return status
	}
}

func (a *App) formatStatusPlain(status string) string {
	switch status {
	case "pending":
		return "○"
	case "claimed":
		return "◐"
	case "running":
		return "◑"
	case "completed":
		return "●"
	case "failed":
		return "✗"
	default:
		return "?"
	}
}

func (a *App) fetchTasks() tea.Cmd {
	a.loading = true
	return func() tea.Msg {
		tasks, err := a.client.ListTasks(a.filter)
		if err != nil {
			return errMsg{err}
		}
		return tasksLoadedMsg{tasks}
	}
}

func (a *App) fetchTaskDetail(taskID string) tea.Cmd {
	return func() tea.Msg {
		task, err := a.client.GetTask(taskID)
		if err != nil {
			return errMsg{err}
		}
		runs, _ := a.client.GetTaskLogs(taskID)
		memory, _ := a.client.GetTaskMemory(taskID)
		return taskDetailLoadedMsg{task, runs, memory}
	}
}

func (a *App) scanAgents() tea.Cmd {
	return func() tea.Msg {
		detector := agents.NewDetector()
		found := detector.Scan()
		return agentsScanMsg{found}
	}
}

func (a *App) checkDaemon() tea.Cmd {
	return func() tea.Msg {
		_, err := a.client.ListTasks("")
		return daemonStatusMsg{online: err == nil}
	}
}

func (a *App) executeCommand(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmd := parts[0]
	args := parts[1:]

	return func() tea.Msg {
		switch cmd {
		case "add":
			if len(args) < 1 {
				return commandResultMsg{"Usage: add <title>"}
			}
			title := strings.Join(args, " ")
			id, err := a.client.CreateTask(title, "")
			if err != nil {
				return commandResultMsg{"Error: " + err.Error()}
			}
			return commandResultMsg{fmt.Sprintf("✓ Created task: %s", id[:8])}

		case "claim":
			if len(a.tasks) == 0 {
				return commandResultMsg{"No task selected"}
			}
			taskID := a.tasks[a.selectedIdx].ID
			if err := a.client.ClaimTask(taskID); err != nil {
				return commandResultMsg{"Error: " + err.Error()}
			}
			return commandResultMsg{"✓ Task claimed"}

		case "release":
			if len(a.tasks) == 0 {
				return commandResultMsg{"No task selected"}
			}
			taskID := a.tasks[a.selectedIdx].ID
			if err := a.client.ReleaseTask(taskID); err != nil {
				return commandResultMsg{"Error: " + err.Error()}
			}
			return commandResultMsg{"✓ Task released"}

		case "run":
			if len(a.tasks) == 0 {
				return commandResultMsg{"No task selected"}
			}
			if len(args) < 1 {
				return commandResultMsg{"Usage: run <command>"}
			}
			taskID := a.tasks[a.selectedIdx].ID
			runCmd := args[0]
			runArgs := args[1:]
			exitCode, err := a.client.RunTask(taskID, runCmd, runArgs)
			if err != nil {
				return commandResultMsg{"Error: " + err.Error()}
			}
			return commandResultMsg{fmt.Sprintf("✓ Run completed (exit: %d)", exitCode)}

		case "note":
			if len(args) < 1 {
				return commandResultMsg{"Usage: note <content>"}
			}
			taskID := ""
			if len(a.tasks) > 0 {
				taskID = a.tasks[a.selectedIdx].ID
			}
			content := strings.Join(args, " ")
			if _, err := a.client.AddMemory(taskID, content); err != nil {
				return commandResultMsg{"Error: " + err.Error()}
			}
			return commandResultMsg{"✓ Note added"}

		case "query", "search":
			if len(args) < 1 {
				return commandResultMsg{"Usage: query <term>"}
			}
			query := strings.Join(args, " ")
			items, err := a.client.QueryMemory(query)
			if err != nil {
				return commandResultMsg{"Error: " + err.Error()}
			}
			return commandResultMsg{fmt.Sprintf("Found %d items", len(items))}

		case "scan":
			detector := agents.NewDetector()
			found := detector.Scan()
			a.agents = found
			return commandResultMsg{fmt.Sprintf("✓ Detected %d agents", len(found))}

		case "agents":
			a.mode = "agents"
			return commandResultMsg{fmt.Sprintf("%d agents connected", len(a.agents))}

		case "agent":
			if len(args) < 2 {
				return commandResultMsg{"Usage: agent add <name> <type>"}
			}
			if args[0] == "add" && len(args) >= 3 {
				name := args[1]
				agentType := args[2]
				newAgent := agents.Agent{
					ID:           fmt.Sprintf("custom-%s", name),
					Name:         name,
					Type:         agentType,
					Status:       "unknown",
					AutoDetected: false,
				}
				a.agents = append(a.agents, newAgent)
				return commandResultMsg{fmt.Sprintf("✓ Added agent: %s", name)}
			}
			return commandResultMsg{"Usage: agent add <name> <type>"}

		case "q", "quit", "exit":
			return tea.Quit

		case "login":
			// Trigger browser-based login
			if a.authManager == nil {
				return commandResultMsg{"Error: Auth not initialized"}
			}
			if a.currentUser != nil {
				return commandResultMsg{fmt.Sprintf("Already signed in as %s", a.currentUser.Username)}
			}
			// Perform login in background
			go func() {
				ctx := context.Background()
				session, err := a.authManager.Login(ctx)
				if err == nil && session != nil {
					a.currentUser = &session.User
				}
			}()
			return commandResultMsg{"Opening browser for login... Check your browser."}

		case "logout":
			if a.authManager == nil {
				return commandResultMsg{"Error: Auth not initialized"}
			}
			if a.currentUser == nil {
				return commandResultMsg{"Not signed in"}
			}
			username := a.currentUser.Username
			if err := a.authManager.Logout(); err != nil {
				return commandResultMsg{"Error: " + err.Error()}
			}
			a.currentUser = nil
			return commandResultMsg{fmt.Sprintf("✓ Signed out from %s", username)}

		case "whoami":
			if a.currentUser == nil {
				return commandResultMsg{"Not signed in. Use 'login' to authenticate."}
			}
			return commandResultMsg{fmt.Sprintf("Signed in as %s (%s)", a.currentUser.Username, a.currentUser.Email)}

		default:
			return commandResultMsg{fmt.Sprintf("Unknown: %s (try: add, claim, run, scan, login)", cmd)}
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type commandResultMsg struct {
	message string
}

type errMsg struct {
	err error
}

type tasksLoadedMsg struct {
	tasks []TaskItem
}

type taskDetailLoadedMsg struct {
	task   *TaskDetail
	runs   []RunDetail
	memory []MemoryDetail
}

type agentsScanMsg struct {
	agents []agents.Agent
}

type daemonStatusMsg struct {
	online bool
}

type workersFetchedMsg struct {
	stats *WorkersStats
}

type tickMsg time.Time

func (a *App) fetchWorkers() tea.Cmd {
	return func() tea.Msg {
		stats, err := a.client.GetWorkers()
		if err != nil {
			return errMsg{err}
		}
		return workersFetchedMsg{stats}
	}
}

func (a *App) tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a *App) renderWorkersPanel(height int) string {
	var b strings.Builder

	b.WriteString("\n  ⚙️  Worker Pool Monitor\n")
	b.WriteString("  " + strings.Repeat("─", 50) + "\n\n")

	if a.workersStats == nil {
		b.WriteString("  Loading...\n")
		return b.String()
	}

	stats := a.workersStats

	// Summary stats
	activeStyle := lipgloss.NewStyle().Foreground(successColor).Bold(true)
	maxStyle := lipgloss.NewStyle().Foreground(mutedColor)

	b.WriteString(fmt.Sprintf("  Active Workers: %s / %s\n\n",
		activeStyle.Render(fmt.Sprintf("%d", stats.ActiveWorkers)),
		maxStyle.Render(fmt.Sprintf("%d", stats.GlobalMax))))

	// Connector counts
	if len(stats.ConnectorCounts) > 0 {
		b.WriteString("  Connector Limits:\n")
		for name, count := range stats.ConnectorCounts {
			b.WriteString(fmt.Sprintf("    • %s: %d\n", name, count))
		}
		b.WriteString("\n")
	}

	// Workers table
	if len(stats.Workers) == 0 {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(mutedColor).Render("No active workers") + "\n")
	} else {
		b.WriteString("  Active Workers:\n")
		b.WriteString("  " + strings.Repeat("─", 60) + "\n")

		// Header
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(cyanColor)
		b.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
			headerStyle.Render(fmt.Sprintf("%-8s", "WORKER")),
			headerStyle.Render(fmt.Sprintf("%-30s", "TASK")),
			headerStyle.Render(fmt.Sprintf("%-10s", "TTL")),
			headerStyle.Render(fmt.Sprintf("%-10s", "CONNECTOR")),
		))
		b.WriteString("  " + strings.Repeat("─", 60) + "\n")

		for _, w := range stats.Workers {
			// Calculate TTL remaining
			ttlRemaining := time.Until(w.LeaseExpires)
			ttlStr := formatDuration(ttlRemaining)
			ttlStyle := lipgloss.NewStyle().Foreground(successColor)
			if ttlRemaining < 60*time.Second {
				ttlStyle = lipgloss.NewStyle().Foreground(warningColor)
			}
			if ttlRemaining < 30*time.Second {
				ttlStyle = lipgloss.NewStyle().Foreground(errorColor)
			}

			taskTitle := w.TaskTitle
			if len(taskTitle) > 28 {
				taskTitle = taskTitle[:25] + "..."
			}

			workerShort := w.WorkerID
			if len(workerShort) > 8 {
				workerShort = workerShort[:8]
			}

			b.WriteString(fmt.Sprintf("  %-8s  %-30s  %s  %-10s\n",
				workerShort,
				taskTitle,
				ttlStyle.Render(fmt.Sprintf("%-10s", ttlStr)),
				w.ConnectorName,
			))
		}
	}

	b.WriteString("\n  " + helpStyle.Render("Press Esc to go back, w to refresh") + "\n")

	return b.String()
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "EXPIRED"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}
