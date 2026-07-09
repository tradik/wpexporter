package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"

	"github.com/tradik/wpexporter/internal/cache"
	"github.com/tradik/wpexporter/internal/checkpoint"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// maxAPIResponseBytes bounds a single WordPress REST API response to guard against
// unbounded memory allocation from a hostile or misbehaving endpoint (SEC-002).
const maxAPIResponseBytes = 64 << 20 // 64 MiB

// ProgressCallback is called after each page is fetched for checkpoint saving
type ProgressCallback func() error

// Client represents a WordPress REST API client
type Client struct {
	config     *config.Config
	httpClient *resty.Client
	baseURL    string
	cache      *cache.FileCache // Optional cache (nil if disabled)
}

// NewClient creates a new WordPress API client
func NewClient(cfg *config.Config) (*Client, error) {
	// Validate URL
	if cfg.URL == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Check for valid scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("URL must have http or https scheme")
	}

	// Check for valid host
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("URL must have a valid host")
	}

	// Construct base API URL
	baseURL := strings.TrimSuffix(parsedURL.String(), "/") + "/wp-json/wp/v2"

	// Create HTTP client
	httpClient := resty.New()
	httpClient.SetTimeout(time.Duration(cfg.Timeout) * time.Second)
	httpClient.SetRetryCount(cfg.Retries)
	httpClient.SetHeader("User-Agent", cfg.UserAgent)
	httpClient.SetHeader("Accept", "application/json")
	// Bound the response size to avoid unbounded memory use on a hostile or
	// misbehaving endpoint (SEC-002).
	httpClient.SetResponseBodyLimit(maxAPIResponseBytes)

	// Set authentication if configured
	if cfg.AuthToken != "" {
		httpClient.SetAuthToken(cfg.AuthToken)
	} else if cfg.AuthUser != "" && cfg.AuthPass != "" {
		httpClient.SetBasicAuth(cfg.AuthUser, cfg.AuthPass)
	}

	return &Client{
		config:     cfg,
		httpClient: httpClient,
		baseURL:    baseURL,
		cache:      nil, // Set via SetCache()
	}, nil
}

// SetCache sets the cache for the client
func (c *Client) SetCache(cache *cache.FileCache) {
	c.cache = cache
}

// applyRateLimit applies delay between API requests if rate limiting is configured
func (c *Client) applyRateLimit() {
	if c.config.RateLimit > 0 {
		time.Sleep(time.Duration(c.config.RateLimit) * time.Millisecond)
	}
}

// getFromCache tries to get data from cache
func (c *Client) getFromCache(key string, target interface{}) bool {
	if c.cache == nil {
		return false
	}

	data, found, err := c.cache.Get(key)
	if err != nil || !found {
		return false
	}

	if err := json.Unmarshal(data, target); err != nil {
		return false
	}

	return true
}

