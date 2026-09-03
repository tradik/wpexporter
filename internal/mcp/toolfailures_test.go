package mcp

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// brokenSite answers its own root — so the client is built and the tool gets
// past configuration — and then fails every collection. An assistant has no
// console to read a stack trace from, so what matters is that each tool comes
// back with an error naming the collection it could not read.
func brokenSite(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json" || r.URL.Path == "/wp-json/" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(rootBody))

			return
		}

		http.Error(w, "upstream is down", http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	return server.URL
}

// TestToolsReportAFailedCollection: every listing tool turns an unreachable
// collection into an error rather than an empty answer, which an assistant
// would otherwise report to a user as "this site has none".
//
// get_site_info is deliberately absent: it still answers an all-empty record
// for a site that never responded. That is tradik/wpexporter#79, not a rule
// this test is asserting.
func TestToolsReportAFailedCollection(t *testing.T) {
	site := brokenSite(t)

	tests := []struct {
		name    string
		handler ToolHandler
		args    map[string]interface{}
		wants   string
	}{
		{name: "list_posts", handler: handleListPosts, wants: "posts"},
		{name: "list_pages", handler: handleListPages, wants: "pages"},
		{name: "list_categories", handler: handleListCategories, wants: "categories"},
		{name: "list_media", handler: handleListMedia, wants: "media"},
		{
			name:    "get_post",
			handler: handleGetPost,
			args:    map[string]interface{}{"postId": float64(1)},
			wants:   "post 1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := map[string]interface{}{"url": site}
			for key, value := range tc.args {
				args[key] = value
			}

			_, err := tc.handler(args)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wants)
		})
	}
}

// TestExportSiteRecordsAFailedCollectionAsAGap: a collection the API refuses
// is a hole in the export, not the end of the run (#37, #57). The export
// completes and names the hole, because an agent has no console to read a
// warning from — `incomplete` is the only place it can see one.
func TestExportSiteRecordsAFailedCollectionAsAGap(t *testing.T) {
	result, err := handleExportSite(map[string]interface{}{
		"url":    brokenSite(t),
		"format": "json",
		"output": filepath.Join(t.TempDir(), "export"),
	})
	require.NoError(t, err)

	answer := text(t, result)
	assert.Contains(t, answer, "incomplete")
	assert.Contains(t, answer, "posts: stopped at page 1")
	assert.Contains(t, strings.ToLower(answer), "status 500")
}

// TestListingsHonourAZeroLimit: limit is the assistant's own guard against
// pulling a whole site into a reply, so it is applied even when it asks for
// nothing.
func TestListingsHonourAZeroLimit(t *testing.T) {
	site := stubSite(t)

	pages, err := handleListPages(map[string]interface{}{"url": site, "limit": float64(0)})
	require.NoError(t, err)
	assert.Contains(t, text(t, pages), "Found 0 pages")

	media, err := handleListMedia(map[string]interface{}{"url": site, "limit": float64(0)})
	require.NoError(t, err)
	assert.Contains(t, text(t, media), "Found 0 media")
}
