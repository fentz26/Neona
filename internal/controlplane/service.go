// Package controlplane provides the HTTP API and service layer for Neona.
package controlplane

import (
	"context"
	"fmt"

	"github.com/fentz26/neona/internal/audit"
	"github.com/fentz26/neona/internal/connectors"
	"github.com/fentz26/neona/internal/models"
	"github.com/fentz26/neona/internal/store"
)

// Service provides the control plane business logic.
type Service struct {
	store     *store.Store
	pdr       *audit.PDRWriter
	connector connectors.Connector
}

// NewService creates a new control plane service.
func NewService(s *store.Store, pdr *audit.PDRWriter, conn connectors.Connector) *Service {
	return &Service{
		store:     s,
		pdr:       pdr,
		connector: conn,
	}
}

// --- Task Operations ---

// CreateTask creates a new task.
func (s *Service) CreateTask(title, description string) (*models.Task, error) {
	task, err := s.store.CreateTask(title, description)
	if err != nil {
		return nil, err
	}

	s.pdr.Record("task.create", map[string]string{"title": title}, "success", task.ID, "")
	return task, nil
}

// GetTask retrieves a task by ID.
func (s *Service) GetTask(id string) (*models.Task, error) {
	return s.store.GetTask(id)
}

// ListTasks returns filtered tasks.
func (s *Service) ListTasks(status string) ([]models.Task, error) {
	return s.store.ListTasks(status)
}

// ClaimTask claims a task with a lease.
func (s *Service) ClaimTask(taskID, holderID string, ttlSec int) (*models.Lease, error) {
	// Check existing lease
	existing, err := s.store.GetActiveLease(taskID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrAlreadyClaimed
	}

	// Claim task
	if err := s.store.ClaimTask(taskID, holderID); err != nil {
		return nil, err
	}

	// Create lease
	lease, err := s.store.CreateLease(taskID, holderID, ttlSec)
	if err != nil {
		return nil, err
	}

	s.pdr.Record("task.claim", map[string]interface{}{"task_id": taskID, "holder_id": holderID, "ttl": ttlSec}, "success", taskID, "")
	return lease, nil
}

// ReleaseTask releases a task claim.
func (s *Service) ReleaseTask(taskID, holderID string) error {
	lease, err := s.store.GetActiveLease(taskID)
	if err != nil {
		return err
	}
	if lease == nil {
		return ErrNoLease
	}
	if lease.HolderID != holderID {
		return ErrNotOwner
	}

	if err := s.store.DeleteLease(lease.ID); err != nil {
		return err
	}
	if err := s.store.ReleaseTask(taskID); err != nil {
		return err
	}

	s.pdr.Record("task.release", map[string]string{"task_id": taskID, "holder_id": holderID}, "success", taskID, "")
	return nil
}

// RunTask executes a command for a task.
func (s *Service) RunTask(taskID, holderID, command string, args []string) (*models.Run, error) {
	// Verify claim
	lease, err := s.store.GetActiveLease(taskID)
	if err != nil {
		return nil, err
	}
	if lease == nil || lease.HolderID != holderID {
		return nil, ErrNotOwner
	}

	// Update task status
	if err := s.store.UpdateTaskStatus(taskID, models.TaskStatusRunning); err != nil {
		return nil, err
	}

	// Create run record
	run, err := s.store.CreateRun(taskID, command, args)
	if err != nil {
		return nil, err
	}

	// Execute via connector
	result, execErr := s.connector.Execute(context.Background(), command, args)

	outcome := "success"
	var exitCode int
	var stdout, stderr string

	if execErr != nil {
		outcome = "error"
		stderr = execErr.Error()
		exitCode = -1
	} else {
		exitCode = result.ExitCode
		stdout = result.Stdout
		stderr = result.Stderr
		if exitCode != 0 {
			outcome = "failed"
		}
	}

	// Update run record
	if err := s.store.UpdateRun(run.ID, exitCode, stdout, stderr); err != nil {
		return nil, err
	}

	// Update task status
	status := models.TaskStatusCompleted
	if outcome != "success" {
		status = models.TaskStatusFailed
	}
	s.store.UpdateTaskStatus(taskID, status)

	// Record PDR
	s.pdr.Record("task.run", map[string]interface{}{"task_id": taskID, "command": command, "args": args}, outcome, taskID, "")

	// Store run as memory item
	s.store.AddMemory(taskID, "Run: "+command+" "+joinArgs(args)+"\nOutput: "+stdout, "run,log")

	run.ExitCode = exitCode
	run.Stdout = stdout
	run.Stderr = stderr
	return run, nil
}

// GetTaskLogs returns run logs for a task.
func (s *Service) GetTaskLogs(taskID string) ([]models.Run, error) {
	return s.store.GetRunsForTask(taskID)
}

// RenewLease renews a lease (heartbeat).
func (s *Service) RenewLease(taskID, holderID string, ttlSec int) error {
	lease, err := s.store.GetActiveLease(taskID)
	if err != nil {
		return err
	}
	if lease == nil || lease.HolderID != holderID {
		return ErrNotOwner
	}
	return s.store.RenewLease(lease.ID, ttlSec)
}

// --- Memory Operations ---

// AddMemory adds a memory item.
func (s *Service) AddMemory(taskID, content, tags string) (*models.MemoryItem, error) {
	item, err := s.store.AddMemory(taskID, content, tags)
	if err != nil {
		return nil, err
	}
	s.pdr.Record("memory.add", map[string]string{"task_id": taskID, "content_len": fmt.Sprintf("%d", len(content))}, "success", taskID, "")
	return item, nil
}

// QueryMemory searches memory items.
func (s *Service) QueryMemory(query string) ([]models.MemoryItem, error) {
	return s.store.QueryMemory(query)
}

// GetTaskMemory returns memory items for a task.
func (s *Service) GetTaskMemory(taskID string) ([]models.MemoryItem, error) {
	return s.store.GetMemoryForTask(taskID)
}

// --- Lock Operations ---

// AcquireLock acquires a lock on a resource.
func (s *Service) AcquireLock(resourceID, holderID, lockType string, ttlSec int) (*models.Lock, error) {
	lock, err := s.store.AcquireLock(resourceID, holderID, lockType, ttlSec)
	if err != nil {
		return nil, err
	}
	s.pdr.Record("lock.acquire", map[string]string{"resource_id": resourceID, "holder_id": holderID}, "success", "", "")
	return lock, nil
}

// ReleaseLock releases a lock.
func (s *Service) ReleaseLock(lockID string) error {
	if err := s.store.ReleaseLock(lockID); err != nil {
		return err
	}
	s.pdr.Record("lock.release", map[string]string{"lock_id": lockID}, "success", "", "")
	return nil
}

func joinArgs(args []string) string {
	result := ""
	for _, a := range args {
		result += a + " "
	}
	return result
}
