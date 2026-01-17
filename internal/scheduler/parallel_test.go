package scheduler

import (
	"testing"
	"time"

	"github.com/fentz26/neona/internal/audit"
)

// Test10ParallelWorkers verifies that the scheduler can run 10 workers in parallel
// without double-claiming tasks, meeting the acceptance criteria.
func Test10ParallelWorkers(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	
	pdr := audit.NewPDRWriter(s)
	conn := &mockConnector{name: "test"}
	
	// Configure for 10 parallel workers
	cfg := &Config{
		GlobalMax: 10,
		ByConnector: map[string]int{
			"test": 10,
		},
	}
	
	sch := New(s, pdr, conn, cfg)
	sch.workerDuration = 15 * time.Second // Long enough to keep all 10 tasks claimed simultaneously
	
	// Create exactly 10 tasks
	numTasks := 10
	taskIDs := make([]string, numTasks)
	for i := 0; i < numTasks; i++ {
		task, err := s.CreateTask("Parallel Task", "Description")
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}
		taskIDs[i] = task.ID
	}
	
	// Start scheduler
	sch.Start()
	defer sch.Stop() // Ensure scheduler stops even on test failure to prevent goroutine leaks
	
	// Poll until all tasks are claimed or timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	var activeWorkers int
	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for 10 workers to be active, got %d", activeWorkers)
		case <-ticker.C:
			stats := sch.GetStats()
			activeWorkers = stats["active_workers"].(int)
			if activeWorkers == 10 {
				goto workersReady
			}
		}
	}
workersReady:
	// activeWorkers already has the value from polling loop
	
	// Verify we have 10 active workers (should be true since we only exit loop when activeWorkers == 10)
	if activeWorkers != 10 {
		t.Errorf("Expected 10 active workers, got %d", activeWorkers)
	}
	
	// Verify all tasks are claimed
	tasks, err := s.ListTasks("")
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	
	claimedCount := 0
	claimedByMap := make(map[string]string) // taskID -> workerID
	
	for _, task := range tasks {
		if task.Status == "claimed" {
			claimedCount++
			
			// Verify each task has a unique holder
			if existing, found := claimedByMap[task.ID]; found {
				t.Errorf("Task %s claimed multiple times! Previous holder: %s, Current: %s", 
					task.ID, existing, task.ClaimedBy)
			}
			claimedByMap[task.ID] = task.ClaimedBy
			
			// Verify the task has an active lease
			lease, err := s.GetActiveLease(task.ID)
			if err != nil {
				t.Errorf("Error getting lease for task %s: %v", task.ID, err)
			}
			if lease == nil {
				t.Errorf("Task %s is claimed but has no active lease", task.ID)
			}
		}
	}
	
	if claimedCount != numTasks {
		t.Errorf("Expected %d claimed tasks, got %d", numTasks, claimedCount)
	}
	
	// Verify no task was claimed by multiple workers (all workers are unique)
	uniqueWorkers := make(map[string]bool)
	for _, workerID := range claimedByMap {
		if uniqueWorkers[workerID] {
			t.Errorf("Worker %s claimed multiple tasks!", workerID)
		}
		uniqueWorkers[workerID] = true
	}
	
	if len(uniqueWorkers) != numTasks {
		t.Errorf("Expected %d unique workers, got %d", numTasks, len(uniqueWorkers))
	}
	
	t.Logf("SUCCESS: 10 workers running in parallel without double-claim")
	t.Logf("Active workers: %d", activeWorkers)
	t.Logf("Claimed tasks: %d", claimedCount)
	t.Logf("Unique workers: %d", len(uniqueWorkers))
}
