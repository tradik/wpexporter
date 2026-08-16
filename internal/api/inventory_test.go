package api

// Reading what the site says it publishes (#40). Neither document is required,
// so the shape of this test is mostly "the site does not have one".

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

const sitemapIndexBody = `<?xml version="1.0"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>%s/wp-sitemap-posts-post-1.xml</loc></sitemap>
  <sitemap><loc>%s/wp-sitemap-posts-page-1.xml</loc></sitemap>
</sitemapindex>`

const urlsetBody = `<?xml version="1.0"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://x.test/blog/hello/</loc></url>
  <url><loc>https://x.test/events/gala/</loc></url>
</urlset>`

const feedBody = `<?xml version="1.0"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/"
     xmlns:dc="http://purl.org/dc/elements/1.1/"><channel>
  <title>x</title>
  <item>
    <title>Hello</title>
    <link>https://x.test/blog/hello/</link>
    <pubDate>Fri, 01 Mar 2024 10:00:00 +0000</pubDate>
    <dc:creator>Ewa</dc:creator>
    <description>An excerpt.</description>
    <content:encoded>&lt;p&gt;The whole post.&lt;/p&gt;</content:encoded>
  </item>
</channel></rss>`

func newInventoryClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	return client
}

// TestFetchInventoryFollowsTheIndex: WordPress's own sitemap is an index, so
// reading only the first document would report a site as publishing nothing.
func TestFetchInventoryFollowsTheIndex(t *testing.T) {
	var base string

	client := newInventoryClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/wp-sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(strings.ReplaceAll(sitemapIndexBody, "%s", base)))
		case strings.HasPrefix(r.URL.Path, "/wp-sitemap-posts-"):
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(urlsetBody))
		case r.URL.Path == "/feed/":
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(feedBody))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	base = strings.TrimSuffix(client.baseURL, "/wp-json/wp/v2")

	inventory := client.FetchInventory()

	assert.True(t, inventory.Published())
	assert.Contains(t, inventory.Sitemap, "/wp-sitemap.xml")
	assert.ElementsMatch(t,
		[]string{"https://x.test/blog/hello/", "https://x.test/events/gala/"},
		inventory.SitemapURLs, "the same URL listed twice is one address")
	assert.Equal(t, []string{"https://x.test/blog/hello/"}, inventory.FeedURLs)
	assert.Contains(t, inventory.Describe(), "sitemap (2 URLs) and feed (1 items)")
}

// TestFetchInventoryIgnoresAHomePage: a site with no sitemap answers the
// request with its home page and a 200. Believing that would report every page
// of the site as an uncovered URL.
func TestFetchInventoryIgnoresAHomePage(t *testing.T) {
	client := newInventoryClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>Not a sitemap</body></html>"))
	})

	inventory := client.FetchInventory()

	assert.False(t, inventory.Published())
	assert.Empty(t, inventory.SitemapURLs)
	assert.Equal(t, "no sitemap or feed published", inventory.Describe())
}

// TestFetchInventoryTriesTheOtherSpellings: core writes wp-sitemap.xml, Yoast
// and RankMath write the other two.
func TestFetchInventoryTriesTheOtherSpellings(t *testing.T) {
	var asked atomic.Int32

	client := newInventoryClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sitemap_index.xml" {
			asked.Add(1)
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(urlsetBody))

			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	inventory := client.FetchInventory()

	assert.Equal(t, int32(1), asked.Load())
	assert.Contains(t, inventory.Sitemap, "/sitemap_index.xml")
	assert.Len(t, inventory.SitemapURLs, 2)
	assert.Empty(t, inventory.FeedURLs, "a site can publish one inventory and not the other")
}

// TestFetchInventorySurvivesRubbish: malformed XML, an empty feed and a refused
// request are all "no inventory", never an error — the export is not this
// check's to fail.
func TestFetchInventorySurvivesRubbish(t *testing.T) {
	client := newInventoryClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wp-sitemap.xml":
			_, _ = w.Write([]byte("<urlset><url><loc>truncated"))
		case "/feed/":
			_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel></channel></rss>`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	inventory := client.FetchInventory()

	assert.False(t, inventory.Published())
}

// TestLooksLikeSitemap examines only the head of a body, because a sitemap
// declares itself in its first element and a large one is not worth scanning.
func TestLooksLikeSitemap(t *testing.T) {
	assert.True(t, looksLikeSitemap([]byte(`<?xml version="1.0"?><urlset>`)))
	assert.True(t, looksLikeSitemap([]byte(`<?xml version="1.0"?><SITEMAPINDEX>`)))
	assert.False(t, looksLikeSitemap([]byte("<!DOCTYPE html><html>")))
	assert.False(t, looksLikeSitemap([]byte(strings.Repeat("x", 600)+"<urlset>")))
}

// TestInventoryDescribe: a report has to be able to say where its numbers came
// from, including when only one of the two documents answered.
func TestInventoryDescribe(t *testing.T) {
	assert.Equal(t, "sitemap (2 URLs)",
		Inventory{Sitemap: "s", SitemapURLs: []string{"a", "b"}}.Describe())
	assert.Equal(t, "feed (1 items)",
		Inventory{Feed: "f", FeedURLs: []string{"a"}}.Describe())
	assert.True(t, Inventory{Feed: "f"}.Published())
}

// TestFeedItemsBecomeRecords: on a site whose REST routes are broken, the feed
// is the only copy of the content left to read (#40). What it can give is
// given; what it cannot — IDs, terms, featured images — is left plainly empty
// rather than invented.
func TestFeedItemsBecomeRecords(t *testing.T) {
	var base string

	client := newInventoryClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/feed/" {
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(feedBody))

			return
		}

		w.WriteHeader(http.StatusNotFound)
	})
	base = strings.TrimSuffix(client.baseURL, "/wp-json/wp/v2")
	_ = base

	inventory := client.FetchInventory()
	require.Len(t, inventory.FeedPosts, 1)

	post := inventory.FeedPosts[0]
	assert.Equal(t, "Hello", post.Title.Rendered)
	assert.Equal(t, "https://x.test/blog/hello/", post.Link)
	assert.Equal(t, "hello", post.Slug, "the last path segment is the slug on every permalink structure")
	assert.Equal(t, "<p>The whole post.</p>", post.Content.Rendered)
	assert.Equal(t, "An excerpt.", post.Excerpt.Rendered)
	assert.Equal(t, 2024, post.Date.Year())
	assert.Equal(t, "publish", post.Status)
	assert.Zero(t, post.ID, "nothing may mistake a recovered record for one WordPress numbered")
}

// TestFeedDatesAreReadInEveryUsualSpelling: WordPress writes RFC1123 with a
// numeric zone; caches and plugins rewrite it.
func TestFeedDatesAreReadInEveryUsualSpelling(t *testing.T) {
	for _, raw := range []string{
		"Fri, 01 Mar 2024 10:00:00 +0000",
		"Fri, 01 Mar 2024 10:00:00 UTC",
		"2024-03-01T10:00:00Z",
	} {
		parsed, err := parseFeedDate(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, 2024, parsed.Year())
	}

	_, err := parseFeedDate("last Tuesday")
	assert.Error(t, err, "a date nobody can read is left unset rather than guessed")
}

// TestSlugFromURL is what addresses a recovered record.
func TestSlugFromURL(t *testing.T) {
	assert.Equal(t, "hello", slugFromURL("https://x.test/blog/hello/"))
	assert.Equal(t, "hello", slugFromURL("https://x.test/hello"))
	assert.Equal(t, "", slugFromURL("https://x.test/"))
	assert.Equal(t, "", slugFromURL("://not a url"))
}
