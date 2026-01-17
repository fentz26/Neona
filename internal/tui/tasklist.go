package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	listTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	statusPending   = lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // Yellow
	statusClaimed   = lipgloss.NewStyle().Foreground(lipgloss.Color("4")) // Blue
	statusRunning   = lipgloss.NewStyle().Foreground(lipgloss.Color("6")) // Cyan
	statusCompleted = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // Green
	statusFailed    = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // Red
)

// TaskItem implements list.Item for the task list
type TaskItem struct {
	ID        string
	TaskTitle string
	Status    string
	ClaimedBy string
}

func (i TaskItem) FilterValue() string { return i.TaskTitle }
func (i TaskItem) Title() string       { return i.TaskTitle }
func (i TaskItem) Description() string {
	status := formatStatus(i.Status)
	if i.ClaimedBy != "" {
		return fmt.Sprintf("%s • %s", status, i.ClaimedBy)
	}
	return status
}

func formatStatus(status string) string {
	switch status {
	case "pending":
		return statusPending.Render("● pending")
	case "claimed":
		return statusClaimed.Render("● claimed")
	case "running":
		return statusRunning.Render("● running")
	case "completed":
		return statusCompleted.Render("● completed")
	case "failed":
		return statusFailed.Render("● failed")
	default:
		return status
	}
}

// TaskListModel manages the task list screen
type TaskListModel struct {
	client      *Client
	list        list.Model
	tasks       []TaskItem
	filter      string
	filterIndex int
	width       int
	height      int
	loading     bool
}

var filters = []string{"", "pending", "claimed", "running", "completed", "failed"}
var filterLabels = []string{"all", "pending", "claimed", "running", "completed", "failed"}

// NewTaskListModel creates a new task list model
func NewTaskListModel(client *Client) *TaskListModel {
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 80, 20)
	l.Title = "Tasks"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = listTitleStyle

	return &TaskListModel{
		client: client,
		list:   l,
	}
}

// Init initializes the task list
func (m *TaskListModel) Init() tea.Cmd {
	return m.Refresh()
}

// SetSize sets the list dimensions
func (m *TaskListModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.list.SetSize(w, h)
}

// SelectedTask returns the currently selected task
func (m *TaskListModel) SelectedTask() *TaskItem {
	if item := m.list.SelectedItem(); item != nil {
		task := item.(TaskItem)
		return &task
	}
	return nil
}

// CycleFilter cycles through status filters
func (m *TaskListModel) CycleFilter() {
	m.filterIndex = (m.filterIndex + 1) % len(filters)
	m.filter = filters[m.filterIndex]
	m.list.Title = fmt.Sprintf("Tasks [%s]", filterLabels[m.filterIndex])
}

// Refresh fetches tasks from the API
func (m *TaskListModel) Refresh() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		tasks, err := m.client.ListTasks(m.filter)
		if err != nil {
			return errMsg{err}
		}
		return tasksLoadedMsg{tasks}
	}
}

// Update handles messages
func (m *TaskListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tasksLoadedMsg:
		m.loading = false
		m.tasks = msg.tasks
		items := make([]list.Item, len(m.tasks))
		for i, t := range m.tasks {
			items[i] = t
		}
		m.list.SetItems(items)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			return m, m.Refresh()
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the task list
func (m *TaskListModel) View() string {
	if m.loading {
		return "Loading tasks..."
	}
	return m.list.View()
}

type tasksLoadedMsg struct {
	tasks []TaskItem
}
