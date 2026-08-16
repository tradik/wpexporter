package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestGetProducts_Success(t *testing.T) {
	products := []models.WooCommerceProduct{
		{
			ID:     1,
			Name:   "Product 1",
			Slug:   "product-1",
			Status: "publish",
		},
		{
			ID:     2,
			Name:   "Product 2",
			Slug:   "product-2",
			Status: "publish",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wc/v3/products" {
			w.Header().Set("Content-Type", "application/json")

			page := r.URL.Query().Get("page")
			if page == "" || page == "1" {
				w.WriteHeader(http.StatusOK)
				response, _ := json.Marshal(products)
				_, _ = w.Write(response)
				return
			}
			// Return empty for page 2 to stop pagination
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("[]"))
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
		Verbose:   true,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	result, err := client.GetProducts()
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 1, result[0].ID)
}

func TestGetProducts_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wc/v3/products" {
			w.WriteHeader(http.StatusNotFound)
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
		Verbose:   true,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	result, err := client.GetProducts()
	require.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestGetProducts_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
		Verbose:   true,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	// 401 is the shop refusing an unauthenticated request, which is a fact about
	// its configuration rather than an absence of products — and the caller
	// answers it by reading /wp/v2/product instead (#55).
	result, err := client.GetProducts()
	require.ErrorIs(t, err, ErrProductsNeedKeys)
	assert.Len(t, result, 0)
}

func TestGetProducts_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	result, err := client.GetProducts()
	require.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestGetProducts_OtherStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		Timeout:   5,
		Retries:   1,
		UserAgent: "test-agent",
		Verbose:   true,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	result, err := client.GetProducts()
	require.Error(t, err) // 503 is a genuine failure, not "no WooCommerce" (GO-003)
	assert.Len(t, result, 0)
}

func TestGetProducts_InvalidJSON(t *testing.T) {
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
		Verbose:   true,
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)

	result, err := client.GetProducts()
	require.Error(t, err) // malformed JSON is a genuine parse failure (GO-003)
	assert.Len(t, result, 0)
}

func TestNewClient_WithAuth(t *testing.T) {
	// Test with Bearer token
	cfg := &config.Config{
		URL:       "https://example.com",
		Timeout:   30,
		Retries:   3,
		UserAgent: "test-agent",
		AuthToken: "test-token",
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)

	// Test with Basic auth
	cfg2 := &config.Config{
		URL:       "https://example.com",
		Timeout:   30,
		Retries:   3,
		UserAgent: "test-agent",
		AuthUser:  "user",
		AuthPass:  "pass",
	}

	client2, err := NewClient(cfg2)
	require.NoError(t, err)
	assert.NotNil(t, client2)
}

func TestNewClient_InvalidURL(t *testing.T) {
	// Test with no host
	cfg := &config.Config{
		URL:     "http://",
		Timeout: 30,
	}

	_, err := NewClient(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "valid host")
}

func TestNewClient_MalformedURL(t *testing.T) {
	cfg := &config.Config{
		URL:     "://invalid-url",
		Timeout: 30,
	}

	_, err := NewClient(cfg)
	assert.Error(t, err)
}

func TestBruteForceContent_Pages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/pages/1" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			page := models.WordPressPost{ID: 1, Slug: "page-1"}
			response, _ := json.Marshal(page)
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
	require.NoError(t, err)

	found := make(chan interface{}, 10)
	progress := make(chan int, 10)

	go client.BruteForceContent("pages", 2, found, progress)

	var foundContent []interface{}
	for content := range found {
		// Check for non-nil interface and non-nil underlying value
		if content != nil && !isNilInterface(content) {
			foundContent = append(foundContent, content)
		}
	}

	assert.GreaterOrEqual(t, len(foundContent), 1)
}

// isNilInterface checks if an interface contains a nil pointer
func isNilInterface(i interface{}) bool {
	if i == nil {
		return true
	}
	switch v := i.(type) {
	case *models.WordPressPost:
		return v == nil
	case *models.WordPressMedia:
		return v == nil
	default:
		return false
	}
}

func TestBruteForceContent_Media(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wp-json/wp/v2/media/1" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			media := models.WordPressMedia{ID: 1, Slug: "media-1"}
			response, _ := json.Marshal(media)
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
	require.NoError(t, err)

	found := make(chan interface{}, 10)
	progress := make(chan int, 10)

	go client.BruteForceContent("media", 2, found, progress)

	var foundContent []interface{}
	for content := range found {
		// Check for non-nil interface and non-nil underlying value
		if content != nil && !isNilInterface(content) {
			foundContent = append(foundContent, content)
		}
	}

	assert.GreaterOrEqual(t, len(foundContent), 1)
}

func TestBruteForceContent_UnknownType(t *testing.T) {
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
	require.NoError(t, err)

	found := make(chan interface{}, 10)
	progress := make(chan int, 10)

	go client.BruteForceContent("unknown", 2, found, progress)

	var foundContent []interface{}
	for content := range found {
		foundContent = append(foundContent, content)
	}

	// Unknown type should find nothing
	assert.Len(t, foundContent, 0)
}

func TestGetMedia_Error(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetMedia()
	assert.Error(t, err)
}

func TestGetCategories_Error(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetCategories()
	assert.Error(t, err)
}

func TestGetTags_Error(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetTags()
	assert.Error(t, err)
}

func TestGetUsers_Error(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetUsers()
	assert.Error(t, err)
}

func TestGetPostByID_Error(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetPostByID(1)
	assert.Error(t, err)
}

func TestGetPageByID_Error(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetPageByID(1)
	assert.Error(t, err)
}

func TestGetMediaByID_Error(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetMediaByID(1)
	assert.Error(t, err)
}

func TestGetMedia_InvalidJSON(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetMedia()
	assert.Error(t, err)
}

func TestGetCategories_InvalidJSON(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetCategories()
	assert.Error(t, err)
}

func TestGetTags_InvalidJSON(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetTags()
	assert.Error(t, err)
}

func TestGetUsers_InvalidJSON(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetUsers()
	assert.Error(t, err)
}

func TestGetPostByID_InvalidJSON(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetPostByID(1)
	assert.Error(t, err)
}

func TestGetPageByID_InvalidJSON(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetPageByID(1)
	assert.Error(t, err)
}

func TestGetMediaByID_InvalidJSON(t *testing.T) {
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
	require.NoError(t, err)

	_, err = client.GetMediaByID(1)
	assert.Error(t, err)
}

func TestGetAllContent_Error(t *testing.T) {
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
	require.NoError(t, err)

	// Test through GetPosts which uses getAllContent
	_, err = client.GetPosts()
	assert.Error(t, err)
}

func TestGetAllContent_InvalidJSON(t *testing.T) {
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
	require.NoError(t, err)

	// Test through GetPages which uses getAllContent
	_, err = client.GetPages()
	assert.Error(t, err)
}
