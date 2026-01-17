package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Suggestions provides autocomplete for commands
type Suggestions struct {
	items        []SuggestionItem
	filtered     []SuggestionItem
	selectedIdx  int
	visible      bool
	prefix       string // "/", "@", or "!"
	currentInput string
}

// SuggestionItem represents a single autocomplete suggestion
type SuggestionItem struct {
	Text        string
	Description string
	Type        string // "command", "agent", "task", "action"
}

var commandSuggestions = []SuggestionItem{
	{Text: "add", Description: "Create a new task", Type: "command"},
	{Text: "claim", Description: "Claim the selected task", Type: "command"},
	{Text: "release", Description: "Release the selected task", Type: "command"},
	{Text: "run", Description: "Execute a command on selected task", Type: "command"},
	{Text: "note", Description: "Add a memory note", Type: "command"},
	{Text: "query", Description: "Search memory items", Type: "command"},
	{Text: "scan", Description: "Scan for AI agents", Type: "command"},
	{Text: "agents", Description: "View connected agents", Type: "command"},
	{Text: "agent add", Description: "Manually add an agent", Type: "command"},
	{Text: "login", Description: "Sign in to your Neona account", Type: "command"},
	{Text: "logout", Description: "Sign out of your account", Type: "command"},
	{Text: "whoami", Description: "Show current user info", Type: "command"},
}

var actionSuggestions = []SuggestionItem{
	{Text: "claim-and-run", Description: "Claim task and run git status", Type: "action"},
	{Text: "test", Description: "Run 'go test ./...'", Type: "action"},
	{Text: "diff", Description: "Run 'git diff'", Type: "action"},
	{Text: "status", Description: "Run 'git status'", Type: "action"},
}

// NewSuggestions creates a new suggestions handler
func NewSuggestions() *Suggestions {
	return &Suggestions{
		items:   commandSuggestions,
		visible: false,
	}
}

// Update updates suggestions based on current input
func (s *Suggestions) Update(input string) {
	if input == "" {
		s.visible = false
		s.filtered = nil
		s.prefix = ""
		return
	}

	// Check for trigger characters
	firstChar := string(input[0])
	if firstChar == "/" {
		s.prefix = "/"
		s.items = commandSuggestions // Reset to commands
		s.visible = true
		query := strings.ToLower(strings.TrimPrefix(input, "/"))
		s.filter(query)
	} else if firstChar == "@" {
		s.prefix = "@"
		// For @, start with empty list until agents/tasks are populated
		// Do NOT reuse previous s.items if it was commands
		if len(s.items) > 0 && s.items[0].Type == "command" {
			s.items = []SuggestionItem{}
		}
		s.visible = true
		query := strings.ToLower(strings.TrimPrefix(input, "@"))
		s.filter(query)
	} else if firstChar == "!" {
		s.prefix = "!"
		s.items = actionSuggestions // Reset to actions
		s.visible = true
		query := strings.ToLower(strings.TrimPrefix(input, "!"))
		s.filter(query)
	} else {
		s.visible = false
		s.filtered = nil
		s.prefix = ""
	}

	s.currentInput = input
}

// SetAgents updates the agent suggestions
func (s *Suggestions) SetAgents(agents []string) {
	if s.prefix == "@" {
		s.items = make([]SuggestionItem, len(agents))
		for i, agent := range agents {
			s.items[i] = SuggestionItem{
				Text:        agent,
				Description: "Reference this agent",
				Type:        "agent",
			}
		}
		query := strings.ToLower(strings.TrimPrefix(s.currentInput, "@"))
		s.filter(query)
	}
}

// SetTasks updates the task suggestions
func (s *Suggestions) SetTasks(tasks []string) {
	if s.prefix == "@" {
		// Add tasks to agent suggestions
		taskItems := make([]SuggestionItem, len(tasks))
		for i, task := range tasks {
			taskItems[i] = SuggestionItem{
				Text:        task,
				Description: "Reference this task",
				Type:        "task",
			}
		}
		s.items = append(s.items, taskItems...)
		query := strings.ToLower(strings.TrimPrefix(s.currentInput, "@"))
		s.filter(query)
	}
}

func (s *Suggestions) filter(query string) {
	if query == "" {
		s.filtered = s.items
		s.selectedIdx = 0
		return
	}

	s.filtered = []SuggestionItem{}
	for _, item := range s.items {
		if strings.Contains(strings.ToLower(item.Text), query) {
			s.filtered = append(s.filtered, item)
		}
	}
	s.selectedIdx = 0
}

// Next moves to the next suggestion
func (s *Suggestions) Next() {
	if len(s.filtered) == 0 {
		return
	}
	s.selectedIdx = (s.selectedIdx + 1) % len(s.filtered)
}

// Prev moves to the previous suggestion
func (s *Suggestions) Prev() {
	if len(s.filtered) == 0 {
		return
	}
	s.selectedIdx--
	if s.selectedIdx < 0 {
		s.selectedIdx = len(s.filtered) - 1
	}
}

// Selected returns the currently selected suggestion
func (s *Suggestions) Selected() *SuggestionItem {
	if !s.visible || len(s.filtered) == 0 || s.selectedIdx >= len(s.filtered) {
		return nil
	}
	return &s.filtered[s.selectedIdx]
}

// IsVisible returns whether suggestions are currently visible
func (s *Suggestions) IsVisible() bool {
	return s.visible && len(s.filtered) > 0
}

// Render renders the suggestions dropdown
func (s *Suggestions) Render(width int) string {
	if !s.IsVisible() {
		return ""
	}

	var b strings.Builder

	suggestionStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#6366F1")).
		Padding(0, 1).
		Width(width - 4)

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7C3AED")).
		Foreground(lipgloss.Color("#F9FAFB")).
		Bold(true)

	itemStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9FAFB"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Italic(true)

	// Header
	var header string
	switch s.prefix {
	case "/":
		header = "💡 Commands"
	case "@":
		header = "🔗 References"
	case "!":
		header = "⚡ Quick Actions"
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED")).Render(header))
	b.WriteString("\n")

	// Show max 5 suggestions
	maxVisible := 5
	for i, item := range s.filtered {
		if i >= maxVisible {
			more := len(s.filtered) - maxVisible
			b.WriteString(descStyle.Render(fmt.Sprintf("  ... and %d more", more)))
			break
		}

		line := ""
		if i == s.selectedIdx {
			line = selectedStyle.Render("▶ " + item.Text)
			if item.Description != "" {
				line += " " + selectedStyle.Render(item.Description)
			}
		} else {
			line = itemStyle.Render("  " + item.Text)
			if item.Description != "" {
				line += " " + descStyle.Render(item.Description)
			}
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	return suggestionStyle.Render(b.String())
}
