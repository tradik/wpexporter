package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tradik/wpexportjson/internal/config"
	"github.com/tradik/wpexportjson/pkg/models"
)

// Client represents a WordPress REST API client
type Client struct {
	config     *config.Config
	httpClient *resty.Client
	baseURL    string
}

// NewClient creates a new WordPress API client
func NewClient(cfg *config.Config) (*Client, error) {
	// Parse and validate URL
	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Construct base API URL
	baseURL := strings.TrimSuffix(parsedURL.String(), "/") + "/wp-json/wp/v2"

	// Create HTTP client
	httpClient := resty.New()
	httpClient.SetTimeout(time.Duration(cfg.Timeout) * time.Second)
	httpClient.SetRetryCount(cfg.Retries)
	httpClient.SetHeader("User-Agent", cfg.UserAgent)
	httpClient.SetHeader("Accept", "application/json")

	return &Client{
		config:     cfg,
		httpClient: httpClient,
		baseURL:    baseURL,
	}, nil
}

// GetSiteInfo retrieves WordPress site information
func (c *Client) GetSiteInfo() (*models.SiteInfo, error) {
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

	return &siteInfo, nil
}

// GetPosts retrieves all posts with pagination
func (c *Client) GetPosts() ([]models.WordPressPost, error) {
	return c.getAllContent("posts", func() interface{} {
		return &[]models.WordPressPost{}
	})
}

// GetPages retrieves all pages with pagination
func (c *Client) GetPages() ([]models.WordPressPost, error) {
	return c.getAllContent("pages", func() interface{} {
		return &[]models.WordPressPost{}
	})
}

// GetMedia retrieves all media items with pagination
func (c *Client) GetMedia() ([]models.WordPressMedia, error) {
	var allMedia []models.WordPressMedia
	page := 1
	perPage := 100

	for {
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

	return allMedia, nil
}

// GetCategories retrieves all categories
func (c *Client) GetCategories() ([]models.WordPressCategory, error) {
	var allCategories []models.WordPressCategory
	page := 1
	perPage := 100

	for {
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

	return allCategories, nil
}

// GetTags retrieves all tags
func (c *Client) GetTags() ([]models.WordPressTag, error) {
	var allTags []models.WordPressTag
	page := 1
	perPage := 100

	for {
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

	return allTags, nil
}

// GetUsers retrieves all users
func (c *Client) GetUsers() ([]models.WordPressUser, error) {
	var allUsers []models.WordPressUser
	page := 1
	perPage := 100

	for {
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
func (c *Client) getAllContent(endpoint string, factory func() interface{}) ([]models.WordPressPost, error) {
	var allContent []models.WordPressPost
	page := 1
	perPage := 100

	for {
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
