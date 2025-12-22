package ngrok

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/jedarden/tunnel/internal/providers"
)

// NgrokProvider implements the Provider interface for ngrok
type NgrokProvider struct {
	*providers.BaseProvider
	apiURL string
}

// New creates a new ngrok provider
func New() *NgrokProvider {
	return &NgrokProvider{
		BaseProvider: providers.NewBaseProvider("ngrok", providers.CategoryTunnel),
		apiURL:       "http://localhost:4040/api",
	}
}

// Install installs ngrok
func (n *NgrokProvider) Install() error {
	if n.IsInstalled() {
		return providers.ErrAlreadyInstalled
	}
	return fmt.Errorf("please install ngrok manually from https://ngrok.com/download")
}

// Uninstall uninstalls ngrok
func (n *NgrokProvider) Uninstall() error {
	if !n.IsInstalled() {
		return providers.ErrNotInstalled
	}
	return fmt.Errorf("please uninstall ngrok manually")
}

// IsInstalled checks if ngrok is installed
func (n *NgrokProvider) IsInstalled() bool {
	cmd := exec.Command("ngrok", "version")
	err := cmd.Run()
	return err == nil
}

// Connect establishes an ngrok tunnel
func (n *NgrokProvider) Connect() error {
	if !n.IsInstalled() {
		return providers.ErrNotInstalled
	}

	config, err := n.GetConfig()
	if err != nil {
		return err
	}

	// Set auth token if provided
	if config.AuthToken != "" {
		cmd := exec.Command("ngrok", "config", "add-authtoken", config.AuthToken)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set auth token: %w", err)
		}
	}

	// Default to port 22 for SSH if not specified
	port := config.LocalPort
	if port == 0 {
		port = 22
	}

	// Start ngrok TCP tunnel in background
	args := []string{"tcp", fmt.Sprintf("%d", port), "--log", "stdout"}
	cmd := exec.Command("ngrok", args...)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %v", providers.ErrConnectionFailed, err)
	}

	// Wait for ngrok to start
	time.Sleep(3 * time.Second)

	return nil
}

// Disconnect terminates the ngrok tunnel
func (n *NgrokProvider) Disconnect() error {
	if !n.IsInstalled() {
		return providers.ErrNotInstalled
	}

	// Kill ngrok process
	cmd := exec.Command("pkill", "-f", "ngrok tcp")
	_ = cmd.Run() // Ignore errors if no process found

	return nil
}

// IsConnected checks if ngrok is connected
func (n *NgrokProvider) IsConnected() bool {
	cmd := exec.Command("pgrep", "-f", "ngrok tcp")
	err := cmd.Run()
	return err == nil
}

// GetConnectionInfo retrieves current connection information
func (n *NgrokProvider) GetConnectionInfo() (*providers.ConnectionInfo, error) {
	if !n.IsInstalled() {
		return nil, providers.ErrNotInstalled
	}

	info := &providers.ConnectionInfo{
		Status: "disconnected",
		Extra:  make(map[string]interface{}),
	}

	if !n.IsConnected() {
		return info, nil
	}

	info.Status = "connected"

	// Query ngrok API for tunnel information
	tunnels, err := n.getTunnels()
	if err != nil {
		return info, nil
	}

	if len(tunnels) > 0 {
		tunnel := tunnels[0]
		info.TunnelURL = tunnel.PublicURL
		info.Extra["name"] = tunnel.Name
		info.Extra["proto"] = tunnel.Proto

		// Extract host and port from public URL
		// e.g., tcp://0.tcp.ngrok.io:12345
		if strings.HasPrefix(tunnel.PublicURL, "tcp://") {
			parts := strings.Split(strings.TrimPrefix(tunnel.PublicURL, "tcp://"), ":")
			if len(parts) == 2 {
				info.RemoteIP = parts[0]
			}
		}
	}

	return info, nil
}

// HealthCheck performs a health check
func (n *NgrokProvider) HealthCheck() (*providers.HealthStatus, error) {
	if !n.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "ngrok is not installed",
			LastCheck: time.Now(),
		}, nil
	}

	connected := n.IsConnected()
	status := "disconnected"
	message := "ngrok is not running"

	if connected {
		status = "connected"
		message = "ngrok tunnel is active"

		// Try to get tunnel info
		info, err := n.GetConnectionInfo()
		if err == nil && info.TunnelURL != "" {
			message = fmt.Sprintf("ngrok tunnel active at %s", info.TunnelURL)
		}
	}

	return &providers.HealthStatus{
		Healthy:   connected,
		Status:    status,
		Message:   message,
		LastCheck: time.Now(),
	}, nil
}

