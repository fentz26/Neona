package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultClientTimeout is the default timeout for API requests.
const DefaultClientTimeout = 10 * time.Second

// apiClient is the shared HTTP client with timeout.
var apiClient = &http.Client{
	Timeout: DefaultClientTimeout,
}

// apiGet performs a GET request to the API with timeout.
func apiGet(path string) ([]byte, error) {
	url := apiAddr + path
	resp, err := apiClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// apiPost performs a POST request to the API with timeout.
func apiPost(path string, data interface{}) ([]byte, error) {
	url := apiAddr + path
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	resp, err := apiClient.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// CheckHealth checks if the daemon is healthy and returns the health response.
func CheckHealth() (*HealthResponse, error) {
	resp, err := apiGet("/health")
	if err != nil {
		return nil, err
	}

	var health HealthResponse
	if err := json.Unmarshal(resp, &health); err != nil {
		return nil, fmt.Errorf("failed to parse health response: %w", err)
	}

	return &health, nil
}

// HealthResponse matches the server's health response structure.
type HealthResponse struct {
	OK      bool   `json:"ok"`
	DB      string `json:"db"`
	Version string `json:"version"`
	Time    string `json:"time"`
}
