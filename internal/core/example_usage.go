package core

// This file provides example usage of the credential and configuration system.
// These examples are meant for documentation purposes.

import (
	"fmt"
	"log"
	"time"

	"github.com/jedarden/tunnel/pkg/config"
)

// ExampleCredentialStoreUsage demonstrates basic credential store operations
func ExampleCredentialStoreUsage() {
	// Create a file-based credential store
	store, err := NewFileStore("/tmp/tunnel-creds", "my-secure-passphrase")
	if err != nil {
		log.Fatal(err)
	}

	// Store credentials for different services
	credentials := map[string]map[string][]byte{
		"ssh": {
			"password":    []byte("my-ssh-password"),
			"private_key": []byte("-----BEGIN PRIVATE KEY-----\n..."),
		},
		"oauth": {
			"access_token":  []byte("github_token_xyz123"),
			"refresh_token": []byte("refresh_xyz456"),
		},
		"totp": {
			"secret": []byte("JBSWY3DPEHPK3PXP"),
		},
	}

	// Store all credentials
	for service, keys := range credentials {
		for key, value := range keys {
			if err := store.Set(service, key, value); err != nil {
				log.Printf("Failed to store %s/%s: %v", service, key, err)
			}
		}
	}

	// Retrieve a specific credential
	sshPassword, err := store.Get("ssh", "password")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("SSH Password: %s\n", sshPassword)

	// List all keys for a service
	sshKeys, err := store.List("ssh")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("SSH credentials stored: %v\n", sshKeys)
}

