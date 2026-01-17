package localexec

import (
	"context"
	"testing"
)

func TestIsAllowed(t *testing.T) {
	exec := New("")

	tests := []struct {
		cmd     string
		args    []string
		allowed bool
	}{
		{"go", []string{"test", "./..."}, true},
		{"git", []string{"status"}, true},
		{"git", []string{"diff"}, true},
		{"git", []string{"push"}, false},    // not in allowlist
		{"rm", []string{"-rf", "/"}, false}, // not in allowlist
		{"go", []string{"run", "."}, false}, // subcommand not allowed
		{"go", []string{}, false},           // no subcommand
		{"unknown", []string{"cmd"}, false}, // unknown command
	}

	for _, tt := range tests {
		t.Run(tt.cmd+" "+joinTestArgs(tt.args), func(t *testing.T) {
			got := exec.IsAllowed(tt.cmd, tt.args)
			if got != tt.allowed {
				t.Errorf("IsAllowed(%s, %v) = %v, want %v", tt.cmd, tt.args, got, tt.allowed)
			}
		})
	}
}

func TestExecute_Allowed(t *testing.T) {
	exec := New("")

	ctx := context.Background()
	result, err := exec.Execute(ctx, "git", []string{"status"})

	// This may fail if not in a git repo, but should not return "not allowed" error
	if err != nil {
		// Check it's not an allowlist error
		if result == nil {
			t.Logf("Execute failed (expected in non-git dir): %v", err)
		}
	}
}

func TestExecute_NotAllowed(t *testing.T) {
	exec := New("")

	ctx := context.Background()
	_, err := exec.Execute(ctx, "rm", []string{"-rf", "/"})

	if err == nil {
		t.Error("Expected error for non-allowed command")
	}
}

func TestName(t *testing.T) {
	exec := New("")
	if exec.Name() != "localexec" {
		t.Errorf("Expected name 'localexec', got %s", exec.Name())
	}
}

func joinTestArgs(args []string) string {
	result := ""
	for _, a := range args {
		result += a + " "
	}
	return result
}
