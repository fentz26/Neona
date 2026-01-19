package mcp

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds MCP router configuration.
type Config struct {
	// Enabled toggles the MCP router on/off.
	Enabled bool `yaml:"enabled"`
	// Strategy determines routing approach: auto, keywords, manual.
	Strategy string `yaml:"strategy"`
	// MaxToolsPerTask is the tool budget per task.
	MaxToolsPerTask int `yaml:"max_tools_per_task"`
	// Priority assigns importance scores to MCP servers (higher = more likely to include).
	Priority map[string]int `yaml:"priority"`
	// Groups define named collections of MCP servers.
	Groups map[string][]string `yaml:"groups"`
	// AlwaysOn lists MCPs that are always included.
	AlwaysOn []string `yaml:"always_on"`
	// AlwaysOff lists MCPs that are never included.
	AlwaysOff []string `yaml:"always_off"`
	// Rules define keyword-based routing rules.
	Rules []RoutingRule `yaml:"rules"`
}

// RoutingRule defines a keyword-based routing rule.
type RoutingRule struct {
	// Keywords trigger this rule when found in task description.
	Keywords []string `yaml:"keywords"`
	// Enable specifies which MCPs or groups to enable.
	Enable []string `yaml:"enable"`
	// Pattern is an optional regex pattern for matching.
	Pattern string `yaml:"pattern,omitempty"`
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		Strategy:        "keywords",
		MaxToolsPerTask: 80,
		Priority: map[string]int{
			"filesystem": 100,
			"git":        90,
			"github":     80,
			"vercel":     70,
			"database":   60,
			"browser":    50,
			"slack":      30,
		},
		Groups: map[string][]string{
			"development": {"filesystem", "git", "github", "terminal"},
			"deployment":  {"vercel", "cloudflare", "git"},
			"data":        {"database", "filesystem"},
			"research":    {"browser", "search", "filesystem"},
		},
		AlwaysOn:  []string{"filesystem"},
		AlwaysOff: []string{},
		Rules: []RoutingRule{
			{
				Keywords: []string{"github", "pr", "pull request", "issue", "repository"},
				Enable:   []string{"github", "git", "filesystem"},
			},
			{
				Keywords: []string{"deploy", "vercel", "production", "preview"},
				Enable:   []string{"vercel", "git", "filesystem"},
			},
			{
				Keywords: []string{"database", "sql", "query", "postgres"},
				Enable:   []string{"database", "filesystem"},
			},
			{
				Keywords: []string{"browser", "web", "screenshot", "scrape"},
				Enable:   []string{"browser", "filesystem"},
			},
		},
	}
}

// LoadConfig loads configuration from a YAML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// LoadConfigFromHome loads configuration from ~/.neona/mcp.yaml.
func LoadConfigFromHome() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultConfig(), nil
	}

	path := filepath.Join(home, ".neona", "mcp.yaml")
	return LoadConfig(path)
}

// SaveConfig saves configuration to a YAML file, creating parent directories if needed.
func SaveConfig(path string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}

// SaveConfigToHome saves configuration to ~/.neona/mcp.yaml.
func SaveConfigToHome(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}
	path := filepath.Join(home, ".neona", "mcp.yaml")
	return SaveConfig(path, cfg)
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if c.MaxToolsPerTask < 1 {
		return fmt.Errorf("max_tools_per_task must be at least 1")
	}

	validStrategies := map[string]bool{
		"auto":     true,
		"keywords": true,
		"manual":   true,
	}
	if !validStrategies[c.Strategy] {
		return fmt.Errorf("invalid strategy %q, must be: auto, keywords, or manual", c.Strategy)
	}

	return nil
}

// GetPriority returns the priority for an MCP server (higher = more important).
func (c *Config) GetPriority(name string) int {
	if p, ok := c.Priority[name]; ok {
		return p
	}
	return 50 // Default priority
}

// IsAlwaysOn checks if an MCP is in the always-on list.
func (c *Config) IsAlwaysOn(name string) bool {
	for _, n := range c.AlwaysOn {
		if n == name {
			return true
		}
	}
	return false
}

// IsAlwaysOff checks if an MCP is in the always-off list.
func (c *Config) IsAlwaysOff(name string) bool {
	for _, n := range c.AlwaysOff {
		if n == name {
			return true
		}
	}
	return false
}

// ExpandGroup expands a group name to its member MCPs.
func (c *Config) ExpandGroup(name string) []string {
	if members, ok := c.Groups[name]; ok {
		return members
	}
	// Not a group, return as-is
	return []string{name}
}
