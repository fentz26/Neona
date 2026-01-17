package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(lipgloss.Color("240"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("255"))

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("99")).
			MarginTop(1)
)

// TaskDetailModel manages the task detail screen
type TaskDetailModel struct {
	client  *Client
	taskID  string
	task    *TaskDetail
	runs    []RunDetail
	memory  []MemoryDetail
	width   int
	height  int
	loading bool
	scroll  int
}

// TaskDetail represents detailed task info
type TaskDetail struct {
	ID          string
	Title       string
	Description string
	Status      string
	ClaimedBy   string
	CreatedAt   string
	UpdatedAt   string
}

// RunDetail represents a run record
type RunDetail struct {
	ID       string
	Command  string
	ExitCode int
	Stdout   string
	Stderr   string
}

// MemoryDetail represents a memory item
type MemoryDetail struct {
	ID      string
	Content string
	Tags    string
}

// NewTaskDetailModel creates a new task detail model
func NewTaskDetailModel(client *Client) *TaskDetailModel {
	return &TaskDetailModel{
		client: client,
	}
}

// Init initializes the task detail model
func (m *TaskDetailModel) Init() tea.Cmd {
	return nil
}

// SetTask sets the task ID to display
func (m *TaskDetailModel) SetTask(id string) {
	m.taskID = id
	m.task = nil
	m.runs = nil
	m.memory = nil
	m.scroll = 0
}

// SetSize sets the dimensions
func (m *TaskDetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Refresh fetches task details
func (m *TaskDetailModel) Refresh() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		task, err := m.client.GetTask(m.taskID)
		if err != nil {
			return errMsg{err}
		}
		runs, _ := m.client.GetTaskLogs(m.taskID)
		memory, _ := m.client.GetTaskMemory(m.taskID)
		return taskDetailLoadedMsg{task, runs, memory}
	}
}

// Update handles messages
func (m *TaskDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case taskDetailLoadedMsg:
		m.loading = false
		m.task = msg.task
		m.runs = msg.runs
		m.memory = msg.memory
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.scroll++
		case "k", "up":
			if m.scroll > 0 {
				m.scroll--
			}
		case "r":
			return m, m.Refresh()
		}
	}
	return m, nil
}

// View renders the task detail
func (m *TaskDetailModel) View() string {
	if m.loading || m.task == nil {
		return "Loading task details..."
	}

	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render(m.task.Title))
	b.WriteString("\n\n")

	// Task fields
	b.WriteString(m.renderField("ID", m.task.ID))
	b.WriteString(m.renderField("Status", formatStatus(m.task.Status)))
	b.WriteString(m.renderField("Description", m.task.Description))
	if m.task.ClaimedBy != "" {
		b.WriteString(m.renderField("Claimed By", m.task.ClaimedBy))
	}
	b.WriteString(m.renderField("Created", m.task.CreatedAt))
	b.WriteString(m.renderField("Updated", m.task.UpdatedAt))

	// Runs section
	if len(m.runs) > 0 {
		b.WriteString(sectionStyle.Render("Run Logs"))
		b.WriteString("\n")
		for i, run := range m.runs {
			if i >= 3 {
				b.WriteString(fmt.Sprintf("  ... and %d more runs\n", len(m.runs)-3))
				break
			}
			exitStr := fmt.Sprintf("%d", run.ExitCode)
			if run.ExitCode == 0 {
				exitStr = statusCompleted.Render("0")
			} else {
				exitStr = statusFailed.Render(exitStr)
			}
			b.WriteString(fmt.Sprintf("  %s (exit: %s)\n", run.Command, exitStr))
			if run.Stdout != "" {
				stdout := truncate(run.Stdout, 100)
				b.WriteString(fmt.Sprintf("    → %s\n", stdout))
			}
		}
	}

	// Memory section
	if len(m.memory) > 0 {
		b.WriteString(sectionStyle.Render("Memory"))
		b.WriteString("\n")
		for i, mem := range m.memory {
			if i >= 5 {
				b.WriteString(fmt.Sprintf("  ... and %d more items\n", len(m.memory)-5))
				break
			}
			content := truncate(mem.Content, 60)
			b.WriteString(fmt.Sprintf("  • %s\n", content))
		}
	}

	// Apply scroll
	lines := strings.Split(b.String(), "\n")
	if m.scroll >= len(lines) {
		m.scroll = len(lines) - 1
	}
	if m.scroll < 0 {
		m.scroll = 0
	}
	visible := lines[m.scroll:]
	if len(visible) > m.height {
		visible = visible[:m.height]
	}

	return strings.Join(visible, "\n")
}

func (m *TaskDetailModel) renderField(label, value string) string {
	return fmt.Sprintf("%s %s\n", labelStyle.Render(label+":"), valueStyle.Render(value))
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

type taskDetailLoadedMsg struct {
	task   *TaskDetail
	runs   []RunDetail
	memory []MemoryDetail
}
