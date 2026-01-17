package scheduler

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fentz26/neona/internal/audit"
	"github.com/fentz26/neona/internal/connectors"
	"github.com/fentz26/neona/internal/store"
)

// mockConnector implements a simple mock connector for testing.
type mockConnector struct {
	name string
}

func (m *mockConnector) Name() string {
	return m.name
}

func (m *mockConnector) Execute(ctx context.Context, cmd string, args []string) (*connectors.ExecResult, error) {
	return &connectors.ExecResult{
		Command:  cmd,
		Args:     args,
		ExitCode: 0,
		Stdout:   "mock output",
		Stderr:   "",
	}, nil
}

func (m *mockConnector) IsAllowed(cmd string, args []string) bool {
	return true
}

func TestAtomicClaim(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	
	// Create multiple pending tasks
	for i := 0; i < 5; i++ {
		_, err := s.CreateTask("Task", "Description")
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}
	}
	
	// Attempt to claim tasks concurrently
	var wg sync.WaitGroup
	claimedTasks := make(map[string]bool)
	var mu sync.Mutex
	errors := 0
	
	numWorkers := 10
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerNum int) {
			defer wg.Done()
			
			// Add a small delay to spread out the claims
			time.Sleep(time.Duration(workerNum*10) * time.Millisecond)
			
			task, lease, err := s.AtomicClaimTask("worker", 300)
			if err != nil {
				mu.Lock()
				errors++
				mu.Unlock()
				return
			}
			
			if task != nil {
				mu.Lock()
				if claimedTasks[task.ID] {
					t.Errorf("Task %s was claimed multiple times!", task.ID)
				}
				claimedTasks[task.ID] = true
				mu.Unlock()
				
				// Clean up lease
				s.DeleteLease(lease.ID)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify we claimed exactly 5 tasks (no double claims)
	if len(claimedTasks) != 5 {
		t.Errorf("Expected 5 unique claimed tasks, got %d (errors: %d)", len(claimedTasks), errors)
	}
}

func TestSchedulerConcurrencyLimits(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	
	pdr := audit.NewPDRWriter(s)
	conn := &mockConnector{name: "test"}
	
	cfg := &Config{
		GlobalMax: 3,
		ByConnector: map[string]int{
			"test": 2,
		},
	}
	
	sch := New(s, pdr, conn, cfg)
	
	// Create multiple pending tasks
	for i := 0; i < 10; i++ {
		_, err := s.CreateTask("Task", "Description")
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}
	}
	
	// Start scheduler
	sch.Start()
	defer sch.Stop()
	
	// Poll until workers are active or timeout
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	var stats map[string]interface{}
	var activeWorkers int
	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for scheduler to dispatch tasks")
		case <-ticker.C:
			stats = sch.GetStats()
			activeWorkers = stats["active_workers"].(int)
			if activeWorkers > 0 {
				goto hasWorkers
			}
		}
	}
hasWorkers:
	// Give scheduler a moment to potentially exceed limits if buggy
	time.Sleep(500 * time.Millisecond)
	stats = sch.GetStats()
	activeWorkers = stats["active_workers"].(int)
	
	if activeWorkers > cfg.GlobalMax {
		t.Errorf("Active workers %d exceeds global max %d", activeWorkers, cfg.GlobalMax)
	}
	
	connectorCounts := stats["connector_counts"].(map[string]int)
	if count := connectorCounts["test"]; count > cfg.ByConnector["test"] {
		t.Errorf("Connector workers %d exceeds limit %d", count, cfg.ByConnector["test"])
	}
}

func TestSchedulerDispatchPDR(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	
	pdr := audit.NewPDRWriter(s)
	conn := &mockConnector{name: "test"}
	
	cfg := &Config{
		GlobalMax: 5,
		ByConnector: map[string]int{
			"test": 5,
		},
	}
	
	sch := New(s, pdr, conn, cfg)
	
	// Create a task
	task, err := s.CreateTask("Test Task", "Description")
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}
	
	// Start scheduler
	sch.Start()
	defer sch.Stop()
	
	// Wait for scheduler to dispatch
	time.Sleep(2 * time.Second)
	
	// Verify task was claimed
	claimedTask, err := s.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	
	if claimedTask.Status != "claimed" {
		t.Errorf("Expected task to be claimed, got status: %s", claimedTask.Status)
	}
	
	// Note: Verifying PDR entries would require querying the PDR table
	// which is not exposed in the current store API
}

func TestSchedulerNoDoubleClaim(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	
	pdr := audit.NewPDRWriter(s)
	conn := &mockConnector{name: "test"}
	
	cfg := &Config{
		GlobalMax: 10,
		ByConnector: map[string]int{
			"test": 10,
		},
	}
	
	sch := New(s, pdr, conn, cfg)
	sch.workerDuration = 10 * time.Second // Long enough to keep tasks claimed
	
	// Create tasks
	numTasks := 5
	for i := 0; i < numTasks; i++ {
		_, err := s.CreateTask("Task", "Description")
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}
	}
	
	// Start scheduler
	sch.Start()
	defer sch.Stop()
	
	// Poll until all tasks are claimed or timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for all tasks to be claimed")
		case <-ticker.C:
			tasks, err := s.ListTasks("")
			if err != nil {
				t.Fatalf("Failed to list tasks: %v", err)
			}
			claimedCount := 0
			for _, task := range tasks {
				if task.Status == "claimed" {
					claimedCount++
				}
			}
			if claimedCount == numTasks {
				goto allClaimed
			}
		}
	}
allClaimed:
	// Verify all tasks are claimed exactly once
	tasks, err := s.ListTasks("")
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}
	
	claimedCount := 0
	for _, task := range tasks {
		if task.Status == "claimed" {
			claimedCount++
			if task.ClaimedBy == "" {
				t.Error("Claimed task has no holder")
			}
		}
	}
	
	if claimedCount != numTasks {
		t.Errorf("Expected %d claimed tasks, got %d", numTasks, claimedCount)
	}
}

func newTestStore(t *testing.T) *store.Store {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	return s
}
