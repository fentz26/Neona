package store

import (
	"os"
	"path/filepath"
	"testing"

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

func newTestStore(t *testing.T) *Store {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return s
}
