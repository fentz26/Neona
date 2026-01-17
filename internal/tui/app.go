// Package tui provides the interactive terminal UI for Neona.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
)

// App is the main TUI application model.
type App struct {
	client      *Client
	tasks       []TaskItem
	selectedIdx int
	input       textinput.Model
	viewport    viewport.Model
	width       int
	height      int
	mode        string // "list" or "detail"
	currentTask *TaskDetail
	runs        []RunDetail
	memory      []MemoryDetail
	message     string
	filter      string
	filterIdx   int
	loading     bool
}

var filters = []string{"", "pending", "claimed", "running", "completed", "failed"}
var filterNames = []string{"ALL", "PENDING", "CLAIMED", "RUNNING", "DONE", "FAILED"}

// New creates a new TUI application.
func New(apiAddr string) *App {
	ti := textinput.New()
	ti.Placeholder = "Type command: add <title> | claim | run <cmd> | release | note <text> | query <term>"
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 80

	vp := viewport.New(80, 20)

	return &App{
		client:   NewClient(apiAddr),
		input:    ti,
		viewport: vp,
		mode:     "list",
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
			if a.mode == "detail" {
				a.mode = "list"
				a.currentTask = nil
				return a, a.fetchTasks()
			}

		case "up", "k":
			if a.mode == "list" && a.selectedIdx > 0 {
				a.selectedIdx--
			}

		case "down", "j":
			if a.mode == "list" && a.selectedIdx < len(a.tasks)-1 {
				a.selectedIdx++
			}

		case "tab":
			a.filterIdx = (a.filterIdx + 1) % len(filters)
			a.filter = filters[a.filterIdx]
			return a, a.fetchTasks()

		case "enter":
			cmd := strings.TrimSpace(a.input.Value())
			if cmd != "" {
				a.input.SetValue("")
				return a, a.executeCommand(cmd)
			} else if a.mode == "list" && len(a.tasks) > 0 {
				// Enter task detail
				task := a.tasks[a.selectedIdx]
				a.mode = "detail"
				return a, a.fetchTaskDetail(task.ID)
			}

		case "r":
			if a.mode == "list" {
				return a, a.fetchTasks()
			}
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

	return a, tea.Batch(cmds...)
}

// View implements tea.Model
func (a *App) View() string {
	var b strings.Builder

	// Header
	header := titleStyle.Render("🚀 NEONA Control Plane")
	filterLabel := fmt.Sprintf(" [%s]", filterNames[a.filterIdx])
	header += lipgloss.NewStyle().Foreground(mutedColor).Render(filterLabel)
	b.WriteString(header + "\n")
	b.WriteString(strings.Repeat("─", a.width) + "\n")

	// Main content area
	contentHeight := a.height - 8
	if contentHeight < 5 {
		contentHeight = 5
	}

	if a.mode == "list" {
		b.WriteString(a.renderTaskList(contentHeight))
	} else {
		b.WriteString(a.renderTaskDetail(contentHeight))
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
	b.WriteString("\n")

	// Status bar
	var status string
	if a.mode == "list" {
		status = fmt.Sprintf(" Tasks: %d | ↑↓:nav | Tab:filter | Enter:select | r:refresh | Ctrl+C:quit", len(a.tasks))
	} else {
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
		line := fmt.Sprintf("%s  %s", status, task.TaskTitle)

		if i == a.selectedIdx {
			line = selectedStyle.Render(fmt.Sprintf("▶ %s  %s", a.formatStatusPlain(task.Status), task.TaskTitle))
		} else {
			line = taskItemStyle.Render(fmt.Sprintf("  %s  %s", status, task.TaskTitle))
		}
		lines = append(lines, line)
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

		case "q", "quit", "exit":
			return tea.Quit

		default:
			return commandResultMsg{fmt.Sprintf("Unknown: %s (try: add, claim, run, release, note, query)", cmd)}
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