// saveToCache saves data to cache
func (c *Client) saveToCache(key string, data interface{}) {
	if c.cache == nil {
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	_ = c.cache.Set(key, jsonData)
}

// GetSiteInfo retrieves WordPress site information
func (c *Client) GetSiteInfo() (*models.SiteInfo, error) {
	cacheKey := cache.GenerateAPIKey("site_info", 0)

	// Try cache first
	var cachedInfo models.SiteInfo
	if c.getFromCache(cacheKey, &cachedInfo) {
		return &cachedInfo, nil
	}

	settingsURL := strings.Replace(c.baseURL, "/wp/v2", "", 1) + "/wp/v2/settings"

	resp, err := c.httpClient.R().Get(settingsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get site info: %w", err)
	}

	if resp.StatusCode() != 200 {
		// Try alternative endpoint
		resp, err = c.httpClient.R().Get(c.baseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to get site info: %w", err)
		}
	}

	var siteInfo models.SiteInfo
	if err := json.Unmarshal(resp.Body(), &siteInfo); err != nil {
		// If settings endpoint fails, create basic site info
		siteInfo = models.SiteInfo{
			URL:  c.config.URL,
			Name: "WordPress Site",
		}
	}

	// Cache the result
	c.saveToCache(cacheKey, &siteInfo)

	return &siteInfo, nil
}

// GetPosts retrieves all posts with pagination
func (c *Client) GetPosts() ([]models.WordPressPost, error) {
	cacheKey := cache.GenerateAPIKey("posts", 0)

	// Try cache first
	var cachedPosts []models.WordPressPost
	if c.getFromCache(cacheKey, &cachedPosts) {
		return cachedPosts, nil
	}

	posts, err := c.getAllContent("posts")
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.saveToCache(cacheKey, posts)

	return posts, nil
}

// GetPages retrieves all pages with pagination
func (c *Client) GetPages() ([]models.WordPressPost, error) {
	cacheKey := cache.GenerateAPIKey("pages", 0)

	// Try cache first
	var cachedPages []models.WordPressPost
	if c.getFromCache(cacheKey, &cachedPages) {
		return cachedPages, nil
	}

	pages, err := c.getAllContent("pages")
	if err != nil {
		return nil, err
	}

	// Cache the result
	c.saveToCache(cacheKey, pages)

	return pages, nil
}

// GetProducts retrieves all WooCommerce products with pagination
func (c *Client) GetProducts() ([]models.WooCommerceProduct, error) {
	cacheKey := cache.GenerateAPIKey("products", 0)

	// Try cache first
	var cachedProducts []models.WooCommerceProduct
	if c.getFromCache(cacheKey, &cachedProducts) {
		return cachedProducts, nil
	}

	var allProducts []models.WooCommerceProduct
	page := 1
	perPage := 100

	// WooCommerce uses a different API base
	wooBaseURL := strings.Replace(c.baseURL, "/wp/v2", "/wc/v3", 1)

	for {
		// Apply rate limiting between requests
		if page > 1 {
			c.applyRateLimit()
		}

		url := fmt.Sprintf("%s/products?page=%d&per_page=%d", wooBaseURL, page, perPage)

		resp, err := c.httpClient.R().Get(url)
		if err != nil {
			// WooCommerce might not be installed, return empty list
			if c.config.Verbose {
				fmt.Printf("Note: Could not fetch WooCommerce products: %v\n", err)
			}
			return allProducts, nil
		}

		if resp.StatusCode() == 404 || resp.StatusCode() == 401 {
			// WooCommerce not installed or no access
			return allProducts, nil
		}

		if resp.StatusCode() == 400 {
			// No more pages
			break
		}

		if resp.StatusCode() != 200 {
			// WooCommerce might require authentication
			if c.config.Verbose {
				fmt.Printf("Note: WooCommerce API returned status %d (may require authentication)\n", resp.StatusCode())
			}
			return allProducts, nil
		}

		var products []models.WooCommerceProduct
		if err := json.Unmarshal(resp.Body(), &products); err != nil {
			// Parsing error, WooCommerce might not be the expected version
			if c.config.Verbose {
				fmt.Printf("Note: Could not parse WooCommerce products: %v\n", err)
			}
			return allProducts, nil
		}

		if len(products) == 0 {
			break
		}

		allProducts = append(allProducts, products...)
		page++
	}

	// Cache the result
	c.saveToCache(cacheKey, allProducts)

	return allProducts, nil
}

// GetMedia retrieves all media items with pagination
func (c *Client) GetMedia() ([]models.WordPressMedia, error) {
	cacheKey := cache.GenerateAPIKey("media", 0)

	// Try cache first
	var cachedMedia []models.WordPressMedia
	if c.getFromCache(cacheKey, &cachedMedia) {
		return cachedMedia, nil
	}

	var allMedia []models.WordPressMedia
	page := 1
	perPage := 100

	for {
		// Apply rate limiting between requests
		if page > 1 {
			c.applyRateLimit()
		}

		url := fmt.Sprintf("%s/media?page=%d&per_page=%d", c.baseURL, page, perPage)

		resp, err := c.httpClient.R().Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get media page %d: %w", page, err)
		}

		if resp.StatusCode() == 400 {
			// No more pages
			break
		}

		if resp.StatusCode() != 200 {
			return nil, fmt.Errorf("API returned status %d for media page %d", resp.StatusCode(), page)
		}

		var media []models.WordPressMedia
		if err := json.Unmarshal(resp.Body(), &media); err != nil {
			return nil, fmt.Errorf("failed to parse media response: %w", err)
		}

		if len(media) == 0 {
			break
		}

		allMedia = append(allMedia, media...)
		page++
	}

	// Cache the result
	c.saveToCache(cacheKey, allMedia)

	return allMedia, nil
}

