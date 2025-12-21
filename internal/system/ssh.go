package system

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SSHConfig contains SSH configuration information
type SSHConfig struct {
	Port               int
	ListenAddress      string
	AuthorizedKeysFile string
	SSHDir             string
}

// SSHServerStatus represents the status of the SSH server
type SSHServerStatus struct {
	IsRunning bool
	Port      int
	Process   string
	PID       int
}

// GetSSHServerStatus checks if SSH server is running
func GetSSHServerStatus() (*SSHServerStatus, error) {
	status := &SSHServerStatus{
		IsRunning: false,
	}

	// Try to connect to common SSH ports
	ports := []int{22, 2222}
	for _, port := range ports {
		if TestConnectivity("localhost", port, 2*time.Second) == nil {
			status.IsRunning = true
			status.Port = port
			break
		}
	}

	// Try to get SSH process info
	if pid, process := getSSHProcessInfo(); pid > 0 {
		status.PID = pid
		status.Process = process
		status.IsRunning = true
	}

	return status, nil
}

// getSSHProcessInfo tries to find SSH server process
func getSSHProcessInfo() (int, string) {
	// Try common SSH server process names
	processNames := []string{"sshd", "ssh"}

	for _, name := range processNames {
		cmd := exec.Command("pgrep", "-x", name)
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			var pid int
			fmt.Sscanf(string(output), "%d", &pid)
			return pid, name
		}
	}

	return 0, ""
}

// GetSSHPort attempts to determine the SSH server port
func GetSSHPort() (int, error) {
	// First, try reading from sshd_config
	if port, err := readSSHConfigPort(); err == nil {
		return port, nil
	}

	// Try connecting to common ports
	ports := []int{22, 2222}
	for _, port := range ports {
		if TestConnectivity("localhost", port, 2*time.Second) == nil {
			return port, nil
		}
	}

	return 0, fmt.Errorf("could not determine SSH port")
}

// readSSHConfigPort reads the SSH port from sshd_config
func readSSHConfigPort() (int, error) {
	configPaths := []string{
		"/etc/ssh/sshd_config",
		"/etc/sshd_config",
	}

	for _, path := range configPaths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "Port ") {
				var port int
				fmt.Sscanf(line, "Port %d", &port)
				if port > 0 {
					return port, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("port not found in SSH config")
}

// GetSSHDir returns the SSH directory for the current user
func GetSSHDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	return sshDir, nil
}

// EnsureSSHDir creates the SSH directory if it doesn't exist
func EnsureSSHDir() (string, error) {
	sshDir, err := GetSSHDir()
	if err != nil {
		return "", err
	}

	// Create directory with proper permissions (700)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create SSH directory: %w", err)
	}

	return sshDir, nil
}

// GetAuthorizedKeysFile returns the path to authorized_keys file
func GetAuthorizedKeysFile() (string, error) {
	sshDir, err := GetSSHDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(sshDir, "authorized_keys"), nil
}

// AddPublicKey adds a public key to authorized_keys
func AddPublicKey(publicKey string) error {
	sshDir, err := EnsureSSHDir()
	if err != nil {
		return err
	}

	authKeysFile := filepath.Join(sshDir, "authorized_keys")

	// Check if key already exists
	if exists, err := keyExistsInAuthorizedKeys(authKeysFile, publicKey); err == nil && exists {
		return fmt.Errorf("key already exists in authorized_keys")
	}

	// Open file in append mode
	file, err := os.OpenFile(authKeysFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open authorized_keys: %w", err)
	}
	defer file.Close()

	// Ensure key ends with newline
	if !strings.HasSuffix(publicKey, "\n") {
		publicKey += "\n"
	}

	// Write key
	if _, err := file.WriteString(publicKey); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}

