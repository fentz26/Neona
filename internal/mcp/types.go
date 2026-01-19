// Package mcp provides the MCP Tool Router for dynamic tool selection.
package mcp

// MCPServer represents a registered MCP server with its tools and metadata.
type MCPServer struct {
	Name       string   `yaml:"name" json:"name"`
	Tools      []Tool   `yaml:"tools" json:"tools"`
	ToolCount  int      `yaml:"tool_count" json:"tool_count"`
	Categories []string `yaml:"categories" json:"categories"`
	Priority   int      `yaml:"priority" json:"priority"`
	Enabled    bool     `yaml:"enabled" json:"enabled"`
}

// Tool represents an individual MCP tool.
type Tool struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Server      string `yaml:"server" json:"server"` // Parent server name
}

// Task represents a task for routing decisions.
type Task struct {
	ID          string
	Title       string
	Description string
}

// RoutingResult contains the result of a routing decision.
type RoutingResult struct {
	Task          Task        `json:"task"`
	SelectedMCPs  []MCPServer `json:"selected_mcps"`
	MatchedRules  []string    `json:"matched_rules"`
	TotalTools    int         `json:"total_tools"`
	FilteredTools int         `json:"filtered_tools"`
}