// GetCategories retrieves all categories
func (c *Client) GetCategories() ([]models.WordPressCategory, error) {
	cacheKey := cache.GenerateAPIKey("categories", 0)

	// Try cache first
	var cachedCategories []models.WordPressCategory
	if c.getFromCache(cacheKey, &cachedCategories) {
		return cachedCategories, nil
	}

	var allCategories []models.WordPressCategory
	page := 1
	perPage := 100

	for {
		// Apply rate limiting between requests
		if page > 1 {
			c.applyRateLimit()
		}

		url := fmt.Sprintf("%s/categories?page=%d&per_page=%d", c.baseURL, page, perPage)

		resp, err := c.httpClient.R().Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get categories page %d: %w", page, err)
		}

		if resp.StatusCode() == 400 {
			break
		}

		if resp.StatusCode() != 200 {
			return nil, fmt.Errorf("API returned status %d for categories page %d", resp.StatusCode(), page)
		}

		var categories []models.WordPressCategory
		if err := json.Unmarshal(resp.Body(), &categories); err != nil {
			return nil, fmt.Errorf("failed to parse categories response: %w", err)
		}

		if len(categories) == 0 {
			break
		}

		allCategories = append(allCategories, categories...)
		page++
	}

	// Cache the result
	c.saveToCache(cacheKey, allCategories)

	return allCategories, nil
}

// GetTags retrieves all tags
func (c *Client) GetTags() ([]models.WordPressTag, error) {
	cacheKey := cache.GenerateAPIKey("tags", 0)

	// Try cache first
	var cachedTags []models.WordPressTag
	if c.getFromCache(cacheKey, &cachedTags) {
		return cachedTags, nil
	}

	var allTags []models.WordPressTag
	page := 1
	perPage := 100

	for {
		// Apply rate limiting between requests
		if page > 1 {
			c.applyRateLimit()
		}

		url := fmt.Sprintf("%s/tags?page=%d&per_page=%d", c.baseURL, page, perPage)

		resp, err := c.httpClient.R().Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get tags page %d: %w", page, err)
		}

		if resp.StatusCode() == 400 {
			break
		}

		if resp.StatusCode() != 200 {
			return nil, fmt.Errorf("API returned status %d for tags page %d", resp.StatusCode(), page)
		}

		var tags []models.WordPressTag
		if err := json.Unmarshal(resp.Body(), &tags); err != nil {
			return nil, fmt.Errorf("failed to parse tags response: %w", err)
		}

		if len(tags) == 0 {
			break
		}

		allTags = append(allTags, tags...)
		page++
	}

	// Cache the result
	c.saveToCache(cacheKey, allTags)

	return allTags, nil
}

// GetUsers retrieves all users
func (c *Client) GetUsers() ([]models.WordPressUser, error) {
	cacheKey := cache.GenerateAPIKey("users", 0)

	// Try cache first
	var cachedUsers []models.WordPressUser
	if c.getFromCache(cacheKey, &cachedUsers) {
		return cachedUsers, nil
	}

	var allUsers []models.WordPressUser
	page := 1
	perPage := 100

	for {
		// Apply rate limiting between requests
		if page > 1 {
			c.applyRateLimit()
		}

		url := fmt.Sprintf("%s/users?page=%d&per_page=%d", c.baseURL, page, perPage)

		resp, err := c.httpClient.R().Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get users page %d: %w", page, err)
		}

		if resp.StatusCode() == 400 {
			break
		}

		if resp.StatusCode() != 200 {
			return nil, fmt.Errorf("API returned status %d for users page %d", resp.StatusCode(), page)
		}

		var users []models.WordPressUser
		if err := json.Unmarshal(resp.Body(), &users); err != nil {
			return nil, fmt.Errorf("failed to parse users response: %w", err)
		}

		if len(users) == 0 {
			break
		}

		allUsers = append(allUsers, users...)
		page++
	}

	// Cache the result
	c.saveToCache(cacheKey, allUsers)

	return allUsers, nil
}

