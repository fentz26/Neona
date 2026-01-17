// Package connectors defines the connector interface for Neona.
package connectors

import "context"

// ExecResult holds the result of a command execution.
type ExecResult struct {
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	ExitCode int      `json:"exit_code"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
}

// Connector defines the interface for executing commands.
type Connector interface {
	// Name returns the connector identifier.
	Name() string

	// Execute runs a command and returns the result.
	Execute(ctx context.Context, cmd string, args []string) (*ExecResult, error)

	// IsAllowed checks if a command is allowed to execute.
	IsAllowed(cmd string, args []string) bool
}
