package exportcli

// The small things the summary is made of, and the check that runs before a
// long export starts.
//
// None of them are clever, and all of them are read by somebody deciding
// whether the export worked: a size printed wrong reads as a failed download,
// and a permission problem found after forty minutes of crawling is forty
// minutes nobody gets back.

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatFileSize: the units a person reads. The boundary between them is
// where an off-by-one shows up as "1024 B" or "0.0 KB".
func TestFormatFileSize(t *testing.T) {
	for size, want := range map[int64]string{
		0:                "0 B",
		512:              "512 B",
		1023:             "1023 B",
		1024:             "1.0 KB",
		1536:             "1.5 KB",
		1024 * 1024:      "1.0 MB",
		3 * 1024 * 1024:  "3.0 MB",
		1024 * 1024 * 10: "10.0 MB",
	} {
		assert.Equal(t, want, formatFileSize(size), "%d bytes", size)
	}

	assert.Contains(t, formatFileSize(5*1024*1024*1024), "GB")
}

// TestDirectorySizeCountsFilesOnly: the number the summary prints for an
// export, which is the sum of what was written rather than of what was walked.
func TestDirectorySizeCountsFilesOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "pages", "nested"), 0750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "metadata.json"), []byte("12345"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pages", "nested", "a.md"), []byte("12345"), 0600))

	size, err := getDirSize(dir)
	require.NoError(t, err)
	assert.Equal(t, int64(10), size, "both files, no directory entries")

	_, err = getDirSize(filepath.Join(dir, "does-not-exist"))
	assert.Error(t, err)
}

// TestFileSize: the same for a single archive.
func TestFileSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.zip")
	require.NoError(t, os.WriteFile(path, []byte("1234567"), 0600))

	size, err := getFileSize(path)
	require.NoError(t, err)
	assert.Equal(t, int64(7), size)

	_, err = getFileSize(path + ".missing")
	assert.Error(t, err)
}

// TestOutputPermissionsAreCheckedBeforeTheCrawl: an export that cannot write
// where it was told should say so in the first second, not the fortieth minute.
func TestOutputPermissionsAreCheckedBeforeTheCrawl(t *testing.T) {
	base := t.TempDir()

	require.NoError(t, checkOutputPermissions(filepath.Join(base, "export")))
	assert.NoFileExists(t, filepath.Join(base, ".wpexporter_permission_test"),
		"the probe cleans up after itself")

	// A directory nobody may write into is the case the check exists for.
	readOnly := filepath.Join(base, "read-only")
	require.NoError(t, os.MkdirAll(readOnly, 0500))
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0700) })

	if os.Geteuid() == 0 {
		t.Skip("root may write anywhere, so there is nothing to refuse")
	}

	err := checkOutputPermissions(filepath.Join(readOnly, "sub", "export"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output directory")
}

// TestZipArchiveCarriesTheExport: --zip is what an operator hands to somebody
// else, so it has to contain the tree rather than a path prefix of the machine
// it was built on.
func TestZipArchiveCarriesTheExport(t *testing.T) {
	source := filepath.Join(t.TempDir(), "export")
	require.NoError(t, os.MkdirAll(filepath.Join(source, "pages"), 0750))
	require.NoError(t, os.WriteFile(filepath.Join(source, "metadata.json"), []byte(`{"a":1}`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(source, "pages", "about.md"), []byte("# About"), 0600))

	archive := source + ".zip"
	require.NoError(t, createZipArchive(source, archive))

	size, err := getFileSize(archive)
	require.NoError(t, err)
	assert.Positive(t, size)

	names := zipEntries(t, archive)
	assert.Contains(t, names, "metadata.json")
	assert.Contains(t, names, filepath.Join("pages", "about.md"))
	for _, name := range names {
		assert.False(t, filepath.IsAbs(name), "an archive must not carry this machine's paths: %q", name)
	}
}

// TestZipArchiveRefusesAMissingSource: nothing to archive is a mistake worth
// reporting, not an empty file to hand over.
func TestZipArchiveRefusesAMissingSource(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "never-exported")

	assert.Error(t, createZipArchive(missing, missing+".zip"))
}

// zipEntries lists the names inside an archive.
func zipEntries(t *testing.T, path string) []string {
	t.Helper()

	reader, err := zip.OpenReader(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })

	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		names = append(names, file.Name)
	}

	return names
}

// TestCountLineSaysWhenItTruncated: a truncated export that cannot say what it
// truncated is the failure mode of every silent cap (#60).
func TestCountLineSaysWhenItTruncated(t *testing.T) {
	assert.Equal(t, "Posts: 5 (limited from 75)", countLine("Posts", 5, 75, true))
	assert.Equal(t, "Posts: 75", countLine("Posts", 75, 75, true),
		"a limit that did not bite says nothing")
	assert.Equal(t, "Posts: 5", countLine("Posts", 5, 75, false),
		"without a limit the number is simply the number")
	assert.Equal(t, "Pages: 3", countLine("Pages", 3, 0, true),
		"a site that stated no total gives nothing to compare against")
}
