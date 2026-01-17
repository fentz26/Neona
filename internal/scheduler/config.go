// Package scheduler provides task dispatching with worker pool management.
package scheduler

// Config defines the scheduler configuration.
type Config struct {
	// GlobalMax is the maximum number of concurrent workers across all connectors.
	GlobalMax int `yaml:"global_max"`
	// ByConnector defines per-connector concurrency limits.
	ByConnector map[string]int `yaml:"by_connector"`
}

// DefaultConfig returns the default scheduler configuration.
func DefaultConfig() *Config {
	return &Config{
		GlobalMax: 10,
		ByConnector: map[string]int{
			"localexec": 5,
		},
	}
}

// GetConnectorLimit returns the concurrency limit for a connector.
func (c *Config) GetConnectorLimit(connectorName string) int {
	if limit, ok := c.ByConnector[connectorName]; ok {
		return limit
	}
	// Default limit if not specified
	return 1
}