// GetLogs retrieves logs since the specified time
func (n *NgrokProvider) GetLogs(since time.Time) ([]providers.LogEntry, error) {
	if !n.IsInstalled() {
		return []providers.LogEntry{}, nil
	}

	var logs []providers.LogEntry

	// ngrok typically logs to ~/.ngrok2/ngrok.log
	homeDir := ""
	cmd := exec.Command("sh", "-c", "echo $HOME")
	if output, err := cmd.Output(); err == nil {
		homeDir = strings.TrimSpace(string(output))
	}

	if homeDir == "" {
		return []providers.LogEntry{}, nil
	}

	logFile := homeDir + "/.ngrok2/ngrok.log"

	// Try to read the log file
	cmd = exec.Command("tail", "-n", "100", logFile)
	output, err := cmd.Output()
	if err != nil {
		// If log file doesn't exist or can't be read, return empty array
		return []providers.LogEntry{}, nil
	}

	// Parse ngrok log format
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// ngrok log format: lvl=info msg="..." t=2006-01-02T15:04:05-0700
		var timestamp time.Time
		var level string
		var message string

		// Extract timestamp
		if idx := strings.Index(line, "t="); idx != -1 {
			timeStr := line[idx+2:]
			if spaceIdx := strings.Index(timeStr, " "); spaceIdx != -1 {
				timeStr = timeStr[:spaceIdx]
			}
			if ts, err := time.Parse(time.RFC3339, timeStr); err == nil {
				timestamp = ts
			}
		}

		// Extract level
		if idx := strings.Index(line, "lvl="); idx != -1 {
			levelStr := line[idx+4:]
			if spaceIdx := strings.Index(levelStr, " "); spaceIdx != -1 {
				levelStr = levelStr[:spaceIdx]
			}
			switch strings.ToLower(levelStr) {
			case "error", "eror", "err":
				level = "Error"
			case "warning", "warn":
				level = "Warning"
			case "info":
				level = "Info"
			default:
				level = "Info"
			}
		}

		// Extract message
		if idx := strings.Index(line, "msg=\""); idx != -1 {
			msgStart := idx + 5
			msgEnd := strings.Index(line[msgStart:], "\"")
			if msgEnd != -1 {
				message = line[msgStart : msgStart+msgEnd]
			}
		}

		// Fallback: use entire line as message if parsing fails
		if message == "" {
			message = line
		}

		if timestamp.IsZero() {
			timestamp = time.Now()
		}

		if level == "" {
			level = "Info"
		}

		// Filter by time
		if timestamp.Before(since) {
			continue
		}

		logs = append(logs, providers.LogEntry{
			Timestamp: timestamp,
			Level:     level,
			Message:   message,
			Source:    "ngrok",
		})
	}

	// Limit to last 100 entries
	if len(logs) > 100 {
		logs = logs[len(logs)-100:]
	}

	return logs, nil
}

// ValidateConfig validates ngrok-specific configuration
func (n *NgrokProvider) ValidateConfig(config *providers.ProviderConfig) error {
	if err := n.BaseProvider.ValidateConfig(config); err != nil {
		return err
	}
	// AuthToken is optional for free tier with limits
	return nil
}

// NgrokTunnel represents a tunnel from the ngrok API
type NgrokTunnel struct {
	Name      string `json:"name"`
	PublicURL string `json:"public_url"`
	Proto     string `json:"proto"`
	Config    struct {
		Addr string `json:"addr"`
	} `json:"config"`
}

// NgrokAPIResponse represents the response from ngrok's local API
type NgrokAPIResponse struct {
	Tunnels []NgrokTunnel `json:"tunnels"`
}

// getTunnels retrieves active tunnels from ngrok's local API
func (n *NgrokProvider) getTunnels() ([]NgrokTunnel, error) {
	resp, err := http.Get(n.apiURL + "/tunnels")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp NgrokAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	return apiResp.Tunnels, nil
}
