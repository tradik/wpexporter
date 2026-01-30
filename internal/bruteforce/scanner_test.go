package bruteforce

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// createTestServer creates a mock WordPress API server
func createTestServer(posts map[int]models.WordPressPost, pages map[int]models.WordPressPost, media map[int]models.WordPressMedia) *httptest.Server {
	mux := http.NewServeMux()

	// Handle site info
	mux.HandleFunc("/wp-json/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        "Test Site",
			"description": "Test Description",
			"url":         "http://test.local",
		})
	})

	// Handle individual post by ID
	mux.HandleFunc("/wp-json/wp/v2/posts/", func(w http.ResponseWriter, r *http.Request) {
		var id int
		fmt.Sscanf(r.URL.Path, "/wp-json/wp/v2/posts/%d", &id)
		if post, ok := posts[id]; ok {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(post)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	// Handle individual page by ID
	mux.HandleFunc("/wp-json/wp/v2/pages/", func(w http.ResponseWriter, r *http.Request) {
		var id int
		fmt.Sscanf(r.URL.Path, "/wp-json/wp/v2/pages/%d", &id)
		if page, ok := pages[id]; ok {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(page)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	// Handle individual media by ID
	mux.HandleFunc("/wp-json/wp/v2/media/", func(w http.ResponseWriter, r *http.Request) {
		var id int
		fmt.Sscanf(r.URL.Path, "/wp-json/wp/v2/media/%d", &id)
		if m, ok := media[id]; ok {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(m)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	return httptest.NewServer(mux)
}

func TestNewScanner(t *testing.T) {
	cfg := &config.Config{
		URL:        "https://example.com",
		BruteForce: true,
		MaxID:      100,
		Concurrent: 5,
		Timeout:    30,
		Retries:    3,
	}

	server := createTestServer(nil, nil, nil)
	defer server.Close()

	cfg.URL = server.URL
	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)

	assert.NotNil(t, scanner)
	assert.Equal(t, cfg, scanner.config)
	assert.Equal(t, client, scanner.apiClient)
}

func TestScanForContent_Disabled(t *testing.T) {
	cfg := &config.Config{
		URL:        "https://example.com",
		BruteForce: false, // Disabled
		MaxID:      10,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	server := createTestServer(nil, nil, nil)
	defer server.Close()

	cfg.URL = server.URL
	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)
	result, err := scanner.ScanForContent(nil, nil, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0, result.Found)
	assert.Empty(t, result.Posts)
	assert.Empty(t, result.Pages)
	assert.Empty(t, result.Media)
}

func TestScanForContent_FindsNewContent(t *testing.T) {
	posts := map[int]models.WordPressPost{
		5: {ID: 5, Title: models.RenderedContent{Rendered: "New Post 5"}, Slug: "new-post-5"},
	}
	pages := map[int]models.WordPressPost{
		7: {ID: 7, Title: models.RenderedContent{Rendered: "New Page 7"}, Slug: "new-page-7"},
	}
	media := map[int]models.WordPressMedia{
		9: {ID: 9, Title: models.RenderedContent{Rendered: "New Media 9"}, Slug: "new-media-9"},
	}

	server := createTestServer(posts, pages, media)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		BruteForce: true,
		MaxID:      10,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)

	// Existing content (should be skipped)
	existingPosts := []models.WordPressPost{{ID: 1}}
	existingPages := []models.WordPressPost{{ID: 2}}
	existingMedia := []models.WordPressMedia{{ID: 3}}

	result, err := scanner.ScanForContent(existingPosts, existingPages, existingMedia)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, result.Found)
	assert.Len(t, result.Posts, 1)
	assert.Len(t, result.Pages, 1)
	assert.Len(t, result.Media, 1)
	assert.Equal(t, 5, result.Posts[0].ID)
	assert.Equal(t, 7, result.Pages[0].ID)
	assert.Equal(t, 9, result.Media[0].ID)
}

func TestScanForContent_SkipsDuplicates(t *testing.T) {
	posts := map[int]models.WordPressPost{
		1: {ID: 1, Title: models.RenderedContent{Rendered: "Existing Post"}},
		3: {ID: 3, Title: models.RenderedContent{Rendered: "New Post"}},
	}

	server := createTestServer(posts, nil, nil)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		BruteForce: true,
		MaxID:      5,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)

	// Post 1 already exists
	existingPosts := []models.WordPressPost{{ID: 1}}

	result, err := scanner.ScanForContent(existingPosts, nil, nil)

	assert.NoError(t, err)
	// Should only find post 3, not post 1 (duplicate)
	assert.Equal(t, 1, result.Found)
	assert.Len(t, result.Posts, 1)
	assert.Equal(t, 3, result.Posts[0].ID)
}

func TestScanForContent_EmptyResults(t *testing.T) {
	// No content on server
	server := createTestServer(nil, nil, nil)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		BruteForce: true,
		MaxID:      5,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)
	result, err := scanner.ScanForContent(nil, nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, 0, result.Found)
	assert.Empty(t, result.Posts)
	assert.Empty(t, result.Pages)
	assert.Empty(t, result.Media)
}

func TestScanForContent_VerboseMode(t *testing.T) {
	posts := map[int]models.WordPressPost{
		1: {ID: 1, Title: models.RenderedContent{Rendered: "Test Post"}},
	}

	server := createTestServer(posts, nil, nil)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		BruteForce: true,
		MaxID:      3,
		Concurrent: 1,
		Timeout:    30,
		Retries:    1,
		Verbose:    true,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)
	result, err := scanner.ScanForContent(nil, nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, 1, result.Found)
}

func TestScanSpecificRange_Posts(t *testing.T) {
	posts := map[int]models.WordPressPost{
		10: {ID: 10, Title: models.RenderedContent{Rendered: "Post 10"}},
		15: {ID: 15, Title: models.RenderedContent{Rendered: "Post 15"}},
		20: {ID: 20, Title: models.RenderedContent{Rendered: "Post 20"}},
	}

	server := createTestServer(posts, nil, nil)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		MaxID:      100,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)
	result, err := scanner.ScanSpecificRange("posts", 10, 20)

	assert.NoError(t, err)
	foundPosts, ok := result.([]models.WordPressPost)
	assert.True(t, ok)
	assert.Len(t, foundPosts, 3)
}

func TestScanSpecificRange_Pages(t *testing.T) {
	pages := map[int]models.WordPressPost{
		12: {ID: 12, Title: models.RenderedContent{Rendered: "Page 12"}},
	}

	server := createTestServer(nil, pages, nil)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		MaxID:      100,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)
	result, err := scanner.ScanSpecificRange("pages", 10, 20)

	assert.NoError(t, err)
	foundPages, ok := result.([]models.WordPressPost)
	assert.True(t, ok)
	assert.Len(t, foundPages, 1)
	assert.Equal(t, 12, foundPages[0].ID)
}

