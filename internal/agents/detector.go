// Package agents provides detection and management of AI tool connections.
package agents

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Agent represents an AI tool that can connect to Neona
type Agent struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`   // cursor, antigravity, claude, zencoder, custom
	Status       string    `json:"status"` // online, offline, unknown
	Path         string    `json:"path,omitempty"`
	Version      string    `json:"version,omitempty"`
	LastSeen     time.Time `json:"last_seen,omitempty"`
	AutoDetected bool      `json:"auto_detected"`
}

// Detector scans for installed AI tools
type Detector struct {
	agents []Agent
}

// NewDetector creates a new agent detector
func NewDetector() *Detector {
	return &Detector{}
}

// Scan detects installed AI tools
func (d *Detector) Scan() []Agent {
	d.agents = []Agent{}

	// Detect Cursor
	if agent := d.detectCursor(); agent != nil {
		d.agents = append(d.agents, *agent)
	}

	// Detect Claude CLI
	if agent := d.detectClaudeCLI(); agent != nil {
		d.agents = append(d.agents, *agent)
	}

	// Detect AntiGravity (Gemini CLI)
	if agent := d.detectAntiGravity(); agent != nil {
		d.agents = append(d.agents, *agent)
	}

	// Detect Zed
	if agent := d.detectZed(); agent != nil {
		d.agents = append(d.agents, *agent)
	}

	// Detect VS Code with Copilot
	if agent := d.detectVSCodeCopilot(); agent != nil {
		d.agents = append(d.agents, *agent)
	}

	// Detect Windsurf
	if agent := d.detectWindsurf(); agent != nil {
		d.agents = append(d.agents, *agent)
	}

	// Detect Aider
	if agent := d.detectAider(); agent != nil {
		d.agents = append(d.agents, *agent)
	}

	return d.agents
}

// GetAgents returns the detected agents
func (d *Detector) GetAgents() []Agent {
	return d.agents
}

func (d *Detector) detectCursor() *Agent {
	// Check common Cursor paths
	paths := []string{
		"/usr/bin/cursor",
		"/usr/local/bin/cursor",
		filepath.Join(os.Getenv("HOME"), ".local/bin/cursor"),
		filepath.Join(os.Getenv("HOME"), "Applications/Cursor.app"),
		"/Applications/Cursor.app",
	}

	for _, p := range paths {
		if fileExists(p) {
			return &Agent{
				ID:           "cursor",
				Name:         "Cursor",
				Type:         "cursor",
				Status:       "online",
				Path:         p,
				AutoDetected: true,
			}
		}
	}

	// Check if cursor command exists
	if path, err := exec.LookPath("cursor"); err == nil {
		return &Agent{
			ID:           "cursor",
			Name:         "Cursor",
			Type:         "cursor",
			Status:       "online",
			Path:         path,
			AutoDetected: true,
		}
	}

	return nil
}

func (d *Detector) detectClaudeCLI() *Agent {
	// Check for claude CLI
	if path, err := exec.LookPath("claude"); err == nil {
		version := getCommandVersion(path, "--version")
		return &Agent{
			ID:           "claude-cli",
			Name:         "Claude CLI",
			Type:         "claude",
			Status:       "online",
			Path:         path,
			Version:      version,
			AutoDetected: true,
		}
	}

	// Check ~/.claude directory
	claudeDir := filepath.Join(os.Getenv("HOME"), ".claude")
	if fileExists(claudeDir) {
		return &Agent{
			ID:           "claude-cli",
			Name:         "Claude CLI",
			Type:         "claude",
			Status:       "unknown",
			Path:         claudeDir,
			AutoDetected: true,
		}
	}

	return nil
}

func (d *Detector) detectAntiGravity() *Agent {
	// Check for gemini CLI (AntiGravity uses Gemini)
	geminiDir := filepath.Join(os.Getenv("HOME"), ".gemini")
	if fileExists(geminiDir) {
		return &Agent{
			ID:           "antigravity",
			Name:         "AntiGravity (Gemini)",
			Type:         "antigravity",
			Status:       "online",
			Path:         geminiDir,
			AutoDetected: true,
		}
	}

	// Check for gemini command
	if path, err := exec.LookPath("gemini"); err == nil {
		return &Agent{
			ID:           "antigravity",
			Name:         "AntiGravity (Gemini)",
			Type:         "antigravity",
			Status:       "online",
			Path:         path,
			AutoDetected: true,
		}
	}

	return nil
}

func (d *Detector) detectZed() *Agent {
	paths := []string{
		"/usr/bin/zed",
		"/usr/local/bin/zed",
		filepath.Join(os.Getenv("HOME"), ".local/bin/zed"),
		"/Applications/Zed.app",
	}

	for _, p := range paths {
		if fileExists(p) {
			return &Agent{
				ID:           "zed",
				Name:         "Zed Editor",
				Type:         "zed",
				Status:       "online",
				Path:         p,
				AutoDetected: true,
			}
		}
	}

	if path, err := exec.LookPath("zed"); err == nil {
		return &Agent{
			ID:           "zed",
			Name:         "Zed Editor",
			Type:         "zed",
			Status:       "online",
			Path:         path,
			AutoDetected: true,
		}
	}

	return nil
}

func (d *Detector) detectVSCodeCopilot() *Agent {
	// Check for code command
	if path, err := exec.LookPath("code"); err == nil {
		// Check if Copilot extension is installed
		extensionsDir := filepath.Join(os.Getenv("HOME"), ".vscode/extensions")
		if fileExists(extensionsDir) {
			entries, _ := os.ReadDir(extensionsDir)
			for _, e := range entries {
				if strings.Contains(e.Name(), "github.copilot") {
					return &Agent{
						ID:           "vscode-copilot",
						Name:         "VS Code + Copilot",
						Type:         "copilot",
						Status:       "online",
						Path:         path,
						AutoDetected: true,
					}
				}
			}
		}
	}
	return nil
}

func (d *Detector) detectWindsurf() *Agent {
	paths := []string{
		"/usr/bin/windsurf",
		"/usr/local/bin/windsurf",
		filepath.Join(os.Getenv("HOME"), ".local/bin/windsurf"),
		"/Applications/Windsurf.app",
	}

	for _, p := range paths {
		if fileExists(p) {
			return &Agent{
				ID:           "windsurf",
				Name:         "Windsurf",
				Type:         "windsurf",
				Status:       "online",
				Path:         p,
				AutoDetected: true,
			}
		}
	}

	if path, err := exec.LookPath("windsurf"); err == nil {
		return &Agent{
			ID:           "windsurf",
			Name:         "Windsurf",
			Type:         "windsurf",
			Status:       "online",
			Path:         path,
			AutoDetected: true,
		}
	}

	return nil
}

func (d *Detector) detectAider() *Agent {
	if path, err := exec.LookPath("aider"); err == nil {
		version := getCommandVersion(path, "--version")
		return &Agent{
			ID:           "aider",
			Name:         "Aider",
			Type:         "aider",
			Status:       "online",
			Path:         path,
			Version:      version,
			AutoDetected: true,
		}
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func getCommandVersion(cmd string, flag string) string {
	out, err := exec.Command(cmd, flag).Output()
	if err != nil {
		return ""
	}
	version := strings.TrimSpace(string(out))
	// Take first line only
	if idx := strings.Index(version, "\n"); idx > 0 {
		version = version[:idx]
	}
	// Limit length
	if len(version) > 30 {
		version = version[:30]
	}
	return version
}
