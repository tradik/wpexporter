// Command wpexportjson is the REST API entry point of the wpexporter toolkit.
//
// The command tree lives in internal/cli/exportcli so that the umbrella command,
// wpexporter, can mount the same tree rather than reimplementing it.
package main

import (
	"github.com/tradik/wpexporter/internal/cli"
	"github.com/tradik/wpexporter/internal/cli/exportcli"
)

func main() {
	cli.Main(exportcli.Execute)
}
