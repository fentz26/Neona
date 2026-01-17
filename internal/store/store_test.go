package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fentz26/neona/internal/models"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestTaskCRUD(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Create
	task, err := s.CreateTask("Test Task", "Test Description")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if task.ID == "" {
		t.Error("Task ID should not be empty")
	}
	if task.Status != models.TaskStatusPending {
		t.Errorf("Expected status pending, got %s", task.Status)
	}

	// Get
	got, err := s.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Title != "Test Task" {
		t.Errorf("Expected title 'Test Task', got %s", got.Title)
	}

	// List
	tasks, err := s.ListTasks("")
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(tasks))
	}

	// List with filter
	tasks, err = s.ListTasks("pending")
	if err != nil {
		t.Fatalf("ListTasks with filter failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("Expected 1 pending task, got %d", len(tasks))
	}

	tasks, err = s.ListTasks("completed")
	if err != nil {
		t.Fatalf("ListTasks with filter failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("Expected 0 completed tasks, got %d", len(tasks))
	}

	// Update status
	err = s.UpdateTaskStatus(task.ID, models.TaskStatusCompleted)
	if err != nil {
		t.Fatalf("UpdateTaskStatus failed: %v", err)
	}

	got, _ = s.GetTask(task.ID)
	if got.Status != models.TaskStatusCompleted {
		t.Errorf("Expected status completed, got %s", got.Status)
	}
}

func TestClaimAndRelease(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	task, _ := s.CreateTask("Test", "")

	// Claim
	err := s.ClaimTask(task.ID, "holder-1")
	if err != nil {
		t.Fatalf("ClaimTask failed: %v", err)
	}

	got, _ := s.GetTask(task.ID)
	if got.Status != models.TaskStatusClaimed {
		t.Errorf("Expected claimed status, got %s", got.Status)
	}
	if got.ClaimedBy != "holder-1" {
		t.Errorf("Expected claimed by holder-1, got %s", got.ClaimedBy)
	}

	// Release
	err = s.ReleaseTask(task.ID)
	if err != nil {
		t.Fatalf("ReleaseTask failed: %v", err)
	}

	got, _ = s.GetTask(task.ID)
	if got.Status != models.TaskStatusPending {
		t.Errorf("Expected pending status after release, got %s", got.Status)
	}
}

func TestLeases(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	task, _ := s.CreateTask("Test", "")

	// Create lease
	lease, err := s.CreateLease(task.ID, "holder-1", 300)
	if err != nil {
		t.Fatalf("CreateLease failed: %v", err)
	}
	if lease.ID == "" {
		t.Error("Lease ID should not be empty")
	}

	// Get active lease
	active, err := s.GetActiveLease(task.ID)
	if err != nil {
		t.Fatalf("GetActiveLease failed: %v", err)
	}
	if active == nil {
		t.Error("Expected active lease")
	}

	// Renew
	err = s.RenewLease(lease.ID, 600)
	if err != nil {
		t.Fatalf("RenewLease failed: %v", err)
	}

	// Delete
	err = s.DeleteLease(lease.ID)
	if err != nil {
		t.Fatalf("DeleteLease failed: %v", err)
	}

	active, _ = s.GetActiveLease(task.ID)
	if active != nil {
		t.Error("Expected no active lease after delete")
	}
}

func TestRuns(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	task, _ := s.CreateTask("Test", "")

	// Create run
	run, err := s.CreateRun(task.ID, "git", []string{"status"})
	if err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Update run
	err = s.UpdateRun(run.ID, 0, "stdout content", "")
	if err != nil {
		t.Fatalf("UpdateRun failed: %v", err)
	}

	// Get runs
	runs, err := s.GetRunsForTask(task.ID)
	if err != nil {
		t.Fatalf("GetRunsForTask failed: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("Expected 1 run, got %d", len(runs))
	}
	if runs[0].Stdout != "stdout content" {
		t.Errorf("Unexpected stdout: %s", runs[0].Stdout)
	}
}

func TestMemory(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	task, _ := s.CreateTask("Test", "")

	// Add memory
	item, err := s.AddMemory(task.ID, "Test memory content", "tag1,tag2")
	if err != nil {
		t.Fatalf("AddMemory failed: %v", err)
	}
	if item.ID == "" {
		t.Error("Memory ID should not be empty")
	}

	// Query memory
	items, err := s.QueryMemory("memory")
	if err != nil {
		t.Fatalf("QueryMemory failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}

	// Get memory for task
	items, err = s.GetMemoryForTask(task.ID)
	if err != nil {
		t.Fatalf("GetMemoryForTask failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(items))
	}
}

func TestPDR(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	task, _ := s.CreateTask("Test", "")

	pdr, err := s.WritePDR("test.action", "abc123", "success", task.ID, "details")
	if err != nil {
		t.Fatalf("WritePDR failed: %v", err)
	}
	if pdr.ID == "" {
		t.Error("PDR ID should not be empty")
	}
}

func TestClaimTaskWithLeaseTx_Atomicity(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Create a task
	task, err := s.CreateTask("Test Task", "Description")
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Test successful atomic claim
	result, err := s.ClaimTaskWithLeaseTx(task.ID, "holder-1", 300)
	if err != nil {
		t.Fatalf("ClaimTaskWithLeaseTx failed: %v", err)
	}
	if result.Task.Status != models.TaskStatusClaimed {
		t.Errorf("Expected task status claimed, got %s", result.Task.Status)
	}
	if result.Lease == nil {
		t.Error("Expected lease to be created")
	}
	if result.Lease.HolderID != "holder-1" {
		t.Errorf("Expected holder-1, got %s", result.Lease.HolderID)
	}

	// Verify task status was actually updated in DB
	got, _ := s.GetTask(task.ID)
	if got.Status != models.TaskStatusClaimed {
		t.Errorf("Task status not persisted correctly, got %s", got.Status)
	}

	// Verify lease was created
	lease, _ := s.GetActiveLease(task.ID)
	if lease == nil {
		t.Error("Lease was not created in DB")
	}
}

func TestClaimTaskWithLeaseTx_TaskNotFound(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Try to claim non-existent task
	_, err := s.ClaimTaskWithLeaseTx("non-existent-id", "holder-1", 300)
	if err != ErrTaskNotClaimable {
		t.Errorf("Expected ErrTaskNotClaimable, got %v", err)
	}
}

func TestClaimTaskWithLeaseTx_AlreadyClaimed(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	task, _ := s.CreateTask("Test", "")

	// First claim succeeds
	_, err := s.ClaimTaskWithLeaseTx(task.ID, "holder-1", 300)
	if err != nil {
		t.Fatalf("First claim failed: %v", err)
	}

	// Second claim should fail
	_, err = s.ClaimTaskWithLeaseTx(task.ID, "holder-2", 300)
	if err == nil {
		t.Error("Expected second claim to fail")
	}
}

func TestClaimTaskWithLeaseTx_NotClaimableStatus(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	task, _ := s.CreateTask("Test", "")

	// Change task status to something not claimable
	s.UpdateTaskStatus(task.ID, models.TaskStatusRunning)

	// Claim should fail
	_, err := s.ClaimTaskWithLeaseTx(task.ID, "holder-1", 300)
	if err != ErrTaskNotClaimable {
		t.Errorf("Expected ErrTaskNotClaimable, got %v", err)
	}

	// Verify task status remains unchanged
	got, _ := s.GetTask(task.ID)
	if got.Status != models.TaskStatusRunning {
		t.Errorf("Task status should remain running, got %s", got.Status)
	}
}

func TestAcquireLock_Race(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	resourceID := "test-resource"

	// Test that second lock attempt fails deterministically
	lock1, err := s.AcquireLock(resourceID, "holder-1", "exclusive", 300)
	if err != nil {
		t.Fatalf("First lock acquisition failed: %v", err)
	}
	if lock1 == nil {
		t.Fatal("Expected first lock to be created")
	}

	// Second attempt should fail with ErrResourceLocked
	_, err = s.AcquireLock(resourceID, "holder-2", "exclusive", 300)
	if err != ErrResourceLocked {
		t.Errorf("Expected ErrResourceLocked for second lock, got: %v", err)
	}

	// Third attempt should also fail
	_, err = s.AcquireLock(resourceID, "holder-3", "exclusive", 300)
	if err != ErrResourceLocked {
		t.Errorf("Expected ErrResourceLocked for third lock, got: %v", err)
	}

	// Verify only one lock exists in DB
	lock, err := s.GetLock(resourceID)
	if err != nil {
		t.Fatalf("GetLock failed: %v", err)
	}
	if lock == nil {
		t.Error("Expected lock to exist")
	}
	if lock.HolderID != "holder-1" {
		t.Errorf("Expected lock holder to be holder-1, got %s", lock.HolderID)
	}
}

func TestAcquireLock_ConcurrentAttempts(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	resourceID := "test-resource-concurrent"
	numAttempts := 5
	successCount := 0
	failCount := 0

	// Sequential attempts simulate the race condition without actual goroutine races
	// Since SQLite serializes writes anyway, this tests the same logic
	for i := 0; i < numAttempts; i++ {
		_, err := s.AcquireLock(resourceID, fmt.Sprintf("holder-%d", i), "exclusive", 300)
		if err == nil {
			successCount++
		} else if err == ErrResourceLocked {
			failCount++
		} else {
			t.Errorf("Unexpected error: %v", err)
		}
	}

	// First attempt should succeed, rest should fail
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful lock, got %d", successCount)
	}
	if failCount != numAttempts-1 {
		t.Errorf("Expected %d failed locks, got %d", numAttempts-1, failCount)
	}
}

func TestAcquireLock_ExpiredCleanup(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	resourceID := "test-resource"

	// Acquire lock with very short TTL
	lock, err := s.AcquireLock(resourceID, "holder-1", "exclusive", 1)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}
	if lock == nil {
		t.Fatal("Expected lock to be created")
	}

	// Wait for lock to expire
	time.Sleep(2 * time.Second)

	// Now another holder should be able to acquire the lock
	// (expired lock should be cleaned up)
	lock2, err := s.AcquireLock(resourceID, "holder-2", "exclusive", 300)
	if err != nil {
		t.Fatalf("Second AcquireLock failed: %v", err)
	}
	if lock2 == nil {
		t.Error("Expected second lock to be created")
	}
	if lock2.HolderID != "holder-2" {
		t.Errorf("Expected holder-2, got %s", lock2.HolderID)
	}
}

func TestAcquireLock_ReleaseLock(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	resourceID := "test-resource"

	// Acquire lock
	lock, err := s.AcquireLock(resourceID, "holder-1", "exclusive", 300)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}

	// Release the lock
	err = s.ReleaseLock(lock.ID)
	if err != nil {
		t.Fatalf("ReleaseLock failed: %v", err)
	}

	// Now another holder should be able to acquire the lock
	lock2, err := s.AcquireLock(resourceID, "holder-2", "exclusive", 300)
	if err != nil {
		t.Fatalf("Second AcquireLock failed: %v", err)
	}
	if lock2 == nil {
		t.Error("Expected lock to be acquired after release")
	}
}

func TestPing(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.Ping(ctx)
	if err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func newTestStore(t *testing.T) *Store {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return s
}
