package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/fentz26/neona/internal/update"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Neona to the latest version",
	Long:  `Check for and install the latest version of Neona from GitHub releases.`,
	RunE:  runUpdate,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of Neona",
	Long:  `Display the current version of Neona CLI.`,
	Run:   runVersion,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(versionCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	if err := update.RunSelfUpdate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}

	return nil
}

func runVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("Neona CLI version %s\n", update.GetCurrentVersion())
	fmt.Printf("  OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  Go version: %s\n", runtime.Version())
}
