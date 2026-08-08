// Command wpxmlrpc is the XML-RPC entry point of the wpexporter toolkit.
//
// The command tree lives in internal/cli/xmlrpccli so that the umbrella command,
// wpexporter, can mount the same tree rather than reimplementing it.
package main

import (
	"github.com/tradik/wpexporter/internal/cli"
	"github.com/tradik/wpexporter/internal/cli/xmlrpccli"
)

func main() {
	cli.Main(xmlrpccli.Execute)
}
