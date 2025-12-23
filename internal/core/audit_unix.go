//go:build !windows

package core

import (
	"fmt"
	"log/syslog"
)

// syslogWriter wraps syslog.Writer for Unix systems
type syslogWriter struct {
	writer *syslog.Writer
}

// newSyslogWriter creates a new syslog writer
func newSyslogWriter(server string) (*syslogWriter, error) {
	var writer *syslog.Writer
	var err error

	if server != "" {
		// Remote syslog
		writer, err = syslog.Dial("udp", server, syslog.LOG_INFO|syslog.LOG_AUTH, "tunnel")
	} else {
		// Local syslog
		writer, err = syslog.New(syslog.LOG_INFO|syslog.LOG_AUTH, "tunnel")
	}

	if err != nil {
		return nil, fmt.Errorf("connect to syslog: %w", err)
	}

	return &syslogWriter{writer: writer}, nil
}

// Info logs an info message
func (s *syslogWriter) Info(msg string) error {
	return s.writer.Info(msg)
}

// Warning logs a warning message
func (s *syslogWriter) Warning(msg string) error {
	return s.writer.Warning(msg)
}

// Close closes the syslog writer
func (s *syslogWriter) Close() error {
	return s.writer.Close()
}