// GetPostByID retrieves a specific post by ID
func (c *Client) GetPostByID(id int) (*models.WordPressPost, error) {
	url := fmt.Sprintf("%s/posts/%d", c.baseURL, id)

	resp, err := c.httpClient.R().Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get post %d: %w", id, err)
	}

	if resp.StatusCode() == 404 {
		return nil, nil // Post not found
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("API returned status %d for post %d", resp.StatusCode(), id)
	}

	var post models.WordPressPost
	if err := json.Unmarshal(resp.Body(), &post); err != nil {
		return nil, fmt.Errorf("failed to parse post response: %w", err)
	}

	return &post, nil
}

// GetPageByID retrieves a specific page by ID
func (c *Client) GetPageByID(id int) (*models.WordPressPost, error) {
	url := fmt.Sprintf("%s/pages/%d", c.baseURL, id)

	resp, err := c.httpClient.R().Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get page %d: %w", id, err)
	}

	if resp.StatusCode() == 404 {
		return nil, nil // Page not found
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("API returned status %d for page %d", resp.StatusCode(), id)
	}

	var page models.WordPressPost
	if err := json.Unmarshal(resp.Body(), &page); err != nil {
		return nil, fmt.Errorf("failed to parse page response: %w", err)
	}

	return &page, nil
}

// GetMediaByID retrieves a specific media item by ID
func (c *Client) GetMediaByID(id int) (*models.WordPressMedia, error) {
	url := fmt.Sprintf("%s/media/%d", c.baseURL, id)

	resp, err := c.httpClient.R().Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get media %d: %w", id, err)
	}

	if resp.StatusCode() == 404 {
		return nil, nil // Media not found
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("API returned status %d for media %d", resp.StatusCode(), id)
	}

	var media models.WordPressMedia
	if err := json.Unmarshal(resp.Body(), &media); err != nil {
		return nil, fmt.Errorf("failed to parse media response: %w", err)
	}

	return &media, nil
}

// getAllContent is a generic function to retrieve all content with pagination
func (c *Client) getAllContent(endpoint string) ([]models.WordPressPost, error) {
	var allContent []models.WordPressPost
	page := 1
	perPage := 100

	for {
		// Apply rate limiting between requests
		if page > 1 {
			c.applyRateLimit()
		}

		url := fmt.Sprintf("%s/%s?page=%d&per_page=%d", c.baseURL, endpoint, page, perPage)

		resp, err := c.httpClient.R().Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s page %d: %w", endpoint, page, err)
		}

		if resp.StatusCode() == 400 {
			// No more pages
			break
		}

		if resp.StatusCode() != 200 {
			return nil, fmt.Errorf("API returned status %d for %s page %d", resp.StatusCode(), endpoint, page)
		}

		var content []models.WordPressPost
		if err := json.Unmarshal(resp.Body(), &content); err != nil {
			return nil, fmt.Errorf("failed to parse %s response: %w", endpoint, err)
		}

		if len(content) == 0 {
			break
		}

		allContent = append(allContent, content...)
		page++
	}

	return allContent, nil
}

// BruteForceContent attempts to discover content by ID enumeration
func (c *Client) BruteForceContent(contentType string, maxID int, found chan<- interface{}, progress chan<- int) {
	defer close(found)
	defer close(progress)

	for id := 1; id <= maxID; id++ {
		var content interface{}
		var err error

		switch contentType {
		case "posts":
			content, err = c.GetPostByID(id)
		case "pages":
			content, err = c.GetPageByID(id)
		case "media":
			content, err = c.GetMediaByID(id)
		default:
			continue
		}

		if err == nil && content != nil {
			found <- content
		}

		// Send progress update
		select {
		case progress <- id:
		default:
		}

		// Small delay to avoid overwhelming the server
		time.Sleep(10 * time.Millisecond)
	}
}

