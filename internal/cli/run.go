// Package cli holds what the toolkit's entry points share.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Main runs a command tree and applies the toolkit's exit convention: the error
// goes to stderr, the status is 1.
//
// Every binary's main() is this one call, so the convention is defined once
// rather than copied into each.
func Main(execute func() error) {
	os.Exit(run(execute, os.Stderr))
}

// run reports the status Main should exit with, so the failure path is testable
// without terminating the test binary.
func run(execute func() error, stderr io.Writer) int {
	if err := execute(); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)

		return 1
	}

	return 0
}

// ConfigFileExists reports whether a config file is present in any of the places
// a tool reads one from: the working directory, the user's per-tool directory,
// and /etc.
//
// toolName selects the per-tool directory, which is the only thing that differed
// between the tools' own copies of this search.
func ConfigFileExists(toolName string) bool {
	home := os.Getenv("HOME")

	candidates := []string{
		"./config.yaml",
		"./config.yml",
		filepath.Join(home, "."+toolName, "config.yaml"),
		filepath.Join(home, "."+toolName, "config.yml"),
		filepath.Join("/etc", toolName, "config.yaml"),
		filepath.Join("/etc", toolName, "config.yml"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}
