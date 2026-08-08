package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commandNames lists a command's immediate subcommands, ignoring the ones cobra
// adds itself.
func commandNames(command *cobra.Command) []string {
	var names []string

	for _, sub := range command.Commands() {
		if sub.Name() == "help" || sub.Name() == "completion" {
			continue
		}

		names = append(names, sub.Name())
	}

	return names
}

// TestUmbrellaMountsEveryTool pins that the umbrella reaches all three tools —
// the whole point of the command is that `wpexporter --help` shows the toolkit.
func TestUmbrellaMountsEveryTool(t *testing.T) {
	root := newRootCommand()

	assert.ElementsMatch(t, []string{"export", "xmlrpc", "mcp"}, commandNames(root))
}

// TestUmbrellaExportIsTheRESTExporter pins that the common case sits at the top
// level rather than behind a group.
func TestUmbrellaExportIsTheRESTExporter(t *testing.T) {
	root := newRootCommand()

	export, _, err := root.Find([]string{"export"})
	require.NoError(t, err)

	assert.Equal(t, "export", export.Name())
	assert.NotNil(t, export.RunE, "export must be runnable, not a group")

	// A flag only the REST exporter defines, to confirm it is that tree.
	assert.NotNil(t, export.Flags().Lookup("assisted-crawl"))
	assert.NotNil(t, export.Flags().Lookup("extract-meta"))
}

// TestUmbrellaGroupsKeepTheirSubcommands pins that the other two tools are
// reachable through their groups.
func TestUmbrellaGroupsKeepTheirSubcommands(t *testing.T) {
	root := newRootCommand()

	xmlrpcExport, _, err := root.Find([]string{"xmlrpc", "export"})
	require.NoError(t, err)
	assert.Equal(t, "export", xmlrpcExport.Name())
	assert.NotNil(t, xmlrpcExport.RunE)

	mcpServe, _, err := root.Find([]string{"mcp", "serve"})
	require.NoError(t, err)
	assert.Equal(t, "serve", mcpServe.Name())
	assert.NotNil(t, mcpServe.RunE)
}

// TestUmbrellaCarriesExporterPersistentFlags pins that mounting the export
// subcommand alone did not leave its global flags behind — runExport reads the
// variables they are bound to.
func TestUmbrellaCarriesExporterPersistentFlags(t *testing.T) {
	root := newRootCommand()

	assert.NotNil(t, root.PersistentFlags().Lookup("config"))
	assert.NotNil(t, root.PersistentFlags().Lookup("verbose"))
}

// TestUmbrellaReportsAVersion pins that --version works on the umbrella.
func TestUmbrellaReportsAVersion(t *testing.T) {
	assert.NotEmpty(t, newRootCommand().Version)
}

// TestAsGroupClearsTheStandaloneBanner pins that a mounted tool does not
// advertise its own --version, which would print the wrong invocation.
func TestAsGroupClearsTheStandaloneBanner(t *testing.T) {
	command := &cobra.Command{Use: "wpxmlrpc", Short: "old", Version: "9.9.9"}

	grouped := asGroup(command, "xmlrpc", "new short")

	assert.Equal(t, "xmlrpc", grouped.Use)
	assert.Equal(t, "new short", grouped.Short)
	assert.Empty(t, grouped.Version)
}
