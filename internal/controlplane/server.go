package controlplane

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fentz26/neona/internal/models"
)

// Server provides the HTTP API for Neona.
type Server struct {
	service *Service
	addr    string
	server  *http.Server
}

// NewServer creates a new HTTP server.
func NewServer(service *Service, addr string) *Server {
	return &Server{
		service: service,
		addr:    addr,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Task endpoints
	mux.HandleFunc("/tasks", s.handleTasks)
	mux.HandleFunc("/tasks/", s.handleTaskByID)

	// Memory endpoints
	mux.HandleFunc("/memory", s.handleMemory)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

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
