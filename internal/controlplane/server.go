package controlplane

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fentz26/neona/internal/models"
	"github.com/fentz26/neona/internal/store"
)

// Version is set at build time or defaults to "dev".
var Version = "dev"

// SchedulerStatsProvider provides scheduler statistics for the /workers endpoint.
type SchedulerStatsProvider interface {
	GetStats() map[string]interface{}
}

// Server provides the HTTP API for Neona.
type Server struct {
	service   *Service
	store     *store.Store
	addr      string
	server    *http.Server
	scheduler SchedulerStatsProvider
}

// NewServer creates a new HTTP server.
func NewServer(service *Service, s *store.Store, addr string) *Server {
	return &Server{
		service: service,
		store:   s,
		addr:    addr,
	}
}

// SetScheduler sets the scheduler stats provider for the /workers endpoint.
// Must be called before Start() - not safe for concurrent use.
func (s *Server) SetScheduler(sched SchedulerStatsProvider) {
	s.scheduler = sched
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Task endpoints
	mux.HandleFunc("/tasks", s.handleTasks)
	mux.HandleFunc("/tasks/", s.handleTaskByID)

	// Memory endpoints
	mux.HandleFunc("/memory", s.handleMemory)

	// Worker pool monitor endpoint
	mux.HandleFunc("/workers", s.handleWorkers)

	// Health check with DB ping
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Printf("Starting Neona daemon on %s", s.addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// HealthResponse represents the /health endpoint response.
type HealthResponse struct {
	OK      bool   `json:"ok"`
	DB      string `json:"db"`
	Version string `json:"version"`
	Time    string `json:"time"`
}

// handleHealth handles GET /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp := HealthResponse{
		OK:      true,
		DB:      "ok",
		Version: Version,
		Time:    time.Now().UTC().Format(time.RFC3339),
	}

	// Perform lightweight DB ping
	if err := s.store.Ping(ctx); err != nil {
		log.Printf("health check: database ping failed: %v", err)
		resp.OK = false
		resp.DB = "unavailable"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(resp)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// handleTasks handles POST /tasks and GET /tasks
func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createTask(w, r)
	case http.MethodGet:
		s.listTasks(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTaskByID handles /tasks/{id}/*
func (s *Server) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/tasks/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "task id required", http.StatusBadRequest)
		return
	}

	taskID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case action == "" && r.Method == http.MethodGet:
		s.getTask(w, r, taskID)
	case action == "claim" && r.Method == http.MethodPost:
		s.claimTask(w, r, taskID)
	case action == "release" && r.Method == http.MethodPost:
		s.releaseTask(w, r, taskID)
	case action == "run" && r.Method == http.MethodPost:
		s.runTask(w, r, taskID)
	case action == "logs" && r.Method == http.MethodGet:
		s.getTaskLogs(w, r, taskID)
	case action == "memory" && r.Method == http.MethodGet:
		s.getTaskMemory(w, r, taskID)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// handleMemory handles POST /memory and GET /memory
func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.addMemory(w, r)
	case http.MethodGet:
		s.queryMemory(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- Task Handlers ---

type createTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	task, err := s.service.CreateTask(req.Title, req.Description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	tasks, err := s.service.ListTasks(status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if tasks == nil {
		tasks = []models.Task{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request, taskID string) {
	task, err := s.service.GetTask(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if task == nil {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

type claimRequest struct {
	HolderID string `json:"holder_id"`
	TTLSec   int    `json:"ttl_sec"`
}

func (s *Server) claimTask(w http.ResponseWriter, r *http.Request, taskID string) {
	var req claimRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if req.TTLSec == 0 {
		req.TTLSec = 300 // default 5 minutes
	}

	lease, err := s.service.ClaimTask(taskID, req.HolderID, req.TTLSec)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrAlreadyClaimed {
			status = http.StatusConflict
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(lease)
}

type releaseRequest struct {
	HolderID string `json:"holder_id"`
}

func (s *Server) releaseTask(w http.ResponseWriter, r *http.Request, taskID string) {
	var req releaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := s.service.ReleaseTask(taskID, req.HolderID); err != nil {
		status := http.StatusInternalServerError
		if err == ErrNotOwner || err == ErrNoLease {
			status = http.StatusForbidden
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"released"}`))
}

type runRequest struct {
	HolderID string   `json:"holder_id"`
	Command  string   `json:"command"`
	Args     []string `json:"args"`
}

func (s *Server) runTask(w http.ResponseWriter, r *http.Request, taskID string) {
	var req runRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	run, err := s.service.RunTask(taskID, req.HolderID, req.Command, req.Args)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrNotOwner {
			status = http.StatusForbidden
		}
		http.Error(w, err.Error(), status)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}

func (s *Server) getTaskLogs(w http.ResponseWriter, r *http.Request, taskID string) {
	runs, err := s.service.GetTaskLogs(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if runs == nil {
		runs = []models.Run{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runs)
}

func (s *Server) getTaskMemory(w http.ResponseWriter, r *http.Request, taskID string) {
	items, err := s.service.GetTaskMemory(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if items == nil {
		items = []models.MemoryItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// --- Memory Handlers ---

type addMemoryRequest struct {
	TaskID  string `json:"task_id"`
	Content string `json:"content"`
	Tags    string `json:"tags"`
}

func (s *Server) addMemory(w http.ResponseWriter, r *http.Request) {
	var req addMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	item, err := s.service.AddMemory(req.TaskID, req.Content, req.Tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item)
}

func (s *Server) queryMemory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	items, err := s.service.QueryMemory(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if items == nil {
		items = []models.MemoryItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// --- Worker Pool Handlers ---

// handleWorkers handles GET /workers
func (s *Server) handleWorkers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.scheduler == nil {
		// Return empty response if scheduler not configured
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"active_workers":   0,
			"global_max":       0,
			"connector_counts": map[string]int{},
			"workers":          []interface{}{},
		})
		return
	}

	stats := s.scheduler.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
