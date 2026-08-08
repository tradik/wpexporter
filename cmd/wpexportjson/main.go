// Command wpexportjson is the REST API entry point of the wpexporter toolkit.
//
// The command tree lives in internal/cli/exportcli so that the umbrella command,
// wpexporter, can mount the same tree rather than reimplementing it.
package main

import (
	"fmt"
	"os"

	"github.com/tradik/wpexporter/internal/cli/exportcli"
)

func main() {
	if err := exportcli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
