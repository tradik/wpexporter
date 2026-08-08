// Command wpmcp is the MCP server entry point of the wpexporter toolkit.
//
// The command tree lives in internal/cli/mcpcli so that the umbrella command,
// wpexporter, can mount the same tree rather than reimplementing it.
package main

import (
	"fmt"
	"os"

	"github.com/tradik/wpexporter/internal/cli/mcpcli"
)

func main() {
	if err := mcpcli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
