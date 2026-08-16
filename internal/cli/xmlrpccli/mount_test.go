package xmlrpccli

// What the umbrella command mounts.
//
// wpxmlrpc is both a standalone binary and a group under `wpexporter`. The
// three accessors below are how the umbrella reaches into this package, and a
// mistake in them does not fail a build — it produces a command that runs with
// unset flags, which is the worst way to find out.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMountedCommandsAreTheRealOnes: the umbrella must mount this package's own
// commands, not copies, or the flags it parses land in variables nobody reads.
func TestMountedCommandsAreTheRealOnes(t *testing.T) {
	root := RootCommand()
	require.NotNil(t, root)
	assert.Same(t, rootCmd, root)

	export := ExportCommand()
	require.NotNil(t, export)
	assert.Same(t, exportCmd, export)
	assert.Equal(t, "export", export.Name())
}

// TestPersistentFlagsAreBoundHere: mounting the subcommand without these would
// leave this package's variables unset, so the flag set has to be the root's.
func TestPersistentFlagsAreBoundHere(t *testing.T) {
	flags := PersistentFlags()
	require.NotNil(t, flags)

	assert.NotNil(t, flags.Lookup("config"), "the config flag the umbrella forwards")
	assert.Same(t, rootCmd.PersistentFlags(), flags)
}

// TestExportRequiresASite: the one thing the command cannot do without.
func TestExportRequiresASite(t *testing.T) {
	export := ExportCommand()

	for _, name := range []string{"url", "username", "password", "output", "format"} {
		assert.NotNil(t, export.Flags().Lookup(name), "flag %q", name)
	}
}

// TestConfigFileExistsIsAnswerable: it reads the user's home, and must answer
// rather than fail when there is nothing there.
func TestConfigFileExistsIsAnswerable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	assert.False(t, configFileExists(), "a home with no config has none")
}
