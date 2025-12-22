package system

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// NetworkInfo contains network-related information
type NetworkInfo struct {
	LocalIPs       []string
	PublicIP       string
	Hostname       string
	DefaultGateway string
}

// GetLocalIPs returns all local IP addresses
func GetLocalIPs() ([]string, error) {
	var ips []string

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get interface addresses: %w", err)
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ips = append(ips, ipNet.IP.String())
			}
		}
	}

	return ips, nil
}

// GetPublicIP attempts to get the public IP address
func GetPublicIP() (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Try multiple services
	services := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}

	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			var ip string
			_, _ = fmt.Fscanf(resp.Body, "%s", &ip)
			if net.ParseIP(ip) != nil {
				return ip, nil
			}
		}
	}

	return "", fmt.Errorf("failed to get public IP from any service")
}

// GetHostname returns the system hostname
func GetHostname() (string, error) {
	hostname, err := net.LookupHost("localhost")
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	if len(hostname) > 0 {
		return hostname[0], nil
	}

	return "", fmt.Errorf("no hostname found")
}

// GetNetworkInfo retrieves comprehensive network information
func GetNetworkInfo() (*NetworkInfo, error) {
	info := &NetworkInfo{}

	// Get local IPs
	localIPs, err := GetLocalIPs()
	if err != nil {
		return nil, err
	}
	info.LocalIPs = localIPs

	// Get public IP (non-blocking, use goroutine)
	publicIPChan := make(chan string, 1)
	go func() {
		if ip, err := GetPublicIP(); err == nil {
			publicIPChan <- ip
		} else {
			publicIPChan <- ""
		}
	}()

	// Get hostname
	hostname, err := net.LookupHost("localhost")
	if err == nil && len(hostname) > 0 {
		info.Hostname = hostname[0]
	}

	// Wait for public IP (with timeout)
	select {
	case ip := <-publicIPChan:
		info.PublicIP = ip
	case <-time.After(5 * time.Second):
		info.PublicIP = "timeout"
	}

	return info, nil
}

// IsPortAvailable checks if a port is available for binding
func IsPortAvailable(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// IsPortInUse checks if a port is currently in use
func IsPortInUse(port int) bool {
	return !IsPortAvailable(port)
}

// GetAvailablePort finds an available port starting from the given port
func GetAvailablePort(startPort int) (int, error) {
	for port := startPort; port < startPort+100; port++ {
		if IsPortAvailable(port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available ports found in range %d-%d", startPort, startPort+100)
}

// TestConnectivity tests connectivity to a host and port
func TestConnectivity(host string, port int, timeout time.Duration) error {
	address := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	conn.Close()
	return nil
}

// TestHTTPConnectivity tests HTTP/HTTPS connectivity to a URL
func TestHTTPConnectivity(url string, timeout time.Duration) error {
	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	return nil
}

// GetInterfaceByIP finds the network interface for a given IP address
func GetInterfaceByIP(ip string) (*net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	targetIP := net.ParseIP(ip)
	if targetIP == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ipNet.IP.Equal(targetIP) {
					return &iface, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no interface found for IP: %s", ip)
}

// ListNetworkInterfaces returns all network interfaces with their details
func ListNetworkInterfaces() ([]InterfaceInfo, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces: %w", err)
	}

	var infos []InterfaceInfo
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		info := InterfaceInfo{
			Name:         iface.Name,
			HardwareAddr: iface.HardwareAddr.String(),
			Flags:        iface.Flags.String(),
			MTU:          iface.MTU,
		}

		for _, addr := range addrs {
			info.Addresses = append(info.Addresses, addr.String())
		}

		infos = append(infos, info)
	}

	return infos, nil
}

// InterfaceInfo contains information about a network interface
type InterfaceInfo struct {
	Name         string
	HardwareAddr string
	Flags        string
	MTU          int
	Addresses    []string
}

// ResolveHostIP resolves a hostname to IP addresses
func ResolveHostIP(hostname string) ([]string, error) {
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve hostname: %w", err)
	}

	var ipStrings []string
	for _, ip := range ips {
		ipStrings = append(ipStrings, ip.String())
	}

	return ipStrings, nil
}

// ValidateIPAddress validates if a string is a valid IP address
func ValidateIPAddress(ip string) bool {
	return net.ParseIP(ip) != nil
}

// ValidatePort validates if a port number is valid
func ValidatePort(port int) bool {
	return port > 0 && port <= 65535
}
