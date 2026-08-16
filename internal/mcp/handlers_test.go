package mcp

// The tools an assistant actually calls.
//
// Everything below was reachable only over MCP and untested: the handlers are
// the whole product as far as an agent is concerned, and an agent has no
// console to notice a wrong answer in. They are exercised here against a stub
// WordPress, which is what the export path already does.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rootBody = `{"name":"Test site","description":"A site","url":"https://x.test"}`

	postsBody = `[
		{"id":1,"slug":"hello","link":"https://x.test/blog/hello/","status":"publish","author":2,
		 "title":{"rendered":"Hello"},"content":{"rendered":"<p>Hi</p>"}},
		{"id":2,"slug":"second","link":"https://x.test/news/second/","status":"publish","author":2,
		 "title":{"rendered":"Second"},"content":{"rendered":"<p>More</p>"}}
	]`

	pagesBody = `[{"id":7,"slug":"about","link":"https://x.test/about/","status":"publish",
		"title":{"rendered":"About"},"content":{"rendered":"<p>Us</p>"}}]`

	categoriesBody = `[{"id":3,"name":"Recipes","slug":"recipes","description":"Food","parent":0,"count":12}]`

	mediaBody = `[{"id":11,"slug":"cake","mime_type":"image/jpeg",
		"source_url":"https://x.test/wp-content/uploads/cake.jpg","title":{"rendered":"Cake"}}]`

	onePostBody = `{"id":1,"slug":"hello","link":"https://x.test/blog/hello/","title":{"rendered":"Hello"}}`
)

// stubSite answers the routes the handlers read. Anything else is an empty
// collection, which is what a site without that content would serve.
func stubSite(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/wp-json" || r.URL.Path == "/wp-json/":
			_, _ = w.Write([]byte(rootBody))
		case strings.HasSuffix(r.URL.Path, "/wp/v2/posts/1"):
			_, _ = w.Write([]byte(onePostBody))
		case strings.HasSuffix(r.URL.Path, "/wp/v2/posts"):
			writeFirstPage(w, r, postsBody)
		case strings.HasSuffix(r.URL.Path, "/wp/v2/pages"):
			writeFirstPage(w, r, pagesBody)
		case strings.HasSuffix(r.URL.Path, "/wp/v2/categories"):
			writeFirstPage(w, r, categoriesBody)
		case strings.HasSuffix(r.URL.Path, "/wp/v2/media"):
			writeFirstPage(w, r, mediaBody)
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)

	return server.URL
}

// writeFirstPage serves a collection's only page, and nothing after it.
func writeFirstPage(w http.ResponseWriter, r *http.Request, body string) {
	if page := r.URL.Query().Get("page"); page != "" && page != "1" {
		_, _ = w.Write([]byte(`[]`))

		return
	}

	_, _ = w.Write([]byte(body))
}

// text is the handler's answer as the assistant would read it.
func text(t *testing.T, result *CallToolResult) string {
	t.Helper()

	require.NotNil(t, result)
	require.Len(t, result.Content, 1)

	return result.Content[0].Text
}

// TestHandleGetSiteInfo: the first call any assistant makes.
func TestHandleGetSiteInfo(t *testing.T) {
	result, err := handleGetSiteInfo(map[string]interface{}{"url": stubSite(t)})
	require.NoError(t, err)

	assert.Contains(t, text(t, result), "Test site")
}

// TestHandleListPosts: the listing, its limit and its path filter — the three
// things an assistant can get wrong and never find out about.
func TestHandleListPosts(t *testing.T) {
	site := stubSite(t)

	all, err := handleListPosts(map[string]interface{}{"url": site})
	require.NoError(t, err)
	assert.Contains(t, text(t, all), "Found 2 posts")
	assert.Contains(t, text(t, all), "Hello")

	limited, err := handleListPosts(map[string]interface{}{"url": site, "limit": float64(1)})
	require.NoError(t, err)
	assert.Contains(t, text(t, limited), "Found 1 posts")

	filtered, err := handleListPosts(map[string]interface{}{"url": site, "pathFilter": "/news/"})
	require.NoError(t, err)
	assert.Contains(t, text(t, filtered), "Second")
	assert.NotContains(t, text(t, filtered), "Hello")
}

