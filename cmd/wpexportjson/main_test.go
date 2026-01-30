package main

import (
	"archive/zip"
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
	// Change to a temp directory
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
	configDir := filepath.Join(tempDir, ".wpexportjson")
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

func TestCreateZipArchive_Success(t *testing.T) {
	// Create a temp source directory with some files
	sourceDir := t.TempDir()

	// Create some files
	err := os.WriteFile(filepath.Join(sourceDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)

	subDir := filepath.Join(sourceDir, "subdir")
	err = os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(subDir, "file2.txt"), []byte("content2"), 0644)
	require.NoError(t, err)

	// Create zip in a different temp directory
	targetDir := t.TempDir()
	zipPath := filepath.Join(targetDir, "archive.zip")

	err = createZipArchive(sourceDir, zipPath)
	require.NoError(t, err)

	// Verify zip was created
	assert.FileExists(t, zipPath)

	// Verify zip contents
	zipReader, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer func() {
		_ = zipReader.Close()
	}()

	fileNames := make(map[string]bool)
	for _, f := range zipReader.File {
		fileNames[f.Name] = true
	}

	assert.True(t, fileNames["file1.txt"])
	assert.True(t, fileNames["subdir/"])
	assert.True(t, fileNames["subdir/file2.txt"])
}

func TestCreateZipArchive_EmptyDirectory(t *testing.T) {
	sourceDir := t.TempDir()
	targetDir := t.TempDir()
	zipPath := filepath.Join(targetDir, "empty.zip")

	err := createZipArchive(sourceDir, zipPath)
	require.NoError(t, err)

	assert.FileExists(t, zipPath)
}

func TestCreateZipArchive_TargetInsideSource(t *testing.T) {
	sourceDir := t.TempDir()
	zipPath := filepath.Join(sourceDir, "archive.zip")

	err := createZipArchive(sourceDir, zipPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "target zip cannot be inside source directory")
}

func TestCreateZipArchive_NonExistentSource(t *testing.T) {
	sourceDir := "/nonexistent/path/12345"
	targetDir := t.TempDir()
	zipPath := filepath.Join(targetDir, "archive.zip")

	err := createZipArchive(sourceDir, zipPath)
	assert.Error(t, err)
}

func TestCreateZipArchive_InvalidTargetPath(t *testing.T) {
	sourceDir := t.TempDir()
	// Try to create zip in a non-existent directory
	zipPath := "/nonexistent/path/12345/archive.zip"

	err := createZipArchive(sourceDir, zipPath)
	assert.Error(t, err)
}

func TestCreateZipArchive_WithNestedDirectories(t *testing.T) {
	sourceDir := t.TempDir()

	// Create nested structure
	deepDir := filepath.Join(sourceDir, "a", "b", "c")
	err := os.MkdirAll(deepDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(deepDir, "deep.txt"), []byte("deep content"), 0644)
	require.NoError(t, err)

	targetDir := t.TempDir()
	zipPath := filepath.Join(targetDir, "nested.zip")

	err = createZipArchive(sourceDir, zipPath)
	require.NoError(t, err)

	// Verify nested structure is preserved
	zipReader, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer func() {
		_ = zipReader.Close()
	}()

	fileNames := make(map[string]bool)
	for _, f := range zipReader.File {
		fileNames[f.Name] = true
	}

	assert.True(t, fileNames["a/"])
	assert.True(t, fileNames["a/b/"])
	assert.True(t, fileNames["a/b/c/"])
	assert.True(t, fileNames["a/b/c/deep.txt"])
}

func TestRootCmd_Execute(t *testing.T) {
	// Test that root command exists and has expected properties
	assert.Equal(t, "wpexportjson", rootCmd.Use)
	assert.NotEmpty(t, rootCmd.Short)
	assert.NotEmpty(t, rootCmd.Long)
}

func TestExportCmd_RequiredFlags(t *testing.T) {
	// Verify export command has url as required flag
	urlFlag := exportCmd.Flags().Lookup("url")
	assert.NotNil(t, urlFlag)

	// Check other flags exist
	assert.NotNil(t, exportCmd.Flags().Lookup("output"))
	assert.NotNil(t, exportCmd.Flags().Lookup("format"))
	assert.NotNil(t, exportCmd.Flags().Lookup("brute-force"))
	assert.NotNil(t, exportCmd.Flags().Lookup("download-media"))
	assert.NotNil(t, exportCmd.Flags().Lookup("no-media"))
	assert.NotNil(t, exportCmd.Flags().Lookup("relevant-media-only"))
	assert.NotNil(t, exportCmd.Flags().Lookup("concurrent"))
	assert.NotNil(t, exportCmd.Flags().Lookup("zip"))
	assert.NotNil(t, exportCmd.Flags().Lookup("no-files"))
	assert.NotNil(t, exportCmd.Flags().Lookup("auth-user"))
	assert.NotNil(t, exportCmd.Flags().Lookup("auth-pass"))
	assert.NotNil(t, exportCmd.Flags().Lookup("auth-token"))
	assert.NotNil(t, exportCmd.Flags().Lookup("path-filter"))
	assert.NotNil(t, exportCmd.Flags().Lookup("assisted-crawl"))
}

func TestExportCmd_FlagDefaults(t *testing.T) {
	formatFlag := exportCmd.Flags().Lookup("format")
	assert.Equal(t, "json", formatFlag.DefValue)

	concurrentFlag := exportCmd.Flags().Lookup("concurrent")
	assert.Equal(t, "5", concurrentFlag.DefValue)

	downloadMediaFlag := exportCmd.Flags().Lookup("download-media")
	assert.Equal(t, "true", downloadMediaFlag.DefValue)

	bruteForceFlag := exportCmd.Flags().Lookup("brute-force")
	assert.Equal(t, "false", bruteForceFlag.DefValue)

	maxIDFlag := exportCmd.Flags().Lookup("max-id")
	assert.Equal(t, "10000", maxIDFlag.DefValue)
}

func TestInitConfig(t *testing.T) {
	// initConfig should not panic
	assert.NotPanics(t, func() {
		initConfig()
	})
}

func TestGlobalFlags(t *testing.T) {
	// Verify global flags exist on rootCmd
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	assert.NotNil(t, configFlag)

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	assert.NotNil(t, verboseFlag)
}
