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
	"github.com/fentz26/neona/internal/models"
	"github.com/fentz26/neona/internal/store"
	"github.com/google/uuid"
)

// Scheduler manages task dispatching and worker pools.
type Scheduler struct {
	store     *store.Store
	pdr       *audit.PDRWriter
	connector connectors.Connector
	config    *Config
	
	// Worker pool state
	mu              sync.Mutex
	activeWorkers   int
	connectorCounts map[string]int
	
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
		ctx:             ctx,
		cancel:          cancel,
		workerDuration:  5 * time.Second, // Default duration
	}
}

// Start begins the scheduler loop.
func (sch *Scheduler) Start() {
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
	
	log.Printf("Dispatched task %s (%s) to worker %s", task.ID, task.Title, workerID)
	
	// Increment worker counts
	sch.mu.Lock()
	sch.activeWorkers++
	sch.connectorCounts[connectorName]++
	sch.mu.Unlock()
	
	// Start worker in goroutine
	sch.wg.Add(1)
	go sch.runWorker(task, lease, workerID)
}

// runWorker executes a task in a worker.
func (sch *Scheduler) runWorker(task *models.Task, lease *models.Lease, workerID string) {
	defer sch.wg.Done()
	defer func() {
		// Decrement worker counts
		sch.mu.Lock()
		sch.activeWorkers--
		sch.connectorCounts[sch.connector.Name()]--
		sch.mu.Unlock()
	}()
	
	// Always release the lease and task on exit to prevent permanently claimed tasks
	defer func() {
		if err := sch.store.DeleteLease(lease.ID); err != nil {
			log.Printf("Error deleting lease: %v", err)
		}
		if err := sch.store.ReleaseTask(task.ID); err != nil {
			log.Printf("Error releasing task: %v", err)
		}
	}()
	
	// For now, workers just hold the claim without executing
	// In a real implementation, this would execute the task via the connector
	log.Printf("Worker %s holding task %s (%s)", workerID, task.ID, task.Title)
	
	// Simulate some work
	select {
	case <-sch.ctx.Done():
		log.Printf("Worker %s interrupted, releasing task %s", workerID, task.ID)
		return
	case <-time.After(sch.workerDuration):
		// Work complete
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
	
	return map[string]interface{}{
		"active_workers":   sch.activeWorkers,
		"global_max":       sch.config.GlobalMax,
		"connector_counts": connectorCounts,
	}
}
