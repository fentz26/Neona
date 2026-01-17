package controlplane

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fentz26/neona/internal/audit"
	"github.com/fentz26/neona/internal/connectors/localexec"
	"github.com/fentz26/neona/internal/store"
)

func TestHealthEndpoint_OK(t *testing.T) {
	s, cleanup := newTestServer(t)
	defer cleanup()

	// Create a test request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	// Call the handler
	s.handleHealth(w, req)

	// Check response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !health.OK {
		t.Error("Expected health.OK to be true")
	}
	if health.DB != "ok" {
		t.Errorf("Expected DB status 'ok', got '%s'", health.DB)
	}
	if health.Version == "" {
		t.Error("Expected version to be set")
	}
	if health.Time == "" {
		t.Error("Expected time to be set")
	}
}

func TestHealthEndpoint_MethodNotAllowed(t *testing.T) {
	s, cleanup := newTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", resp.StatusCode)
	}
}

func TestHealthEndpoint_DBError(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	pdr := audit.NewPDRWriter(st)
	workDir, _ := os.Getwd()
	connector := localexec.New(workDir)
	service := NewService(st, pdr, connector)
	server := NewServer(service, st, "127.0.0.1:0")

	// Close the store to simulate DB error
	st.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if health.OK {
		t.Error("Expected health.OK to be false when DB is down")
	}
	if health.DB == "ok" {
		t.Error("Expected DB status to indicate error")
	}
}

func newTestServer(t *testing.T) (*Server, func()) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	pdr := audit.NewPDRWriter(st)
	workDir, _ := os.Getwd()
	connector := localexec.New(workDir)
	service := NewService(st, pdr, connector)
	server := NewServer(service, st, "127.0.0.1:0")

	cleanup := func() {
		st.Close()
	}

	return server, cleanup
}
