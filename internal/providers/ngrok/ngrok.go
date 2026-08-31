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

	// Try different installation methods
	installMethods := []struct {
		name string
		cmd  string
		args []string
	}{
		// Direct binary download (Linux amd64)
		{"binary", "bash", []string{"-c", "curl -fsSL https://bin.equinox.io/c/bNyj1mQVY4c/ngrok-v3-stable-linux-amd64.tgz | tar -xz -C /tmp && sudo mv /tmp/ngrok /usr/local/bin/ngrok && chmod +x /usr/local/bin/ngrok"}},
		// apt (via snap)
		{"snap", "sudo", []string{"snap", "install", "ngrok"}},
		// Homebrew (macOS)
		{"brew", "brew", []string{"install", "ngrok/ngrok/ngrok"}},
	}

	var lastErr error
	for _, method := range installMethods {
		cmd := exec.Command(method.cmd, method.args...)
		if err := cmd.Run(); err != nil {
			lastErr = err
			continue
		}
		// Verify installation
		if n.IsInstalled() {
			return nil
		}
	}

	if lastErr != nil {
		return fmt.Errorf("installation failed: %w", lastErr)
	}
	return fmt.Errorf("installation failed: unknown error")
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
	start := time.Now()

	if !n.IsInstalled() {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "not_installed",
			Message:   "ngrok is not installed",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Check if ngrok process is running
	connected := n.IsConnected()
	if !connected {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "disconnected",
			Message:   "ngrok tunnel is not active",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Get tunnel info to verify actual HTTP reachability
	info, err := n.GetConnectionInfo()
	if err != nil {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "error",
			Message:   fmt.Sprintf("Failed to get tunnel info: %v", err),
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	if info.TunnelURL == "" {
		return &providers.HealthStatus{
			Healthy:   false,
			Status:    "no_tunnel",
			Message:   "ngrok process is running but no tunnel URL found",
			LastCheck: time.Now(),
			Latency:   time.Since(start),
		}, nil
	}

	// Test HTTP reachability to the tunnel URL
	timeout := 5 * time.Second
	client := &http.Client{
		Timeout: timeout,
		// Don't follow redirects - we just want to verify the tunnel is reachable
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Make a simple HTTP request to verify tunnel is responding
	testURL := info.TunnelURL
	if strings.HasPrefix(testURL, "tcp://") {
		// For TCP tunnels, we need to test the port directly
		// Extract host:port from tcp://host:port
		parts := strings.TrimPrefix(testURL, "tcp://")
		timeout := 2 * time.Second
		conn, err := net.DialTimeout("tcp", parts, timeout)
		if err != nil {
			return &providers.HealthStatus{
				Healthy:   false,
				Status:    "unreachable",
				Message:   fmt.Sprintf("ngrok TCP tunnel at %s is not reachable: %v", testURL, err),
				LastCheck: time.Now(),
				Latency:   time.Since(start),
			}, nil
		}
		conn.Close()
	} else {
		// For HTTP/HTTPS tunnels, make an HTTP request
		resp, err := client.Get(testURL)
		if err != nil {
			return &providers.HealthStatus{
				Healthy:   false,
				Status:    "unreachable",
				Message:   fmt.Sprintf("ngrok tunnel at %s is not reachable: %v", testURL, err),
				LastCheck: time.Now(),
				Latency:   time.Since(start),
			}, nil
		}
		resp.Body.Close()
	}

	return &providers.HealthStatus{
		Healthy:   true,
		Status:    "connected",
		Message:   fmt.Sprintf("ngrok tunnel is reachable at %s", info.TunnelURL),
		LastCheck: time.Now(),
		Latency:   time.Since(start),
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
