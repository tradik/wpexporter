package exportcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/localdb/localdbtest"
	"github.com/tradik/wpexporter/internal/wpconfig"
)

// Fragments of the limited database reads.
const (
	newestDocs  = "ORDER BY post_date DESC, ID DESC LIMIT ? OFFSET ?"
	newestMedia = "ORDER BY a.post_date DESC, a.ID DESC LIMIT ? OFFSET ?"
)

// expectNewestPost registers a limited read that finds post 2 alone.
func expectNewestPost(budget int) func(sqlmock.Sqlmock) {
	return func(m sqlmock.Sqlmock) {
		localdbtest.ExpectNamedPages(m)
		m.ExpectQuery(newestDocs).WithArgs("post", budget, 0).WillReturnRows(sqlmock.NewRows([]string{"ID",
			"post_author", "post_date", "post_date_gmt", "post_content", "post_title", "post_excerpt", "comment_status",
			"ping_status", "post_name", "post_modified", "post_modified_gmt", "post_parent", "guid"}).
			AddRow(2, 1, "2024-05-06 07:08:09", "2024-05-06 05:08:09", "<p>Second</p>", "Second", "", "open", "open",
				"second", "2024-05-07 07:08:09", "2024-05-07 05:08:09", 0, "g"))
		m.ExpectQuery("FROM wp_postmeta WHERE post_id IN").WithArgs(2, `\_%`).
			WillReturnRows(sqlmock.NewRows([]string{"post_id", "meta_key", "meta_value"}))
		m.ExpectQuery("FROM wp_term_relationships tr").WithArgs(2).
			WillReturnRows(sqlmock.NewRows([]string{"object_id", "term_id", "taxonomy", "slug"}))
	}
}

// --limit-posts in local mode: the database is asked for the newest post and
// no more, the summary says what was left, and media narrows to what the
// exported documents reference — as on the REST path.
func TestLocalExportLimited(t *testing.T) {
	var opened wpconfig.Settings
	rec := mockDatabaseWith(t, &opened, expectNewestPost(1))
	var seen []string
	srv := fakeOrigin(t, &seen)
	out := filepath.Join(t.TempDir(), "out")

	log, err := runWithFlags(t, map[string]string{"local": localSite(t), "origin": srv.URL, "format": "json",
		"output": out, "limit-posts": "1", "retries": "0", "timeout": "5"})
	require.NoError(t, err, log)

	data := readExport(t, out)
	require.Len(t, data.Posts, 1)
	assert.Equal(t, "second", data.Posts[0].Slug)
	assert.Len(t, data.Pages, 4, "only posts were limited")
	assert.Contains(t, log, "Posts: 1 (limited from 2)")
	assert.Contains(t, log, "Limiting media to what the exported documents reference (--limit)")
	assert.Empty(t, data.Media, "nothing the one post references")
	assert.Empty(t, rec.Violations())
}

// One --limit across the database and the REST client: products come after
// posts and pages, and a total already spent is not granted again.
func TestLocalExportLimitIsShared(t *testing.T) {
	var opened wpconfig.Settings
	mockDatabaseWith(t, &opened, expectNewestPost(1))
	var seen []string
	srv := fakeOrigin(t, &seen)
	out := filepath.Join(t.TempDir(), "out")

	log, err := runWithFlags(t, map[string]string{"local": localSite(t), "origin": srv.URL, "format": "json",
		"output": out, "limit": "1", "retries": "0", "timeout": "5"})
	require.NoError(t, err, log)

	data := readExport(t, out)
	assert.Len(t, data.Posts, 1)
	assert.Empty(t, data.Pages)
	assert.Empty(t, data.CustomTypes)
	assert.Contains(t, log, "Pages: 0 (none within --limit)")
	for _, s := range seen {
		assert.NotContains(t, s, "/wc/", "the products walk did not start: the total was spent")
		assert.NotContains(t, s, "/store", "the products walk did not start: the total was spent")
	}
}

// --cache in local mode caches what the crawl read through the origin: the
// second run takes the rendered pages from the cache.
func TestLocalExportCache(t *testing.T) {
	site := localSite(t)
	cacheDir := filepath.Join(t.TempDir(), "cache")
	crawled := func(seen []string) []string {
		var pages []string
		for _, s := range seen {
			path := strings.TrimPrefix(s, "example.com ")
			if !strings.Contains(path, "?") && !strings.HasPrefix(path, "/wp-json/") && path != "/" {
				pages = append(pages, path)
			}
		}
		return pages
	}

	run := func() ([]string, string) {
		var opened wpconfig.Settings
		mockDatabase(t, &opened)
		var seen []string
		srv := fakeOrigin(t, &seen)
		log, err := runWithFlags(t, map[string]string{"local": site, "origin": srv.URL, "format": "json",
			"output": filepath.Join(t.TempDir(), "out"), "cache": "true", "cache-dir": cacheDir,
			"no-media": "true", "retries": "0", "timeout": "5"})
		require.NoError(t, err, log)
		return crawled(seen), log
	}

	first, log := run()
	assert.Contains(t, log, "Cache enabled")
	assert.Contains(t, first, "/2024/05/hello-world/", "the first run crawls")
	entries, err := os.ReadDir(cacheDir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	second, _ := run()
	assert.NotContains(t, second, "/2024/05/hello-world/", "the second run reads the crawl from the cache")
}

// The cache settings are checked in local mode as on the REST path.
func TestLocalExportCacheRefusal(t *testing.T) {
	noOrigin(t)
	var opened wpconfig.Settings
	mockDatabase(t, &opened)
	_, err := runWithFlags(t, map[string]string{"local": localSite(t), "cache": "true", "cache-ttl": "soon",
		"cache-dir": t.TempDir()})
	assert.ErrorContains(t, err, "invalid cache TTL")
}
