package seo

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestNewCrawler(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)
	assert.NotNil(t, c)
	assert.NotNil(t, c.httpClient)
}

func TestExtractSEO_FullPage(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
	<title>Test Page Title - Site Name</title>
	<meta name="description" content="This is a test meta description for SEO." />
	<meta name="keywords" content="test, keywords, seo" />
	<meta property="og:title" content="OG Title for Social" />
	<meta property="og:description" content="OG Description for sharing" />
	<meta property="og:image" content="https://example.com/og-image.jpg" />
	<link rel="canonical" href="https://example.com/canonical-url" />
</head>
<body>Content</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	assert.Equal(t, "Test Page Title - Site Name", seo.Title)
	assert.Equal(t, "This is a test meta description for SEO.", seo.MetaDescription)
	assert.Equal(t, "test, keywords, seo", seo.MetaKeywords)
	assert.Equal(t, "OG Title for Social", seo.OGTitle)
	assert.Equal(t, "OG Description for sharing", seo.OGDescription)
	assert.Equal(t, "https://example.com/og-image.jpg", seo.OGImage)
	assert.Equal(t, "https://example.com/canonical-url", seo.CanonicalURL)
}

func TestExtractSEO_ReverseAttributeOrder(t *testing.T) {
	// Test when content comes before name/property
	html := `<!DOCTYPE html>
<html>
<head>
	<meta content="Description first" name="description" />
	<meta content="OG Title first" property="og:title" />
	<link href="https://example.com/canonical" rel="canonical" />
</head>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	assert.Equal(t, "Description first", seo.MetaDescription)
	assert.Equal(t, "OG Title first", seo.OGTitle)
	assert.Equal(t, "https://example.com/canonical", seo.CanonicalURL)
}

func TestExtractSEO_HTMLEntities(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
	<title>Tom &amp; Jerry&#39;s &quot;Adventure&quot;</title>
	<meta name="description" content="A &lt;great&gt; story &ndash; really!" />
</head>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	assert.Equal(t, "Tom & Jerry's \"Adventure\"", seo.Title)
	assert.Equal(t, "A <great> story - really!", seo.MetaDescription)
}

func TestExtractSEO_CaseInsensitive(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
	<TITLE>Uppercase Title</TITLE>
	<META NAME="DESCRIPTION" CONTENT="Uppercase meta" />
	<META PROPERTY="OG:TITLE" CONTENT="Uppercase OG" />
</head>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	assert.Equal(t, "Uppercase Title", seo.Title)
	assert.Equal(t, "Uppercase meta", seo.MetaDescription)
	assert.Equal(t, "Uppercase OG", seo.OGTitle)
}

func TestExtractSEO_EmptyURL(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	seo := c.extractSEO("")

	assert.Empty(t, seo.Title)
	assert.Empty(t, seo.MetaDescription)
}

func TestExtractSEO_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	// Should return empty SEO on error
	assert.Empty(t, seo.Title)
}

func TestExtractSEO_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	assert.Empty(t, seo.Title)
}

func TestExtractSEO_WithBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "testuser" || pass != "testpass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><head><title>Protected Page</title></head></html>"))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.AuthUser = "testuser"
	cfg.AuthPass = "testpass"
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	assert.Equal(t, "Protected Page", seo.Title)
}

func TestExtractSEO_WithBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer my-secret-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><head><title>Token Protected</title></head></html>"))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.AuthToken = "my-secret-token"
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	assert.Equal(t, "Token Protected", seo.Title)
}

func TestEnrichPostsWithSEO(t *testing.T) {
	html1 := `<html><head><title>Post 1 Title</title><meta name="description" content="Post 1 desc" /></head></html>`
	html2 := `<html><head><title>Post 2 Title</title><meta name="description" content="Post 2 desc" /></head></html>`

	mux := http.NewServeMux()
	mux.HandleFunc("/post1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html1))
	})
	mux.HandleFunc("/post2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(html2))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	posts := []models.WordPressPost{
		{ID: 1, Link: server.URL + "/post1"},
		{ID: 2, Link: server.URL + "/post2"},
	}

	cfg := config.DefaultConfig()
	cfg.Concurrent = 2
	c := NewCrawler(cfg)

	enriched := c.EnrichPostsWithSEO(posts)

	assert.Len(t, enriched, 2)

	// Find posts by ID since order may vary due to concurrency
	postMap := make(map[int]models.WordPressPost)
	for _, p := range enriched {
		postMap[p.ID] = p
	}

	assert.Equal(t, "Post 1 Title", postMap[1].SEO.Title)
	assert.Equal(t, "Post 1 desc", postMap[1].SEO.MetaDescription)
	assert.Equal(t, "Post 2 Title", postMap[2].SEO.Title)
	assert.Equal(t, "Post 2 desc", postMap[2].SEO.MetaDescription)
}

func TestEnrichPostsWithSEO_EmptyPosts(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	result := c.EnrichPostsWithSEO(nil)
	assert.Nil(t, result)

	result = c.EnrichPostsWithSEO([]models.WordPressPost{})
	assert.Len(t, result, 0)
}

func TestDecodeHTMLEntities(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	tests := []struct {
		input    string
		expected string
	}{
		{"&amp;", "&"},
		{"&lt;", "<"},
		{"&gt;", ">"},
		{"&quot;", "\""},
		{"&#39;", "'"},
		{"&apos;", "'"},
		{"&nbsp;", " "},
		{"&#x27;", "'"},
		{"&#x2F;", "/"},
		{"&ndash;", "-"},
		{"&mdash;", "-"},
		{"&lsquo;", "'"},
		{"&rsquo;", "'"},
		{"&ldquo;", "\""},
		{"&rdquo;", "\""},
		{"Tom &amp; Jerry", "Tom & Jerry"},
		{"No entities here", "No entities here"},
	}

	for _, tt := range tests {
		result := c.decodeHTMLEntities(tt.input)
		assert.Equal(t, tt.expected, result, "Failed for input: %s", tt.input)
	}
}

func TestExtractSEO_InvalidURL(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	// Invalid URL that will fail http.NewRequest
	seo := c.extractSEO("://invalid-url")

	// Should return empty SEO data
	assert.Empty(t, seo.Title)
	assert.Empty(t, seo.MetaDescription)
}

func TestExtractSEO_NetworkError(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Timeout = 1 // Very short timeout
	c := NewCrawler(cfg)

	// Non-routable IP will cause network timeout
	seo := c.extractSEO("http://192.0.2.1/page")

	// Should return empty SEO data on network error
	assert.Empty(t, seo.Title)
	assert.Empty(t, seo.MetaDescription)
}

func TestExtractSEO_NoMatchingTags(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
</head>
<body>No SEO tags here</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	// All fields should be empty
	assert.Empty(t, seo.Title)
	assert.Empty(t, seo.MetaDescription)
	assert.Empty(t, seo.MetaKeywords)
	assert.Empty(t, seo.OGTitle)
	assert.Empty(t, seo.OGDescription)
	assert.Empty(t, seo.OGImage)
	assert.Empty(t, seo.CanonicalURL)
}

func TestExtractTitle_NoTitle(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	result := c.extractTitle("<html><head></head></html>")
	assert.Empty(t, result)
}

func TestExtractMetaContent_NoMatch(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	result := c.extractMetaContent("<html><head></head></html>", "description")
	assert.Empty(t, result)
}

func TestExtractOGContent_NoMatch(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	result := c.extractOGContent("<html><head></head></html>", "og:title")
	assert.Empty(t, result)
}

func TestExtractCanonical_NoMatch(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	result := c.extractCanonical("<html><head></head></html>")
	assert.Empty(t, result)
}

func TestEnrichPostsWithSEO_WithEmptyLink(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Link: ""},
		{ID: 2, Link: ""},
	}

	cfg := config.DefaultConfig()
	cfg.Concurrent = 2
	c := NewCrawler(cfg)

	enriched := c.EnrichPostsWithSEO(posts)

	assert.Len(t, enriched, 2)
	// SEO should be empty for posts with no link
	assert.Empty(t, enriched[0].SEO.Title)
	assert.Empty(t, enriched[1].SEO.Title)
}

func TestExtractSEO_ReadBodyError(t *testing.T) {
	// Create a server that sends incomplete/broken response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000") // Claim large content
		w.WriteHeader(http.StatusOK)
		// Write partial data then close - this won't trigger read error in standard case
		// Instead, test that we handle empty body gracefully
		_, _ = w.Write([]byte(""))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	// Should return empty SEO data for empty body
	assert.Empty(t, seo.Title)
}

func TestExtractSEO_WithUserAgentHeader(t *testing.T) {
	var receivedUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUserAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><head><title>Test</title></head></html>"))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.UserAgent = "CustomCrawler/1.0"
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	assert.Equal(t, "Test", seo.Title)
	assert.Equal(t, "CustomCrawler/1.0", receivedUserAgent)
}

func TestExtractSEO_OnlyAuthUserNoPass(t *testing.T) {
	// Test case where AuthUser is set but AuthPass is empty - should NOT set basic auth
	var hasBasicAuth bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _, hasBasicAuth = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><head><title>Test</title></head></html>"))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.AuthUser = "testuser"
	cfg.AuthPass = "" // Empty password
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	assert.Equal(t, "Test", seo.Title)
	assert.False(t, hasBasicAuth, "Basic auth should not be set when password is empty")
}

// Tests for content crawling

func TestIsContentEmpty(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{"empty string", "", true},
		{"only whitespace", "   \n\t   ", true},
		{"only HTML tags", "<p></p>", true},
		{"only nbsp", "&nbsp;&nbsp;", true},
		{"empty paragraph", "\n<p></p>\n\n\n\n<p></p>\n", true},
		{"small content", "Hi", true}, // Less than 10 chars after stripping
		{"real content", "<p>This is some actual content for the page.</p>", false},
		{"content with tags", "<div><p>Hello World</p></div>", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isContentEmpty(tt.content)
			assert.Equal(t, tt.expected, result, "isContentEmpty(%q) = %v, want %v", tt.content, result, tt.expected)
		})
	}
}

func TestFilterEmptyContent(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Content: models.RenderedContent{Rendered: "<p>Real content here</p>"}},
		{ID: 2, Content: models.RenderedContent{Rendered: ""}},
		{ID: 3, Content: models.RenderedContent{Rendered: "<p></p>"}},
		{ID: 4, Content: models.RenderedContent{Rendered: "<p>Another post with content</p>"}},
		{ID: 5, Content: models.RenderedContent{Rendered: "&nbsp;"}},
	}

	filtered := FilterEmptyContent(posts)

	assert.Len(t, filtered, 2)
	assert.Equal(t, 1, filtered[0].ID)
	assert.Equal(t, 4, filtered[1].ID)
}

func TestFilterEmptyContent_AllEmpty(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Content: models.RenderedContent{Rendered: ""}},
		{ID: 2, Content: models.RenderedContent{Rendered: "<p></p>"}},
	}

	filtered := FilterEmptyContent(posts)
	assert.Len(t, filtered, 0)
}

func TestFilterEmptyContent_NoneEmpty(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Content: models.RenderedContent{Rendered: "<p>This is some longer content that should not be empty.</p>"}},
		{ID: 2, Content: models.RenderedContent{Rendered: "<p>Another post with substantial content for testing purposes.</p>"}},
	}

	filtered := FilterEmptyContent(posts)
	assert.Len(t, filtered, 2)
}

func TestExtractMainContent_Article(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
<header>Navigation</header>
<article class="post">
<h1>Article Title</h1>
<p>This is the article content.</p>
</article>
<footer>Footer</footer>
</body>
</html>`

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	content := c.extractMainContent(html)

	assert.Contains(t, content, "Article Title")
	assert.Contains(t, content, "article content")
	assert.NotContains(t, content, "Navigation")
	assert.NotContains(t, content, "Footer")
}

