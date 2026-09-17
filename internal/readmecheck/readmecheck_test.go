// Package readmecheck locks the README's documented surface to the code that
// implements it. The key-management workflow shipped after the README
// documented `tunnel keys` commands that did not exist; these tests fail
// whenever the README claims a command or API endpoint again without a
// matching definition, and whenever the documented key workflow loses its
// implementation.
package readmecheck

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var useTokenRe = regexp.MustCompile(`Use:\s+"([^"]+)"`)

func repoFile(t *testing.T, elems ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, elems...)...))
	if err != nil {
		t.Fatalf("read %v: %v", elems, err)
	}
	return string(data)
}

// cliUseTokens returns the first word of every cobra Use string across the
// CLI sources (cmd/tunnel is split across cli.go, doctor.go, version.go, ...).
func cliUseTokens(t *testing.T) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("..", "..", "cmd", "tunnel"))
	if err != nil {
		t.Fatalf("list cmd/tunnel: %v", err)
	}
	tokens := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		for _, match := range useTokenRe.FindAllStringSubmatch(repoFile(t, "cmd", "tunnel", name), -1) {
			fields := strings.Fields(match[1])
			if len(fields) > 0 {
				tokens[fields[0]] = true
			}
		}
	}
	return tokens
}

// readmeCommandLines returns the `tunnel ...` command lines inside ```bash
// fences, skipping comments and shell around them.
func readmeCommandLines(t *testing.T) []string {
	t.Helper()
	var commands []string
	inFence := false
	for _, line := range strings.Split(repoFile(t, "README.md"), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			if inFence {
				inFence = false
			} else if strings.HasPrefix(trimmed, "```bash") {
				inFence = true
			}
		case inFence && strings.HasPrefix(trimmed, "tunnel "):
			commands = append(commands, trimmed)
		}
	}
	return commands
}

func TestReadmeCommandsExistInCLI(t *testing.T) {
	tokens := cliUseTokens(t)
	commands := readmeCommandLines(t)
	if len(commands) < 10 {
		t.Fatalf("expected to find the README command reference, got %d commands", len(commands))
	}
	for _, command := range commands {
		fields := strings.Fields(command)
		if len(fields) < 2 {
			continue
		}
		word := fields[1]
		// Skip usage-syntax placeholders (`tunnel [--port 8080]`) and
		// trailing comments on bare `tunnel` lines; only real subcommand
		// words can be checked against command definitions.
		if !regexp.MustCompile(`^[a-z][a-z0-9-]*$`).MatchString(word) {
			continue
		}
		if !tokens[word] {
			t.Errorf("README documents %q but no cobra command in cmd/tunnel defines it", word)
		}
	}
}

func TestKeyWorkflowDocumentedAndImplemented(t *testing.T) {
	tokens := cliUseTokens(t)

	// The SSH key workflow the README promises: the command family plus the
	// four documented operations.
	for _, token := range []string{"keys", "import", "import-github", "add", "list", "revoke"} {
		if !tokens[token] {
			t.Errorf("documented key workflow command %q is missing from the CLI", token)
		}
	}

	// The README must keep documenting the workflow it now implements.
	documented := map[string]bool{}
	for _, command := range readmeCommandLines(t) {
		fields := strings.Fields(command)
		if len(fields) >= 3 && fields[1] == "keys" && !strings.HasPrefix(fields[2], "-") {
			documented[fields[2]] = true
		}
	}
	for _, sub := range []string{"import", "add", "list", "revoke"} {
		if !documented[sub] {
			t.Errorf("README no longer documents `tunnel keys %s`", sub)
		}
	}
}

func TestReadmeAPITreeMatchesRouter(t *testing.T) {
	router := repoFile(t, "internal", "web", "api", "router.go")
	treeEntry := regexp.MustCompile(`(?:├──|└──)\s+/api/([a-z]+)`)
	claims := treeEntry.FindAllStringSubmatch(repoFile(t, "README.md"), -1)
	if len(claims) == 0 {
		t.Fatal("expected the architecture tree to list /api endpoints")
	}
	for _, claim := range claims {
		name := claim[1]
		// A claimed endpoint must appear as a route path in the router.
		if !strings.Contains(router, `"/`+name) {
			t.Errorf("README claims /api/%s but internal/web/api/router.go defines no such route", name)
		}
	}
}