func TestScanSpecificRange_Media(t *testing.T) {
	media := map[int]models.WordPressMedia{
		18: {ID: 18, Title: models.RenderedContent{Rendered: "Media 18"}},
	}

	server := createTestServer(nil, nil, media)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		MaxID:      100,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)
	result, err := scanner.ScanSpecificRange("media", 10, 20)

	assert.NoError(t, err)
	foundMedia, ok := result.([]models.WordPressMedia)
	assert.True(t, ok)
	assert.Len(t, foundMedia, 1)
	assert.Equal(t, 18, foundMedia[0].ID)
}

func TestScanSpecificRange_InvalidContentType(t *testing.T) {
	server := createTestServer(nil, nil, nil)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		MaxID:      100,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)
	result, err := scanner.ScanSpecificRange("invalid", 1, 10)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported content type")
}

func TestScanSpecificRange_EmptyRange(t *testing.T) {
	server := createTestServer(nil, nil, nil)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		MaxID:      100,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)

	// Start > End should return empty
	result, err := scanner.ScanSpecificRange("posts", 20, 10)

	assert.NoError(t, err)
	posts, ok := result.([]models.WordPressPost)
	assert.True(t, ok)
	assert.Empty(t, posts)
}

func TestScanResult_Structure(t *testing.T) {
	result := &ScanResult{
		Posts: []models.WordPressPost{
			{ID: 1, Title: models.RenderedContent{Rendered: "Post 1"}},
		},
		Pages: []models.WordPressPost{
			{ID: 2, Title: models.RenderedContent{Rendered: "Page 1"}},
		},
		Media: []models.WordPressMedia{
			{ID: 3, Title: models.RenderedContent{Rendered: "Media 1"}},
		},
		Found: 3,
	}

	assert.Len(t, result.Posts, 1)
	assert.Len(t, result.Pages, 1)
	assert.Len(t, result.Media, 1)
	assert.Equal(t, 3, result.Found)

	// Verify Found matches total items
	expectedFound := len(result.Posts) + len(result.Pages) + len(result.Media)
	assert.Equal(t, expectedFound, result.Found)
}

