package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive TUI",
	RunE:  runTUI,
}

func runTUI(cmd *cobra.Command, args []string) error {
	// 1. Check if Daemon is running
	if !isDaemonRunning(apiAddr) {
		fmt.Println("⚡ Neona Daemon not running. Starting background service...")
		if err := startDaemon(); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
	}

	// 2. Launch Python TUI (Rich/Textual)
	pythonScript, err := findPythonTUI()
	if err != nil {
		return fmt.Errorf("failed to find neona-tui: %w\nPlease ensure neona-tui is installed or built", err)
	}

	return runPythonTUI(pythonScript)
}

func findPythonTUI() (string, error) {
	// 1. Check for local dev environment relative to CWD
	cwd, err := os.Getwd()
	if err == nil {
		// Try ./neona-tui/.venv/bin/neona-tui
		localPath := filepath.Join(cwd, "neona-tui", ".venv", "bin", "neona-tui")
		if _, err := os.Stat(localPath); err == nil {
			return localPath, nil
		}
	}

	// 2. Check relative to executable (if running from bin/)
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		siblingPath := filepath.Join(exeDir, "neona-tui", ".venv", "bin", "neona-tui")
		if _, err := os.Stat(siblingPath); err == nil {
			return siblingPath, nil
		}
	}

	// 3. Check for binary in PATH
	if path, err := exec.LookPath("neona-tui"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("could not locate neona-tui executable")
}

func runPythonTUI(path string) error {
	cmd := exec.Command(path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func isDaemonRunning(addr string) bool {
	// Simple health check (timeout 1s)
	client := http.Client{Timeout: 500 * time.Millisecond}
	// We'll just check the root or a health endpoint.
	// Since we don't have a dedicated health endpoint documented, checking root is usually safe if it returns something or even 404 vs connection refused.
	// Actually, let's assume if we can connect, it's up.
	resp, err := client.Get(addr)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return true
}

func startDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	// Start "neona daemon" in background
	cmd := exec.Command(exe, "daemon")
	// Detach process so it survives TUI exit
	configureDaemonProc(cmd)

	// Redirect output to avoiding writing to TUI screen, or log to file?
	// For now, let's silence it or it might mess up the TUI.
	// Ideally log to ~/.neona/daemon.log, but nil is fine for now (goes to /dev/null usually if not set or inherits).
	// Better to explicitly nil stdin/out/err to avoid holding terminal open.
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return err
	}

	// Wait for it to become ready
	fmt.Print("   Waiting for daemon...")
	for i := 0; i < 20; i++ { // Wait up to 5 seconds
		if isDaemonRunning(apiAddr) {
			fmt.Println(" Done.")
			return nil
		}
		time.Sleep(250 * time.Millisecond)
		fmt.Print(".")
	}
	fmt.Println(" Timeout!")
	return fmt.Errorf("daemon started but API not reachable at %s", apiAddr)
}