// RemovePublicKey removes a public key from authorized_keys
func RemovePublicKey(publicKey string) error {
	authKeysFile, err := GetAuthorizedKeysFile()
	if err != nil {
		return err
	}

	// Read all keys
	file, err := os.Open(authKeysFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, nothing to remove
		}
		return fmt.Errorf("failed to open authorized_keys: %w", err)
	}
	defer file.Close()

	var keys []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Keep lines that don't match the key to remove
		if !strings.Contains(line, strings.TrimSpace(publicKey)) {
			keys = append(keys, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read authorized_keys: %w", err)
	}

	// Write back
	return os.WriteFile(authKeysFile, []byte(strings.Join(keys, "\n")+"\n"), 0600)
}

// keyExistsInAuthorizedKeys checks if a key exists in authorized_keys
func keyExistsInAuthorizedKeys(authKeysFile, publicKey string) (bool, error) {
	file, err := os.Open(authKeysFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer file.Close()

	publicKey = strings.TrimSpace(publicKey)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), publicKey) {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// ListAuthorizedKeys returns all public keys in authorized_keys
func ListAuthorizedKeys() ([]string, error) {
	authKeysFile, err := GetAuthorizedKeysFile()
	if err != nil {
		return nil, err
	}

	file, err := os.Open(authKeysFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to open authorized_keys: %w", err)
	}
	defer file.Close()

	var keys []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line != "" && !strings.HasPrefix(line, "#") {
			keys = append(keys, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read authorized_keys: %w", err)
	}

	return keys, nil
}

// GenerateSSHConfig generates an SSH config snippet for a tunnel
func GenerateSSHConfig(hostname, user, identityFile string, port int) string {
	config := fmt.Sprintf(`Host %s
    HostName %s
    User %s
    Port %d`,
		hostname, hostname, user, port)

	if identityFile != "" {
		config += fmt.Sprintf("\n    IdentityFile %s", identityFile)
	}

	config += "\n    StrictHostKeyChecking no\n    UserKnownHostsFile /dev/null\n"
	return config
}

// IsSSHServerInstalled checks if SSH server is installed
func IsSSHServerInstalled() bool {
	// Check for sshd binary
	if _, err := exec.LookPath("sshd"); err == nil {
		return true
	}

	// Check for systemd service
	cmd := exec.Command("systemctl", "list-unit-files", "ssh.service")
	if err := cmd.Run(); err == nil {
		return true
	}

	cmd = exec.Command("systemctl", "list-unit-files", "sshd.service")
	if err := cmd.Run(); err == nil {
		return true
	}

	return false
}

// StartSSHServer attempts to start the SSH server
func StartSSHServer() error {
	// Try systemd first
	cmd := exec.Command("systemctl", "start", "ssh")
	if err := cmd.Run(); err == nil {
		return nil
	}

	cmd = exec.Command("systemctl", "start", "sshd")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Try service command
	cmd = exec.Command("service", "ssh", "start")
	if err := cmd.Run(); err == nil {
		return nil
	}

	return fmt.Errorf("failed to start SSH server")
}

// GetSSHFingerprint gets the SSH host key fingerprint
func GetSSHFingerprint() (string, error) {
	keyPaths := []string{
		"/etc/ssh/ssh_host_ed25519_key.pub",
		"/etc/ssh/ssh_host_rsa_key.pub",
		"/etc/ssh/ssh_host_ecdsa_key.pub",
	}

	for _, keyPath := range keyPaths {
		cmd := exec.Command("ssh-keygen", "-lf", keyPath)
		output, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
	}

	return "", fmt.Errorf("could not get SSH fingerprint")
}

// GenerateSSHKeyPair generates a new SSH key pair
func GenerateSSHKeyPair(keyPath, comment string) error {
	sshDir := filepath.Dir(keyPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create SSH directory: %w", err)
	}

	args := []string{
		"-t", "ed25519",
		"-f", keyPath,
		"-N", "", // No passphrase
		"-C", comment,
	}

	cmd := exec.Command("ssh-keygen", args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate SSH key pair: %w", err)
	}

	return nil
}

// ValidateSSHPublicKey validates if a string is a valid SSH public key
func ValidateSSHPublicKey(key string) bool {
	key = strings.TrimSpace(key)

	// Basic validation: should start with known key types
	validPrefixes := []string{"ssh-rsa", "ssh-ed25519", "ecdsa-sha2-"}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}

	return false
}
