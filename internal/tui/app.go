// Package tui provides the interactive terminal UI for Neona.
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Screen type identifiers
type Screen int

const (
	ScreenTaskList Screen = iota
	ScreenTaskDetail
)

// App is the main TUI application model.
type App struct {
	client        *Client
	currentScreen Screen
	taskList      *TaskListModel
	taskDetail    *TaskDetailModel
	cmdBar        *CmdBarModel
	width         int
	height        int
	err           error
}

// New creates a new TUI application.
func New(apiAddr string) *App {
	client := NewClient(apiAddr)
	app := &App{
		client:        client,
		currentScreen: ScreenTaskList,
	}
	app.taskList = NewTaskListModel(client)
	app.taskDetail = NewTaskDetailModel(client)
	app.cmdBar = NewCmdBarModel()
	return app
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
		a.taskList.Init(),
		a.cmdBar.Init(),
	)
}

// Update implements tea.Model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global key handlers
		switch msg.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "esc":
			if a.currentScreen == ScreenTaskDetail {
				a.currentScreen = ScreenTaskList
				return a, a.taskList.Refresh()
			}
		case "enter":
			// Handle command bar input
			if a.cmdBar.focused {
				cmd := a.cmdBar.Submit()
				return a, a.handleCommand(cmd)
			}
			// Select task in list
			if a.currentScreen == ScreenTaskList {
				if task := a.taskList.SelectedTask(); task != nil {
					a.taskDetail.SetTask(task.ID)
					a.currentScreen = ScreenTaskDetail
					return a, a.taskDetail.Refresh()
				}
			}
		case ":":
			// Focus command bar
			a.cmdBar.Focus()
			return a, nil
		case "tab":
			if a.currentScreen == ScreenTaskList {
				a.taskList.CycleFilter()
				return a, a.taskList.Refresh()
			}
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.taskList.SetSize(msg.Width, msg.Height-3)
		a.taskDetail.SetSize(msg.Width, msg.Height-3)

	case errMsg:
		a.err = msg.err
		return a, nil

	case refreshMsg:
		return a, a.taskList.Refresh()
	}

	// Update focused component
	if a.cmdBar.focused {
		newCmdBar, cmd := a.cmdBar.Update(msg)
		a.cmdBar = newCmdBar.(*CmdBarModel)
		cmds = append(cmds, cmd)
	} else {
		switch a.currentScreen {
		case ScreenTaskList:
			newList, cmd := a.taskList.Update(msg)
			a.taskList = newList.(*TaskListModel)
			cmds = append(cmds, cmd)
		case ScreenTaskDetail:
			newDetail, cmd := a.taskDetail.Update(msg)
			a.taskDetail = newDetail.(*TaskDetailModel)
			cmds = append(cmds, cmd)
		}
	}

	return a, tea.Batch(cmds...)
}

// View implements tea.Model
func (a *App) View() string {
	var content string

	switch a.currentScreen {
	case ScreenTaskList:
		content = a.taskList.View()
	case ScreenTaskDetail:
		content = a.taskDetail.View()
	}

	// Add command bar at bottom
	cmdBarView := a.cmdBar.View()

	// Add status bar
	statusBar := a.renderStatusBar()

	return content + "\n" + statusBar + "\n" + cmdBarView
}

func (a *App) renderStatusBar() string {
	screen := "Tasks"
	if a.currentScreen == ScreenTaskDetail {
		screen = "Detail"
	}

	help := "q:quit | :cmd | tab:filter | enter:select"
	if a.currentScreen == ScreenTaskDetail {
		help = "q:quit | esc:back | :cmd"
	}

	padding := a.width - len(screen) - len(help) - 4
	if padding < 0 {
		padding = 0
	}

	return fmt.Sprintf(" [%s]%*s%s ", screen, padding, "", help)
}

func (a *App) handleCommand(input string) tea.Cmd {
	return a.cmdBar.Execute(a.client, input, func() string {
		if task := a.taskList.SelectedTask(); task != nil {
			return task.ID
		}
		return ""
	})
}

// Message types
type errMsg struct{ err error }
type refreshMsg struct{}
