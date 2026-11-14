package exportcli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The REST export end to end. Written before runExport was split into stages,
// and unchanged since: it is what proves the split changed no behavior.
func TestRESTExportEndToEnd(t *testing.T) {
	srv := fakeWordPress(t)
	out := filepath.Join(t.TempDir(), "out")

	log, err := runWithFlags(t, map[string]string{
		"url": srv.URL, "format": "json", "output": out, "retries": "0", "timeout": "5",
		"assisted-crawl": "true",
	})
	require.NoError(t, err, log)

	data := readExport(t, out)
	assert.Equal(t, "Fake Site", data.Site.Name)
	require.Len(t, data.Posts, 1)
	assert.Equal(t, "Hello", data.Posts[0].Title.Rendered)
	assert.Equal(t, "Desc", data.Posts[0].SEO.MetaDescription, "crawled")
	require.Len(t, data.Pages, 1)
	assert.Len(t, data.Categories, 1)
	assert.Len(t, data.Users, 1)
	assert.Len(t, data.Comments, 1)
	assert.Len(t, data.Media, 1)
	assert.Equal(t, 1, data.Stats.MediaDownloaded)
	assert.Empty(t, data.Menus)

	assert.Contains(t, log, "Fetching posts...")
	assert.Contains(t, log, "=== Export Summary ===")
	assert.Contains(t, log, "Menus are not readable on this site")
}

// The same site with most collections switched off, flat HTML and a ZIP.
func TestRESTExportSkipsAndZip(t *testing.T) {
	srv := fakeWordPress(t)
	out := filepath.Join(t.TempDir(), "out")

	log, err := runWithFlags(t, map[string]string{
		"url": srv.URL, "format": "json", "output": out, "retries": "0", "timeout": "5",
		"no-posts": "true", "no-pages": "true", "no-products": "true", "no-tags": "true", "no-users": "true",
		"no-menus": "true", "no-comments": "true", "no-media": "true", "no-inventory-check": "true",
		"no-custom-types": "true", "zip": "true", "flat-html": "true", "crawl-content": "true",
	})
	require.NoError(t, err, log)
	assert.FileExists(t, out+".zip")
	for _, line := range []string{"Skipping posts", "Skipping pages", "Skipping products", "Skipping tags",
		"Skipping users", "Skipping menus", "Skipping media", "ZIP:"} {
		assert.Contains(t, log, line)
	}
}

func TestRESTExportRefusals(t *testing.T) {
	for name, flags := range map[string]map[string]string{
		"no-files without zip": {"url": "http://127.0.0.1:1", "no-files": "true"},
		"flat and basic":       {"url": "http://127.0.0.1:1", "flat-html": "true", "basic-html": "true"},
		"no url":               {},
		"bad origin":           {"url": "http://127.0.0.1:1", "origin": "ftp://x"},
	} {
		_, err := runWithFlags(t, flags)
		assert.Error(t, err, name)
	}
}
