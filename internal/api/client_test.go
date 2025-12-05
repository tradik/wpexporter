package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name: "Valid config creates client",
			cfg: &config.Config{
				URL:       "https://example.com",
				Timeout:   30,
				Retries:   3,
				UserAgent: "test-agent",
			},
			wantErr: false,
		},
		{
			name: "Invalid URL returns error",
			cfg: &config.Config{
				URL: "ftp://example.com",
			},
			wantErr: true,
		},
		{
			name: "Empty URL returns error",
			cfg: &config.Config{
				URL: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// NewClient should never return nil when there's no error
				if client.config != tt.cfg {
					t.Error("NewClient() should store config reference")
				}

				if client.httpClient == nil {
					t.Error("NewClient() should create HTTP client")
				}

				expectedBaseURL := "https://example.com/wp-json/wp/v2"
				if client.baseURL != expectedBaseURL {
					t.Errorf("NewClient() baseURL = %v, want %v", client.baseURL, expectedBaseURL)
				}
			}
		})
	}
}

func TestGetSiteInfo(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/settings" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := `{
				"name": "Test Site",
				"description": "Test Description",
				"url": "https://example.com",
				"home": "https://example.com",
				"admin_email": "admin@example.com",
				"timezone": "UTC",
				"date_format": "Y-m-d",
				"time_format": "H:i:s",
				"start_of_week": 1,
				"language": "en_US"
			}`
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	siteInfo, err := client.GetSiteInfo()
	if err != nil {
		t.Fatalf("GetSiteInfo() error = %v", err)
	}

	if siteInfo.Name != "Test Site" {
		t.Errorf("GetSiteInfo() Name = %v, want %v", siteInfo.Name, "Test Site")
	}

	if siteInfo.URL != server.URL {
		t.Errorf("GetSiteInfo() URL = %v, want %v", siteInfo.URL, server.URL)
	}
}

func TestGetSiteInfoFallback(t *testing.T) {
	// Create a test server that returns 404 for settings but works for base
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/settings" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/wp-json/wp/v2" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name": "Fallback Site"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	siteInfo, err := client.GetSiteInfo()
	if err != nil {
		t.Fatalf("GetSiteInfo() error = %v", err)
	}

	// Should fallback to basic site info
	if siteInfo.Name != "WordPress Site" {
		t.Errorf("GetSiteInfo() fallback Name = %v, want %v", siteInfo.Name, "WordPress Site")
	}
}

func TestGetPosts(t *testing.T) {
	// Mock posts data
	posts := []models.WordPressPost{
		{
			ID:    1,
			Slug:  "test-post-1",
			Title: models.RenderedContent{Rendered: "Test Post 1"},
			Link:  "https://example.com/test-post-1",
		},
		{
			ID:    2,
			Slug:  "test-post-2",
			Title: models.RenderedContent{Rendered: "Test Post 2"},
			Link:  "https://example.com/test-post-2",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/posts" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			// Handle pagination
			page := r.URL.Query().Get("page")
			if page == "2" {
				_, _ = w.Write([]byte("[]")) // Empty array for second page
				return
			}

			response, _ := json.Marshal(posts)
			_, _ = w.Write(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetPosts()
	if err != nil {
		t.Fatalf("GetPosts() error = %v", err)
	}

	if len(result) != 2 {
		t.Errorf("GetPosts() returned %d posts, want %d", len(result), 2)
	}

	if result[0].ID != 1 {
		t.Errorf("GetPosts() first post ID = %d, want %d", result[0].ID, 1)
	}
}

func TestGetPostByID(t *testing.T) {
	post := models.WordPressPost{
		ID:    123,
		Slug:  "test-post",
		Title: models.RenderedContent{Rendered: "Test Post"},
		Link:  "https://example.com/test-post",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/posts/123" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(post)
			_, _ = w.Write(response)
			return
		}
		if r.URL.Path == "/wp-json/wp/v2/posts/404" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test existing post
	result, err := client.GetPostByID(123)
	if err != nil {
		t.Fatalf("GetPostByID() error = %v", err)
	}

	if result == nil {
		t.Fatal("GetPostByID() should return post for existing ID")
	}

	if result.ID != 123 {
		t.Errorf("GetPostByID() ID = %d, want %d", result.ID, 123)
	}

	// Test non-existent post
	result, err = client.GetPostByID(404)
	if err != nil {
		t.Fatalf("GetPostByID() error for non-existent post = %v", err)
	}

	if result != nil {
		t.Error("GetPostByID() should return nil for non-existent post")
	}
}

func TestGetPages(t *testing.T) {
	pages := []models.WordPressPost{
		{
			ID:    1,
			Slug:  "test-page-1",
			Title: models.RenderedContent{Rendered: "Test Page 1"},
			Type:  "page",
			Link:  "https://example.com/test-page-1",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/pages" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(pages)
			_, _ = w.Write(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetPages()
	if err != nil {
		t.Fatalf("GetPages() error = %v", err)
	}

	if len(result) != 1 {
		t.Errorf("GetPages() returned %d pages, want %d", len(result), 1)
	}

	if result[0].Type != "page" {
		t.Errorf("GetPages() page type = %s, want %s", result[0].Type, "page")
	}
}

func TestGetCategories(t *testing.T) {
	categories := []models.WordPressCategory{
		{
			ID:   1,
			Name: "Technology",
			Slug: "technology",
		},
		{
			ID:   2,
			Name: "News",
			Slug: "news",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/categories" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(categories)
			_, _ = w.Write(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetCategories()
	if err != nil {
		t.Fatalf("GetCategories() error = %v", err)
	}

	if len(result) != 2 {
		t.Errorf("GetCategories() returned %d categories, want %d", len(result), 2)
	}

	if result[0].Name != "Technology" {
		t.Errorf("GetCategories() first category name = %s, want %s", result[0].Name, "Technology")
	}
}

func TestGetTags(t *testing.T) {
	tags := []models.WordPressTag{
		{
			ID:   1,
			Name: "golang",
			Slug: "golang",
		},
		{
			ID:   2,
			Name: "wordpress",
			Slug: "wordpress",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/tags" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(tags)
			_, _ = w.Write(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetTags()
	if err != nil {
		t.Fatalf("GetTags() error = %v", err)
	}

	if len(result) != 2 {
		t.Errorf("GetTags() returned %d tags, want %d", len(result), 2)
	}

	if result[0].Name != "golang" {
		t.Errorf("GetTags() first tag name = %s, want %s", result[0].Name, "golang")
	}
}

func TestGetUsers(t *testing.T) {
	users := []models.WordPressUser{
		{
			ID:   1,
			Name: "Admin User",
			Slug: "admin",
		},
		{
			ID:   2,
			Name: "Editor User",
			Slug: "editor",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/users" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(users)
			_, _ = w.Write(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers() error = %v", err)
	}

	if len(result) != 2 {
		t.Errorf("GetUsers() returned %d users, want %d", len(result), 2)
	}

	if result[0].Name != "Admin User" {
		t.Errorf("GetUsers() first user name = %s, want %s", result[0].Name, "Admin User")
	}
}

func TestGetMedia(t *testing.T) {
	media := []models.WordPressMedia{
		{
			ID:        1,
			Slug:      "test-image",
			MimeType:  "image/jpeg",
			MediaType: "image",
			SourceURL: "https://example.com/wp-content/uploads/2024/01/test.jpg",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/media" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(media)
			_, _ = w.Write(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetMedia()
	if err != nil {
		t.Fatalf("GetMedia() error = %v", err)
	}

	if len(result) != 1 {
		t.Errorf("GetMedia() returned %d media items, want %d", len(result), 1)
	}

	if result[0].MimeType != "image/jpeg" {
		t.Errorf("GetMedia() media mime type = %s, want %s", result[0].MimeType, "image/jpeg")
	}
}

func TestBruteForceContent(t *testing.T) {
	// Mock server that returns content for specific IDs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/posts/1" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			post := models.WordPressPost{
				ID:   1,
				Slug: "found-post",
			}
			response, _ := json.Marshal(post)
			_, _ = w.Write(response)
			return
		}
		if r.URL.Path == "/wp-json/wp/v2/posts/2" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	found := make(chan interface{}, 10)
	progress := make(chan int, 10)

	// Run brute force in goroutine
	go client.BruteForceContent("posts", 3, found, progress)

	// Collect results
	var foundContent []interface{}
	var progressUpdates []int

	done := false
	for !done {
		select {
		case content, ok := <-found:
			if !ok {
				done = true
				break
			}
			foundContent = append(foundContent, content)
		case p, ok := <-progress:
			if !ok {
				done = true
				break
			}
			progressUpdates = append(progressUpdates, p)
		case <-time.After(5 * time.Second):
			t.Fatal("BruteForceContent() timed out")
		}
	}

	if len(foundContent) != 1 {
		t.Errorf("BruteForceContent() found %d items, want %d", len(foundContent), 1)
	}

	if len(progressUpdates) == 0 {
		t.Error("BruteForceContent() should send progress updates")
	}
}

func TestGetPageByID(t *testing.T) {
	page := models.WordPressPost{
		ID:    456,
		Slug:  "test-page",
		Title: models.RenderedContent{Rendered: "Test Page"},
		Type:  "page",
		Link:  "https://example.com/test-page",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/pages/456" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(page)
			_, _ = w.Write(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetPageByID(456)
	if err != nil {
		t.Fatalf("GetPageByID() error = %v", err)
	}

	if result == nil {
		t.Fatal("GetPageByID() should return page for existing ID")
	}

	if result.ID != 456 {
		t.Errorf("GetPageByID() ID = %d, want %d", result.ID, 456)
	}

	if result.Type != "page" {
		t.Errorf("GetPageByID() Type = %s, want %s", result.Type, "page")
	}
}

func TestGetMediaByID(t *testing.T) {
	media := models.WordPressMedia{
		ID:        789,
		Slug:      "test-media",
		MimeType:  "image/png",
		MediaType: "image",
		SourceURL: "https://example.com/wp-content/uploads/2024/01/test.png",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/media/789" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response, _ := json.Marshal(media)
			_, _ = w.Write(response)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetMediaByID(789)
	if err != nil {
		t.Fatalf("GetMediaByID() error = %v", err)
	}

	if result == nil {
		t.Fatal("GetMediaByID() should return media for existing ID")
	}

	if result.ID != 789 {
		t.Errorf("GetMediaByID() ID = %d, want %d", result.ID, 789)
	}

	if result.MimeType != "image/png" {
		t.Errorf("GetMediaByID() MimeType = %s, want %s", result.MimeType, "image/png")
	}
}

func TestPaginationHandling(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if r.URL.Path == "/wp-json/wp/v2/posts" {
			page := r.URL.Query().Get("page")
			switch page {
			case "1":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				posts := []models.WordPressPost{
					{ID: 1, Slug: "post-1"},
					{ID: 2, Slug: "post-2"},
				}
				response, _ := json.Marshal(posts)
				_, _ = w.Write(response)
				return
			case "2":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				posts := []models.WordPressPost{
					{ID: 3, Slug: "post-3"},
				}
				response, _ := json.Marshal(posts)
				_, _ = w.Write(response)
				return
			default:
				// Return 400 for pages beyond 2 to signal end
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   10,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	result, err := client.GetPosts()
	if err != nil {
		t.Fatalf("GetPosts() error = %v", err)
	}

	if len(result) != 3 {
		t.Errorf("GetPosts() returned %d posts, want %d", len(result), 3)
	}

	if callCount < 3 {
		t.Errorf("Expected at least 3 API calls for pagination, got %d", callCount)
	}
}
