package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AuditEvent represents an audit log entry
type AuditEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	EventType string                 `json:"event_type"`
	Method    string                 `json:"method"`
	User      string                 `json:"user"`
	SourceIP  string                 `json:"source_ip"`
	Details   map[string]interface{} `json:"details"`
	Success   bool                   `json:"success"`
}

// AuditLogger handles audit logging
type AuditLogger struct {
	filePath     string
	syslogWriter *syslogWriter
	file         *os.File
	mu           sync.Mutex
	enabled      bool
	useSyslog    bool
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(filePath string, useSyslog bool, syslogServer string) (*AuditLogger, error) {
	logger := &AuditLogger{
		filePath:  filePath,
		enabled:   true,
		useSyslog: useSyslog,
	}

	// Setup file logging
	if filePath != "" {
		// Ensure directory exists
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create audit log directory: %w", err)
		}

		// Open file for appending
		file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("open audit log file: %w", err)
		}
		logger.file = file
	}

	// Setup syslog if enabled
	if useSyslog {
		writer, err := newSyslogWriter(syslogServer)
		if err != nil {
			// Log warning but don't fail - syslog might not be available (e.g., Windows)
			fmt.Fprintf(os.Stderr, "warning: syslog not available: %v\n", err)
		} else {
			logger.syslogWriter = writer
		}
	}

	return logger, nil
}

// Log writes an audit event
func (al *AuditLogger) Log(event AuditEvent) error {
	if !al.enabled {
		return nil
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	// Ensure timestamp is set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Marshal to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	// Write to file
	if al.file != nil {
		if _, err := al.file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("write to audit log: %w", err)
		}
	}

	// Write to syslog
	if al.syslogWriter != nil {
		msg := fmt.Sprintf("type=%s method=%s user=%s source_ip=%s success=%t",
			event.EventType, event.Method, event.User, event.SourceIP, event.Success)

		if event.Success {
			_ = al.syslogWriter.Info(msg)
		} else {
			_ = al.syslogWriter.Warning(msg)
		}
	}

	return nil
}

// LogConnectionAttempt logs an authentication attempt
func (al *AuditLogger) LogConnectionAttempt(method, user, sourceIP string, success bool, details map[string]interface{}) error {
	return al.Log(AuditEvent{
		Timestamp: time.Now(),
		EventType: "connection_attempt",
		Method:    method,
		User:      user,
		SourceIP:  sourceIP,
		Details:   details,
		Success:   success,
	})
}

// LogConnectionEstablished logs a successful connection
func (al *AuditLogger) LogConnectionEstablished(method, user, sourceIP string, details map[string]interface{}) error {
	return al.Log(AuditEvent{
		Timestamp: time.Now(),
		EventType: "connection_established",
		Method:    method,
		User:      user,
		SourceIP:  sourceIP,
		Details:   details,
		Success:   true,
	})
}

// LogConnectionClosed logs a closed connection
func (al *AuditLogger) LogConnectionClosed(method, user, sourceIP string, duration time.Duration, details map[string]interface{}) error {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["duration_seconds"] = duration.Seconds()

	return al.Log(AuditEvent{
		Timestamp: time.Now(),
		EventType: "connection_closed",
		Method:    method,
		User:      user,
		SourceIP:  sourceIP,
		Details:   details,
		Success:   true,
	})
}

// LogKeyOperation logs a key management operation
func (al *AuditLogger) LogKeyOperation(operation, user string, success bool, details map[string]interface{}) error {
	return al.Log(AuditEvent{
		Timestamp: time.Now(),
		EventType: operation,
		Method:    "ssh-key",
		User:      user,
		Details:   details,
		Success:   success,
	})
}

// LogConfigChange logs a configuration change
func (al *AuditLogger) LogConfigChange(user string, changes map[string]interface{}) error {
	return al.Log(AuditEvent{
		Timestamp: time.Now(),
		EventType: "config_change",
		Method:    "config",
		User:      user,
		Details:   changes,
		Success:   true,
	})
}

// LogError logs an error event
func (al *AuditLogger) LogError(eventType, method, user string, err error, details map[string]interface{}) error {
	if details == nil {
		details = make(map[string]interface{})
	}
	details["error"] = err.Error()

	return al.Log(AuditEvent{
		Timestamp: time.Now(),
		EventType: eventType,
		Method:    method,
		User:      user,
		Details:   details,
		Success:   false,
	})
}

// Rotate rotates the audit log file
func (al *AuditLogger) Rotate() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.file == nil {
		return nil
	}

	// Close current file
	if err := al.file.Close(); err != nil {
		return fmt.Errorf("close audit log: %w", err)
	}

	// Rename current file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s", al.filePath, timestamp)
	if err := os.Rename(al.filePath, backupPath); err != nil {
		return fmt.Errorf("rotate audit log: %w", err)
	}

	// Open new file
	file, err := os.OpenFile(al.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open new audit log: %w", err)
	}
	al.file = file

	return nil
}

// Close closes the audit logger
func (al *AuditLogger) Close() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	var errors []error

	if al.file != nil {
		if err := al.file.Close(); err != nil {
			errors = append(errors, fmt.Errorf("close file: %w", err))
		}
	}

	if al.syslogWriter != nil {
		if err := al.syslogWriter.Close(); err != nil {
			errors = append(errors, fmt.Errorf("close syslog: %w", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("close audit logger: %v", errors)
	}

	return nil
}

// SetEnabled enables or disables audit logging
func (al *AuditLogger) SetEnabled(enabled bool) {
	al.mu.Lock()
	defer al.mu.Unlock()
	al.enabled = enabled
}

// IsEnabled returns whether audit logging is enabled
func (al *AuditLogger) IsEnabled() bool {
	al.mu.Lock()
	defer al.mu.Unlock()
	return al.enabled
}
