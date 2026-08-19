package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// siteServing builds a client against a handler, for asking what the export
// learns about a site's shape.
func siteServing(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	return client
}

// TestFrontPageFromSettings: the authenticated answer, which is the site's own
// record and wins outright.
func TestFrontPageFromSettings(t *testing.T) {
	client := siteServing(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/wp-json/wp/v2/settings":
			_, _ = w.Write([]byte(`{"title":"Serwis","show_on_front":"page",` +
				`"page_on_front":4211,"page_for_posts":827}`))
		case "/wp-json/wp/v2/pages/4211":
			_, _ = w.Write([]byte(`{"id":4211,"slug":"welcome","link":"https://x.test/"}`))
		case "/wp-json/wp/v2/pages/827":
			_, _ = w.Write([]byte(`{"id":827,"slug":"aktualnosci","link":"https://x.test/aktualnosci/"}`))
		default:
			_, _ = w.Write([]byte(`{"name":"Serwis"}`))
		}
	})

	info, err := client.GetSiteInfo()
	require.NoError(t, err)

	assert.Equal(t, "page", info.ShowOnFront)
	require.NotNil(t, info.FrontPage)
	assert.Equal(t, 4211, info.FrontPage.ID)
	assert.Equal(t, "welcome", info.FrontPage.Slug)

	// The half the slug convention could never have found: this site calls its
	// archive `aktualnosci`, and "is there a page called blog?" answers no.
	require.NotNil(t, info.PostsPage)
	assert.Equal(t, 827, info.PostsPage.ID)
	assert.Equal(t, "aktualnosci", info.PostsPage.Slug)
	assert.Equal(t, "https://x.test/aktualnosci/", info.PostsPage.Link)
}

// TestFrontPageFromRenderedMarkup: the unauthenticated answer. WordPress states
// this in the markup of every page it renders, which is a fact the site
// publishes to every visitor and needs no credentials.
func TestFrontPageFromRenderedMarkup(t *testing.T) {
	client := siteServing(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body class="home page page-id-4211 theme-astra">` +
				`<h1>Witamy</h1></body></html>`))
		case "/wp-json/wp/v2/pages/4211":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":4211,"slug":"welcome","link":"https://x.test/"}`))
		case "/wp-json/wp/v2/settings":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"Serwis"}`))
		}
	})

	info, err := client.GetSiteInfo()
	require.NoError(t, err)

	assert.Equal(t, "page", info.ShowOnFront)
	require.NotNil(t, info.FrontPage)
	assert.Equal(t, "welcome", info.FrontPage.Slug)
}

// TestHomeIsTheArchive: the other shape. `home blog` on the body means the home
// *is* the listing, so there is no static front page and no separate posts page
// to look for.
func TestHomeIsTheArchive(t *testing.T) {
	client := siteServing(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body class="home blog logged-out"></body></html>`))

			return
		}

		if r.URL.Path == "/wp-json/wp/v2/settings" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Blog"}`))
	})

	info, err := client.GetSiteInfo()
	require.NoError(t, err)

	assert.Equal(t, "posts", info.ShowOnFront)
	assert.Nil(t, info.FrontPage)
	assert.Nil(t, info.PostsPage)
}

// TestNothingLearnedIsNothingWritten: absent keys mean nobody could work it
// out, which a consumer can tell apart from "there is no posts page". A guessed
// default would read like an answer.
func TestNothingLearnedIsNothingWritten(t *testing.T) {
	client := siteServing(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body>no classes here</body></html>`))

			return
		}

		if r.URL.Path == "/wp-json/wp/v2/settings" {
			w.WriteHeader(http.StatusUnauthorized)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"Serwis"}`))
	})

	info, err := client.GetSiteInfo()
	require.NoError(t, err)

	assert.Empty(t, info.ShowOnFront)
	assert.Nil(t, info.FrontPage)
	assert.Nil(t, info.PostsPage)
}

// TestBodyClassesAndPageID: the two readings the markup answer rests on.
func TestBodyClassesAndPageID(t *testing.T) {
	classes := bodyClasses(`<body id="top" class="home Page PAGE-ID-4211">`)
	assert.True(t, classes["home"])
	assert.True(t, classes["page"], "themes are inconsistent about case")
	assert.Equal(t, 4211, frontPageID(classes))

	assert.Nil(t, bodyClasses(`<body>`))
	assert.Zero(t, frontPageID(map[string]bool{"home": true, "page": true}))
}

// TestAPageTheAPIWillNotNameIsStillRecorded: "the home is page 4211" is worth
// more than nothing, even where the slug and address could not be read.
func TestAPageTheAPIWillNotNameIsStillRecorded(t *testing.T) {
	client := siteServing(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	var page *models.SitePage
	client.namePage(&page, 4211)

	require.NotNil(t, page)
	assert.Equal(t, 4211, page.ID)
	assert.Empty(t, page.Slug)

	// And an id the settings did not carry names nothing at all.
	var absent *models.SitePage
	client.namePage(&absent, 0)
	assert.Nil(t, absent)
}