// GetPostsWithCheckpoint retrieves posts with checkpoint support for resume
func (c *Client) GetPostsWithCheckpoint(state *checkpoint.State, onProgress ProgressCallback) ([]models.WordPressPost, error) {
	if state != nil && state.IsPostsCompleted() {
		return []models.WordPressPost{}, nil // Already completed
	}

	var allContent []models.WordPressPost
	startPage := 1
	if state != nil {
		startPage = state.GetPostsPage()
	}

	page := startPage
	perPage := 100

	for {
		if page > 1 {
			c.applyRateLimit()
		}

		url := fmt.Sprintf("%s/posts?page=%d&per_page=%d", c.baseURL, page, perPage)

		resp, err := c.httpClient.R().Get(url)
		if err != nil {
			if state != nil {
				state.SetLastError(err.Error())
			}
			return nil, fmt.Errorf("failed to get posts page %d: %w", page, err)
		}

		if resp.StatusCode() == 400 {
			break
		}

		if resp.StatusCode() != 200 {
			errMsg := fmt.Sprintf("API returned status %d for posts page %d", resp.StatusCode(), page)
			if state != nil {
				state.SetLastError(errMsg)
			}
			return nil, errors.New(errMsg)
		}

		var content []models.WordPressPost
		if err := json.Unmarshal(resp.Body(), &content); err != nil {
			return nil, fmt.Errorf("failed to parse posts response: %w", err)
		}

		if len(content) == 0 {
			break
		}

		allContent = append(allContent, content...)

		// Update checkpoint
		if state != nil {
			ids := make([]int, len(content))
			for i, c := range content {
				ids[i] = c.ID
			}
			state.AddPostIDs(ids)
			state.SetPostsPage(page + 1)
			if onProgress != nil {
				if err := onProgress(); err != nil {
					return nil, fmt.Errorf("failed to save checkpoint: %w", err)
				}
			}
		}

		page++
	}

	if state != nil {
		state.SetPostsCompleted()
		if onProgress != nil {
			_ = onProgress()
		}
	}

	return allContent, nil
}

// GetPagesWithCheckpoint retrieves pages with checkpoint support for resume
func (c *Client) GetPagesWithCheckpoint(state *checkpoint.State, onProgress ProgressCallback) ([]models.WordPressPost, error) {
	if state != nil && state.IsPagesCompleted() {
		return []models.WordPressPost{}, nil
	}

	var allContent []models.WordPressPost
	startPage := 1
	if state != nil {
		startPage = state.GetPagesPage()
	}

	page := startPage
	perPage := 100

	for {
		if page > 1 {
			c.applyRateLimit()
		}

		url := fmt.Sprintf("%s/pages?page=%d&per_page=%d", c.baseURL, page, perPage)

		resp, err := c.httpClient.R().Get(url)
		if err != nil {
			if state != nil {
				state.SetLastError(err.Error())
			}
			return nil, fmt.Errorf("failed to get pages page %d: %w", page, err)
		}

		if resp.StatusCode() == 400 {
			break
		}

		if resp.StatusCode() != 200 {
			errMsg := fmt.Sprintf("API returned status %d for pages page %d", resp.StatusCode(), page)
			if state != nil {
				state.SetLastError(errMsg)
			}
			return nil, errors.New(errMsg)
		}

		var content []models.WordPressPost
		if err := json.Unmarshal(resp.Body(), &content); err != nil {
			return nil, fmt.Errorf("failed to parse pages response: %w", err)
		}

		if len(content) == 0 {
			break
		}

		allContent = append(allContent, content...)

		if state != nil {
			ids := make([]int, len(content))
			for i, c := range content {
				ids[i] = c.ID
			}
			state.AddPageIDs(ids)
			state.SetPagesPage(page + 1)
			if onProgress != nil {
				if err := onProgress(); err != nil {
					return nil, fmt.Errorf("failed to save checkpoint: %w", err)
				}
			}
		}

		page++
	}

	if state != nil {
		state.SetPagesCompleted()
		if onProgress != nil {
			_ = onProgress()
		}
	}

	return allContent, nil
}

