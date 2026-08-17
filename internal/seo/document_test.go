package seo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// TestFetchDocumentBuildsAPageFromWhatTheSitePublishes: the walk #68 needs. A
// WordPress 4.5.3 has no content API in either spelling, and its pages answer
// 200 to anyone who asks — this is what turns one of those answers into a
// document.
func TestFetchDocumentBuildsAPageFromWhatTheSitePublishes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>About us — Knine</title>` +
			`<meta property="og:title" content="About us"></head><body>` +
			`<header>menu</header><main id="content"><p>` +
			`We have trained working dogs since 2011, for families and for handlers.` +
			`</p></main><footer>c</footer></body></html>`))
		_ = r
	}))
	t.Cleanup(server.Close)

	crawler := NewCrawler(&config.Config{Timeout: 5, Concurrent: 1, UserAgent: "test"})

	document, ok := crawler.FetchDocument(server.URL + "/about-knine/")
	require.True(t, ok)

	assert.Equal(t, "About us", document.Title.Rendered,
		"og:title over <title>, which carries the site name on most themes")
	assert.Equal(t, "about-knine", document.Slug)
	assert.Equal(t, "publish", document.Status)
	assert.Contains(t, document.Content.Rendered, "trained working dogs")
	assert.NotContains(t, document.Content.Rendered, "menu", "the chrome is not the page")
}

// TestFetchDocumentRefusesAnEmptyPage: an address in a sitemap can be a
// redirect, a feed or an attachment page. Writing an empty document for one
// would put a file on disk that says the page has no content, which is a
// different claim from "this address had nothing to read".
func TestFetchDocumentRefusesAnEmptyPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body></body></html>`))
		_ = r
	}))
	t.Cleanup(server.Close)

	crawler := NewCrawler(&config.Config{Timeout: 5, Concurrent: 1, UserAgent: "test"})

	_, ok := crawler.FetchDocument(server.URL + "/empty/")
	assert.False(t, ok)
}

// TestDocumentSlug: a site with no IDs to offer is named after its addresses,
// and the root has no segment to be named after.
func TestDocumentSlug(t *testing.T) {
	assert.Equal(t, "about-knine", documentSlug("https://x.test/about-knine/"))
	assert.Equal(t, "gala", documentSlug("https://x.test/events/2026/gala"))
	assert.Equal(t, "home", documentSlug("https://x.test/"))
	assert.Equal(t, "home", documentSlug("https://x.test"))
}

// TestDocumentTitleFallsBackToTheTagTitle: not every theme declares og:title,
// and a document with no title at all is one a migration cannot place.
func TestDocumentTitleFallsBackToTheTagTitle(t *testing.T) {
	assert.Equal(t, "About us", documentTitle(models.SEOData{Title: "About us"}))
	assert.Equal(t, "Shared", documentTitle(models.SEOData{Title: "Tab title", OGTitle: "Shared"}))
	assert.Empty(t, documentTitle(models.SEOData{}))
}

// TestDocumentSlugSurvivesAnUnparseableAddress: a sitemap is a document from
// the site, not a promise, and one bad entry must not end the walk.
func TestDocumentSlugSurvivesAnUnparseableAddress(t *testing.T) {
	assert.Empty(t, documentSlug("://not a url"))
}
