package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigFileExists covers each place a tool reads a config file from.
//
// This lived twice, once per tool, with twenty lines of HOME and
// working-directory juggling repeated across four near-identical test
// functions in each copy. One table against the shared helper covers both.
func TestConfigFileExists(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, home string)
		want  bool
	}{
		{
			name:  "no config anywhere",
			setup: func(*testing.T, string) {},
			want:  false,
		},
		{
			name: "local config.yaml",
			setup: func(t *testing.T, _ string) {
				writeConfig(t, "config.yaml")
			},
			want: true,
		},
		{
			name: "local config.yml",
			setup: func(t *testing.T, _ string) {
				writeConfig(t, "config.yml")
			},
			want: true,
		},
		{
			name: "config.yaml in the tool's HOME directory",
			setup: func(t *testing.T, home string) {
				dir := filepath.Join(home, ".wpexportjson")
				require.NoError(t, os.MkdirAll(dir, 0750))
				writeConfig(t, filepath.Join(dir, "config.yaml"))
			},
			want: true,
		},
		{
			name: "config.yml in the tool's HOME directory",
			setup: func(t *testing.T, home string) {
				dir := filepath.Join(home, ".wpexportjson")
				require.NoError(t, os.MkdirAll(dir, 0750))
				writeConfig(t, filepath.Join(dir, "config.yml"))
			},
			want: true,
		},
		{
			name: "another tool's directory does not count",
			setup: func(t *testing.T, home string) {
				dir := filepath.Join(home, ".wpxmlrpc")
				require.NoError(t, os.MkdirAll(dir, 0750))
				writeConfig(t, filepath.Join(dir, "config.yaml"))
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateEnvironment(t)
			tt.setup(t, home)

			assert.Equal(t, tt.want, ConfigFileExists("wpexportjson"))
		})
	}
}

// TestConfigFileExistsIsPerTool pins that the tool name selects the directory,
// which is the only thing that differed between the copies this replaced.
func TestConfigFileExistsIsPerTool(t *testing.T) {
	home := isolateEnvironment(t)

	dir := filepath.Join(home, ".wpxmlrpc")
	require.NoError(t, os.MkdirAll(dir, 0750))
	writeConfig(t, filepath.Join(dir, "config.yaml"))

	assert.True(t, ConfigFileExists("wpxmlrpc"))
	assert.False(t, ConfigFileExists("wpexportjson"))
}

// TestRunExitStatus pins the exit convention both binaries and the umbrella
// rely on: 0 on success, 1 with the error on stderr otherwise.
func TestRunExitStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var stderr bytes.Buffer
		called := false

		status := run(func() error {
			called = true
			return nil
		}, &stderr)

		assert.True(t, called)
		assert.Equal(t, 0, status)
		assert.Empty(t, stderr.String(), "a successful run says nothing")
	})

	t.Run("failure", func(t *testing.T) {
		var stderr bytes.Buffer

		status := run(func() error { return errors.New("boom") }, &stderr)

		assert.Equal(t, 1, status)
		assert.Equal(t, "Error: boom\n", stderr.String())
	})
}

// isolateEnvironment points HOME and the working directory at fresh temporary
// directories, restoring both afterwards, and returns the temporary HOME.
func isolateEnvironment(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	originalWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(originalWd) })

	require.NoError(t, os.Chdir(t.TempDir()))

	return home
}

// writeConfig creates a minimal config file at path.
func writeConfig(t *testing.T, path string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte("url: https://example.com"), 0600))
}
