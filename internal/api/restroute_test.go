package api

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

// restRouteSite serves a WordPress that only answers the ?rest_route= spelling:
// /wp-json/ is a 404, as it is on a site with plain permalinks or a plugin that
// hides the pretty route.
func restRouteSite(t *testing.T, pretty *atomic.Int32) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := r.URL.Query().Get("rest_route")
		if route == "" {
			pretty.Add(1)
			w.WriteHeader(http.StatusNotFound)

			return
		}

		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(route, "/types"):
			_, _ = w.Write([]byte(`{"post":{"slug":"post"}}`))
		case strings.HasSuffix(route, "/posts") && r.URL.Query().Get("page") == "1":
			_, _ = w.Write([]byte(onePost))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	return client
}

// TestRestRouteFallbackReadsTheSite: the failure #66 was reported as. Every
// request 404s, the export stops with a message about categories, and the site
// is serving its whole API one question mark away.
func TestRestRouteFallbackReadsTheSite(t *testing.T) {
	var pretty atomic.Int32
	client := restRouteSite(t, &pretty)

	posts, err := client.GetPosts()
	require.NoError(t, err)
	require.Len(t, posts, 1)

	assert.True(t, client.UsesRestRouteFallback(), "the run settled on the spelling that answers")
	assert.False(t, client.RestAPIAbsent(), "the API is there, spelled differently")

	prettyAfterDiscovery := pretty.Load()

	// The discovery is spent once. A second collection is addressed correctly
	// from its first request, so a large site does not pay a 404 per page.
	_, err = client.GetPages()
	require.NoError(t, err)
	assert.Equal(t, prettyAfterDiscovery, pretty.Load(), "nothing asks the pretty route again")
}

// TestRestRouteFallbackJoinsTheQuery: the trap in the fallback spelling. Its
// route is already a query parameter, so a page number appended with "?" makes
// WordPress read `\/wp\/v2\/posts?page=1` as one route name and answer 404 — the
// fallback would look broken in exactly the way it was added to fix.
func TestRestRouteFallbackJoinsTheQuery(t *testing.T) {
	client := restRouteClient(t)

	assert.Equal(t, "https://x.test/?rest_route=/wp/v2/posts&page=2&per_page=100",
		client.endpointURL("posts", "page=2&per_page=100"))
	assert.Equal(t, "https://x.test/?rest_route=/wp/v2/types", client.endpointURL("types", ""))
	assert.Equal(t, "https://x.test/?rest_route=/", client.apiRootURL(),
		"the API index has no namespace, so it is not addressed with one")
}

// TestPrettyRouteIsUnchanged: backward compatibility. Nearly every site answers
// the pretty spelling, and the addresses it is asked for must be the ones seven
// releases of this exporter have been asking for.
func TestPrettyRouteIsUnchanged(t *testing.T) {
	client, err := NewClient(&config.Config{URL: "https://x.test", Timeout: 5})
	require.NoError(t, err)

	assert.Equal(t, "https://x.test/wp-json/wp/v2/posts?page=2&per_page=100",
		client.endpointURL("posts", "page=2&per_page=100"))
	assert.Equal(t, "https://x.test/wp-json/wp/v2/media", client.endpointURL("media", ""))
	assert.Equal(t, "https://x.test/wp-json", client.apiRootURL())
	assert.False(t, client.UsesRestRouteFallback())
	assert.False(t, client.RestAPIAbsent())
}

// restRouteClient is a client that has already settled on the fallback, for
// asking about addresses without a server to answer them.
func restRouteClient(t *testing.T) *Client {
	t.Helper()

	client, err := NewClient(&config.Config{URL: "https://x.test", Timeout: 5})
	require.NoError(t, err)
	client.probe.fallback = true

	return client
}

// TestHealthySiteNeverProbes: the price of the exception, paid by the rule. A
// site whose API answers is never asked which spelling it uses, because nothing
// it answered was a 404.
func TestHealthySiteNeverProbes(t *testing.T) {
	var types atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/types") || strings.Contains(r.URL.RawQuery, "types") {
			types.Add(1)
		}

		w.Header().Set("Content-Type", "application/json")

		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(onePost))

			return
		}

		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	posts, err := client.GetPosts()
	require.NoError(t, err)
	require.Len(t, posts, 1)

	assert.Zero(t, types.Load(), "a healthy site is never asked the question")
	assert.False(t, client.UsesRestRouteFallback())
}

// TestRestAPIAbsentOnPre47: #68. WordPress gained the wp/v2 content routes in
// 4.7; an older install answers rest_no_route to both spellings, and there is
// nothing to fall back to. Reporting that is the only thing left to do, and it
// is what turns an inexplicably empty export into an explained one.
func TestRestAPIAbsentOnPre47(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// The shape a pre-4.7 site answers with, in both spellings: the
		// namespace does not exist, so there is nothing to fall back to.
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"rest_no_route","message":"No route was found"}`))
		_ = r
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	// The walk fails, as it must — but it fails having learned why.
	_, _ = client.GetPosts()

	assert.True(t, client.RestAPIAbsent(), "neither spelling serves content routes")
	assert.False(t, client.UsesRestRouteFallback(), "there is nothing to fall back to")
	assert.Contains(t, RestAPINotice(), "4.7")
	assert.Contains(t, RestAPINotice(), "--from-sitemap")
}

// TestAnswersContentRejectsARefusal: the 200 that means no. A pre-4.7 site
// answers rest_no_route with a success status, so a probe that trusted the
// status would settle on a spelling that serves nothing.
func TestAnswersContentRejectsARefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(r.URL.String(), "refuse") {
			_, _ = w.Write([]byte(`{"code":"rest_no_route"}`))

			return
		}

		_, _ = w.Write([]byte(`{"post":{"slug":"post"}}`))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	assert.False(t, client.answersContent(server.URL+"/refuse"))
	assert.True(t, client.answersContent(server.URL+"/types"))
	assert.False(t, client.answersContent("http://127.0.0.1:1/types"), "an unreachable address answers nothing")
}

// TestA404OnOneCollectionIsNotAFallback: the probe's third answer, and the one
// that must change nothing. A site can 404 a single route — posts disabled by a
// plugin, a custom type that no longer exists — while its API is perfectly
// healthy, and switching such a site to the fallback spelling would break every
// collection that was working.
func TestA404OnOneCollectionIsNotAFallback(t *testing.T) {
	var restRoute atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("rest_route") != "" {
			restRoute.Add(1)
		}

		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(r.URL.Path, "/types"):
			_, _ = w.Write([]byte(`{"post":{"slug":"post"}}`))
		case strings.HasSuffix(r.URL.Path, "/posts"):
			w.WriteHeader(http.StatusNotFound)
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	_, err = client.GetPosts()
	require.Error(t, err, "the route really is not there, and the run says so")

	assert.False(t, client.UsesRestRouteFallback(), "the pretty spelling answers; nothing moves")
	assert.False(t, client.RestAPIAbsent())
	assert.Zero(t, restRoute.Load(), "the fallback is never even asked once the pretty form answers")

	// And the collections that do exist are still addressed the way they were.
	pages, err := client.GetPages()
	require.NoError(t, err)
	assert.Empty(t, pages)
}
