// Package update provides version checking and self-update functionality.
package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	// GitHubRepo is the repository to check for updates.
	GitHubRepo = "Neona-AI/Neona"
	// GitHubAPIURL is the GitHub releases API endpoint.
	GitHubAPIURL = "https://api.github.com/repos/%s/releases/latest"
	// CheckInterval is the minimum time between update checks.
	CheckInterval = 24 * time.Hour
)

// Version is set at build time via -ldflags.
var Version = "0.0.1-beta"

// GitHubRelease represents a GitHub release response.
type GitHubRelease struct {
	TagName     string  `json:"tag_name"`
	Name        string  `json:"name"`
	HTMLURL     string  `json:"html_url"`
	PublishedAt string  `json:"published_at"`
	Assets      []Asset `json:"assets"`
}

// Asset represents a release asset.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// UpdateCache stores the last update check info.
type UpdateCache struct {
	LastCheck     int64  `json:"last_check"`
	LatestVersion string `json:"latest_version"`
	DownloadURL   string `json:"download_url"`
}

// Checker handles update checking and caching.
type Checker struct {
	configDir string
	cache     *UpdateCache
}

// NewChecker creates a new update checker.
func NewChecker() (*Checker, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "neona")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	c := &Checker{
		configDir: configDir,
	}

	// Load existing cache
	_ = c.loadCache()

	return c, nil
}

// GetCurrentVersion returns the current build version.
func GetCurrentVersion() string {
	return Version
}

// ShouldCheck returns true if enough time has passed since the last check.
func (c *Checker) ShouldCheck() bool {
	if c.cache == nil {
		return true
	}

	lastCheck := time.Unix(c.cache.LastCheck, 0)
	return time.Since(lastCheck) > CheckInterval
}

// CheckForUpdate checks GitHub for a newer version.
// Returns (hasUpdate, latestVersion, error).
func (c *Checker) CheckForUpdate() (bool, string, error) {
	// Use /releases endpoint (not /releases/latest) because all our releases are prereleases
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases", GitHubRepo)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, "", fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return false, "", fmt.Errorf("failed to parse release info: %w", err)
	}

	if len(releases) == 0 {
		return false, "", fmt.Errorf("no releases found")
	}

	// Use the first (latest) release
	release := releases[0]

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	currentVersion := strings.TrimPrefix(Version, "v")

	// Find the appropriate asset for this OS/arch
	downloadURL := findAssetURL(release.Assets)

	// Update cache
	c.cache = &UpdateCache{
		LastCheck:     time.Now().Unix(),
		LatestVersion: latestVersion,
		DownloadURL:   downloadURL,
	}
	_ = c.saveCache()

	// Compare versions (simple string comparison, works for semver)
	hasUpdate := latestVersion != currentVersion && currentVersion != "dev"

	return hasUpdate, latestVersion, nil
}

// GetCachedVersion returns the cached latest version if available.
func (c *Checker) GetCachedVersion() (string, bool) {
	if c.cache == nil || c.cache.LatestVersion == "" {
		return "", false
	}
	return c.cache.LatestVersion, true
}

// GetDownloadURL returns the download URL for the latest release.
func (c *Checker) GetDownloadURL() string {
	if c.cache == nil {
		return ""
	}
	return c.cache.DownloadURL
}