func TestExtractMainContent_BricksBuilder(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Bricks Page</title></head>
<body>
<header>Nav</header>
<div class="brxe-text">First paragraph text</div>
<div class="brxe-text">Second paragraph text</div>
<div class="brxe-text">Third paragraph text</div>
<footer>Footer</footer>
</body>
</html>`

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	content := c.extractMainContent(html)

	assert.Contains(t, content, "First paragraph")
	assert.Contains(t, content, "Second paragraph")
	assert.Contains(t, content, "Third paragraph")
}

func TestExtractMainContent_Main(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
<header>Header</header>
<main id="content">
<h1>Main Content</h1>
<p>Page body text goes here.</p>
</main>
<footer>Footer</footer>
</body>
</html>`

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	content := c.extractMainContent(html)

	assert.Contains(t, content, "Main Content")
	assert.Contains(t, content, "body text")
}

func TestCleanHTMLContent(t *testing.T) {
	html := `<div>
<script>alert('bad');</script>
<style>.foo { color: red; }</style>
<!-- comment -->
<noscript>No JS</noscript>
<p>Actual content</p>
<p></p>
</div>`

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	cleaned := c.cleanHTMLContent(html)

	assert.NotContains(t, cleaned, "alert")
	assert.NotContains(t, cleaned, "color")
	assert.NotContains(t, cleaned, "comment")
	assert.NotContains(t, cleaned, "No JS")
	assert.Contains(t, cleaned, "Actual content")
}

