package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fentz26/neona/internal/auth"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  `Manage authentication with your Neona account.`,
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Sign in to your Neona account",
	Long: `Sign in to your Neona account using browser-based OAuth.

This will open your default browser to complete the authentication flow.
Once authenticated, your CLI will be connected to your Neona account.`,
	RunE: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out of your Neona account",
	Long:  `Sign out and remove stored credentials.`,
	RunE:  runLogout,
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Display current user information",
	Long:  `Show information about the currently authenticated user.`,
	RunE:  runWhoami,
}

func init() {
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(whoamiCmd)

	// Add neona login as an alias
	rootCmd.AddCommand(authCmd)

	// Also add login directly to root for convenience
	directLoginCmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in to your Neona account",
		Long:  `Sign in to your Neona account using browser-based OAuth.`,
		RunE:  runLogin,
	}
	directLogoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Sign out of your Neona account",
		RunE:  runLogout,
	}
	rootCmd.AddCommand(directLoginCmd)
	rootCmd.AddCommand(directLogoutCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	fmt.Println("🔐 Neona CLI Authentication")
	fmt.Println()

	manager, err := auth.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize auth: %w", err)
	}

	// Check if already authenticated
	if manager.IsAuthenticated() {
		user := manager.GetUser()
		fmt.Printf("✓ Already signed in as %s (%s)\n", user.Username, user.Email)
		fmt.Println()
		fmt.Println("Use 'neona logout' to sign out, or 'neona auth login' to re-authenticate.")
		return nil
	}

	fmt.Println("Opening browser for authentication...")
	fmt.Println("Please complete the sign-in process in your browser.")
	fmt.Println()
	fmt.Println("Waiting for authentication... (Press Ctrl+C to cancel)")
	fmt.Println()

	// Create context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\nAuthentication cancelled.")
		cancel()
	}()

	// Perform login
	session, err := manager.Login(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil // User cancelled
		}
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Println("┌────────────────────────────────────────────────┐")
	fmt.Println("│                                                │")
	fmt.Printf("│  ✓ Signed in as %-29s │\n", truncateString(session.User.Username, 29))
	fmt.Printf("│    %s%-40s │\n", "📧 ", truncateString(session.User.Email, 38))
	fmt.Println("│                                                │")
	fmt.Println("└────────────────────────────────────────────────┘")
	fmt.Println()
	fmt.Println("Your CLI is now connected to your Neona account.")

	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	manager, err := auth.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize auth: %w", err)
	}

	if !manager.IsAuthenticated() {
		fmt.Println("You are not currently signed in.")
		return nil
	}

	user := manager.GetUser()
	if err := manager.Logout(); err != nil {
		return fmt.Errorf("failed to sign out: %w", err)
	}

	fmt.Printf("✓ Signed out from %s\n", user.Username)
	return nil
}

func runWhoami(cmd *cobra.Command, args []string) error {
	manager, err := auth.NewManager()
	if err != nil {
		return fmt.Errorf("failed to initialize auth: %w", err)
	}

	if !manager.IsAuthenticated() {
		fmt.Println("Not signed in.")
		fmt.Println()
		fmt.Println("Use 'neona login' to sign in to your Neona account.")
		return nil
	}

	user := manager.GetUser()
	session := manager.GetSession()

	fmt.Println("┌────────────────────────────────────────────────┐")
	fmt.Println("│              Current User                      │")
	fmt.Println("├────────────────────────────────────────────────┤")
	fmt.Printf("│  Username: %-35s │\n", truncateString(user.Username, 35))
	fmt.Printf("│  Email:    %-35s │\n", truncateString(user.Email, 35))
	fmt.Printf("│  User ID:  %-35s │\n", truncateString(user.ID[:8]+"...", 35))
	fmt.Println("└────────────────────────────────────────────────┘")

	// Show token expiry if available
	if session != nil && session.ExpiresAt > 0 {
		fmt.Println()
		fmt.Printf("Session expires: %s\n", formatExpiry(session.ExpiresAt))
	}

	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatExpiry(expiresAt int64) string {
	if expiresAt == 0 {
		return "unknown"
	}
	// Format as relative time or absolute
	return fmt.Sprintf("Unix timestamp %d", expiresAt)
}
