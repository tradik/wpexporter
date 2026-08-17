package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// TestSitemapFoundThroughRobots: an SEO plugin can put the sitemap at a path
// nobody would guess, and every one of them writes the address into robots.txt
// — the file that exists to answer this question. Guessing three paths and
// giving up reported such a site as publishing no inventory at all.
func TestSitemapFoundThroughRobots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			_, _ = w.Write([]byte("User-agent: *\nDisallow:\n\nSitemap: " +
				"http://" + r.Host + "/custom/seo-sitemap.xml\n"))
		case "/custom/seo-sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<?xml version="1.0"?><urlset><url><loc>https://x.test/a/</loc></url></urlset>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	inventory := client.FetchInventory()
	assert.Contains(t, inventory.Sitemap, "/custom/seo-sitemap.xml")
	assert.Equal(t, []string{"https://x.test/a/"}, inventory.SitemapURLs)
}

// TestFeedFoundWhereTheSiteSaysItIs: /feed/ is a permalink, and a site with
// permalinks set to plain — the same site that serves its REST API only at
// ?rest_route= — has no such address. Its home page declares the feed it does
// have, which is how every feed reader in existence finds one.
func TestFeedFoundWhereTheSiteSaysItIs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.URL.RawQuery == "feed=rss2" {
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = w.Write([]byte(`<?xml version="1.0"?><rss><channel>` +
				`<item><title>Hello</title><link>https://x.test/hello/</link></item>` +
				`</channel></rss>`))

			return
		}

		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><link rel="alternate" ` +
				`type="application/rss+xml" title="Feed" href="/?feed=rss2"></head></html>`))

			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	inventory := client.FetchInventory()
	assert.Contains(t, inventory.Feed, "feed=rss2")
	require.Len(t, inventory.FeedPosts, 1)
	assert.Equal(t, "https://x.test/hello/", inventory.FeedURLs[0])
}

// TestFeedsDeclaredIn: both attribute orders occur in the wild, a comment feed
// is not the main one, and a stylesheet link is not a feed.
func TestFeedsDeclaredIn(t *testing.T) {
	html := `<link rel="stylesheet" href="/style.css">` +
		`<link type="application/rss+xml" rel="alternate" href="https://x.test/feed/">` +
		`<link rel="alternate" type="application/atom+xml" href="//x.test/atom/">` +
		`<link rel="alternate" type="text/html" href="/fr/">`

	assert.Equal(t,
		[]string{"https://x.test/feed/", "https://x.test/atom/"},
		feedsDeclaredIn(html, "https://x.test"))
}

// TestAbsoluteAddress: a theme writes the address however it likes, and every
// spelling has to end up fetchable.
func TestAbsoluteAddress(t *testing.T) {
	assert.Equal(t, "https://x.test/feed/", absoluteAddress("https://x.test/feed/", "https://x.test"))
	assert.Equal(t, "https://x.test/feed/", absoluteAddress("/feed/", "https://x.test"))
	assert.Equal(t, "https://x.test/feed/", absoluteAddress("//x.test/feed/", "https://x.test"))
	assert.Equal(t, "https://x.test/feed/", absoluteAddress("feed/", "https://x.test"))
}

// TestRobotsIsOnlyAskedWhenNeeded: the cost of the fallback, not paid by the
// sites that do not need it.
func TestRobotsIsOnlyAskedWhenNeeded(t *testing.T) {
	var robots int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			robots++
		}

		if r.URL.Path == "/wp-sitemap.xml" {
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<?xml version="1.0"?><urlset><url><loc>https://x.test/a/</loc></url></urlset>`))

			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	require.NotEmpty(t, client.FetchInventory().SitemapURLs)
	assert.Zero(t, robots, "the first address answered; nothing else was asked")
}
