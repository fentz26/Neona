package tui

import "time"

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

// WorkerInfo represents an active worker
type WorkerInfo struct {
	WorkerID      string    `json:"worker_id"`
	TaskID        string    `json:"task_id"`
	TaskTitle     string    `json:"task_title"`
	LeaseID       string    `json:"lease_id"`
	LeaseExpires  time.Time `json:"lease_expires"`
	StartedAt     time.Time `json:"started_at"`
	ConnectorName string    `json:"connector_name"`
}

// WorkersStats contains scheduler worker pool statistics
type WorkersStats struct {
	ActiveWorkers   int            `json:"active_workers"`
	GlobalMax       int            `json:"global_max"`
	ConnectorCounts map[string]int `json:"connector_counts"`
	Workers         []WorkerInfo   `json:"workers"`
}
