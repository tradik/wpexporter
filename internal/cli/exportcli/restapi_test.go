package exportcli

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
)

// pre47Client is a client that has discovered the site has no content routes.
func pre47Client(t *testing.T) *api.Client {
	t.Helper()

	// What WordPress before 4.7 actually answers to a wp/v2 address: a 404
	// carrying rest_no_route, in both spellings, because the namespace does not
	// exist rather than the route.
	return probedClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"rest_no_route","message":"No route was found"}`))
	})
}

// probedClient builds a client against a stub and makes it ask the question, so
// a test can look at the answer rather than at the machinery.
func probedClient(t *testing.T, handler http.HandlerFunc) *api.Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := api.NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	_, _ = client.GetPosts()

	return client
}

// TestSiteAPINoticeStaysSilentForOrdinarySites: the case that must cost nothing.
// A site whose API answers normally has no notice to make, and inventing one
// would put a line about REST routes in every report this tool has ever written.
func TestSiteAPINoticeStaysSilentForOrdinarySites(t *testing.T) {
	client := probedClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`[{"id":1,"slug":"hello"}]`))

			return
		}

		_, _ = w.Write([]byte(`[]`))
	})

	assert.Empty(t, siteAPINotice(client))

	var notices []string
	noteSiteAPI(client, &notices)
	assert.Empty(t, notices)
}

// TestSiteAPINoticeNamesTheFallback: an export addressed at ?rest_route= is
// complete, and saying so is the difference between a reader trusting it and a
// reader checking /wp-json/ by hand and concluding the export is of nothing
// (#66).
func TestSiteAPINoticeNamesTheFallback(t *testing.T) {
	client := probedClient(t, func(w http.ResponseWriter, r *http.Request) {
		route := r.URL.Query().Get("rest_route")
		if route == "" {
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(route, "/types") {
			_, _ = w.Write([]byte(`{"post":{"slug":"post"}}`))

			return
		}

		_, _ = w.Write([]byte(`[]`))
	})

	require.True(t, client.UsesRestRouteFallback())

	notice := siteAPINotice(client)
	assert.Contains(t, notice, "?rest_route=")
	assert.NotContains(t, notice, "complete",
		"the collections below say what was read; a note must not conclude for them (#66)")
}

// TestSiteAPINoticeSaysItOnce: the notice lands in metadata.json, and a run that
// asks twice — once after the content, once before the report — must not write
// the same sentence twice into it.
func TestSiteAPINoticeSaysItOnce(t *testing.T) {
	client := pre47Client(t)
	require.True(t, client.RestAPIAbsent())

	var notices []string
	noteSiteAPI(client, &notices)
	noteSiteAPI(client, &notices)

	require.Len(t, notices, 1)
	assert.Contains(t, notices[0], "4.7")
}

// TestFallBackToFeedOnlyWhenThereIsNoAPI: --from-sitemap is the operator's
// decision everywhere except here, where there was nothing for them to decide in
// advance and the alternative is an export of nothing (#68).
func TestFallBackToFeedOnlyWhenThereIsNoAPI(t *testing.T) {
	absent := pre47Client(t)
	require.True(t, absent.RestAPIAbsent())

	cfg := &config.Config{}
	fallBackToFeed(absent, cfg)
	assert.True(t, cfg.FromSitemap, "a site with no content API is read from its feed")

	// The operator asked for no inventory check at all; a fallback that read the
	// feed anyway would be the run overruling them.
	quiet := &config.Config{NoInventoryCheck: true}
	fallBackToFeed(absent, quiet)
	assert.False(t, quiet.FromSitemap)

	// And every ordinary site is left exactly as it was asked for.
	healthy := probedClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	ordinary := &config.Config{}
	fallBackToFeed(healthy, ordinary)
	assert.False(t, ordinary.FromSitemap)
}
