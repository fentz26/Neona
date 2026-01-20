package main

import (
	"fmt"
	"os"

	"github.com/fentz26/neona/internal/update"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "neona",
	Short: "Neona - AI Control Plane CLI",
	Long:  `Neona is a CLI-centric AI Control Plane that coordinates multiple AI tools under shared rules, knowledge, and policy.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip update check for certain commands
		skipCommands := map[string]bool{
			"update":    true,
			"version":   true,
			"uninstall": true,
			"help":      true,
		}

		if skipCommands[cmd.Name()] {
			return
		}

		// Check for updates (blocking with spinner)
		updated, err := update.CheckAndAutoUpdate()
		if err == nil && updated {
			if err := restartSelf(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to restart: %v\n", err)
				os.Exit(1)
			}
		}
	},
	// No RunE - defaults to showing help when no subcommand is provided
}

var (
	apiAddr string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&apiAddr, "api", "http://127.0.0.1:7466", "API server address")

	// Add subcommands
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(memoryCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(mcpCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
