package procman

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeHelper(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "helper")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitForLogs(t *testing.T, manager *Manager, since time.Time, want int) []string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries := manager.Logs(since)
		if len(entries) >= want {
			messages := make([]string, 0, len(entries))
			for _, entry := range entries {
				messages = append(messages, entry.Message)
			}
			return messages
		}
		time.Sleep(10 * time.Millisecond)
	}
	entries := manager.Logs(since)
	messages := make([]string, 0, len(entries))
	for _, entry := range entries {
		messages = append(messages, entry.Message)
	}
	return messages
}

func TestManagerCapturesOutputAndSupportsRestart(t *testing.T) {
	path := writeHelper(t, `
echo helper-started
trap 'echo helper-stopped; exit 0' INT TERM
while true; do sleep 1; done`)

	manager := New("test-provider")
	since := time.Now().Add(-time.Second)
	if err := manager.Start(path, []string{"argument"}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !manager.Running() {
		t.Fatal("Running() = false immediately after Start")
	}
	if manager.PID() == 0 {
		t.Fatal("PID() = 0 for running process")
	}

	messages := waitForLogs(t, manager, since, 2)
	if !containsMessage(messages, "helper-started") {
		t.Fatalf("captured logs = %v, missing helper output", messages)
	}
	if logs := manager.Logs(time.Now().Add(time.Second)); logs == nil {
		t.Fatal("Logs() returned nil")
	}

	if err := manager.Stop(time.Second); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if manager.Running() {
		t.Fatal("Running() = true after Stop")
	}

	if err := manager.Start(path, nil); err != nil {
		t.Fatalf("restart Start() error = %v", err)
	}
	if !manager.Running() {
		t.Fatal("restarted manager is not running")
	}
	if err := manager.Stop(time.Second); err != nil {
		t.Fatalf("restart Stop() error = %v", err)
	}
}

func TestManagerReportsChildExit(t *testing.T) {
	path := writeHelper(t, `echo failed-output >&2; exit 7`)
	manager := New("test-provider")
	if err := manager.Start(path, nil); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for manager.Running() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if manager.Running() {
		t.Fatal("child remained running")
	}
	messages := waitForLogs(t, manager, time.Now().Add(-time.Second), 2)
	if !containsMessage(messages, "failed-output") {
		t.Fatalf("captured logs = %v, missing stderr output", messages)
	}
	if !containsMessage(messages, "process exited") {
		t.Fatalf("captured logs = %v, missing exit record", messages)
	}
}

func containsMessage(messages []string, want string) bool {
	for _, message := range messages {
		if strings.Contains(message, want) {
			return true
		}
	}
	return false
}
