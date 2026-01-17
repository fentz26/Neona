package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/fentz26/neona/internal/tui"
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

	// 2. Launch TUI
	app := tui.New(apiAddr)
	if err := app.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
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
