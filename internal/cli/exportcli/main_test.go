package exportcli

import (
	"archive/zip"
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
				dir := filepath.Join(home, ".wpexportjson")
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

func TestGlobalFlags(t *testing.T) {
	// Verify global flags exist on rootCmd
	configFlag := rootCmd.PersistentFlags().Lookup("config")
	assert.NotNil(t, configFlag)

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	assert.NotNil(t, verboseFlag)
}
