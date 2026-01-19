package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fentz26/neona/internal/audit"
	"github.com/fentz26/neona/internal/connectors/localexec"
	"github.com/fentz26/neona/internal/controlplane"
	"github.com/fentz26/neona/internal/mcp"
	"github.com/fentz26/neona/internal/scheduler"
	"github.com/fentz26/neona/internal/store"
	"github.com/spf13/cobra"
)

var (
	listenAddr string
	dbPath     string
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the Neona daemon (neonad)",
	Long:  `Starts the Neona daemon which provides the HTTP API for task coordination.`,
	RunE:  runDaemon,
}

func init() {
	homeDir, _ := os.UserHomeDir()
	defaultDB := filepath.Join(homeDir, ".neona", "neona.db")

	daemonCmd.Flags().StringVar(&listenAddr, "listen", "127.0.0.1:7466", "Listen address for the API server")
	daemonCmd.Flags().StringVar(&dbPath, "db", defaultDB, "Path to SQLite database")
}

func runDaemon(cmd *cobra.Command, args []string) error {
	log.Println("Starting Neona daemon...")

	// Initialize store
	s, err := store.New(dbPath)
	if err != nil {
		return err
	}

	// Initialize components
	pdr := audit.NewPDRWriter(s)
	workDir, _ := os.Getwd()
	connector := localexec.New(workDir)

	// Create service and server
	service := controlplane.NewService(s, pdr, connector)
	server := controlplane.NewServer(service, s, listenAddr)

	// Create and start scheduler
	schedulerCfg := scheduler.DefaultConfig()
	sched := scheduler.New(s, pdr, connector, schedulerCfg)

	// Initialize MCP router
	mcpConfig, err := mcp.LoadConfigFromHome()
	if err != nil {
		log.Printf("Warning: failed to load MCP config: %v (using defaults)", err)
		mcpConfig = mcp.DefaultConfig()
	}
	registry := mcp.NewRegistry()
	registry.RegisterDefaults()
	mcpRouter := mcp.NewRouter(mcpConfig, registry)
	log.Printf("MCP router initialized with %d servers", registry.Count())

	// Wire MCP router to scheduler and server
	sched.SetMCPRouter(mcpRouter)
	server.SetMCPRouter(mcpRouter)

	// Wire scheduler to server for /workers endpoint
	server.SetScheduler(sched)

	sched.Start()
	defer sched.Stop()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Channel to receive server errors
	serverErr := make(chan error, 1)

	// Start server in goroutine
	go func() {
		err := server.Start()
		if err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigCh:
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
	case err := <-serverErr:
		if err != nil {
			log.Printf("Server error: %v", err)
			s.Close()
			return err
		}
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	log.Println("Shutting down HTTP server...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Closing database connection...")
	if err := s.Close(); err != nil {
		log.Printf("Database close error: %v", err)
	}

	log.Println("Shutdown complete")
	return nil
}
