package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	cmdBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1)

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)
)

// CmdBarModel manages the command input bar
type CmdBarModel struct {
	input   textinput.Model
	focused bool
	message string
}

// NewCmdBarModel creates a new command bar
func NewCmdBarModel() *CmdBarModel {
	ti := textinput.New()
	ti.Placeholder = "Enter command..."
	ti.CharLimit = 256
	return &CmdBarModel{
		input: ti,
	}
}

// Init initializes the command bar
func (m *CmdBarModel) Init() tea.Cmd {
	return nil
}

// Focus focuses the command bar
func (m *CmdBarModel) Focus() {
	m.focused = true
	m.input.Focus()
	m.message = ""
}

// Blur unfocuses the command bar
func (m *CmdBarModel) Blur() {
	m.focused = false
	m.input.Blur()
	m.input.SetValue("")
}

// Submit returns the current input and blurs
func (m *CmdBarModel) Submit() string {
	val := m.input.Value()
	m.Blur()
	return val
}

// Update handles messages
func (m *CmdBarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Blur()
			return m, nil
		}
	case cmdResultMsg:
		m.message = msg.message
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the command bar
func (m *CmdBarModel) View() string {
	if m.message != "" {
		return cmdBarStyle.Render(m.message)
	}
	if m.focused {
		prompt := promptStyle.Render(": ")
		return cmdBarStyle.Render(prompt + m.input.View())
	}
	return cmdBarStyle.Render("Press : to enter command (add, claim, run, release, note, query)")
}

// Execute processes a command
func (m *CmdBarModel) Execute(client *Client, input string, getTaskID func() string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmd := parts[0]
	args := parts[1:]

	return func() tea.Msg {
		var result string
		var err error

		switch cmd {
		case "add":
			if len(args) < 1 {
				return cmdResultMsg{"Usage: add <title> [description]"}
			}
			title := args[0]
			desc := ""
			if len(args) > 1 {
				desc = strings.Join(args[1:], " ")
			}
			id, err := client.CreateTask(title, desc)
			if err != nil {
				return cmdResultMsg{fmt.Sprintf("Error: %v", err)}
			}
			result = fmt.Sprintf("Created task: %s", id[:8])

		case "claim":
			taskID := getTaskID()
			if len(args) > 0 {
				taskID = args[0]
			}
			if taskID == "" {
				return cmdResultMsg{"No task selected"}
			}
			err = client.ClaimTask(taskID)
			if err != nil {
				return cmdResultMsg{fmt.Sprintf("Error: %v", err)}
			}
			result = "Task claimed"

		case "release":
			taskID := getTaskID()
			if len(args) > 0 {
				taskID = args[0]
			}
			if taskID == "" {
				return cmdResultMsg{"No task selected"}
			}
			err = client.ReleaseTask(taskID)
			if err != nil {
				return cmdResultMsg{fmt.Sprintf("Error: %v", err)}
			}
			result = "Task released"

		case "run":
			taskID := getTaskID()
			if taskID == "" {
				return cmdResultMsg{"No task selected"}
			}
			if len(args) < 1 {
				return cmdResultMsg{"Usage: run <command>"}
			}
			runCmd := args[0]
			runArgs := args[1:]
			exitCode, err := client.RunTask(taskID, runCmd, runArgs)
			if err != nil {
				return cmdResultMsg{fmt.Sprintf("Error: %v", err)}
			}
			result = fmt.Sprintf("Run completed (exit: %d)", exitCode)

		case "note":
			taskID := getTaskID()
			if len(args) < 1 {
				return cmdResultMsg{"Usage: note <content>"}
			}
			content := strings.Join(args, " ")
			_, err = client.AddMemory(taskID, content)
			if err != nil {
				return cmdResultMsg{fmt.Sprintf("Error: %v", err)}
			}
			result = "Note added"

		case "query":
			if len(args) < 1 {
				return cmdResultMsg{"Usage: query <search-term>"}
			}
			query := strings.Join(args, " ")
			items, err := client.QueryMemory(query)
			if err != nil {
				return cmdResultMsg{fmt.Sprintf("Error: %v", err)}
			}
			result = fmt.Sprintf("Found %d items", len(items))

		default:
			result = fmt.Sprintf("Unknown command: %s", cmd)
		}

		return cmdResultMsg{result}
	}
}

type cmdResultMsg struct {
	message string
}
