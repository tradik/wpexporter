package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestGetSiteInfoFallbackOnError(t *testing.T) {
	// Server that returns error for settings - should fallback to default
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetSiteInfo()
	if err != nil {
		t.Errorf("GetSiteInfo() should not return error, got %v", err)
	}
	// Should return fallback site info
	if result == nil {
		t.Error("GetSiteInfo() should return fallback site info")
	}
}

func TestGetPostsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/posts" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetPosts()
	if err == nil {
		t.Error("GetPosts() should return error for server error")
	}
}

func TestGetPagesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/pages" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetPages()
	if err == nil {
		t.Error("GetPages() should return error for server error")
	}
}

func TestGetMediaError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/media" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetMedia()
	if err == nil {
		t.Error("GetMedia() should return error for server error")
	}
}

func TestGetCategoriesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/categories" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetCategories()
	if err == nil {
		t.Error("GetCategories() should return error for server error")
	}
}

func TestGetTagsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/tags" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetTags()
	if err == nil {
		t.Error("GetTags() should return error for server error")
	}
}

func TestGetUsersError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/users" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetUsers()
	if err == nil {
		t.Error("GetUsers() should return error for server error")
	}
}

func TestGetPostByIDError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetPostByID(1)
	if err == nil {
		t.Error("GetPostByID() should return error for server error")
	}
}

func TestGetPageByIDError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetPageByID(1)
	if err == nil {
		t.Error("GetPageByID() should return error for server error")
	}
}

func TestGetMediaByIDError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetMediaByID(1)
	if err == nil {
		t.Error("GetMediaByID() should return error for server error")
	}
}

func TestGetPostByIDNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetPostByID(999)
	if err != nil {
		t.Errorf("GetPostByID() should not return error for 404, got %v", err)
	}
	if result != nil {
		t.Error("GetPostByID() should return nil for 404")
	}
}

func TestGetPageByIDNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetPageByID(999)
	if err != nil {
		t.Errorf("GetPageByID() should not return error for 404, got %v", err)
	}
	if result != nil {
		t.Error("GetPageByID() should return nil for 404")
	}
}

func TestGetMediaByIDNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetMediaByID(999)
	if err != nil {
		t.Errorf("GetMediaByID() should not return error for 404, got %v", err)
	}
	if result != nil {
		t.Error("GetMediaByID() should return nil for 404")
	}
}

func TestBruteForceContentPosts(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Slug: "post-1"},
		{ID: 3, Slug: "post-3"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wp-json/wp/v2/posts/1":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(posts[0])
			_, _ = w.Write(response)
		case "/wp-json/wp/v2/posts/3":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(posts[1])
			_, _ = w.Write(response)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	found := make(chan interface{}, 10)
	progress := make(chan int, 10)

	go client.BruteForceContent("posts", 5, found, progress)

	var foundItems []interface{}
	for item := range found {
		foundItems = append(foundItems, item)
	}

	// Should find at least 2 items (posts 1 and 3)
	if len(foundItems) < 2 {
		t.Errorf("BruteForceContent() found %d items, want at least 2", len(foundItems))
	}
}

func TestBruteForceContentPages(t *testing.T) {
	pages := []models.WordPressPost{
		{ID: 1, Slug: "page-1"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/pages/1" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(pages[0])
			_, _ = w.Write(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	found := make(chan interface{}, 10)
	progress := make(chan int, 10)

	go client.BruteForceContent("pages", 3, found, progress)

	var foundItems []interface{}
	for item := range found {
		foundItems = append(foundItems, item)
	}

	// Should find at least 1 item
	if len(foundItems) < 1 {
		t.Errorf("BruteForceContent() found %d items, want at least 1", len(foundItems))
	}
}

func TestBruteForceContentMedia(t *testing.T) {
	media := []models.WordPressMedia{
		{ID: 1, SourceURL: "https://example.com/image.jpg"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/media/1" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(media[0])
			_, _ = w.Write(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	found := make(chan interface{}, 10)
	progress := make(chan int, 10)

	go client.BruteForceContent("media", 3, found, progress)

	var foundItems []interface{}
	for item := range found {
		foundItems = append(foundItems, item)
	}

	// Should find at least 1 item
	if len(foundItems) < 1 {
		t.Errorf("BruteForceContent() found %d items, want at least 1", len(foundItems))
	}
}

func TestBruteForceContentUnknownType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	found := make(chan interface{}, 10)
	progress := make(chan int, 10)

	go client.BruteForceContent("unknown", 3, found, progress)

	var foundItems []interface{}
	for item := range found {
		foundItems = append(foundItems, item)
	}

	if len(foundItems) != 0 {
		t.Errorf("BruteForceContent() found %d items for unknown type, want 0", len(foundItems))
	}
}

func TestGetAllContentInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetPosts()
	if err == nil {
		t.Error("GetPosts() should return error for invalid JSON")
	}
}
