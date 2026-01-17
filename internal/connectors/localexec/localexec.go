// Package localexec provides a local command executor with an allowlist.
package localexec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/fentz26/neona/internal/connectors"
)

// allowedCommands defines the strict allowlist of executable commands.
var allowedCommands = map[string][]string{
	"go":  {"test"},
	"git": {"diff", "status"},
}

// LocalExec implements the Connector interface for local command execution.
type LocalExec struct {
	workDir string
}

// New creates a new LocalExec connector.
func New(workDir string) *LocalExec {
	return &LocalExec{workDir: workDir}
}

// Name returns the connector identifier.
func (l *LocalExec) Name() string {
	return "localexec"
}

// IsAllowed checks if a command is in the allowlist.
func (l *LocalExec) IsAllowed(cmd string, args []string) bool {
	allowedSubcmds, ok := allowedCommands[cmd]
	if !ok {
		return false
	}

	if len(args) == 0 {
		return false
	}

	// Check if the first arg (subcommand) is allowed
	subcmd := args[0]
	for _, allowed := range allowedSubcmds {
		if subcmd == allowed {
			return true
		}
	}
	return false
}

// Execute runs a command if it's in the allowlist.
func (l *LocalExec) Execute(ctx context.Context, cmd string, args []string) (*connectors.ExecResult, error) {
	if !l.IsAllowed(cmd, args) {
		return nil, fmt.Errorf("command not allowed: %s %s", cmd, strings.Join(args, " "))
	}

	execCmd := exec.CommandContext(ctx, cmd, args...)
	if l.workDir != "" {
		execCmd.Dir = l.workDir
	}

	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	err := execCmd.Run()

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			return nil, fmt.Errorf("exec error: %w", err)
		}
	}

	return &connectors.ExecResult{
		Command:  cmd,
		Args:     args,
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}