// DownloadAndInstall downloads and installs the latest version.
func (c *Checker) DownloadAndInstall() error {
	if c.cache == nil || c.cache.DownloadURL == "" {
		// Try to get fresh release info
		_, _, err := c.CheckForUpdate()
		if err != nil {
			return err
		}
	}

	downloadURL := c.cache.DownloadURL
	if downloadURL == "" {
		return fmt.Errorf("no download URL available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Download to temp file
	// Download to temp file

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "neona-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	_, err = io.Copy(tmpFile, resp.Body)
	tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Get current binary path
	currentBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable: %w", err)
	}
	currentBin, _ = filepath.EvalSymlinks(currentBin)

	// Replace the binary
	// Replace the binary

	// On some systems, we can't replace a running binary directly
	// Use a temporary rename approach
	backupPath := currentBin + ".old"
	os.Remove(backupPath) // Remove old backup if exists

	if err := os.Rename(currentBin, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := copyFile(tmpPath, currentBin); err != nil {
		// Try to restore backup
		os.Rename(backupPath, currentBin)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	os.Remove(backupPath) // Clean up backup

	return nil
}

// cachePath returns the path to the cache file.
func (c *Checker) cachePath() string {
	return filepath.Join(c.configDir, "update_cache.json")
}

// loadCache loads the cache from disk.
func (c *Checker) loadCache() error {
	data, err := os.ReadFile(c.cachePath())
	if err != nil {
		return err
	}

	var cache UpdateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return err
	}

	c.cache = &cache
	return nil
}

// saveCache saves the cache to disk.
func (c *Checker) saveCache() error {
	if c.cache == nil {
		return nil
	}

	data, err := json.MarshalIndent(c.cache, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.cachePath(), data, 0600)
}

// findAssetURL finds the download URL for the current OS/arch.
func findAssetURL(assets []Asset) string {
	os := runtime.GOOS
	arch := runtime.GOARCH

	// Map common arch names
	archAliases := map[string][]string{
		"amd64": {"amd64", "x86_64", "x64"},
		"arm64": {"arm64", "aarch64"},
		"386":   {"386", "i386", "x86"},
	}

	aliases, ok := archAliases[arch]
	if !ok {
		aliases = []string{arch}
	}

	for _, asset := range assets {
		name := strings.ToLower(asset.Name)

		// Check if it matches our OS
		if !strings.Contains(name, os) {
			continue
		}

		// Check if it matches our arch
		for _, alias := range aliases {
			if strings.Contains(name, alias) {
				return asset.BrowserDownloadURL
			}
		}
	}

	return ""
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy permissions
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

// CheckAndAutoUpdate checks for updates and installs if available.
// Returns true if updated (caller should restart).
func CheckAndAutoUpdate() (bool, error) {
	checker, err := NewChecker()
	if err != nil {
		return false, err
	}

	if !checker.ShouldCheck() {
		return false, nil
	}

	s := newSpinner("Checking for update...")
	s.Start()

	// Ensure stopped if we return early
	defer func() {
		if s != nil {
			s.Stop()
		}
	}()

	hasUpdate, latestVersion, err := checker.CheckForUpdate()
	if err != nil {
		return false, err
	}

	if !hasUpdate {
		return false, nil
	}

	s.UpdateMessage(fmt.Sprintf("Found new update: %s", latestVersion))
	time.Sleep(1 * time.Second)

	s.UpdateMessage("Updating...")
	if err := checker.DownloadAndInstall(); err != nil {
		// Stop spinner to print error clearly
		s.Stop()
		s = nil // Prevent defer from stopping again
		fmt.Printf("\rUpdate failed: %v\n", err)
		return false, err
	}

	s.UpdateMessage("Done")
	time.Sleep(500 * time.Millisecond)

	// Stop spinner before returning
	s.Stop()
	s = nil

	return true, nil
}

// Spinner implementation
type spinner struct {
	frames []string
	delay  time.Duration
	stop   chan struct{}
	msg    string
	mu     sync.Mutex
}

func newSpinner(msg string) *spinner {
	return &spinner{
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		delay:  100 * time.Millisecond,
		stop:   make(chan struct{}),
		msg:    msg,
	}
}

func (s *spinner) Start() {
	go func() {
		for {
			for _, f := range s.frames {
				select {
				case <-s.stop:
					return
				default:
					s.mu.Lock()
					msg := s.msg
					s.mu.Unlock()
					fmt.Printf("\r%s %s\033[K", f, msg)
					time.Sleep(s.delay)
				}
			}
		}
	}()
}

func (s *spinner) Stop() {
	close(s.stop)
	fmt.Printf("\r\033[K") // Clear line
}

func (s *spinner) UpdateMessage(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msg = msg
}

// RunSelfUpdate performs the self-update process.
func RunSelfUpdate() error {
	checker, err := NewChecker()
	if err != nil {
		return err
	}

	fmt.Printf("Current version: %s\n", Version)
	fmt.Println("Checking for updates...")

	hasUpdate, latestVersion, err := checker.CheckForUpdate()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !hasUpdate {
		fmt.Println("✓ You are already running the latest version.")
		return nil
	}

	fmt.Printf("New version available: %s\n", latestVersion)

	if err := checker.DownloadAndInstall(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Successfully updated to version %s\n", latestVersion)
	fmt.Println("  Please restart neona to use the new version.")

	// Optionally exec the new version to show it works
	newBin, _ := os.Executable()
	fmt.Println()
	fmt.Println("Verifying installation...")
	cmd := exec.Command(newBin, "version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	return nil
}
