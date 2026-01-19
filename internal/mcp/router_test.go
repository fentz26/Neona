package mcp

import (
	"context"
	"testing"
)

func TestKeywordRouter_BasicRouting(t *testing.T) {
	cfg := DefaultConfig()
	reg := NewRegistry()
	reg.RegisterDefaults()

	router := NewRouter(cfg, reg)

	tests := []struct {
		name        string
		taskTitle   string
		expectMCPs  []string
		description string
	}{
		{
			name:        "GitHub PR task",
			taskTitle:   "Create a GitHub PR for the feature branch",
			expectMCPs:  []string{"github", "git", "filesystem"},
			description: "",
		},
		{
			name:        "Vercel deployment",
			taskTitle:   "Deploy the app to vercel production",
			expectMCPs:  []string{"vercel", "git", "filesystem"},
			description: "",
		},
		{
			name:        "Database query",
			taskTitle:   "Query the postgres database for users",
			expectMCPs:  []string{"database", "filesystem"},
			description: "",
		},
		{
			name:        "Browser scraping",
			taskTitle:   "Scrape the web page for data",
			expectMCPs:  []string{"browser", "filesystem"},
			description: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{
				ID:          "test-1",
				Title:       tt.taskTitle,
				Description: tt.description,
			}

			result, err := router.Route(context.Background(), task)
			if err != nil {
				t.Fatalf("Route() error = %v", err)
			}

			// Check that expected MCPs are included
			selectedNames := make(map[string]bool)
			for _, mcp := range result.SelectedMCPs {
				selectedNames[mcp.Name] = true
			}

			for _, expected := range tt.expectMCPs {
				if !selectedNames[expected] {
					t.Errorf("Expected MCP %q to be selected, got: %v", expected, result.SelectedMCPs)
				}
			}
		})
	}
}

func TestKeywordRouter_MaxTools(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxToolsPerTask = 30 // Low budget

	reg := NewRegistry()
	reg.RegisterDefaults()

	router := NewRouter(cfg, reg)

	// Task that would match many MCPs
	task := Task{
		ID:    "test-1",
		Title: "Deploy github vercel database browser all the things",
	}

	result, err := router.Route(context.Background(), task)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	// Should respect tool budget
	if result.FilteredTools > cfg.MaxToolsPerTask+20 { // Allow some overflow for always-on
		t.Errorf("Tool budget exceeded: got %d, max was %d", result.FilteredTools, cfg.MaxToolsPerTask)
	}
}

func TestKeywordRouter_AlwaysOn(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AlwaysOn = []string{"filesystem", "terminal"}

	reg := NewRegistry()
	reg.RegisterDefaults()

	router := NewRouter(cfg, reg)

	// Task with no keyword matches
	task := Task{
		ID:    "test-1",
		Title: "Do something random",
	}

	result, err := router.Route(context.Background(), task)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	// Always-on MCPs should be included
	found := make(map[string]bool)
	for _, mcp := range result.SelectedMCPs {
		found[mcp.Name] = true
	}

	for _, expected := range cfg.AlwaysOn {
		if !found[expected] {
			t.Errorf("Always-on MCP %q should be selected", expected)
		}
	}
}

func TestKeywordRouter_Override(t *testing.T) {
	cfg := DefaultConfig()
	reg := NewRegistry()
	reg.RegisterDefaults()

	router := NewRouter(cfg, reg)
	overrideRouter := router.Override([]string{"slack", "browser"})

	task := Task{
		ID:    "test-1",
		Title: "Create a GitHub PR", // Would normally match github
	}

	result, err := overrideRouter.Route(context.Background(), task)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	// Should only include overridden MCPs
	selectedNames := make(map[string]bool)
	for _, mcp := range result.SelectedMCPs {
		selectedNames[mcp.Name] = true
	}

	if selectedNames["github"] {
		t.Error("GitHub should not be selected when override is active")
	}

	if len(result.MatchedRules) != 1 || result.MatchedRules[0] != "override" {
		t.Errorf("Expected matched_rules to be ['override'], got %v", result.MatchedRules)
	}
}

func TestKeywordRouter_Disabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false

	reg := NewRegistry()
	reg.RegisterDefaults()

	router := NewRouter(cfg, reg)

	task := Task{
		ID:    "test-1",
		Title: "Any task",
	}

	result, err := router.Route(context.Background(), task)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	// When disabled, should return all enabled MCPs
	if len(result.SelectedMCPs) != len(reg.GetEnabled()) {
		t.Errorf("Disabled router should return all enabled MCPs, got %d vs %d",
			len(result.SelectedMCPs), len(reg.GetEnabled()))
	}
}

func TestRegistry_BasicOperations(t *testing.T) {
	reg := NewRegistry()

	// Register a server
	server := MCPServer{
		Name:      "test-server",
		ToolCount: 10,
		Priority:  50,
		Enabled:   true,
	}

	if err := reg.Register(server); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Get the server
	got, ok := reg.Get("test-server")
	if !ok {
		t.Fatal("Get() should find registered server")
	}
	if got.Name != "test-server" {
		t.Errorf("Got wrong server name: %s", got.Name)
	}

	// Disable the server
	if err := reg.Disable("test-server"); err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	got, _ = reg.Get("test-server")
	if got.Enabled {
		t.Error("Server should be disabled")
	}

	// Enable the server
	if err := reg.Enable("test-server"); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}

	got, _ = reg.Get("test-server")
	if !got.Enabled {
		t.Error("Server should be enabled")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid max tools",
			cfg: &Config{
				MaxToolsPerTask: 0,
				Strategy:        "keywords",
			},
			wantErr: true,
		},
		{
			name: "invalid strategy",
			cfg: &Config{
				MaxToolsPerTask: 50,
				Strategy:        "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
