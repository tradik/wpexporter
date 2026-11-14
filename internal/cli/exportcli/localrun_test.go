package exportcli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/localdb"
	"github.com/tradik/wpexporter/internal/localdb/localdbtest"
	"github.com/tradik/wpexporter/internal/origin"
	"github.com/tradik/wpexporter/internal/wpconfig"
)

// dockerConfig is wp-config.php as the official WordPress image writes it.
const dockerConfig = `<?php
if (!function_exists('getenv_docker')) {
	function getenv_docker($env, $default) { return getenv($env) ?: $default; }
}
define( 'DB_NAME', getenv_docker('WORDPRESS_DB_NAME', 'wordpress') );
define( 'DB_USER', getenv_docker('WORDPRESS_DB_USER', 'example username') );
define( 'DB_PASSWORD', getenv_docker('WORDPRESS_DB_PASSWORD', 'example password') );
define( 'DB_HOST', getenv_docker('WORDPRESS_DB_HOST', 'mysql') );
define( 'DB_CHARSET', getenv_docker('WORDPRESS_DB_CHARSET', 'utf8') );
$table_prefix = getenv_docker('WORDPRESS_TABLE_PREFIX', 'wp_');
require_once ABSPATH . 'wp-settings.php';
`

// localSite is a WordPress directory with wp-config.php and an uploads
// directory holding the canned site's two files.
func localSite(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "wp-config.php"), []byte(dockerConfig), 0o600))
	uploads := filepath.Join(root, "wp-content", "uploads", "2024", "05")
	require.NoError(t, os.MkdirAll(uploads, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(uploads, "photo.jpeg"), []byte("jpeg from disk"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(uploads, "brochure.pdf"), []byte("%PDF from disk"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(uploads, "photo-150x150.jpeg"), []byte("thumb from disk"), 0o600))
	return root
}

// mockDatabase replaces the database with the canned site and returns the
// recorder of every statement sent.
func mockDatabase(t *testing.T, opened *wpconfig.Settings) *localdbtest.Recorder {
	t.Helper()
	return mockDatabaseWith(t, opened, nil)
}

// mockDatabaseWith is mockDatabase with expectations registered first, which
// win over the canned site's.
func mockDatabaseWith(t *testing.T, opened *wpconfig.Settings, register func(sqlmock.Sqlmock)) *localdbtest.Recorder {
	t.Helper()
	db, mock, rec := localdbtest.New(t)
	localdbtest.ExpectSnapshot(mock)
	if register != nil {
		register(mock)
	}
	localdbtest.ExpectSite(mock)
	openDatabase = func(ctx context.Context, s *wpconfig.Settings, o localdb.Options) (*localdb.Source, error) {
		*opened = *s
		return localdb.New(ctx, db, s, o)
	}
	t.Cleanup(func() { openDatabase = localdb.Open; _ = db.Close() })
	return rec
}

// fakeOrigin is the site's web server: rendered pages for example.com, and the
// redirects WordPress gives query addresses.
func fakeOrigin(t *testing.T, seen *[]string) *httptest.Server {
	t.Helper()
	var mu sync.Mutex // the crawl and the address lookups run concurrently
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		*seen = append(*seen, r.Host+" "+r.URL.RequestURI())
		mu.Unlock()
		switch {
		case r.URL.Query().Get("post_type") == "portfolio":
			http.Redirect(w, r, "https://example.com/work/bridge/", http.StatusMovedPermanently)
		case r.URL.Query().Get("p") == "2":
			http.Redirect(w, r, "https://example.com/2024/05/second-renamed/", http.StatusMovedPermanently)
		case r.URL.Query().Get("p") != "" || r.URL.Query().Get("page_id") != "":
			w.WriteHeader(http.StatusOK) // the computed address stands
		case strings.HasPrefix(r.URL.Path, "/wp-json/"):
			http.NotFound(w, r)
		default:
			_, _ = fmt.Fprintf(w, `<html><head><title>Rendered %s</title><meta name="description" content="SEO for %s">`+
				`</head><body><main><h1>Rendered</h1><p>Body of %s from the web server.</p></main></body></html>`,
				r.URL.Path, r.URL.Path, r.URL.Path)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// The done-when of phase 4: a JSON export of posts, pages, menus, comments,
// media read from disk and SEO from the crawl, through the origin.
func TestLocalExportEndToEnd(t *testing.T) {
	var opened wpconfig.Settings
	rec := mockDatabase(t, &opened)
	var seen []string
	srv := fakeOrigin(t, &seen)
	root := localSite(t)
	out := filepath.Join(t.TempDir(), "out")

	log, err := runWithFlags(t, map[string]string{"local": root, "origin": srv.URL, "format": "json",
		"output": out, "timeout": "5", "retries": "0"})
	require.NoError(t, err, log)

	// wp-config.php was read in the Docker image's form, without PHP.
	assert.Equal(t, "wordpress", opened.DBName)
	assert.Equal(t, "example password", opened.DBPassword)
	assert.Equal(t, "utf8", opened.DBCharset)

	data := readExport(t, out)
	assert.Equal(t, "Local Site", data.Site.Name)
	require.Len(t, data.Posts, 2)
	assert.Equal(t, "SEO for /2024/05/hello-world/", data.Posts[0].SEO.MetaDescription)
	assert.Contains(t, data.Posts[0].Content.Rendered, "from the web server", "the crawl gives the body")
	assert.Equal(t, "https://example.com/2024/05/second-renamed/", data.Posts[1].Link, "the site's redirect wins")
	require.Len(t, data.Pages, 4)
	require.Len(t, data.CustomTypes, 1)
	assert.Equal(t, "https://example.com/work/bridge/", data.CustomTypes[0].Posts[0].Link)
	require.Len(t, data.Menus, 1)
	assert.Len(t, data.Menus[0].Items, 3)
	require.Len(t, data.Comments, 2)
	assert.Len(t, data.Users, 1)
	assert.Len(t, data.Media, 2)
	assert.Equal(t, 2, data.Stats.MediaDownloaded)

	files, err := filepath.Glob(filepath.Join(out, "media", "*", "*"))
	require.NoError(t, err)
	var contents []string
	for _, f := range files {
		b, _ := os.ReadFile(f) // #nosec G304 -- test output
		contents = append(contents, string(b))
	}
	assert.ElementsMatch(t, []string{"jpeg from disk", "%PDF from disk", "thumb from disk"}, contents, "media came from the disk")

	// Every request for the site reached the origin by the site's name.
	require.NotEmpty(t, seen)
	for _, s := range seen {
		assert.True(t, strings.HasPrefix(s, "example.com "), s)
	}
	assert.Contains(t, log, "Connected to database wordpress (read only, consistent snapshot)")
	assert.NotContains(t, log, "example password")
	assert.Empty(t, rec.Violations())
}

// The second done-when: no web server answers — the export comes from the
// database alone, with the cleaned stored bodies and a note in metadata.json.
func TestLocalExportWithoutOrigin(t *testing.T) {
	var opened wpconfig.Settings
	mockDatabase(t, &opened)
	detectOrigin = func(*config.Config) (origin.Origin, bool) { return origin.Origin{}, false }
	t.Cleanup(func() { detectOrigin = origin.Detect })
	out := filepath.Join(t.TempDir(), "out")

	log, err := runWithFlags(t, map[string]string{"local": localSite(t), "format": "markdown", "output": out})
	require.NoError(t, err, log)
	assert.Contains(t, log, "No web server answered")
	assert.Contains(t, log, "3 published WooCommerce products were not exported")

	raw, err := os.ReadFile(filepath.Join(out, "metadata.json")) // #nosec G304 -- test output
	require.NoError(t, err)
	var meta struct {
		Stats struct {
			Notices []string `json:"notices"`
		} `json:"stats"`
	}
	require.NoError(t, json.Unmarshal(raw, &meta))
	assert.Contains(t, strings.Join(meta.Stats.Notices, "\n"),
		"7 of 7 document(s) carry the body stored in the database rather than the rendered page "+
			"(1 with shortcodes left as written); no web server answered")

	post, err := os.ReadFile(filepath.Join(out, "posts", "parent", "child", "hello-world.md")) // #nosec G304 -- test output
	require.NoError(t, err)
	assert.Contains(t, string(post), "Hello", "the cleaned stored body")
	assert.NotContains(t, string(post), "wp:paragraph", "block delimiters are gone")
}