// GetProductsWithCheckpoint retrieves WooCommerce products with checkpoint support
func (c *Client) GetProductsWithCheckpoint(state *checkpoint.State, onProgress ProgressCallback) ([]models.WooCommerceProduct, error) {
	if state != nil && state.IsProductsCompleted() {
		return []models.WooCommerceProduct{}, nil
	}

	var allProducts []models.WooCommerceProduct
	startPage := 1
	if state != nil {
		startPage = state.GetProductsPage()
	}

	page := startPage
	perPage := 100
	wooBaseURL := strings.Replace(c.baseURL, "/wp/v2", "/wc/v3", 1)

	for {
		if page > 1 {
			c.applyRateLimit()
		}

		url := fmt.Sprintf("%s/products?page=%d&per_page=%d", wooBaseURL, page, perPage)

		resp, err := c.httpClient.R().Get(url)
		if err != nil {
			if c.config.Verbose {
				fmt.Printf("Note: Could not fetch WooCommerce products: %v\n", err)
			}
			return allProducts, nil
		}

		if resp.StatusCode() == 404 || resp.StatusCode() == 401 {
			return allProducts, nil
		}

		if resp.StatusCode() == 400 {
			break
		}

		if resp.StatusCode() != 200 {
			if c.config.Verbose {
				fmt.Printf("Note: WooCommerce API returned status %d (may require authentication)\n", resp.StatusCode())
			}
			return allProducts, nil
		}

		var products []models.WooCommerceProduct
		if err := json.Unmarshal(resp.Body(), &products); err != nil {
			if c.config.Verbose {
				fmt.Printf("Note: Could not parse WooCommerce products: %v\n", err)
			}
			return allProducts, nil
		}

		if len(products) == 0 {
			break
		}

		allProducts = append(allProducts, products...)

		if state != nil {
			ids := make([]int, len(products))
			for i, p := range products {
				ids[i] = p.ID
			}
			state.AddProductIDs(ids)
			state.SetProductsPage(page + 1)
			if onProgress != nil {
				if err := onProgress(); err != nil {
					return nil, fmt.Errorf("failed to save checkpoint: %w", err)
				}
			}
		}

		page++
	}

	if state != nil {
		state.SetProductsCompleted()
		if onProgress != nil {
			_ = onProgress()
		}
	}

	return allProducts, nil
}

// GetMediaWithCheckpoint retrieves media with checkpoint support
func (c *Client) GetMediaWithCheckpoint(state *checkpoint.State, onProgress ProgressCallback) ([]models.WordPressMedia, error) {
	if state != nil && state.IsMediaCompleted() {
		return []models.WordPressMedia{}, nil
	}

	var allMedia []models.WordPressMedia
	startPage := 1
	if state != nil {
		startPage = state.GetMediaPage()
	}

	page := startPage
	perPage := 100

	for {
		if page > 1 {
			c.applyRateLimit()
		}

		url := fmt.Sprintf("%s/media?page=%d&per_page=%d", c.baseURL, page, perPage)

		resp, err := c.httpClient.R().Get(url)
		if err != nil {
			if state != nil {
				state.SetLastError(err.Error())
			}
			return nil, fmt.Errorf("failed to get media page %d: %w", page, err)
		}

		if resp.StatusCode() == 400 {
			break
		}

		if resp.StatusCode() != 200 {
			errMsg := fmt.Sprintf("API returned status %d for media page %d", resp.StatusCode(), page)
			if state != nil {
				state.SetLastError(errMsg)
			}
			return nil, errors.New(errMsg)
		}

		var media []models.WordPressMedia
		if err := json.Unmarshal(resp.Body(), &media); err != nil {
			return nil, fmt.Errorf("failed to parse media response: %w", err)
		}

		if len(media) == 0 {
			break
		}

		allMedia = append(allMedia, media...)

		if state != nil {
			ids := make([]int, len(media))
			for i, m := range media {
				ids[i] = m.ID
			}
			state.AddMediaIDs(ids)
			state.SetMediaPage(page + 1)
			if onProgress != nil {
				if err := onProgress(); err != nil {
					return nil, fmt.Errorf("failed to save checkpoint: %w", err)
				}
			}
		}

		page++
	}

	if state != nil {
		state.SetMediaCompleted()
		if onProgress != nil {
			_ = onProgress()
		}
	}

	return allMedia, nil
}
