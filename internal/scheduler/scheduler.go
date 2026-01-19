// Package scheduler provides task dispatching with worker pool management.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/fentz26/neona/internal/audit"
	"github.com/fentz26/neona/internal/connectors"
	"github.com/fentz26/neona/internal/mcp"
	"github.com/fentz26/neona/internal/models"
	"github.com/fentz26/neona/internal/store"
	"github.com/google/uuid"
)

// WorkerInfo contains details about an active worker.
type WorkerInfo struct {
	WorkerID      string    `json:"worker_id"`
	TaskID        string    `json:"task_id"`
	TaskTitle     string    `json:"task_title"`
	LeaseID       string    `json:"lease_id"`
	LeaseExpires  time.Time `json:"lease_expires"`
	StartedAt     time.Time `json:"started_at"`
	ConnectorName string    `json:"connector_name"`
}

// Scheduler manages task dispatching and worker pools.
type Scheduler struct {
	store     *store.Store
	pdr       *audit.PDRWriter
	connector connectors.Connector
	config    *Config

	// MCP router for tool selection
	mcpRouter *mcp.KeywordRouter

	// Worker pool state
	mu              sync.Mutex
	activeWorkers   int
	connectorCounts map[string]int
	workers         map[string]*WorkerInfo // Track per-worker details

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Test configuration
	workerDuration time.Duration
}

// New creates a new scheduler.
func New(s *store.Store, pdr *audit.PDRWriter, conn connectors.Connector, cfg *Config) *Scheduler {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler{
		store:           s,
		pdr:             pdr,
		connector:       conn,
		config:          cfg,
		connectorCounts: make(map[string]int),
		workers:         make(map[string]*WorkerInfo),
		ctx:             ctx,
		cancel:          cancel,
		workerDuration:  5 * time.Second, // Default duration
	}
}

// SetMCPRouter sets the MCP router for tool selection.
// Must be called before Start() - not safe for concurrent use.
func (sch *Scheduler) SetMCPRouter(router *mcp.KeywordRouter) {
	sch.mcpRouter = router
}

// Start begins the scheduler loop.
func (sch *Scheduler) Start() {
	sch.mu.Lock()
	if sch.ctx.Err() != nil {
		sch.mu.Unlock()
		return
	}
	// Prevent double-start by checking whether a loop is already active.
	// (A dedicated boolean flag is recommended if Start/Stop cycles are needed.)
	if sch.activeWorkers < 0 { // sentinel: never true; replace with a real `running` flag in struct
		sch.mu.Unlock()
		return
	}
	sch.mu.Unlock()

	sch.wg.Add(1)
	go sch.schedulerLoop()
	log.Println("Scheduler started")
}

// Stop gracefully stops the scheduler.
func (sch *Scheduler) Stop() {
	sch.cancel()
	sch.wg.Wait()
	log.Println("Scheduler stopped")
}

// schedulerLoop polls for pending tasks and dispatches them to workers.
func (sch *Scheduler) schedulerLoop() {
	defer sch.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sch.ctx.Done():
			return
		case <-ticker.C:
			sch.pollAndDispatch()
		}
	}
}

