package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// DefaultClientTimeout is the default timeout for API requests.
const DefaultClientTimeout = 10 * time.Second

// Client wraps HTTP calls to the Neona API
type Client struct {
	baseURL    string
	holderID   string
	httpClient *http.Client
}

// NewClient creates a new API client with timeout
func NewClient(baseURL string) *Client {
	hostname, _ := os.Hostname()
	return &Client{
		baseURL:  baseURL,
		holderID: fmt.Sprintf("tui@%s", hostname),
		httpClient: &http.Client{
			Timeout: DefaultClientTimeout,
		},
	}
}

// ListTasks fetches tasks from the API
func (c *Client) ListTasks(status string) ([]TaskItem, error) {
	url := c.baseURL + "/tasks"
	if status != "" {
		url += "?status=" + status
	}

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var tasks []struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Status    string `json:"status"`
		ClaimedBy string `json:"claimed_by"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, err
	}

	items := make([]TaskItem, len(tasks))
	for i, t := range tasks {
		items[i] = TaskItem{
			ID:        t.ID,
			TaskTitle: t.Title,
			Status:    t.Status,
			ClaimedBy: t.ClaimedBy,
		}
	}
	return items, nil
}

// GetTask fetches a single task
func (c *Client) GetTask(id string) (*TaskDetail, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/tasks/" + id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var task struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Status      string `json:"status"`
		ClaimedBy   string `json:"claimed_by"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&task); err != nil {
		return nil, err
	}

	return &TaskDetail{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		Status:      task.Status,
		ClaimedBy:   task.ClaimedBy,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}, nil
}

// GetTaskLogs fetches run logs for a task
func (c *Client) GetTaskLogs(taskID string) ([]RunDetail, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/tasks/" + taskID + "/logs")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var runs []struct {
		ID       string `json:"id"`
		Command  string `json:"command"`
		ExitCode int    `json:"exit_code"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
		return nil, err
	}

	details := make([]RunDetail, len(runs))
	for i, r := range runs {
		details[i] = RunDetail{
			ID:       r.ID,
			Command:  r.Command,
			ExitCode: r.ExitCode,
			Stdout:   r.Stdout,
			Stderr:   r.Stderr,
		}
	}
	return details, nil
}

// GetTaskMemory fetches memory items for a task
func (c *Client) GetTaskMemory(taskID string) ([]MemoryDetail, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/tasks/" + taskID + "/memory")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var items []struct {
		ID      string `json:"id"`
		Content string `json:"content"`
		Tags    string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	details := make([]MemoryDetail, len(items))
	for i, m := range items {
		details[i] = MemoryDetail{
			ID:      m.ID,
			Content: m.Content,
			Tags:    m.Tags,
		}
	}
	return details, nil
}

// CreateTask creates a new task
func (c *Client) CreateTask(title, description string) (string, error) {
	body := map[string]string{
		"title":       title,
		"description": description,
	}
	resp, err := c.post("/tasks", body)
	if err != nil {
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}
	return result.ID, nil
}

// ClaimTask claims a task
func (c *Client) ClaimTask(taskID string) error {
	body := map[string]interface{}{
		"holder_id": c.holderID,
		"ttl_sec":   300,
	}
	_, err := c.post("/tasks/"+taskID+"/claim", body)
	return err
}

// ReleaseTask releases a task
func (c *Client) ReleaseTask(taskID string) error {
	body := map[string]string{
		"holder_id": c.holderID,
	}
	_, err := c.post("/tasks/"+taskID+"/release", body)
	return err
}

// RunTask runs a command for a task
func (c *Client) RunTask(taskID, command string, args []string) (int, error) {
	body := map[string]interface{}{
		"holder_id": c.holderID,
		"command":   command,
		"args":      args,
	}
	resp, err := c.post("/tasks/"+taskID+"/run", body)
	if err != nil {
		return -1, err
	}

	var result struct {
		ExitCode int `json:"exit_code"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return -1, err
	}
	return result.ExitCode, nil
}

// AddMemory adds a memory item
func (c *Client) AddMemory(taskID, content string) (string, error) {
	body := map[string]string{
		"task_id": taskID,
		"content": content,
		"tags":    "note",
	}
	resp, err := c.post("/memory", body)
	if err != nil {
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}
	return result.ID, nil
}

// QueryMemory searches memory
func (c *Client) QueryMemory(query string) ([]MemoryDetail, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/memory?q=" + url.QueryEscape(query))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var items []struct {
		ID      string `json:"id"`
		Content string `json:"content"`
		Tags    string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, err
	}

	details := make([]MemoryDetail, len(items))
	for i, m := range items {
		details[i] = MemoryDetail{
			ID:      m.ID,
			Content: m.Content,
			Tags:    m.Tags,
		}
	}
	return details, nil
}

func (c *Client) post(path string, data interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(c.baseURL+path, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	return body, nil
}

// CheckHealth checks if the daemon is healthy
func (c *Client) CheckHealth() (bool, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, nil
	}

	var health struct {
		OK bool `json:"ok"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return false, err
	}

	return health.OK, nil
}
