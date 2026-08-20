package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// cloudflareBlockPage is what a wall serves to a client calling itself
// WordPress-Export-JSON/1.0 — sometimes with a 403, and sometimes, as here,
// with a 200 that the parser met as an unexpected '<'.
const cloudflareBlockPage = `<!DOCTYPE html><html><head><title>Attention Required!</title></head>` +
	`<body>Error code 1020<br>Ray ID: 8a1f</body></html>`

// TestA200OfHTMLIsNamed: the line this replaces was "failed to parse response:
// invalid character '<' looking for beginning of value" — a true sentence that
// helps nobody, and the reason #73's reporter was told their five products
// "published none".
func TestA200OfHTMLIsNamed(t *testing.T) {
	client := siteServing(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(cloudflareBlockPage))
	})

	_, err := client.GetPublicProducts()
	require.Error(t, err)

	assert.ErrorIs(t, err, ErrNotJSON)
	assert.Contains(t, err.Error(), "Cloudflare")
	assert.NotContains(t, err.Error(), "invalid character",
		"the parser's complaint is underneath, not in the operator's way")
}

// TestPlainHTMLWithoutAWall: a WordPress with no REST API answers its own home
// page to any /wp-json/ address, because to it that is a URL like any other.
// There is no wall to name, and saying so is still better than an unexpected
// '<'.
func TestPlainHTMLWithoutAWall(t *testing.T) {
	client := siteServing(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body><h1>Home</h1></body></html>`))
	})

	_, err := client.GetPublicProducts()
	require.Error(t, err)

	assert.ErrorIs(t, err, ErrNotJSON)
	assert.Contains(t, err.Error(), "served by the site")
}

// TestMalformedJSONIsStillAParseError: a plugin printing a notice before the
// payload leaves a document that is neither, and the parser's own words are the
// right ones for it.
func TestMalformedJSONIsStillAParseError(t *testing.T) {
	client := siteServing(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"broken": `))
	})

	_, err := client.GetPublicProducts()
	require.Error(t, err)

	assert.False(t, errors.Is(err, ErrNotJSON))
	assert.Contains(t, err.Error(), "failed to parse response")
}

// TestLooksLikeHTML: what counts as a page rather than a payload.
func TestLooksLikeHTML(t *testing.T) {
	assert.True(t, looksLikeHTML([]byte("  <!DOCTYPE html>")))
	assert.True(t, looksLikeHTML([]byte("<html>")))
	assert.True(t, looksLikeHTML([]byte("<br />\n<html>")), "a PHP notice ahead of the page")
	assert.False(t, looksLikeHTML([]byte(`[{"id":1}]`)))
	assert.False(t, looksLikeHTML([]byte(`{"code":"rest_no_route"}`)))
	assert.False(t, looksLikeHTML(nil))
}

// TestNoProductsSaysWhatHappened: the report may state what happened; it may
// not state what it concluded. "published none" is a claim about the shop, and
// it may only be made when the route answered with an empty collection (#65,
// #73).
func TestNoProductsSaysWhatHappened(t *testing.T) {
	route := "https://sklep.test/wp-json/wp/v2/product"

	blocked := NoProductsFailure(route, wrapPartial(ErrNotJSON))
	assert.Contains(t, blocked, "never read")
	assert.Contains(t, blocked, "--user-agent")
	assert.NotContains(t, blocked, "published none")

	refused := NoProductsNotice(route, http.StatusForbidden)
	assert.Contains(t, refused, "answered 403")

	empty := NoProductsNotice(route, 0)
	assert.Contains(t, empty, "published none", "and here the claim is earned")
}

// wrapPartial puts an error where a walk's failure would carry it.
func wrapPartial(cause error) error {
	return &PartialError{Endpoint: "product", Page: 1, Fetched: 0, Err: cause}
}

// TestUnknownStatusStillReadsAsAFailure: a walk that failed for a reason with
// no status in it says so rather than claiming the shop is empty.
func TestUnknownStatusStillReadsAsAFailure(t *testing.T) {
	notice := NoProductsFailure("", wrapPartial(errors.New("connection reset by peer")))

	assert.Contains(t, notice, "could not be read")
	assert.Contains(t, notice, "connection reset")
	assert.NotContains(t, notice, "published none")
}

// TestUnreadableBodyKeepsTheCause: what a developer needs stays reachable.
func TestUnreadableBodyKeepsTheCause(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html></html>"))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5})
	require.NoError(t, err)

	resp, err := client.httpClient.R().Get(server.URL)
	require.NoError(t, err)

	wrapped := unreadableBody(resp, errors.New("invalid character '<'"))
	assert.ErrorIs(t, wrapped, ErrNotJSON)
}
