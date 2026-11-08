package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"

	"github.com/jedarden/tunnel/internal/registry"
)

// captureListOutput collects everything listMethods writes through both
// fmt (os.Stdout) and the color package's own output writer, which holds its
// own reference to the original stdout and would otherwise escape the pipe.
func captureListOutput(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	originalStdout := os.Stdout
	originalColorOutput := color.Output
	os.Stdout = writer
	color.Output = writer

	collected := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		collected <- buf.String()
	}()

	fn()

	os.Stdout = originalStdout
	color.Output = originalColorOutput
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}
	return <-collected
}

// withDefaultRegistry points the package-level registry at a fresh registry
// carrying all eleven built-in providers for the duration of the test.
func withDefaultRegistry(t *testing.T) {
	t.Helper()
	original := reg
	reg = registry.NewRegistry()
	t.Cleanup(func() { reg = original })
}

// TestListMethodsShowsEveryRegisteredProvider pins tunnel list to surface
// every registered provider. The registry fetch always returned SSH and
// Direct providers, but the renderer dropped those categories, so the text
// output showed 6 of the 11 registered providers while the command's own
// help promised "all available tunnel methods".
func TestListMethodsShowsEveryRegisteredProvider(t *testing.T) {
	withDefaultRegistry(t)

	output := captureListOutput(t, func() { _ = listMethods() })

	for _, want := range []string{
		"VPN Providers:",
		"Tunnel Providers:",
		"SSH Providers:",
		"Direct Providers:",
		"tailscale", "wireguard", "zerotier",
		"cloudflare", "ngrok", "bore",
		"vscode-tunnel", "ssh-forward", "reverse-ssh", "bastion",
		"oauth",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("tunnel list output missing %q; got:\n%s", want, output)
		}
	}
}

// TestListMethodsJSONIncludesEveryRegisteredProvider pins the --json path,
// which always returned all providers, so a future renderer regression in
// the text path cannot hide behind the machine-readable format.
func TestListMethodsJSONIncludesEveryRegisteredProvider(t *testing.T) {
	withDefaultRegistry(t)

	originalJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = originalJSON }()

	output := captureListOutput(t, func() { _ = listMethods() })

	var payload struct {
		Providers []registry.ProviderInfo `json:"providers"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("parse tunnel list --json output: %v\nraw:\n%s", err, output)
	}

	got := map[string]bool{}
	for _, provider := range payload.Providers {
		got[provider.Name] = true
	}
	for _, want := range []string{
		"tailscale", "wireguard", "zerotier", "cloudflare", "ngrok", "bore",
		"vscode-tunnel", "ssh-forward", "reverse-ssh", "bastion", "oauth",
	} {
		if !got[want] {
			t.Errorf("tunnel list --json missing provider %q; got %d providers", want, len(payload.Providers))
		}
	}
}
