// Package store provides SQLite-backed persistence for Neona.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fentz26/neona/internal/models"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Store provides access to the Neona SQLite database.
type Store struct {
	db *sql.DB
}

// New creates a new Store and runs migrations.
func New(dbPath string) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	// Open with WAL mode for better concurrency
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Set connection pool settings for concurrent access
	db.SetMaxOpenConns(1) // SQLite only supports one writer at a time
	db.SetMaxIdleConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping checks the database connection is alive.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// migrate runs idempotent schema migrations.
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		claimed_by TEXT,
		claimed_at DATETIME,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS leases (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		holder_id TEXT NOT NULL,
		ttl_sec INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL,
		FOREIGN KEY (task_id) REFERENCES tasks(id)
	);

	CREATE TABLE IF NOT EXISTS locks (
		id TEXT PRIMARY KEY,
		resource_id TEXT NOT NULL UNIQUE,
		holder_id TEXT NOT NULL,
		lock_type TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		expires_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS runs (
		id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		command TEXT NOT NULL,
		args TEXT,
		exit_code INTEGER,
		stdout TEXT,
		stderr TEXT,
		started_at DATETIME NOT NULL,
		ended_at DATETIME,
		FOREIGN KEY (task_id) REFERENCES tasks(id)
	);

	CREATE TABLE IF NOT EXISTS pdr (
		id TEXT PRIMARY KEY,
		action TEXT NOT NULL,
		inputs_hash TEXT NOT NULL,
		outcome TEXT NOT NULL,
		task_id TEXT,
		details TEXT,
		timestamp DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS memory_items (
		id TEXT PRIMARY KEY,
		task_id TEXT,
		content TEXT NOT NULL,
		tags TEXT,
		created_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
	CREATE INDEX IF NOT EXISTS idx_leases_task_id ON leases(task_id);
	CREATE INDEX IF NOT EXISTS idx_runs_task_id ON runs(task_id);
	CREATE INDEX IF NOT EXISTS idx_memory_items_task_id ON memory_items(task_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// --- Task Operations ---

// CreateTask inserts a new task.
func (s *Store) CreateTask(title, description string) (*models.Task, error) {
	now := time.Now().UTC()
	task := &models.Task{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		Status:      models.TaskStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err := s.db.Exec(
		`INSERT INTO tasks (id, title, description, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		task.ID, task.Title, task.Description, task.Status, task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}
	return task, nil
}

// GetTask retrieves a task by ID.
func (s *Store) GetTask(id string) (*models.Task, error) {
	task := &models.Task{}
	var claimedAt sql.NullTime
	var claimedBy sql.NullString

	err := s.db.QueryRow(
		`SELECT id, title, description, status, claimed_by, claimed_at, created_at, updated_at FROM tasks WHERE id = ?`,
		id,
	).Scan(&task.ID, &task.Title, &task.Description, &task.Status, &claimedBy, &claimedAt, &task.CreatedAt, &task.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query task: %w", err)
	}
	if claimedBy.Valid {
		task.ClaimedBy = claimedBy.String
	}
	if claimedAt.Valid {
		task.ClaimedAt = &claimedAt.Time
	}
	return task, nil
}

// ListTasks returns all tasks, optionally filtered by status.
func (s *Store) ListTasks(status string) ([]models.Task, error) {
	query := `SELECT id, title, description, status, claimed_by, claimed_at, created_at, updated_at FROM tasks`
	var args []interface{}

	if status != "" {
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var task models.Task
		var claimedAt sql.NullTime
		var claimedBy sql.NullString
		if err := rows.Scan(&task.ID, &task.Title, &task.Description, &task.Status, &claimedBy, &claimedAt, &task.CreatedAt, &task.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		if claimedBy.Valid {
			task.ClaimedBy = claimedBy.String
		}
		if claimedAt.Valid {
			task.ClaimedAt = &claimedAt.Time
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// UpdateTaskStatus updates the status of a task.
func (s *Store) UpdateTaskStatus(id string, status models.TaskStatus) error {
	_, err := s.db.Exec(
		`UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now().UTC(), id,
	)
	return err
}

// ClaimTask marks a task as claimed by a holder.
func (s *Store) ClaimTask(id, holderID string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE tasks SET status = ?, claimed_by = ?, claimed_at = ?, updated_at = ? WHERE id = ?`,
		models.TaskStatusClaimed, holderID, now, now, id,
	)
	return err
}

// ClaimResult holds the result of an atomic claim operation.
type ClaimResult struct {
	Task  *models.Task
	Lease *models.Lease
}

// ErrTaskNotClaimable indicates the task cannot be claimed (not found or wrong status).
var ErrTaskNotClaimable = fmt.Errorf("task not found or not claimable")

// ErrTaskAlreadyLeased indicates the task already has an active lease.
var ErrTaskAlreadyLeased = fmt.Errorf("task already has an active lease")

// ClaimTaskWithLeaseTx atomically claims a task and creates a lease in a single transaction.
// It verifies the task exists and is claimable, then updates the task status and creates a lease.
// On any error, neither the task status nor the lease is persisted.
func (s *Store) ClaimTaskWithLeaseTx(taskID, holderID string, ttlSec int) (*ClaimResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()

	// Step 1: Verify task exists and is claimable (pending status)
	var task models.Task
	var claimedAt sql.NullTime
	var claimedBy sql.NullString

	err = tx.QueryRow(
		`SELECT id, title, description, status, claimed_by, claimed_at, created_at, updated_at 
		 FROM tasks WHERE id = ?`,
		taskID,
	).Scan(&task.ID, &task.Title, &task.Description, &task.Status, &claimedBy, &claimedAt, &task.CreatedAt, &task.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrTaskNotClaimable
	}
	if err != nil {
		return nil, fmt.Errorf("query task: %w", err)
	}

	// Check if task is in a claimable state (pending)
	if task.Status != models.TaskStatusPending {
		return nil, ErrTaskNotClaimable
	}

	// Step 2: Check for existing active lease
	var existingLeaseID string
	err = tx.QueryRow(
		`SELECT id FROM leases WHERE task_id = ? AND expires_at > ?`,
		taskID, now,
	).Scan(&existingLeaseID)

	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("check existing lease: %w", err)
	}
	if existingLeaseID != "" {
		return nil, ErrTaskAlreadyLeased
	}

	// Step 3: Update task status to claimed
	result, err := tx.Exec(
		`UPDATE tasks SET status = ?, claimed_by = ?, claimed_at = ?, updated_at = ? WHERE id = ? AND status = ?`,
		models.TaskStatusClaimed, holderID, now, now, taskID, models.TaskStatusPending,
	)
	if err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		// Task was modified by another process between our check and update
		return nil, ErrTaskNotClaimable
	}

	// Step 4: Create lease
	lease := &models.Lease{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		HolderID:  holderID,
		TTLSec:    ttlSec,
		ExpiresAt: now.Add(time.Duration(ttlSec) * time.Second),
		CreatedAt: now,
	}

	_, err = tx.Exec(
		`INSERT INTO leases (id, task_id, holder_id, ttl_sec, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		lease.ID, lease.TaskID, lease.HolderID, lease.TTLSec, lease.ExpiresAt, lease.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert lease: %w", err)
	}

	// Step 5: Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// Update task with claimed info for return
	task.Status = models.TaskStatusClaimed
	task.ClaimedBy = holderID
	task.ClaimedAt = &now
	task.UpdatedAt = now

	return &ClaimResult{
		Task:  &task,
		Lease: lease,
	}, nil
}

// ReleaseTask releases a task claim.
func (s *Store) ReleaseTask(id string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE tasks SET status = ?, claimed_by = NULL, claimed_at = NULL, updated_at = ? WHERE id = ?`,
		models.TaskStatusPending, now, id,
	)
	return err
}

// --- Lease Operations ---

// CreateLease creates a new lease for a task.
func (s *Store) CreateLease(taskID, holderID string, ttlSec int) (*models.Lease, error) {
	now := time.Now().UTC()
	lease := &models.Lease{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		HolderID:  holderID,
		TTLSec:    ttlSec,
		ExpiresAt: now.Add(time.Duration(ttlSec) * time.Second),
		CreatedAt: now,
	}

	_, err := s.db.Exec(
		`INSERT INTO leases (id, task_id, holder_id, ttl_sec, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		lease.ID, lease.TaskID, lease.HolderID, lease.TTLSec, lease.ExpiresAt, lease.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert lease: %w", err)
	}
	return lease, nil
}

// GetActiveLease returns the active lease for a task, if any.
func (s *Store) GetActiveLease(taskID string) (*models.Lease, error) {
	lease := &models.Lease{}
	err := s.db.QueryRow(
		`SELECT id, task_id, holder_id, ttl_sec, expires_at, created_at FROM leases WHERE task_id = ? AND expires_at > ? ORDER BY created_at DESC LIMIT 1`,
		taskID, time.Now().UTC(),
	).Scan(&lease.ID, &lease.TaskID, &lease.HolderID, &lease.TTLSec, &lease.ExpiresAt, &lease.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query lease: %w", err)
	}
	return lease, nil
}

// RenewLease extends the expiry of a lease (heartbeat).
func (s *Store) RenewLease(leaseID string, ttlSec int) error {
	_, err := s.db.Exec(
		`UPDATE leases SET expires_at = ? WHERE id = ?`,
		time.Now().UTC().Add(time.Duration(ttlSec)*time.Second), leaseID,
	)
	return err
}

// DeleteLease removes a lease.
func (s *Store) DeleteLease(leaseID string) error {
	_, err := s.db.Exec(`DELETE FROM leases WHERE id = ?`, leaseID)
	return err
}

// --- Lock Operations ---

// ErrResourceLocked indicates the resource is already locked by another holder.
var ErrResourceLocked = fmt.Errorf("resource already locked")

// LockConflict contains information about an existing lock when acquisition fails.
type LockConflict struct {
	HolderID  string
	ExpiresAt time.Time
}

// AcquireLock attempts to acquire a lock on a resource atomically.
// It first cleans up expired locks, then attempts to insert a new lock.
// If a lock already exists, it returns ErrResourceLocked.
func (s *Store) AcquireLock(resourceID, holderID, lockType string, ttlSec int) (*models.Lock, error) {
	// Use IMMEDIATE transaction to acquire write lock early and prevent races
	tx, err := s.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelDefault})
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()

	// Step 1: Clean up expired locks for this resource within the transaction
	_, err = tx.Exec(`DELETE FROM locks WHERE resource_id = ? AND expires_at <= ?`, resourceID, now)
	if err != nil {
		return nil, fmt.Errorf("clean expired locks: %w", err)
	}

	// Step 2: Check for existing non-expired lock
	var existingHolder string
	var existingExpires time.Time
	err = tx.QueryRow(
		`SELECT holder_id, expires_at FROM locks WHERE resource_id = ? AND expires_at > ?`,
		resourceID, now,
	).Scan(&existingHolder, &existingExpires)

	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("check existing lock: %w", err)
	}
	if err != sql.ErrNoRows {
		// Lock exists and is not expired
		return nil, ErrResourceLocked
	}

	// Step 3: Insert new lock
	lock := &models.Lock{
		ID:         uuid.New().String(),
		ResourceID: resourceID,
		HolderID:   holderID,
		LockType:   lockType,
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Duration(ttlSec) * time.Second),
	}

	_, err = tx.Exec(
		`INSERT INTO locks (id, resource_id, holder_id, lock_type, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		lock.ID, lock.ResourceID, lock.HolderID, lock.LockType, lock.CreatedAt, lock.ExpiresAt,
	)
	if err != nil {
		// Check if this is a UNIQUE constraint violation (race condition)
		if strings.Contains(err.Error(), "UNIQUE constraint") || strings.Contains(err.Error(), "unique constraint") {
			return nil, ErrResourceLocked
		}
		return nil, fmt.Errorf("insert lock: %w", err)
	}

	// Step 4: Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return lock, nil
}

// GetLock retrieves a lock by resource ID if it exists and is not expired.
func (s *Store) GetLock(resourceID string) (*models.Lock, error) {
	now := time.Now().UTC()
	lock := &models.Lock{}

	err := s.db.QueryRow(
		`SELECT id, resource_id, holder_id, lock_type, created_at, expires_at 
		 FROM locks WHERE resource_id = ? AND expires_at > ?`,
		resourceID, now,
	).Scan(&lock.ID, &lock.ResourceID, &lock.HolderID, &lock.LockType, &lock.CreatedAt, &lock.ExpiresAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query lock: %w", err)
	}
	return lock, nil
}

// ReleaseLock releases a lock.
func (s *Store) ReleaseLock(lockID string) error {
	_, err := s.db.Exec(`DELETE FROM locks WHERE id = ?`, lockID)
	return err
}

// --- Run Operations ---

// CreateRun inserts a new run record.
func (s *Store) CreateRun(taskID, command string, args []string) (*models.Run, error) {
	now := time.Now().UTC()
	argsJSON, _ := json.Marshal(args)

	run := &models.Run{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		Command:   command,
		Args:      args,
		StartedAt: now,
	}

	_, err := s.db.Exec(
		`INSERT INTO runs (id, task_id, command, args, started_at) VALUES (?, ?, ?, ?, ?)`,
		run.ID, run.TaskID, run.Command, string(argsJSON), run.StartedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert run: %w", err)
	}
	return run, nil
}

// UpdateRun updates a run with results.
func (s *Store) UpdateRun(id string, exitCode int, stdout, stderr string) error {
	_, err := s.db.Exec(
		`UPDATE runs SET exit_code = ?, stdout = ?, stderr = ?, ended_at = ? WHERE id = ?`,
		exitCode, stdout, stderr, time.Now().UTC(), id,
	)
	return err
}

// GetRunsForTask returns all runs for a task.
func (s *Store) GetRunsForTask(taskID string) ([]models.Run, error) {
	rows, err := s.db.Query(
		`SELECT id, task_id, command, args, exit_code, stdout, stderr, started_at, ended_at FROM runs WHERE task_id = ? ORDER BY started_at DESC`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("query runs: %w", err)
	}
	defer rows.Close()

	var runs []models.Run
	for rows.Next() {
		var run models.Run
		var argsJSON string
		var endedAt sql.NullTime
		var exitCode sql.NullInt64
		var stdout, stderr sql.NullString

		if err := rows.Scan(&run.ID, &run.TaskID, &run.Command, &argsJSON, &exitCode, &stdout, &stderr, &run.StartedAt, &endedAt); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}

		if argsJSON != "" {
			json.Unmarshal([]byte(argsJSON), &run.Args)
		}
		if exitCode.Valid {
			run.ExitCode = int(exitCode.Int64)
		}
		if stdout.Valid {
			run.Stdout = stdout.String
		}
		if stderr.Valid {
			run.Stderr = stderr.String
		}
		if endedAt.Valid {
			run.EndedAt = endedAt.Time
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// --- PDR Operations ---

// WritePDR writes a Process Decision Record.
func (s *Store) WritePDR(action, inputsHash, outcome, taskID, details string) (*models.PDREntry, error) {
	now := time.Now().UTC()
	pdr := &models.PDREntry{
		ID:         uuid.New().String(),
		Action:     action,
		InputsHash: inputsHash,
		Outcome:    outcome,
		TaskID:     taskID,
		Details:    details,
		Timestamp:  now,
	}

	_, err := s.db.Exec(
		`INSERT INTO pdr (id, action, inputs_hash, outcome, task_id, details, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		pdr.ID, pdr.Action, pdr.InputsHash, pdr.Outcome, pdr.TaskID, pdr.Details, pdr.Timestamp,
	)
	if err != nil {
		return nil, fmt.Errorf("insert pdr: %w", err)
	}
	return pdr, nil
}

// --- Memory Operations ---

// AddMemory inserts a memory item.
func (s *Store) AddMemory(taskID, content, tags string) (*models.MemoryItem, error) {
	now := time.Now().UTC()
	item := &models.MemoryItem{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		Content:   content,
		Tags:      tags,
		CreatedAt: now,
	}

	_, err := s.db.Exec(
		`INSERT INTO memory_items (id, task_id, content, tags, created_at) VALUES (?, ?, ?, ?, ?)`,
		item.ID, item.TaskID, item.Content, item.Tags, item.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert memory: %w", err)
	}
	return item, nil
}

// QueryMemory searches memory items by content.
func (s *Store) QueryMemory(query string) ([]models.MemoryItem, error) {
	rows, err := s.db.Query(
		`SELECT id, task_id, content, tags, created_at FROM memory_items WHERE content LIKE ? ORDER BY created_at DESC LIMIT 50`,
		"%"+strings.TrimSpace(query)+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("query memory: %w", err)
	}
	defer rows.Close()

	var items []models.MemoryItem
	for rows.Next() {
		var item models.MemoryItem
		var taskID sql.NullString
		if err := rows.Scan(&item.ID, &taskID, &item.Content, &item.Tags, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		if taskID.Valid {
			item.TaskID = taskID.String
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetMemoryForTask returns memory items for a specific task.
func (s *Store) GetMemoryForTask(taskID string) ([]models.MemoryItem, error) {
	rows, err := s.db.Query(
		`SELECT id, task_id, content, tags, created_at FROM memory_items WHERE task_id = ? ORDER BY created_at DESC`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("query memory for task: %w", err)
	}
	defer rows.Close()

	var items []models.MemoryItem
	for rows.Next() {
		var item models.MemoryItem
		if err := rows.Scan(&item.ID, &item.TaskID, &item.Content, &item.Tags, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
