package xmlrpccli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigFileExists covers each place a config file may live. The four
// cases shared the same twenty lines of HOME/working-directory juggling; the
// table keeps the setup in one place.
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
			name: "config in HOME",
			setup: func(t *testing.T, home string) {
				dir := filepath.Join(home, ".wpxmlrpc")
				require.NoError(t, os.MkdirAll(dir, 0750))
				writeConfig(t, filepath.Join(dir, "config.yaml"))
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := isolateEnvironment(t)
			tt.setup(t, home)

			assert.Equal(t, tt.want, configFileExists())
		})
	}
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

func TestRootCmd_Properties(t *testing.T) {
	assert.Equal(t, "wpxmlrpc", rootCmd.Use)
	assert.NotEmpty(t, rootCmd.Short)
	assert.NotEmpty(t, rootCmd.Long)
}

func TestExportCmd_Flags(t *testing.T) {
	// Verify export command has required flags
	urlFlag := exportCmd.Flags().Lookup("url")
	assert.NotNil(t, urlFlag)

	usernameFlag := exportCmd.Flags().Lookup("username")
	assert.NotNil(t, usernameFlag)

	passwordFlag := exportCmd.Flags().Lookup("password")
	assert.NotNil(t, passwordFlag)

	// Check other flags exist
	assert.NotNil(t, exportCmd.Flags().Lookup("output"))
	assert.NotNil(t, exportCmd.Flags().Lookup("format"))
}

func TestExportCmd_FlagDefaults(t *testing.T) {
	formatFlag := exportCmd.Flags().Lookup("format")
	assert.Equal(t, "json", formatFlag.DefValue)
}

func TestGlobalFlags(t *testing.T) {
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	assert.NotNil(t, configFlag)

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	assert.NotNil(t, verboseFlag)
}