// ExampleConfigurationUsage demonstrates configuration management
func ExampleConfigurationUsage() {
	// Load configuration (creates default if not exists)
	cfg, err := config.Load("~/.config/tunnel/config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Read configuration values
	fmt.Printf("SSH Port: %d\n", cfg.SSH.Port)
	fmt.Printf("Log Level: %s\n", cfg.Settings.LogLevel)
	fmt.Printf("Default Method: %s\n", cfg.Settings.DefaultMethod)

	// Get enabled authentication methods (sorted by priority)
	methods := cfg.GetEnabledMethods()
	fmt.Printf("Enabled methods: %v\n", methods)

	// Check specific method configuration
	if sshKeyConfig, ok := cfg.GetMethod("ssh-key"); ok {
		fmt.Printf("SSH Key enabled: %v (priority: %d)\n",
			sshKeyConfig.Enabled, sshKeyConfig.Priority)
	}

	// Update configuration
	cfg.Settings.LogLevel = "debug"
	cfg.SSH.Port = 2223

	// Save changes
	if err := cfg.Save(); err != nil {
		log.Fatal(err)
	}

	// Watch for configuration changes
	if err := cfg.Watch(); err != nil {
		log.Fatal(err)
	}

	// Register change handler
	cfg.OnChange(func(c *config.Config) {
		fmt.Printf("Configuration reloaded! New log level: %s\n", c.Settings.LogLevel)
	})
}

// ExampleKeyManagement demonstrates SSH key management
func ExampleKeyManagement() {
	// Create audit logger first
	auditLogger, err := NewAuditLogger(
		"/var/log/tunnel/audit.log",
		false, // don't use syslog
		"",
	)
	if err != nil {
		log.Fatal(err)
	}
	defer auditLogger.Close()

	// Create key manager
	keyManager, err := NewFileKeyManager(
		"~/.ssh/authorized_keys",
		auditLogger,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Import keys from GitHub
	fmt.Println("Importing keys from GitHub...")
	keys, err := keyManager.ImportFromGitHub("torvalds")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Imported %d keys\n", len(keys))

	// Add a key manually
	publicKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl user@host"

	validatedKey, err := keyManager.ValidateKey(publicKey)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Key fingerprint: %s\n", validatedKey.Fingerprint)

	if err := keyManager.AddKey("username", *validatedKey); err != nil {
		log.Fatal(err)
	}

	// List all keys
	allKeys, err := keyManager.ListKeys("username")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Authorized keys:")
	for _, key := range allKeys {
		fmt.Printf("  - %s (%s) - %s\n", key.Type, key.Fingerprint, key.Comment)
	}

	// Remove a key
	if len(allKeys) > 0 {
		keyToRemove := allKeys[0]
		if err := keyManager.RemoveKey("username", keyToRemove.ID); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Removed key: %s\n", keyToRemove.Fingerprint)
	}
}

// ExampleAuditLogging demonstrates audit logging
func ExampleAuditLogging() {
	// Create audit logger with syslog support
	logger, err := NewAuditLogger(
		"/var/log/tunnel/audit.log",
		true, // use syslog
		"",   // local syslog
	)
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Close()

	// Log a successful authentication
	err = logger.LogConnectionAttempt(
		"ssh-key",
		"alice",
		"192.168.1.100",
		true,
		map[string]interface{}{
			"fingerprint": "SHA256:abc123...",
			"key_type":    "ssh-ed25519",
		},
	)
	if err != nil {
		log.Printf("Failed to log connection attempt: %v", err)
	}

	// Log connection established
	err = logger.LogConnectionEstablished(
		"ssh-key",
		"alice",
		"192.168.1.100",
		map[string]interface{}{
			"session_id": "sess_123",
		},
	)

	// Simulate some work
	time.Sleep(2 * time.Second)

	// Log connection closed
	err = logger.LogConnectionClosed(
		"ssh-key",
		"alice",
		"192.168.1.100",
		2*time.Second,
		map[string]interface{}{
			"bytes_sent":     1024,
			"bytes_received": 2048,
		},
	)

	// Log a failed authentication
	err = logger.LogConnectionAttempt(
		"password",
		"bob",
		"10.0.0.5",
		false,
		map[string]interface{}{
			"reason": "invalid password",
		},
	)

	// Log key operations
	err = logger.LogKeyOperation(
		"key_added",
		"alice",
		true,
		map[string]interface{}{
			"fingerprint": "SHA256:xyz789...",
			"type":        "ssh-rsa",
			"source":      "github",
		},
	)

	// Log configuration changes
	err = logger.LogConfigChange(
		"admin",
		map[string]interface{}{
			"field":     "ssh.port",
			"old_value": 2222,
			"new_value": 2223,
		},
	)
}

// ExampleIntegratedSetup demonstrates setting up all components together
func ExampleIntegratedSetup() {
	// 1. Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatal(err)
	}

	// 2. Setup audit logging
	auditLogger, err := NewAuditLogger(
		cfg.Monitoring.AuditLog,
		cfg.Monitoring.Syslog,
		cfg.Monitoring.SyslogServer,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer auditLogger.Close()

	// 3. Setup credential store
	credStore, err := NewCredentialStore(
		cfg.Credentials.Store,
		"tunnel",
		cfg.Credentials.BaseDir,
		cfg.Credentials.Passphrase,
	)
	if err != nil {
		log.Fatal(err)
	}

	// 4. Setup key manager
	keyManager, err := NewFileKeyManager(
		cfg.SSH.AuthorizedKeys,
		auditLogger,
	)
	if err != nil {
		log.Fatal(err)
	}

	// 5. Watch configuration changes
	if err := cfg.Watch(); err != nil {
		log.Fatal(err)
	}

	cfg.OnChange(func(c *config.Config) {
		auditLogger.LogConfigChange("system", map[string]interface{}{
			"event": "config_reloaded",
		})
	})

	// 6. Use the components
	fmt.Println("TUNNEL system initialized")
	fmt.Printf("- SSH Port: %d\n", cfg.SSH.Port)
	fmt.Printf("- Max Sessions: %d\n", cfg.SSH.MaxSessions)
	fmt.Printf("- Audit Log: %s\n", cfg.Monitoring.AuditLog)
	fmt.Printf("- Credential Store: %s\n", cfg.Credentials.Store)

	// Example: Store OAuth token
	if err := credStore.Set("oauth", "github_token", []byte("ghp_...")); err != nil {
		log.Printf("Failed to store OAuth token: %v", err)
	}

	// Example: Import SSH keys from GitHub
	keys, err := keyManager.ImportFromGitHub("octocat")
	if err != nil {
		log.Printf("Failed to import GitHub keys: %v", err)
	} else {
		fmt.Printf("Imported %d SSH keys from GitHub\n", len(keys))
	}

	// Example: Log an event
	auditLogger.Log(AuditEvent{
		Timestamp: time.Now(),
		EventType: "system_started",
		Method:    "system",
		User:      "system",
		Details: map[string]interface{}{
			"version": cfg.Version,
		},
		Success: true,
	})
}

// ExampleFailoverWithCredentials demonstrates using credentials with failover
func ExampleFailoverWithCredentials() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatal(err)
	}

	// Setup credential store
	credStore, err := NewCredentialStore(
		cfg.Credentials.Store,
		"tunnel",
		cfg.Credentials.BaseDir,
		cfg.Credentials.Passphrase,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Get enabled methods in priority order
	methods := cfg.GetEnabledMethods()

	// Try each method in order
	for _, methodName := range methods {
		methodConfig, _ := cfg.GetMethod(methodName)

		fmt.Printf("Trying authentication method: %s (priority: %d)\n",
			methodName, methodConfig.Priority)

		// If method needs credentials, fetch them
		if methodConfig.AuthKeyRef != "" {
			// Parse the reference (format: "service:key")
			// Example: "tunnel:ssh-password"
			cred, err := credStore.Get("tunnel", "ssh-password")
			if err != nil {
				fmt.Printf("  Failed to get credentials: %v\n", err)
				continue
			}
			fmt.Printf("  Retrieved credentials: %d bytes\n", len(cred))
		}

		// Attempt authentication with this method
		// (implementation would go here)

		// If successful, break
		fmt.Printf("  Authentication successful!\n")
		break
	}
}
