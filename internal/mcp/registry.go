package mcp

import (
	"fmt"
	"sort"
	"sync"
)

func cloneServer(s *MCPServer) MCPServer {
	c := *s
	if s.Tools != nil {
		c.Tools = append([]Tool(nil), s.Tools...)
	}
	if s.Categories != nil {
		c.Categories = append([]string(nil), s.Categories...)
	}
	return c
}

// Registry manages registered MCP servers.
type Registry struct {
	servers map[string]*MCPServer
	mu      sync.RWMutex
}

// NewRegistry creates a new MCP server registry.
func NewRegistry() *Registry {
	return &Registry{
		servers: make(map[string]*MCPServer),
	}
}

// Register adds or updates an MCP server in the registry.
func (r *Registry) Register(server MCPServer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if server.Name == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	// Calculate tool count if not provided
	if server.ToolCount == 0 && len(server.Tools) > 0 {
		server.ToolCount = len(server.Tools)
	}

	r.servers[server.Name] = &server
	return nil
}

// Get retrieves an MCP server by name.
func (r *Registry) Get(name string) (*MCPServer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	server, ok := r.servers[name]
	if !ok {
		return nil, false
	}

	// Return a deep copy to prevent external mutation
	copy := cloneServer(server)
	return &copy, true
}

// List returns all registered MCP servers sorted by priority (desc).
func (r *Registry) List() []MCPServer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	servers := make([]MCPServer, 0, len(r.servers))
	for _, s := range r.servers {
		servers = append(servers, cloneServer(s))
	}

	// Sort by priority descending
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Priority > servers[j].Priority
	})

	return servers
}

// Enable enables an MCP server.
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, ok := r.servers[name]
	if !ok {
		return fmt.Errorf("server %q not found", name)
	}

	server.Enabled = true
	return nil
}

// Disable disables an MCP server.
func (r *Registry) Disable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, ok := r.servers[name]
	if !ok {
		return fmt.Errorf("server %q not found", name)
	}

	server.Enabled = false
	return nil
}

// Count returns the number of registered servers.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.servers)
}

// GetEnabled returns only enabled MCP servers.
func (r *Registry) GetEnabled() []MCPServer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	servers := make([]MCPServer, 0)
	for _, s := range r.servers {
		if s.Enabled {
			servers = append(servers, cloneServer(s))
		}
	}

	// Sort by priority descending
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Priority > servers[j].Priority
	})

	return servers
}

// TotalToolCount returns the total number of tools across all enabled servers.
func (r *Registry) TotalToolCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := 0
	for _, s := range r.servers {
		if s.Enabled {
			total += s.ToolCount
		}
	}
	return total
}

// RegisterDefaults registers a set of common MCP servers with estimated tool counts.
func (r *Registry) RegisterDefaults() {
	defaults := []MCPServer{
		{Name: "filesystem", ToolCount: 15, Categories: []string{"core", "files"}, Priority: 100, Enabled: true},
		{Name: "git", ToolCount: 12, Categories: []string{"core", "vcs"}, Priority: 90, Enabled: true},
		{Name: "github", ToolCount: 45, Categories: []string{"vcs", "api"}, Priority: 80, Enabled: true},
		{Name: "vercel", ToolCount: 20, Categories: []string{"deployment", "api"}, Priority: 70, Enabled: true},
		{Name: "database", ToolCount: 25, Categories: []string{"data", "api"}, Priority: 60, Enabled: true},
		{Name: "browser", ToolCount: 18, Categories: []string{"web", "scraping"}, Priority: 50, Enabled: true},
		{Name: "terminal", ToolCount: 8, Categories: []string{"core", "shell"}, Priority: 85, Enabled: true},
		{Name: "search", ToolCount: 5, Categories: []string{"web", "research"}, Priority: 55, Enabled: true},
		{Name: "slack", ToolCount: 15, Categories: []string{"communication", "api"}, Priority: 30, Enabled: false},
		{Name: "cloudflare", ToolCount: 22, Categories: []string{"deployment", "api"}, Priority: 65, Enabled: true},
	}

	for _, s := range defaults {
		r.Register(s)
	}
}
