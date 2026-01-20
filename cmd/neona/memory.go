package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage memory items",
}

var memoryAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a memory item",
	RunE:  runMemoryAdd,
}

var memoryQueryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query memory items",
	RunE:  runMemoryQuery,
}

var (
	memContent string
	memTags    string
	memTaskID  string
	memQuery   string
)

func init() {
	memoryCmd.AddCommand(memoryAddCmd, memoryQueryCmd)

	memoryAddCmd.Flags().StringVar(&memContent, "content", "", "Memory content (required)")
	memoryAddCmd.Flags().StringVar(&memTags, "tags", "", "Comma-separated tags")
	memoryAddCmd.Flags().StringVar(&memTaskID, "task", "", "Associated task ID")
	memoryAddCmd.MarkFlagRequired("content")

	memoryQueryCmd.Flags().StringVar(&memQuery, "q", "", "Search query")
}

// MemoryItem represents a memory entry from the API
type MemoryItem struct {
	ID      string `json:"id"`
	TaskID  string `json:"task_id"`
	Content string `json:"content"`
	Tags    string `json:"tags"`
}

func runMemoryAdd(cmd *cobra.Command, args []string) error {
	body := map[string]string{
		"content": memContent,
		"tags":    memTags,
		"task_id": memTaskID,
	}

	resp, err := apiPost("/memory", body)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	fmt.Printf("Created memory item: %s\n", result["id"])
	return nil
}

func runMemoryQuery(cmd *cobra.Command, args []string) error {
	url := "/memory"
	if memQuery != "" {
		url += "?q=" + memQuery
	}

	resp, err := apiGet(url)
	if err != nil {
		return err
	}

	if len(resp) == 0 {
		fmt.Println("No memory items found")
		return nil
	}

	// Parse JSON response using standard library
	var items []MemoryItem
	if err := json.Unmarshal(resp, &items); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(items) == 0 {
		fmt.Println("No memory items found")
		return nil
	}

	// Optional: search filter for priority match
	if memQuery != "" {
		queryLower := strings.ToLower(memQuery)
		for i, item := range items {
			if strings.Contains(strings.ToLower(item.Content), queryLower) ||
				strings.Contains(strings.ToLower(item.Tags), queryLower) {
				fmt.Printf("Search result match found at index %d\n\n", i)
				break
			}
		}
	}

	// Output results in table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTASK\tCONTENT\tTAGS")

	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			truncateID(item.ID),
			truncateID(item.TaskID),
			truncate(item.Content, 50),
			item.Tags)
	}
	w.Flush()
	return nil
}
