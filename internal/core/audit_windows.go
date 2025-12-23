//go:build windows

package core

import (
	"fmt"
)

// syslogWriter is a stub for Windows (syslog not available)
type syslogWriter struct{}

// newSyslogWriter returns an error on Windows since syslog is not available
func newSyslogWriter(server string) (*syslogWriter, error) {
	return nil, fmt.Errorf("syslog is not supported on Windows")
}

// Info is a no-op on Windows
func (s *syslogWriter) Info(msg string) error {
	return nil
}

// Warning is a no-op on Windows
func (s *syslogWriter) Warning(msg string) error {
	return nil
}

// Close is a no-op on Windows
func (s *syslogWriter) Close() error {
	return nil
}
