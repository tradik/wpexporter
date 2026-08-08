// Command wpxmlrpc is the XML-RPC entry point of the wpexporter toolkit.
//
// The command tree lives in internal/cli/xmlrpccli so that the umbrella command,
// wpexporter, can mount the same tree rather than reimplementing it.
package main

import (
	"fmt"
	"os"

	"github.com/tradik/wpexporter/internal/cli/xmlrpccli"
)

func main() {
	if err := xmlrpccli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
