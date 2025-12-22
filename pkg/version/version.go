// Package version provides version information for the tunnel application.
// These variables are typically set at build time using ldflags.
package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version of the application
	Version = "dev"
	// BuildDate is the date when the binary was built
	BuildDate = "unknown"
	// GitCommit is the git commit hash
	GitCommit = "unknown"
	// GoVersion is the Go version used to build the binary
	GoVersion = "unknown"
)

// Info returns a formatted version string
func Info() string {
	return fmt.Sprintf("%s (%s)", Version, GitCommit[:min(7, len(GitCommit))])
}

// Full returns the full version information
func Full() string {
	return fmt.Sprintf("Version: %s\nBuild Date: %s\nGit Commit: %s\nGo Version: %s\nPlatform: %s/%s",
		Version, BuildDate, GitCommit, GoVersion, runtime.GOOS, runtime.GOARCH)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
