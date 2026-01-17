package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fentz26/neona/internal/audit"
	"github.com/fentz26/neona/internal/connectors/localexec"
	"github.com/fentz26/neona/internal/controlplane"
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
	defer s.Close()

	// Initialize components
	pdr := audit.NewPDRWriter(s)
	workDir, _ := os.Getwd()
	connector := localexec.New(workDir)

	// Create service and server
	service := controlplane.NewService(s, pdr, connector)
	server := controlplane.NewServer(service, listenAddr)
	
	// Create and start scheduler
	schedulerCfg := scheduler.DefaultConfig()
	sched := scheduler.New(s, pdr, connector, schedulerCfg)
	sched.Start()
	defer sched.Stop()

	// Handle graceful shutdown
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	
	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start()
	}()
	
	// Wait for shutdown signal or server error
	select {
	case <-shutdownCh:
		log.Println("Shutting down...")
		// Gracefully shutdown HTTP server with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
			return err
		}
		log.Println("HTTP server stopped gracefully")
		return nil
	case err := <-serverErr:
		return err
	}
}
