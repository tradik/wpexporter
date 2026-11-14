package media

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// uploadsFixture is a WordPress uploads directory with one image, a secret
// outside it and a symlink pointing at the secret.
func uploadsFixture(t *testing.T) (*Downloader, string) {
	t.Helper()
	root := t.TempDir()
	uploads := filepath.Join(root, "wp-content", "uploads")
	require.NoError(t, os.MkdirAll(filepath.Join(uploads, "2024", "05"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(uploads, "2024", "05", "photo one.jpg"), []byte("jpeg"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "wp-config.php"), []byte("<?php secret"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(root, "wp-config.php"), filepath.Join(uploads, "leak.jpg")))
	require.NoError(t, os.MkdirAll(filepath.Join(uploads, "adir"), 0o750))

	cfg := config.DefaultConfig()
	cfg.URL = "https://example.com"
	cfg.Output = t.TempDir()
	cfg.UploadsURL = "https://example.com/wp-content/uploads/"
	cfg.UploadsDir = uploads
	cfg.Verbose = true
	d := NewDownloader(cfg)
	require.NoError(t, os.MkdirAll(d.mediaDir, 0o750))
	return d, uploads
}

func TestLocate(t *testing.T) {
	d, uploads := uploadsFixture(t)
	base := "https://example.com/wp-content/uploads/"

	path, where := d.locate(base + "2024/05/photo%20one.jpg?ver=2")
	assert.Equal(t, onDisk, where)
	assert.Equal(t, filepath.Join(uploads, "2024", "05", "photo one.jpg"), path)

	for name, u := range map[string]string{
		"dot-dot":      base + "../../wp-config.php",
		"encoded":      base + "%2e%2e/%2e%2e/wp-config.php",
		"symlink out":  base + "leak.jpg",
		"bad escaping": base + "%zz.jpg",
	} {
		_, where := d.locate(u)
		assert.Equal(t, refused, where, name)
	}

	for name, u := range map[string]string{
		"elsewhere":        "https://cdn.example.net/x.jpg",
		"missing":          base + "2024/05/gone.jpg",
		"prefix lookalike": "https://example.com/wp-content/uploads-old/x.jpg",
	} {
		_, where := d.locate(u)
		assert.Equal(t, notLocal, where, name)
	}

	d.config.UploadsDir = filepath.Join(uploads, "no-such-dir")
	_, where = d.locate(base + "2024/05/photo%20one.jpg")
	assert.Equal(t, notLocal, where, "no uploads directory")

	d.config.UploadsURL = ""
	_, where = d.locate(base + "2024/05/photo%20one.jpg")
	assert.Equal(t, notLocal, where, "not local mode")
}

// The ordinary download path copies from disk, refuses an escape, and keeps
// the size limit.
func TestDownloadFileFromDisk(t *testing.T) {
	d, _ := uploadsFixture(t)
	base := "https://example.com/wp-content/uploads/"
	target := filepath.Join(d.mediaDir, "photo.jpg")

	require.True(t, d.downloadFile(base+"2024/05/photo%20one.jpg", target))
	data, err := os.ReadFile(target) // #nosec G304 -- test output
	require.NoError(t, err)
	assert.Equal(t, "jpeg", string(data))

	log := captureOutput(func() {
		assert.False(t, d.downloadFile(base+"leak.jpg", filepath.Join(d.mediaDir, "leak.jpg")))
	})
	assert.Contains(t, log, "Refusing media outside the uploads directory")
	assert.NoFileExists(t, filepath.Join(d.mediaDir, "leak.jpg"))

	d.config.MaxMediaBytes = 2
	big := filepath.Join(d.mediaDir, "big.jpg")
	assert.False(t, d.downloadFile(base+"2024/05/photo%20one.jpg", big))
	assert.NoFileExists(t, big, "a partial file is not kept")
}

func TestCopyLocalFailures(t *testing.T) {
	d, uploads := uploadsFixture(t)
	assert.False(t, d.copyLocal(filepath.Join(uploads, "nope.jpg"), filepath.Join(d.mediaDir, "a")), "missing")
	assert.False(t, d.copyLocal(filepath.Join(uploads, "adir"), filepath.Join(d.mediaDir, "b")), "not a file")
	assert.False(t, d.copyLocal(filepath.Join(uploads, "2024", "05", "photo one.jpg"),
		filepath.Join(d.mediaDir, "missing-dir", "c")), "cannot create the target")
}

// captureOutput returns what fn printed to stdout.
func captureOutput(fn func()) string {
	stdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan string)
	go func() {
		buf, _ := io.ReadAll(r)
		done <- string(buf)
	}()
	fn()
	_ = w.Close()
	os.Stdout = stdout
	return <-done
}
