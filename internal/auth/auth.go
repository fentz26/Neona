// Package auth provides authentication functionality for the Neona CLI.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const (
	// DefaultCallbackPort is the default port for the local callback server.
	DefaultCallbackPort = 17890
	// AuthTimeout is the maximum time to wait for authentication.
	AuthTimeout = 5 * time.Minute
	// DefaultAuthURL is the Neona website auth URL.
	DefaultAuthURL = "https://neona.app/auth/cli/"
)

// User represents the authenticated user.
type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

// Session represents an authentication session.
type Session struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	User         User   `json:"user"`
}

// Credentials stores the complete auth credentials.
type Credentials struct {
	Session   Session `json:"session"`
	CreatedAt int64   `json:"created_at"`
}

// AuthResult is returned from the authentication flow.
type AuthResult struct {
	Session Session
	Error   error
}

// Manager handles authentication operations.
type Manager struct {
	configDir   string
	authURL     string
	credentials *Credentials
	mu          sync.RWMutex
}

// NewManager creates a new auth manager.
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "neona")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	m := &Manager{
		configDir: configDir,
		authURL:   DefaultAuthURL,
	}

	// Try to load existing credentials
	_ = m.loadCredentials()

	return m, nil
}

// IsAuthenticated checks if the user is currently authenticated.
func (m *Manager) IsAuthenticated() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.credentials == nil {
		return false
	}

	// Check if token is expired (with 5 minute buffer)
	expiresAt := time.Unix(m.credentials.Session.ExpiresAt, 0)
	return time.Now().Before(expiresAt.Add(-5 * time.Minute))
}

// GetUser returns the current user if authenticated.
func (m *Manager) GetUser() *User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.credentials == nil {
		return nil
	}
	return &m.credentials.Session.User
}

// GetSession returns the current session if authenticated.
func (m *Manager) GetSession() *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.credentials == nil {
		return nil
	}
	return &m.credentials.Session
}

// Login initiates the OAuth login flow.
func (m *Manager) Login(ctx context.Context) (*Session, error) {
	// Find an available port
	port, err := findAvailablePort(DefaultCallbackPort)
	if err != nil {
		return nil, fmt.Errorf("failed to find available port: %w", err)
	}

	// Generate a random state for CSRF protection
	state := generateState()

	// Create result channel
	resultCh := make(chan AuthResult, 1)

	// Start callback server
	server, err := startCallbackServer(port, state, resultCh)
	if err != nil {
		return nil, fmt.Errorf("failed to start callback server: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	// Build auth URL
	authURL := fmt.Sprintf("%s?port=%d&state=%s", m.authURL, port, state)

	// Open browser
	if err := openBrowser(authURL); err != nil {
		return nil, fmt.Errorf("failed to open browser: %w\nPlease open this URL manually: %s", err, authURL)
	}

	// Wait for callback or timeout
	select {
	case result := <-resultCh:
		if result.Error != nil {
			return nil, result.Error
		}

		// Save credentials
		m.mu.Lock()
		m.credentials = &Credentials{
			Session:   result.Session,
			CreatedAt: time.Now().Unix(),
		}
		m.mu.Unlock()

		if err := m.saveCredentials(); err != nil {
			return nil, fmt.Errorf("failed to save credentials: %w", err)
		}

		return &result.Session, nil

	case <-time.After(AuthTimeout):
		return nil, fmt.Errorf("authentication timed out after %v", AuthTimeout)

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Logout clears the current session.
func (m *Manager) Logout() error {
	m.mu.Lock()
	m.credentials = nil
	m.mu.Unlock()

	credPath := filepath.Join(m.configDir, "credentials.json")
	if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove credentials: %w", err)
	}

	return nil
}

// credentialsPath returns the path to the credentials file.
func (m *Manager) credentialsPath() string {
	return filepath.Join(m.configDir, "credentials.json")
}

// loadCredentials loads credentials from disk.
func (m *Manager) loadCredentials() error {
	data, err := os.ReadFile(m.credentialsPath())
	if err != nil {
		return err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return err
	}

	m.mu.Lock()
	m.credentials = &creds
	m.mu.Unlock()

	return nil
}

// saveCredentials saves credentials to disk.
func (m *Manager) saveCredentials() error {
	m.mu.RLock()
	creds := m.credentials
	m.mu.RUnlock()

	if creds == nil {
		return nil
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.credentialsPath(), data, 0600)
}

// CallbackData represents the data received from the browser callback.
type CallbackData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	User         User   `json:"user"`
	State        string `json:"state"`
}

// startCallbackServer starts a local HTTP server to receive the OAuth callback.
func startCallbackServer(port int, expectedState string, resultCh chan<- AuthResult) (*http.Server, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Enable CORS for browser requests
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var data CallbackData
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			resultCh <- AuthResult{Error: fmt.Errorf("invalid callback data: %w", err)}
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		// Verify state to prevent CSRF
		if data.State != expectedState {
			resultCh <- AuthResult{Error: fmt.Errorf("state mismatch: possible CSRF attack")}
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		}

		// Send success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

		// Send result
		resultCh <- AuthResult{
			Session: Session{
				AccessToken:  data.AccessToken,
				RefreshToken: data.RefreshToken,
				ExpiresAt:    data.ExpiresAt,
				User:         data.User,
			},
		}
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			resultCh <- AuthResult{Error: fmt.Errorf("callback server error: %w", err)}
		}
	}()

	return server, nil
}

// findAvailablePort finds an available port starting from the given port.
func findAvailablePort(startPort int) (int, error) {
	for port := startPort; port < startPort+100; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", startPort, startPort+100)
}

// generateState generates a random state string for CSRF protection.
func generateState() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() % 256)
		time.Sleep(time.Nanosecond)
	}
	return fmt.Sprintf("%x", b)
}

// openBrowser opens the default browser with the given URL.
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