func TestScanSpecificRange_InvalidType(t *testing.T) {
	server := createTestServer(nil, nil, nil)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		MaxID:      10,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)

	result, err := scanner.ScanSpecificRange("invalid", 1, 5)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestScanForContent_AllTypesWithMixedExisting(t *testing.T) {
	// Server has IDs 1-5 for posts, pages, and media
	posts := map[int]models.WordPressPost{
		1: {ID: 1, Title: models.RenderedContent{Rendered: "Post 1"}},
		2: {ID: 2, Title: models.RenderedContent{Rendered: "Post 2"}},
		3: {ID: 3, Title: models.RenderedContent{Rendered: "Post 3"}},
	}
	pages := map[int]models.WordPressPost{
		1: {ID: 1, Title: models.RenderedContent{Rendered: "Page 1"}},
		2: {ID: 2, Title: models.RenderedContent{Rendered: "Page 2"}},
	}
	media := map[int]models.WordPressMedia{
		1: {ID: 1, Title: models.RenderedContent{Rendered: "Media 1"}},
		2: {ID: 2, Title: models.RenderedContent{Rendered: "Media 2"}},
	}

	server := createTestServer(posts, pages, media)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		BruteForce: true,
		MaxID:      5,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)

	// All IDs 1-2 already exist, only ID 3 is new
	existingPosts := []models.WordPressPost{{ID: 1}, {ID: 2}}
	existingPages := []models.WordPressPost{{ID: 1}}
	existingMedia := []models.WordPressMedia{{ID: 1}}

	result, err := scanner.ScanForContent(existingPosts, existingPages, existingMedia)

	assert.NoError(t, err)
	// Should find new posts (3), pages (2), media (2)
	assert.Equal(t, 1, len(result.Posts))
	assert.Equal(t, 1, len(result.Pages))
	assert.Equal(t, 1, len(result.Media))
}

func TestScanForContent_NoNewContentFound(t *testing.T) {
	// Server has only IDs 1-2
	posts := map[int]models.WordPressPost{
		1: {ID: 1, Title: models.RenderedContent{Rendered: "Post 1"}},
		2: {ID: 2, Title: models.RenderedContent{Rendered: "Post 2"}},
	}

	server := createTestServer(posts, nil, nil)
	defer server.Close()

	cfg := &config.Config{
		URL:        server.URL,
		BruteForce: true,
		MaxID:      3,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	client, err := api.NewClient(cfg)
	require.NoError(t, err)

	scanner := NewScanner(cfg, client)

	// All posts already known
	existingPosts := []models.WordPressPost{{ID: 1}, {ID: 2}}

	result, err := scanner.ScanForContent(existingPosts, nil, nil)

	assert.NoError(t, err)
	// Should find nothing new
	assert.Empty(t, result.Posts)
}
