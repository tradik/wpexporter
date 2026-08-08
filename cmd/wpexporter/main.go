// Command wpexporter is the single entry point to the WordPress export toolkit.
//
// The project has always been called wpexporter — it is the repository, the Go
// module, the Homebrew formula, the Snap package and the Docker image — but
// there was no command by that name. Installing it gave you three differently
// named binaries, and the archive was named after one of them, which reads as
// though the tool you asked for is missing.
//
// This command mounts the same trees those binaries run, so `wpexporter --help`
// shows the whole toolkit. The individual binaries remain, unchanged, for
// anyone scripting against them.
package main

import (
	"github.com/spf13/cobra"

	"github.com/tradik/wpexporter/internal/cli"
	"github.com/tradik/wpexporter/internal/cli/exportcli"
	"github.com/tradik/wpexporter/internal/cli/mcpcli"
	"github.com/tradik/wpexporter/internal/cli/xmlrpccli"
	"github.com/tradik/wpexporter/internal/version"
)

func main() {
	cli.Main(newRootCommand().Execute)
}

// newRootCommand assembles the umbrella from the three tools.
func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:     "wpexporter",
		Short:   "WordPress export toolkit",
		Version: version.String(),
		Long: `WordPress export toolkit.

  wpexporter export        Export via the REST API      (same as wpexportjson export)
  wpexporter xmlrpc        Export via XML-RPC           (same as wpxmlrpc)
  wpexporter mcp           Run the MCP server           (same as wpmcp)

The three commands are also installed as standalone binaries — wpexportjson,
wpxmlrpc and wpmcp — which behave identically.

Docs: https://github.com/tradik/wpexporter`,
	}

	// The REST exporter is the common case, so its subcommand sits at the top
	// level. Its persistent flags come along, since runExport reads the
	// variables they are bound to.
	root.PersistentFlags().AddFlagSet(exportcli.PersistentFlags())
	root.AddCommand(exportcli.ExportCommand())

	// The other two mount as groups, each keeping its own persistent flags:
	// both define --config and --verbose bound to their own variables, so
	// hoisting them all onto one root would collide.
	root.AddCommand(asGroup(xmlrpccli.RootCommand(), "xmlrpc", "Export WordPress content via XML-RPC"))
	root.AddCommand(asGroup(mcpcli.RootCommand(), "mcp", "Run the MCP server for AI assistants"))

	return root
}

// asGroup re-labels a tool's own root so it answers to a name under the
// umbrella rather than to its binary name.
func asGroup(command *cobra.Command, use, short string) *cobra.Command {
	command.Use = use
	command.Short = short
	// The standalone banner would advertise the wrong invocation here.
	command.Version = ""

	return command
}