func TestExtractPageContent_EmptyURL(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	content := c.extractPageContent("")
	assert.Empty(t, content)
}

func TestExtractPageContent_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	content := c.extractPageContent(server.URL)
	assert.Empty(t, content)
}

func TestExtractPageContent_WithAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "user" || pass != "pass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body><main><p>This is protected content that requires authentication to view. It should be longer than 50 characters to pass the content threshold check.</p></main></body></html>`))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	cfg.AuthUser = "user"
	cfg.AuthPass = "pass"
	c := NewCrawler(cfg)

	content := c.extractPageContent(server.URL)
	assert.Contains(t, content, "protected content")
}

func TestEnrichPostsWithContent_NoneEmpty(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Content: models.RenderedContent{Rendered: "<p>Has content</p>"}},
		{ID: 2, Content: models.RenderedContent{Rendered: "<p>Also has content</p>"}},
	}

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	enriched := c.EnrichPostsWithContent(posts)

	// Should return unchanged since no posts have empty content
	assert.Len(t, enriched, 2)
	assert.Equal(t, "<p>Has content</p>", enriched[0].Content.Rendered)
}

func TestEnrichPostsWithContent_SomeEmpty(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
<main>
<h1>Crawled Title</h1>
<p>This content was crawled from the actual page.</p>
</main>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	posts := []models.WordPressPost{
		{ID: 1, Link: server.URL, Content: models.RenderedContent{Rendered: ""}},
		{ID: 2, Link: "", Content: models.RenderedContent{Rendered: "<p>Has content</p>"}},
	}

	cfg := config.DefaultConfig()
	cfg.Concurrent = 1
	c := NewCrawler(cfg)

	enriched := c.EnrichPostsWithContent(posts)

	assert.Len(t, enriched, 2)
	// First post should have crawled content
	assert.Contains(t, enriched[0].Content.Rendered, "Crawled Title")
	assert.Contains(t, enriched[0].Content.Rendered, "crawled from the actual page")
	// Second post should be unchanged
	assert.Equal(t, "<p>Has content</p>", enriched[1].Content.Rendered)
}

