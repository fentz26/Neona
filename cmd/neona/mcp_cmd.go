package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fentz26/neona/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP servers and routing",
	Long:  `Manage Model Context Protocol (MCP) servers and configure dynamic tool routing.`,
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available MCP servers",
	RunE:  runMCPList,
}

var mcpEnableCmd = &cobra.Command{
	Use:   "enable <server>",
	Short: "Enable an MCP server",
	Args:  cobra.ExactArgs(1),
	RunE:  runMCPEnable,
}

var mcpDisableCmd = &cobra.Command{
	Use:   "disable <server>",
	Short: "Disable an MCP server",
	Args:  cobra.ExactArgs(1),
	RunE:  runMCPDisable,
}

var mcpRouteCmd = &cobra.Command{
	Use:   "route <task-description>",
	Short: "Preview which MCPs would be selected for a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runMCPRoute,
}

var mcpConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Show current MCP router configuration",
	RunE:  runMCPConfig,
}

var (
	mcpOverride string
)

func init() {
	mcpCmd.AddCommand(mcpListCmd, mcpEnableCmd, mcpDisableCmd, mcpRouteCmd, mcpConfigCmd)

	mcpRouteCmd.Flags().StringVar(&mcpOverride, "mcp", "", "Override MCP selection (comma-separated)")
}

func getMCPRouter() (*mcp.KeywordRouter, error) {
	cfg, err := mcp.LoadConfigFromHome()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	reg := mcp.NewRegistry()
	reg.RegisterDefaults()

	// Apply config enable/disable preferences to registry for consistent behavior.
	for _, name := range cfg.AlwaysOff {
		_ = reg.Disable(name)
	}
	for _, name := range cfg.AlwaysOn {
		_ = reg.Enable(name)
	}

	return mcp.NewRouter(cfg, reg), nil
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func removeString(xs []string, s string) []string {
	out := xs[:0]
	for _, x := range xs {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

func runMCPList(cmd *cobra.Command, args []string) error {
	router, err := getMCPRouter()
	if err != nil {
		return err
	}

	servers := router.GetRegistry().List()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tTOOLS\tPRIORITY\tENABLED\tCATEGORIES")

	for _, s := range servers {
		enabled := "✓"
		if !s.Enabled {
			enabled = "✗"
		}
		categories := strings.Join(s.Categories, ", ")
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\n", s.Name, s.ToolCount, s.Priority, enabled, categories)
	}

	w.Flush()

	fmt.Printf("\nTotal: %d servers, %d tools\n", router.GetRegistry().Count(), router.GetRegistry().TotalToolCount())
	return nil
}

func runMCPEnable(cmd *cobra.Command, args []string) error {
	router, err := getMCPRouter()
	if err != nil {
		return err
	}

	name := args[0]
	if err := router.GetRegistry().Enable(name); err != nil {
		return err
	}

	// Persist enable by removing from always_off.
	cfg := router.GetConfig()
	cfg.AlwaysOff = removeString(cfg.AlwaysOff, name)
	if err := mcp.SaveConfigToHome(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("✓ Enabled MCP server: %s\n", name)
	return nil
}

func runMCPDisable(cmd *cobra.Command, args []string) error {
	router, err := getMCPRouter()
	if err != nil {
		return err
	}

	name := args[0]
	if err := router.GetRegistry().Disable(name); err != nil {
		return err
	}

	// Persist disable by adding to always_off (and ensuring always_on doesn't conflict).
	cfg := router.GetConfig()
	cfg.AlwaysOn = removeString(cfg.AlwaysOn, name)
	if !containsString(cfg.AlwaysOff, name) {
		cfg.AlwaysOff = append(cfg.AlwaysOff, name)
	}
	if err := mcp.SaveConfigToHome(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("✗ Disabled MCP server: %s\n", name)
	return nil
}

func runMCPRoute(cmd *cobra.Command, args []string) error {
	router, err := getMCPRouter()
	if err != nil {
		return err
	}

	// Apply override if specified
	var r mcp.Router = router
	if mcpOverride != "" {
		overrides := strings.Split(mcpOverride, ",")
		for i := range overrides {
			overrides[i] = strings.TrimSpace(overrides[i])
		}
		r = router.Override(overrides)
	}

	task := mcp.Task{
		ID:    "preview",
		Title: args[0],
	}

	result, err := r.Route(context.Background(), task)
	if err != nil {
		return err
	}

	fmt.Printf("Task: %s\n\n", task.Title)

	fmt.Println("Selected MCPs:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, s := range result.SelectedMCPs {
		fmt.Fprintf(w, "  ✓ %s\t(%d tools)\n", s.Name, s.ToolCount)
	}
	w.Flush()

	if len(result.MatchedRules) > 0 {
		fmt.Printf("\nMatched rules: %s\n", strings.Join(result.MatchedRules, ", "))
	}

	fmt.Printf("\nTool budget: %d/%d", result.FilteredTools, router.GetConfig().MaxToolsPerTask)
	if result.FilteredTools < result.TotalTools {
		fmt.Printf(" (trimmed from %d)", result.TotalTools)
	}
	fmt.Println()

	return nil
}

func runMCPConfig(cmd *cobra.Command, args []string) error {
	router, err := getMCPRouter()
	if err != nil {
		return err
	}

	cfg := router.GetConfig()

	fmt.Println("MCP Router Configuration")
	fmt.Println("========================")
	fmt.Printf("Enabled:  %t\n", cfg.Enabled)
	fmt.Printf("Strategy: %s\n", cfg.Strategy)
	fmt.Printf("Max Tools Per Task: %d\n", cfg.MaxToolsPerTask)

	fmt.Println("\nAlways On:")
	for _, name := range cfg.AlwaysOn {
		fmt.Printf("  - %s\n", name)
	}

	if len(cfg.AlwaysOff) > 0 {
		fmt.Println("\nAlways Off:")
		for _, name := range cfg.AlwaysOff {
			fmt.Printf("  - %s\n", name)
		}
	}

	fmt.Println("\nRouting Rules:")
	for _, rule := range cfg.Rules {
		fmt.Printf("  - Keywords: %s\n", strings.Join(rule.Keywords, ", "))
		fmt.Printf("    Enable:   %s\n", strings.Join(rule.Enable, ", "))
	}

	return nil
}
