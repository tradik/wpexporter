package xmlrpccli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFileExists_NoConfigFiles(t *testing.T) {
	// Save original HOME and restore after test
	originalHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", originalHome)
	}()

	// Set HOME to a temp directory without config files
	tempDir := t.TempDir()
	_ = os.Setenv("HOME", tempDir)

	// Change to a directory without config files
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	emptyDir := t.TempDir()
	err = os.Chdir(emptyDir)
	require.NoError(t, err)

	result := configFileExists()
	assert.False(t, result)
}

func TestConfigFileExists_WithLocalConfig(t *testing.T) {
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	tempDir := t.TempDir()
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Create a local config file
	err = os.WriteFile("config.yaml", []byte("url: https://example.com"), 0644)
	require.NoError(t, err)

	result := configFileExists()
	assert.True(t, result)
}

func TestConfigFileExists_WithLocalConfigYml(t *testing.T) {
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	tempDir := t.TempDir()
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Create a local config.yml file
	err = os.WriteFile("config.yml", []byte("url: https://example.com"), 0644)
	require.NoError(t, err)

	result := configFileExists()
	assert.True(t, result)
}

func TestConfigFileExists_WithHomeConfig(t *testing.T) {
	originalHome := os.Getenv("HOME")
	defer func() {
		_ = os.Setenv("HOME", originalHome)
	}()

	tempDir := t.TempDir()
	_ = os.Setenv("HOME", tempDir)

	// Create config directory and file in HOME
	configDir := filepath.Join(tempDir, ".wpxmlrpc")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("url: https://example.com"), 0644)
	require.NoError(t, err)

	// Change to empty directory so local config doesn't exist
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(originalWd)
	}()

	emptyDir := t.TempDir()
	err = os.Chdir(emptyDir)
	require.NoError(t, err)

	result := configFileExists()
	assert.True(t, result)
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

func TestInitConfig(t *testing.T) {
	assert.NotPanics(t, func() {
		initConfig()
	})
}

func TestGlobalFlags(t *testing.T) {
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	assert.NotNil(t, configFlag)

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	assert.NotNil(t, verboseFlag)
}
