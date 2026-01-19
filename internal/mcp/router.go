package mcp

import (
	"context"
	"regexp"
	"sort"
	"strings"
)

// Router provides the interface for MCP tool routing.
type Router interface {
	// Route returns the MCPs to expose for a given task.
	Route(ctx context.Context, task Task) (*RoutingResult, error)
	// GetToolManifest returns the filtered tool list.
	GetToolManifest(mcps []MCPServer) []Tool
	// Override allows manual MCP selection.
	Override(mcps []string) Router
}

// KeywordRouter implements keyword-based routing.
type KeywordRouter struct {
	config    *Config
	registry  *Registry
	overrides []string
}

// NewRouter creates a new keyword-based MCP router.
func NewRouter(cfg *Config, reg *Registry) *KeywordRouter {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if reg == nil {
		reg = NewRegistry()
		reg.RegisterDefaults()
	}

	return &KeywordRouter{
		config:   cfg,
		registry: reg,
	}
}

// Route determines which MCPs to expose for a given task.
func (r *KeywordRouter) Route(ctx context.Context, task Task) (*RoutingResult, error) {
	if !r.config.Enabled {
		// Router disabled, return all enabled MCPs
		return &RoutingResult{
			Task:         task,
			SelectedMCPs: r.registry.GetEnabled(),
			TotalTools:   r.registry.TotalToolCount(),
		}, nil
	}

	// If overrides are set, use them directly
	if len(r.overrides) > 0 {
		return r.routeWithOverrides(task)
	}

	// Combine title and description for matching
	text := strings.ToLower(task.Title + " " + task.Description)

	// Find matching rules
	matchedMCPs := make(map[string]bool)
	matchedRules := []string{}

	// Always include "always-on" MCPs
	for _, name := range r.config.AlwaysOn {
		if !r.config.IsAlwaysOff(name) {
			matchedMCPs[name] = true
		}
	}

	// Apply keyword rules
	for _, rule := range r.config.Rules {
		if r.matchesRule(text, rule) {
			matchedRules = append(matchedRules, strings.Join(rule.Keywords, ","))
			for _, enable := range rule.Enable {
				// Expand groups
				expanded := r.config.ExpandGroup(enable)
				for _, name := range expanded {
					if !r.config.IsAlwaysOff(name) {
						matchedMCPs[name] = true
					}
				}
			}
		}
	}

	// If no rules matched, include high-priority defaults
	if len(matchedMCPs) == 0 {
		for _, mcp := range r.registry.GetEnabled() {
			if mcp.Priority >= 80 && !r.config.IsAlwaysOff(mcp.Name) {
				matchedMCPs[mcp.Name] = true
			}
		}
	}

	// Build selected MCPs list
	selectedMCPs := r.buildMCPList(matchedMCPs)

	// Apply tool budget
	selectedMCPs, totalTools, filteredTools := r.applyToolBudget(selectedMCPs)

	return &RoutingResult{
		Task:          task,
		SelectedMCPs:  selectedMCPs,
		MatchedRules:  matchedRules,
		TotalTools:    totalTools,
		FilteredTools: filteredTools,
	}, nil
}

// matchesRule checks if text matches a routing rule.
func (r *KeywordRouter) matchesRule(text string, rule RoutingRule) bool {
	// Check pattern first if specified
	if rule.Pattern != "" {
		matched, err := regexp.MatchString(rule.Pattern, text)
		if err == nil && matched {
			return true
		}
	}

	// Check keywords with word boundary matching
	for _, keyword := range rule.Keywords {
		if containsWord(text, strings.ToLower(keyword)) {
			return true
		}
	}

	return false
}

// containsWord checks if text contains keyword as a whole word.
func containsWord(text, keyword string) bool {
	// For multi-word keywords like "pull request", use simple contains
	if strings.Contains(keyword, " ") {
		return strings.Contains(text, keyword)
	}

	// For single words, check word boundaries
	words := strings.Fields(text)
	for _, word := range words {
		// Clean punctuation from word
		cleaned := strings.Trim(word, ".,;:!?\"'()[]{}")
		if cleaned == keyword {
			return true
		}
	}
	return false
}

// routeWithOverrides returns MCPs based on manual overrides.
func (r *KeywordRouter) routeWithOverrides(task Task) (*RoutingResult, error) {
	matchedMCPs := make(map[string]bool)

	for _, name := range r.overrides {
		if !r.config.IsAlwaysOff(name) {
			matchedMCPs[name] = true
		}
	}

	selectedMCPs := r.buildMCPList(matchedMCPs)

	totalTools := 0
	for _, mcp := range selectedMCPs {
		totalTools += mcp.ToolCount
	}

	return &RoutingResult{
		Task:          task,
		SelectedMCPs:  selectedMCPs,
		MatchedRules:  []string{"override"},
		TotalTools:    totalTools,
		FilteredTools: totalTools,
	}, nil
}

// buildMCPList converts a map of matched names to a sorted list of MCPs.
func (r *KeywordRouter) buildMCPList(matched map[string]bool) []MCPServer {
	mcps := make([]MCPServer, 0, len(matched))

	for name := range matched {
		if mcp, ok := r.registry.Get(name); ok && mcp.Enabled {
			mcps = append(mcps, *mcp)
		}
	}

	// Sort by priority descending
	sort.Slice(mcps, func(i, j int) bool {
		return mcps[i].Priority > mcps[j].Priority
	})

	return mcps
}

// applyToolBudget enforces the max tools per task limit.
func (r *KeywordRouter) applyToolBudget(mcps []MCPServer) ([]MCPServer, int, int) {
	totalTools := 0
	for _, mcp := range mcps {
		totalTools += mcp.ToolCount
	}

	if totalTools <= r.config.MaxToolsPerTask {
		return mcps, totalTools, totalTools
	}

	// Need to trim - keep highest priority MCPs within budget
	filtered := make([]MCPServer, 0)
	filteredTools := 0

	for _, mcp := range mcps {
		if filteredTools+mcp.ToolCount <= r.config.MaxToolsPerTask {
			filtered = append(filtered, mcp)
			filteredTools += mcp.ToolCount
		} else if r.config.IsAlwaysOn(mcp.Name) {
			// Always-on MCPs are included even if over budget
			filtered = append(filtered, mcp)
			filteredTools += mcp.ToolCount
		}
	}

	return filtered, totalTools, filteredTools
}

// GetToolManifest returns the filtered tool list from selected MCPs.
func (r *KeywordRouter) GetToolManifest(mcps []MCPServer) []Tool {
	tools := make([]Tool, 0)

	for _, mcp := range mcps {
		for _, tool := range mcp.Tools {
			tool.Server = mcp.Name
			tools = append(tools, tool)
		}
	}

	return tools
}

// Override returns a new router with manual MCP overrides.
func (r *KeywordRouter) Override(mcps []string) Router {
	return &KeywordRouter{
		config:    r.config,
		registry:  r.registry,
		overrides: mcps,
	}
}

// GetConfig returns the router's configuration.
func (r *KeywordRouter) GetConfig() *Config {
	return r.config
}

// GetRegistry returns the router's registry.
func (r *KeywordRouter) GetRegistry() *Registry {
	return r.registry
}
