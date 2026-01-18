package main

/*
#cgo CFLAGS: -O3 -mavx2 -msse4.2
#include "c/memory/prepare.c"
*/
import "C"
import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"unsafe"

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

	// 1. Initialize storage with estimated capacity
	capacity := uint32(len(resp)/60) + 64 
	store := C.memory_store_init(C.uint32_t(capacity))
	defer C.memory_store_free(store)

	// 2. SIMD-accelerated zero-copy JSON parsing
	C.memory_store_parse_json(store, (*C.char)(unsafe.Pointer(&resp[0])), C.uint32_t(len(resp)))

	realCount := uint32(C.memory_get_count(store))
	if realCount == 0 {
		fmt.Println("No memory items found")
		return nil
	}

	// 3. Optional: Lookup table-based search for priority match
	if memQuery != "" {
		cQ := C.CString(memQuery)
		idx := C.memory_store_find(store, cQ, C.uint32_t(len(memQuery)))
		C.free(unsafe.Pointer(cQ))
		if idx != -1 {
			fmt.Printf("Search result match found at index %d\n\n", idx)
		}
	}

	// 4. Output results in table format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTASK\tCONTENT\tTAGS")
	
	for i := uint32(0); i < realCount; i++ {
		// Leverage the Arena pointers directly
		id := C.GoString(C.memory_get_id(store, C.uint32_t(i)))
		tid := C.GoString(C.memory_get_task_id(store, C.uint32_t(i)))
		cont := C.GoString(C.memory_get_content(store, C.uint32_t(i)))
		tags := C.GoString(C.memory_get_tags(store, C.uint32_t(i)))

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", 
			truncateID(id), 
			truncateID(tid), 
			truncate(cont, 50), 
			tags)
	}
	w.Flush()
	return nil
}
