package tui

// TaskItem is a summary of a task for the list view
type TaskItem struct {
	ID        string
	TaskTitle string
	Status    string
	ClaimedBy string
}

// TaskDetail is the full task information
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