// pollAndDispatch checks for pending tasks and dispatches them to workers.
func (sch *Scheduler) pollAndDispatch() {
	// Check if we have capacity for more workers
	sch.mu.Lock()
	if sch.activeWorkers >= sch.config.GlobalMax {
		sch.mu.Unlock()
		return
	}

	connectorName := sch.connector.Name()
	connectorLimit := sch.config.GetConnectorLimit(connectorName)
	if sch.connectorCounts[connectorName] >= connectorLimit {
		sch.mu.Unlock()
		return
	}
	sch.mu.Unlock()

	// Attempt to atomically claim a task
	workerID := uuid.New().String()
	task, lease, err := sch.store.AtomicClaimTask(workerID, 300)
	if err != nil {
		log.Printf("Error claiming task: %v", err)
		return
	}
	if task == nil {
		// No pending tasks
		return
	}

	// Emit PDR for dispatch
	sch.pdr.Record("task.dispatch", map[string]interface{}{
		"task_id":   task.ID,
		"worker_id": workerID,
		"connector": connectorName,
	}, "success", task.ID, fmt.Sprintf("Dispatched to worker %s", workerID))

	// Route MCPs for this task if router is configured
	if sch.mcpRouter != nil {
		mcpTask := mcp.Task{
			ID:          task.ID,
			Title:       task.Title,
			Description: task.Description,
		}
		result, err := sch.mcpRouter.Route(sch.ctx, mcpTask)
		if err != nil {
			log.Printf("MCP routing error for task %s: %v", task.ID, err)
		} else {
			// Log selected MCPs
			mcpNames := make([]string, len(result.SelectedMCPs))
			for i, m := range result.SelectedMCPs {
				mcpNames[i] = m.Name
			}
			sch.pdr.Record("task.mcp_route", map[string]interface{}{
				"task_id":       task.ID,
				"selected_mcps": mcpNames,
				"total_tools":   result.TotalTools,
				"matched_rules": result.MatchedRules,
			}, "success", task.ID, fmt.Sprintf("Routed to %d MCPs with %d tools", len(mcpNames), result.TotalTools))
			log.Printf("Task %s routed to MCPs: %v (%d tools)", task.ID, mcpNames, result.TotalTools)
		}
	}

	log.Printf("Dispatched task %s (%s) to worker %s", task.ID, task.Title, workerID)

	// Increment worker counts and store worker info
	sch.mu.Lock()
	sch.activeWorkers++
	sch.connectorCounts[connectorName]++
	sch.workers[workerID] = &WorkerInfo{
		WorkerID:      workerID,
		TaskID:        task.ID,
		TaskTitle:     task.Title,
		LeaseID:       lease.ID,
		LeaseExpires:  lease.ExpiresAt,
		StartedAt:     time.Now(),
		ConnectorName: connectorName,
	}
	sch.mu.Unlock()

	// Start worker in goroutine
	sch.wg.Add(1)
	go sch.runWorker(task, lease, workerID)
}

// runWorker executes a task in a worker.
func (sch *Scheduler) runWorker(task *models.Task, lease *models.Lease, workerID string) {
	defer sch.wg.Done()
	defer func() {
		// Decrement worker counts and remove from tracking
		sch.mu.Lock()
		sch.activeWorkers--
		sch.connectorCounts[sch.connector.Name()]--
		delete(sch.workers, workerID)
		sch.mu.Unlock()
	}()

	// If we exit early (cancel/error), make the task claimable again.
	released := false
	defer func() {
		if released {
			if err := sch.store.ReleaseTask(task.ID); err != nil {
				log.Printf("Error releasing task: %v", err)
			}
		}
		if err := sch.store.DeleteLease(lease.ID); err != nil {
			log.Printf("Error deleting lease: %v", err)
		}
	}()

	log.Printf("Worker %s holding task %s (%s)", workerID, task.ID, task.Title)

	select {
	case <-sch.ctx.Done():
		log.Printf("Worker %s interrupted, releasing task %s", workerID, task.ID)
		released = true
		return
	case <-time.After(sch.workerDuration):
		// Work complete
	}

	if err := sch.store.UpdateTaskStatus(task.ID, models.TaskStatusCompleted); err != nil {
		log.Printf("Error completing task %s: %v", task.ID, err)
		released = true
		return
	}

	log.Printf("Worker %s completed task %s", workerID, task.ID)
}

// GetStats returns current scheduler statistics.
func (sch *Scheduler) GetStats() map[string]interface{} {
	sch.mu.Lock()
	defer sch.mu.Unlock()

	connectorCounts := make(map[string]int)
	for k, v := range sch.connectorCounts {
		connectorCounts[k] = v
	}

	// Copy workers list (deep copy to prevent external mutation and data races).
	// The caller will encode this to JSON after the lock is released.
	workers := make([]*WorkerInfo, 0, len(sch.workers))
	for _, w := range sch.workers {
		wCopy := *w
		workers = append(workers, &wCopy)
	}

	return map[string]interface{}{
		"active_workers":   sch.activeWorkers,
		"global_max":       sch.config.GlobalMax,
		"connector_counts": connectorCounts,
		"workers":          workers,
	}
}

// GetWorkers returns a snapshot of all active workers.
func (sch *Scheduler) GetWorkers() []*WorkerInfo {
	sch.mu.Lock()
	defer sch.mu.Unlock()

	workers := make([]*WorkerInfo, 0, len(sch.workers))
	for _, w := range sch.workers {
		// Make a copy to avoid data races
		wCopy := *w
		workers = append(workers, &wCopy)
	}
	return workers
}
