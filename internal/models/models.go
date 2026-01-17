// Package models defines the core domain types for Neona.
package models

import "time"

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusClaimed   TaskStatus = "claimed"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// Task represents a unit of work in the control plane.
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClaimedBy   string     `json:"claimed_by,omitempty"`
	ClaimedAt   *time.Time `json:"claimed_at,omitempty"`
}

// Lease represents a temporary claim on a task with TTL.
type Lease struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	HolderID  string    `json:"holder_id"`
	TTLSec    int       `json:"ttl_sec"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Lock represents a resource lock (task-level or path-glob).
type Lock struct {
	ID         string    `json:"id"`
	ResourceID string    `json:"resource_id"` // task ID or glob pattern
	HolderID   string    `json:"holder_id"`
	LockType   string    `json:"lock_type"` // "task" or "glob"
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// Run represents an execution attempt of a task.
type Run struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Command   string    `json:"command"`
	Args      []string  `json:"args"`
	ExitCode  int       `json:"exit_code"`
	Stdout    string    `json:"stdout"`
	Stderr    string    `json:"stderr"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}

// PDREntry represents a Process Decision Record for audit.
type PDREntry struct {
	ID         string    `json:"id"`
	Action     string    `json:"action"`
	InputsHash string    `json:"inputs_hash"`
	Outcome    string    `json:"outcome"`
	TaskID     string    `json:"task_id,omitempty"`
	Details    string    `json:"details,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// MemoryItem represents a memory/knowledge snippet.
type MemoryItem struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id,omitempty"`
	Content   string    `json:"content"`
	Tags      string    `json:"tags,omitempty"` // comma-separated
	CreatedAt time.Time `json:"created_at"`
}