func TestEnrichPostsWithSEOAndContent(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
<title>Combined Test Page</title>
<meta name="description" content="This is a test description for combined crawl">
<meta property="og:title" content="OG Combined Title">
</head>
<body>
<main>
<h1>Combined Content Title</h1>
<p>This content was extracted along with SEO data in a single request.</p>
</main>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	posts := []models.WordPressPost{
		{ID: 1, Link: server.URL, Content: models.RenderedContent{Rendered: ""}},                   // Empty content - needs crawling
		{ID: 2, Link: server.URL, Content: models.RenderedContent{Rendered: "<p>Has content</p>"}}, // Has content - only needs SEO
	}

	cfg := config.DefaultConfig()
	cfg.Concurrent = 1
	c := NewCrawler(cfg)

	enriched := c.EnrichPostsWithSEOAndContent(posts)

	assert.Len(t, enriched, 2)

	// First post should have both SEO and content
	assert.Equal(t, "Combined Test Page", enriched[0].SEO.Title)
	assert.Equal(t, "This is a test description for combined crawl", enriched[0].SEO.MetaDescription)
	assert.Equal(t, "OG Combined Title", enriched[0].SEO.OGTitle)
	assert.Contains(t, enriched[0].Content.Rendered, "Combined Content Title")
	assert.Contains(t, enriched[0].Content.Rendered, "extracted along with SEO")

	// Second post should have SEO but content unchanged
	assert.Equal(t, "Combined Test Page", enriched[1].SEO.Title)
	assert.Equal(t, "<p>Has content</p>", enriched[1].Content.Rendered)
}