// TestHandleListPages, TestHandleListCategories, TestHandleListMedia: the rest
// of the read-only surface.
func TestHandleListPages(t *testing.T) {
	result, err := handleListPages(map[string]interface{}{"url": stubSite(t), "limit": float64(5)})
	require.NoError(t, err)

	answer := text(t, result)
	assert.Contains(t, answer, "Found 1 pages")
	assert.Contains(t, answer, "About")
}

func TestHandleListCategories(t *testing.T) {
	result, err := handleListCategories(map[string]interface{}{"url": stubSite(t)})
	require.NoError(t, err)

	answer := text(t, result)
	assert.Contains(t, answer, "Found 1 categories")
	assert.Contains(t, answer, "Recipes")
	assert.Contains(t, answer, `"count": 12`)
}

func TestHandleListMedia(t *testing.T) {
	result, err := handleListMedia(map[string]interface{}{"url": stubSite(t), "limit": float64(1)})
	require.NoError(t, err)

	answer := text(t, result)
	assert.Contains(t, answer, "Found 1 media items")
	assert.Contains(t, answer, "image/jpeg")
}

// TestHandleGetPost: one record by ID, and the refusal when no ID was given —
// an assistant that omits it must be told, not handed the wrong post.
func TestHandleGetPost(t *testing.T) {
	site := stubSite(t)

	result, err := handleGetPost(map[string]interface{}{"url": site, "postId": float64(1)})
	require.NoError(t, err)
	assert.Contains(t, text(t, result), "Hello")

	_, err = handleGetPost(map[string]interface{}{"url": site})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "postId is required")
}

// TestHandlersRefuseAnUnusableURL: every handler builds its own client, so each
// has to refuse a URL that is not one rather than panic or answer emptily.
func TestHandlersRefuseAnUnusableURL(t *testing.T) {
	handlers := map[string]func(map[string]interface{}) (*CallToolResult, error){
		"get_site_info":   handleGetSiteInfo,
		"list_posts":      handleListPosts,
		"list_pages":      handleListPages,
		"list_categories": handleListCategories,
		"list_media":      handleListMedia,
		"export_site":     handleExportSite,
	}

	for name, handler := range handlers {
		_, err := handler(map[string]interface{}{"url": "not-a-url"})
		assert.Error(t, err, "handler %s", name)
	}
}

// TestHandleExportSite: the tool that writes files. It must produce an export
// where it was told to, and report the counts back — an assistant has no other
// way to see what it got.
func TestHandleExportSite(t *testing.T) {
	output := filepath.Join(t.TempDir(), "export")

	result, err := handleExportSite(map[string]interface{}{
		"url":           stubSite(t),
		"format":        "markdown",
		"output":        output,
		"downloadMedia": false,
	})
	require.NoError(t, err)

	answer := text(t, result)
	assert.Contains(t, answer, `"status": "success"`)

	var reported map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(answer), &reported))

	stats, ok := reported["stats"].(map[string]interface{})
	require.True(t, ok, "the counts travel with the answer")
	assert.Equal(t, float64(2), stats["posts"])
	assert.Equal(t, float64(1), stats["pages"])

	assert.FileExists(t, filepath.Join(output, "metadata.json"))
	assert.FileExists(t, filepath.Join(output, "pages", "about.md"))
}

// TestHandleExportSiteRejectsABadFormat: a format nobody supports is a mistake
// to report, not a directory to create.
func TestHandleExportSiteRejectsABadFormat(t *testing.T) {
	_, err := handleExportSite(map[string]interface{}{
		"url":    stubSite(t),
		"format": "papyrus",
		"output": filepath.Join(t.TempDir(), "export"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}
