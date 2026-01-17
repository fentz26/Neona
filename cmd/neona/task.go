package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks",
}

var taskAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new task",
	RunE:  runTaskAdd,
}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE:  runTaskList,
}

var taskShowCmd = &cobra.Command{
	Use:   "show [task-id]",
	Short: "Show task details",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskShow,
}

var taskClaimCmd = &cobra.Command{
	Use:   "claim [task-id]",
	Short: "Claim a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskClaim,
}

var taskReleaseCmd = &cobra.Command{
	Use:   "release [task-id]",
	Short: "Release a task claim",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskRelease,
}

var taskRunCmd = &cobra.Command{
	Use:   "run [task-id]",
	Short: "Run a command for a task",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskRun,
}

var taskLogCmd = &cobra.Command{
	Use:   "log [task-id]",
	Short: "Show task run logs",
	Args:  cobra.ExactArgs(1),
	RunE:  runTaskLog,
}

var (
	taskTitle  string
	taskDesc   string
	taskStatus string
	holderID   string
	ttlSec     int
	runCommand string
	runArgs    string
)

func init() {
	taskCmd.AddCommand(taskAddCmd, taskListCmd, taskShowCmd, taskClaimCmd, taskReleaseCmd, taskRunCmd, taskLogCmd)

	taskAddCmd.Flags().StringVar(&taskTitle, "title", "", "Task title (required)")
	taskAddCmd.Flags().StringVar(&taskDesc, "desc", "", "Task description")
	taskAddCmd.MarkFlagRequired("title")

	taskListCmd.Flags().StringVar(&taskStatus, "status", "", "Filter by status (pending, claimed, running, completed, failed)")

	hostname, _ := os.Hostname()
	defaultHolder := fmt.Sprintf("cli@%s", hostname)
	taskClaimCmd.Flags().StringVar(&holderID, "holder", defaultHolder, "Holder ID for the lease")
	taskClaimCmd.Flags().IntVar(&ttlSec, "ttl", 300, "Lease TTL in seconds")

	taskReleaseCmd.Flags().StringVar(&holderID, "holder", defaultHolder, "Holder ID")

	taskRunCmd.Flags().StringVar(&holderID, "holder", defaultHolder, "Holder ID")
	taskRunCmd.Flags().StringVar(&runCommand, "cmd", "", "Command to run (e.g., 'git status')")
	taskRunCmd.MarkFlagRequired("cmd")
}

func runTaskAdd(cmd *cobra.Command, args []string) error {
	body := map[string]string{
		"title":       taskTitle,
		"description": taskDesc,
	}

	resp, err := apiPost("/tasks", body)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	fmt.Printf("Created task: %s\n", result["id"])
	return nil
}

func runTaskList(cmd *cobra.Command, args []string) error {
	url := "/tasks"
	if taskStatus != "" {
		url += "?status=" + taskStatus
	}

	resp, err := apiGet(url)
	if err != nil {
		return err
	}

	var tasks []map[string]interface{}
	if err := json.Unmarshal(resp, &tasks); err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tCLAIMED BY")
	for _, t := range tasks {
		id := truncateID(t["id"].(string))
		title := truncate(t["title"].(string), 40)
		status := t["status"].(string)
		claimedBy := ""
		if cb, ok := t["claimed_by"].(string); ok {
			claimedBy = cb
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", id, title, status, claimedBy)
	}
	w.Flush()
	return nil
}

func runTaskShow(cmd *cobra.Command, args []string) error {
	resp, err := apiGet("/tasks/" + args[0])
	if err != nil {
		return err
	}

	var task map[string]interface{}
	if err := json.Unmarshal(resp, &task); err != nil {
		return err
	}

	fmt.Printf("ID:          %s\n", task["id"])
	fmt.Printf("Title:       %s\n", task["title"])
	fmt.Printf("Description: %s\n", task["description"])
	fmt.Printf("Status:      %s\n", task["status"])
	if cb, ok := task["claimed_by"].(string); ok && cb != "" {
		fmt.Printf("Claimed By:  %s\n", cb)
	}
	fmt.Printf("Created:     %s\n", task["created_at"])
	fmt.Printf("Updated:     %s\n", task["updated_at"])

	return nil
}

func runTaskClaim(cmd *cobra.Command, args []string) error {
	body := map[string]interface{}{
		"holder_id": holderID,
		"ttl_sec":   ttlSec,
	}

	resp, err := apiPost("/tasks/"+args[0]+"/claim", body)
	if err != nil {
		return err
	}

	var lease map[string]interface{}
	if err := json.Unmarshal(resp, &lease); err != nil {
		return err
	}

	fmt.Printf("Claimed task %s\n", args[0])
	fmt.Printf("Lease ID: %s\n", lease["id"])
	fmt.Printf("Expires:  %s\n", lease["expires_at"])
	return nil
}

func runTaskRelease(cmd *cobra.Command, args []string) error {
	body := map[string]interface{}{
		"holder_id": holderID,
	}

	_, err := apiPost("/tasks/"+args[0]+"/release", body)
	if err != nil {
		return err
	}

	fmt.Printf("Released task %s\n", args[0])
	return nil
}

func runTaskRun(cmd *cobra.Command, args []string) error {
	// Parse command string into command and args
	parts := strings.Fields(runCommand)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	body := map[string]interface{}{
		"holder_id": holderID,
		"command":   parts[0],
		"args":      parts[1:],
	}

	resp, err := apiPost("/tasks/"+args[0]+"/run", body)
	if err != nil {
		return err
	}

	var run map[string]interface{}
	if err := json.Unmarshal(resp, &run); err != nil {
		return err
	}

	fmt.Printf("Run ID:    %s\n", run["id"])
	fmt.Printf("Exit Code: %.0f\n", run["exit_code"].(float64))
	fmt.Println("\n--- STDOUT ---")
	fmt.Println(run["stdout"])
	if stderr, ok := run["stderr"].(string); ok && stderr != "" {
		fmt.Println("\n--- STDERR ---")
		fmt.Println(stderr)
	}
	return nil
}

func runTaskLog(cmd *cobra.Command, args []string) error {
	resp, err := apiGet("/tasks/" + args[0] + "/logs")
	if err != nil {
		return err
	}

	var runs []map[string]interface{}
	if err := json.Unmarshal(resp, &runs); err != nil {
		return err
	}

	if len(runs) == 0 {
		fmt.Println("No runs found")
		return nil
	}

	for i, run := range runs {
		fmt.Printf("=== Run %d ===\n", i+1)
		fmt.Printf("ID:        %s\n", run["id"])
		fmt.Printf("Command:   %s\n", run["command"])
		fmt.Printf("Exit Code: %.0f\n", run["exit_code"].(float64))
		fmt.Printf("Started:   %s\n", run["started_at"])
		if stdout, ok := run["stdout"].(string); ok && stdout != "" {
			fmt.Println("Stdout:", truncate(stdout, 200))
		}
		fmt.Println()
	}
	return nil
}

// --- Helpers ---

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func truncateID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}