func TestExtractSEOAndContent(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
<title>Extract Both Test</title>
<meta name="description" content="Both SEO and content">
</head>
<body>
<main><p>Main content here for extraction test. This needs to be longer than 50 characters to pass the content threshold check in the crawler.</p></main>
</body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	// Test with content extraction
	result := c.extractSEOAndContent(server.URL, true)
	assert.Equal(t, "Extract Both Test", result.SEO.Title)
	assert.Equal(t, "Both SEO and content", result.SEO.MetaDescription)
	assert.Contains(t, result.Content, "Main content here")

	// Test without content extraction
	result2 := c.extractSEOAndContent(server.URL, false)
	assert.Equal(t, "Extract Both Test", result2.SEO.Title)
	assert.Empty(t, result2.Content)
}

func TestEnrichPostsWithSEOAndContent_EmptyPosts(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	var posts []models.WordPressPost
	result := c.EnrichPostsWithSEOAndContent(posts)
	assert.Len(t, result, 0)
}

func TestExtractHreflangs(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	// Real-world example from hilo.com
	html := `<!DOCTYPE html>
<html>
<head>
<link rel="alternate" hreflang="de-de" href="https://hilo.com/de/art/schlaf-blutdruck-gesundheit-wohlbefinden/" />
<link rel="alternate" hreflang="en-gb" href="https://hilo.com/uk/art/blood-pressure-at-night/" />
<link rel="alternate" hreflang="de-ch" href="https://hilo.com/ch/art/schlaf-blutdruck-gesundheit-wohlbefinden/" />
<link rel="alternate" hreflang="it-it" href="https://hilo.com/it/art/unisci-puntini-sonno-pressione-sanguigna/" />
<link rel="alternate" hreflang="en-ca" href="https://hilo.com/ca/art/blood-pressure-at-night/" />
<link rel="alternate" hreflang="es-es" href="https://hilo.com/es/art/sueno-presion-arterial-salud-bienestar/" />
<link rel="alternate" hreflang="en-au" href="https://hilo.com/au/art/blood-pressure-at-night/" />
<link rel="alternate" hreflang="fr-fr" href="https://hilo.com/fr/art/sommeil-tension-arterielle-sante/" />
</head>
<body></body>
</html>`

	hreflangs := c.extractHreflangs(html)

	assert.Len(t, hreflangs, 8)

	// Check specific entries
	langMap := make(map[string]string)
	for _, h := range hreflangs {
		langMap[h.Lang] = h.Href
	}

	assert.Equal(t, "https://hilo.com/de/art/schlaf-blutdruck-gesundheit-wohlbefinden/", langMap["de-de"])
	assert.Equal(t, "https://hilo.com/uk/art/blood-pressure-at-night/", langMap["en-gb"])
	assert.Equal(t, "https://hilo.com/fr/art/sommeil-tension-arterielle-sante/", langMap["fr-fr"])
}

func TestExtractHreflangs_Empty(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	html := `<html><head><link rel="canonical" href="https://example.com/" /></head></html>`

	hreflangs := c.extractHreflangs(html)
	assert.Len(t, hreflangs, 0)
}

func TestExtractHreflangs_DifferentAttributeOrder(t *testing.T) {
	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	// Different attribute orders
	html := `<html><head>
<link href="https://example.com/de/" hreflang="de" rel="alternate" />
<link hreflang="fr" rel="alternate" href="https://example.com/fr/" />
<link rel="alternate" href="https://example.com/es/" hreflang="es" />
</head></html>`

	hreflangs := c.extractHreflangs(html)

	assert.Len(t, hreflangs, 3)

	langMap := make(map[string]string)
	for _, h := range hreflangs {
		langMap[h.Lang] = h.Href
	}

	assert.Equal(t, "https://example.com/de/", langMap["de"])
	assert.Equal(t, "https://example.com/fr/", langMap["fr"])
	assert.Equal(t, "https://example.com/es/", langMap["es"])
}

func TestExtractSEO_WithHreflangs(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head>
<title>Test Page</title>
<link rel="alternate" hreflang="en" href="https://example.com/en/" />
<link rel="alternate" hreflang="de" href="https://example.com/de/" />
</head>
<body></body>
</html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	cfg := config.DefaultConfig()
	c := NewCrawler(cfg)

	seo := c.extractSEO(server.URL)

	assert.Equal(t, "Test Page", seo.Title)
	assert.Len(t, seo.Hreflangs, 2)
}
