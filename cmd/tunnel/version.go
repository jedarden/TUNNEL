package main

import (
	"fmt"
	"runtime"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version, build date, and other build information.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return showVersion()
	},
}

func showVersion() error {
	if jsonOutput {
		return printJSON(map[string]interface{}{
			"version":   Version,
			"buildDate": BuildDate,
			"gitCommit": GitCommit,
			"goVersion": GoVersion,
			"compiler":  runtime.Compiler,
			"platform":  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		})
	}

	color.Cyan("TUNNEL - Terminal Unified Network Node Encrypted Link")
	fmt.Println()

	fmt.Printf("Version:      %s\n", color.GreenString(Version))
	fmt.Printf("Build Date:   %s\n", BuildDate)
	fmt.Printf("Git Commit:   %s\n", GitCommit)
	fmt.Printf("Go Version:   %s\n", GoVersion)
	fmt.Printf("Compiler:     %s\n", runtime.Compiler)
	fmt.Printf("Platform:     %s/%s\n", runtime.GOOS, runtime.GOARCH)

	return nil
}
