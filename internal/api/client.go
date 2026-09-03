package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	// limits caps how much this client fetches, so a preview of a site does not
	// download the site (#60).
	limits limits
	// stated remembers each collection's size as the site reported it, so a
	// truncated export can say what it truncated.
	stated   map[string]int
	statedMu sync.Mutex
	// probe remembers which spelling of the REST API this site answers to. It
	// is filled in only after the ordinary one has failed, so a site that
	// answers normally never pays for the question (#66).
	probe routeProbe
	// unreadSitemaps names the index children --max-sitemap-documents kept this
	// run from reading, so a bounded inventory says what it left out.
	unreadSitemaps []string
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

	// Retry what is the source site's weather rather than its answer: a 5xx, a
	// 429, a dropped connection (#37). Without this, resty retried transport
	// errors only, so a single 500 from a shared host ended the whole export.
	httpClient.SetRetryCount(cfg.Retries)
	httpClient.SetRetryWaitTime(retryWaitTime)
	httpClient.SetRetryMaxWaitTime(retryMaxWaitTime)
	httpClient.AddRetryCondition(isTransientFailure)
	httpClient.SetRetryAfter(retryAfterDelay)
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
		limits:     newLimits(cfg.Limit, cfg.LimitPerType, cfg.LimitByType),
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

	siteInfo, gap, err := c.readAPIRootInfo()
	if err != nil {
		return nil, err
	}

	// /wp/v2/settings carries the richer set (admin email, date and time formats,
	// start of week) but needs authentication. Anything it returns wins, because
	// it is the site's own configuration rather than the public projection.
	settings := c.mergeSettingsInfo(&siteInfo)

	// Which page is the home and where the posts went: from those settings when
	// they answered, and from the markup the site publishes to every visitor
	// when they did not (#75).
	c.readFrontPage(&siteInfo, settings)

	if siteInfo.URL == "" {
		siteInfo.URL = c.config.URL
	}

	// The root document is one of three places the identity can come from. A
	// root that did not answer is only a hole when the settings document and
	// the published markup did not fill it either — otherwise the site was
	// described, just not by the endpoint that was asked first.
	if gap != nil && !describesSite(siteInfo) {
		// Not cached: the next run would read the empty record back with no gap
		// attached, and the gap is the whole report.
		return &siteInfo, gap
	}

	// Cache the result
	c.saveToCache(cacheKey, &siteInfo)

	return &siteInfo, nil
}

// siteInfoEndpoint names the document in a gap, as the collections name theirs.
const siteInfoEndpoint = "site info"

// describesSite reports whether anything was actually learned about the site.
//
// URL is excluded deliberately: it is the address the run was pointed at, not
// something the site said about itself, and a record carrying only that is the
// all-empty answer #79 is about.
func describesSite(info models.SiteInfo) bool {
	return info.Name != "" ||
		info.Description != "" ||
		info.HomeURL != "" ||
		info.AdminEmail != "" ||
		info.Timezone != "" ||
		info.DateFormat != "" ||
		info.TimeFormat != "" ||
		info.Language != "" ||
		info.ShowOnFront != "" ||
		info.FrontPage != nil ||
		info.PostsPage != nil
}

// readAPIRootInfo separates a root that could not be reached — which ends the
// run, since nothing else will be reachable either — from one that answered
// with something other than a REST document, which is a gap the caller carries.
func (c *Client) readAPIRootInfo() (info models.SiteInfo, gap, err error) {
	info, err = c.fetchAPIRootInfo()
	if _, partial := Gap(err); partial {
		return info, err, nil
	}

	return info, nil, err
}

// apiRootInfo is the unauthenticated /wp-json/ document. Its field names differ
// from the settings endpoint's, which is why it needs its own shape.
type apiRootInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Home        string `json:"home"`
	// GMTOffset is a number in core's own output and a string elsewhere, so it
	// is read as either — see jsonScalar (#32).
	GMTOffset      jsonScalar `json:"gmt_offset"`
	TimezoneString string     `json:"timezone_string"`
}

// siteSettings is the authenticated /wp/v2/settings document. Note `title`
// rather than `name`: reading it as `name` was why the site title came out
// empty even when the endpoint was reachable.
type siteSettings struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Email       string `json:"email"`
	Timezone    string `json:"timezone"`
	DateFormat  string `json:"date_format"`
	TimeFormat  string `json:"time_format"`
	StartOfWeek int    `json:"start_of_week"`
	Language    string `json:"language"`
	// The three settings that decide the shape of a migrated site (#75).
	ShowOnFront  string `json:"show_on_front"`
	PageOnFront  int    `json:"page_on_front"`
	PageForPosts int    `json:"page_for_posts"`
}

// fetchAPIRootInfo reads the REST API root, which every public WordPress serves
// without authentication and which carries the site's identity.
//
// A transport failure is returned as an error, since it means the site is not
// reachable at all. An unhelpful response — a status other than 200, or a body
// that is not the root document — is not: the export can still proceed and
// record which URL it was pointed at.
func (c *Client) fetchAPIRootInfo() (models.SiteInfo, error) {
	resp, err := c.fetchProbing(c.apiRootURL)
	if err != nil {
		// Named by the caller, which knows whether it is reporting a CLI step,
		// an MCP tool result or a gap. Naming it here too made the message read
		// "failed to get site info: failed to get site info: …".
		return models.SiteInfo{}, err
	}

	// A root that did not answer with a REST document leaves every identity
	// field empty. The empty record is still returned — a locked-down /wp-json
	// does not make a site unexportable — but it travels with a gap saying so,
	// or a caller reports "this site has no name" about a site it never read
	// (#79).
	if resp.StatusCode() != 200 {
		return models.SiteInfo{}, &UnansweredError{Endpoint: siteInfoEndpoint, Status: resp.StatusCode()}
	}

	var root apiRootInfo
	if err := json.Unmarshal(resp.Body(), &root); err != nil {
		return models.SiteInfo{}, &UnansweredError{
			Endpoint: siteInfoEndpoint,
			Status:   resp.StatusCode(),
			Err:      fmt.Errorf("answered with something other than a REST document: %w", err),
		}
	}

	timezone := root.TimezoneString
	if timezone == "" && root.GMTOffset != "" {
		// A site that never set a named zone reports only an offset.
		timezone = "UTC" + gmtOffsetSuffix(root.GMTOffset.String())
	}

	return models.SiteInfo{
		// WordPress stores the name and tagline entity-encoded and serves them
		// that way. They are plain text wherever they land next — a <title>, a
		// meta description, a template variable — so they are decoded here
		// rather than left for every consumer to trip over.
		Name:        html.UnescapeString(root.Name),
		Description: html.UnescapeString(root.Description),
		URL:         root.URL,
		HomeURL:     root.Home,
		Timezone:    timezone,
	}, nil
}

// mergeSettingsInfo overlays the authenticated settings document when it is
// reachable. An unauthenticated site returns 401 here, which is not an error:
// the root document already supplied the identity fields.
func (c *Client) mergeSettingsInfo(siteInfo *models.SiteInfo) siteSettings {
	resp, err := c.httpClient.R().Get(c.endpointURL("settings", ""))
	if err != nil || resp.StatusCode() != 200 {
		return siteSettings{}
	}

	var settings siteSettings
	if err := json.Unmarshal(resp.Body(), &settings); err != nil {
		return siteSettings{}
	}

	overlay(&siteInfo.Name, html.UnescapeString(settings.Title))
	overlay(&siteInfo.Description, html.UnescapeString(settings.Description))
	overlay(&siteInfo.URL, settings.URL)
	overlay(&siteInfo.AdminEmail, settings.Email)
	overlay(&siteInfo.Timezone, settings.Timezone)
	overlay(&siteInfo.DateFormat, settings.DateFormat)
	overlay(&siteInfo.TimeFormat, settings.TimeFormat)
	overlay(&siteInfo.Language, settings.Language)

	if settings.StartOfWeek != 0 {
		siteInfo.StartOfWeek = settings.StartOfWeek
	}

	return settings
}

// overlay replaces a value when the authoritative source supplied one.
func overlay(target *string, value string) {
	if strings.TrimSpace(value) != "" {
		*target = value
	}
}

// gmtOffsetSuffix renders a numeric GMT offset as a signed suffix.
func gmtOffsetSuffix(offset string) string {
	offset = strings.TrimSpace(offset)
	if offset == "" || strings.HasPrefix(offset, "-") || strings.HasPrefix(offset, "+") {
		return offset
	}

	return "+" + offset
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
		// The records read before the gap travel with the error; a partial
		// fetch is never cached, or the next run would inherit the gap as if
		// it were the site's whole content (#37).
		return posts, err
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
		// As with posts: what was read is returned with the gap, and a partial
		// fetch is not cached.
		return pages, err
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

	budget, proceed := c.takeBudget(CollectionProducts)
	if !proceed {
		return nil, nil
	}

	page := 1
	perPage := 100
	if budget > 0 && budget < perPage {
		perPage = budget
	}

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
			// A transport error is a genuine failure (the WP host is reachable —
			// we already fetched posts). Do not mask it as "no WooCommerce".
			return allProducts, fmt.Errorf("failed to fetch WooCommerce products page %d: %w", page, err)
		}

		done, statusErr := shopStatus(resp.StatusCode(), page)
		if statusErr != nil {
			return allProducts, statusErr
		}
		if done {
			break
		}

		var products []models.WooCommerceProduct
		if err := json.Unmarshal(resp.Body(), &products); err != nil {
			return allProducts, fmt.Errorf("failed to parse WooCommerce products page %d: %w", page, err)
		}

		if len(products) == 0 {
			break
		}

		allProducts = append(allProducts, products...)

		if budget > 0 && len(allProducts) >= budget {
			allProducts = allProducts[:budget]
			c.spendBudget(len(allProducts))

			return allProducts, nil
		}

		page++
	}

	c.spendBudget(len(allProducts))

	// Cache the result
	c.saveToCache(cacheKey, allProducts)

	return allProducts, nil
}

// shopStatus reads what the WooCommerce route's status means for the walk:
// carry on, stop, or stop and say why.
//
// The three refusals are deliberately distinct. 404 is a site without
// WooCommerce, which is not an error. 401 and 403 are a shop that exists and
// will not talk to us without consumer keys — a fact about its configuration,
// answered by reading /wp/v2/product instead (#55). Anything else unexpected is
// a genuine failure and must not be mistaken for an empty catalog.
func shopStatus(status, page int) (done bool, err error) {
	switch status {
	case http.StatusOK:
		return false, nil
	case http.StatusNotFound:
		return true, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return true, ErrProductsNeedKeys
	case http.StatusBadRequest:
		// Past the last page.
		return true, nil
	default:
		return true, fmt.Errorf("WooCommerce API returned status %d for products page %d", status, page)
	}
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

	// The media listing was never capped: --limit bounded the documents and
	// still walked the whole library, which on a site with 1204 attachments is
	// thirteen requests nobody asked for (#62).
	budget, proceed := c.takeBudget(CollectionMedia)
	if !proceed {
		return nil, nil
	}

	page := 1
	perPage := 100
	if budget > 0 && budget < perPage {
		perPage = budget
	}

	for {
		// Apply rate limiting between requests
		if page > 1 {
			c.applyRateLimit()
		}

		resp, err := c.fetchCollection("media", fmt.Sprintf("page=%d&per_page=%d", page, perPage))
		if err != nil {
			return allMedia, &PartialError{Endpoint: "media", Page: page, Fetched: len(allMedia), Err: err}
		}

		if resp.StatusCode() == 400 {
			// No more pages
			break
		}

		if resp.StatusCode() != 200 {
			// A gap, not the end of the export: one unreadable page used to
			// discard every record already fetched — and, with media, the whole
			// run including the posts and pages already in hand (#57).
			return allMedia, &PartialError{
				Endpoint: "media",
				Page:     page,
				Fetched:  len(allMedia),
				Err:      fmt.Errorf("API returned status %d", resp.StatusCode()),
			}
		}

		c.recordStated(CollectionMedia, collectionTotal(resp.Header().Get(totalHeader)))

		var media []models.WordPressMedia
		if err := json.Unmarshal(resp.Body(), &media); err != nil {
			return allMedia, &PartialError{
				Endpoint: "media",
				Page:     page,
				Fetched:  len(allMedia),
				Err:      fmt.Errorf("failed to parse response: %w", err),
			}
		}

		if len(media) == 0 {
			break
		}

		allMedia = append(allMedia, media...)

		if budget > 0 && len(allMedia) >= budget {
			allMedia = allMedia[:budget]
			c.spendBudget(len(allMedia))

			return allMedia, nil
		}

		page++
	}

	c.spendBudget(len(allMedia))

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

		resp, err := c.fetchCollection("categories", fmt.Sprintf("page=%d&per_page=%d", page, perPage))
		if err != nil {
			return allCategories, &PartialError{Endpoint: "categories", Page: page, Fetched: len(allCategories), Err: err}
		}

		if resp.StatusCode() == 400 {
			break
		}

		if resp.StatusCode() != 200 {
			// A gap, not the end of the export: one unreadable page used to
			// discard every record already fetched — and, with media, the whole
			// run including the posts and pages already in hand (#57).
			return allCategories, &PartialError{
				Endpoint: "categories",
				Page:     page,
				Fetched:  len(allCategories),
				Err:      fmt.Errorf("API returned status %d", resp.StatusCode()),
			}
		}

		var categories []models.WordPressCategory
		if err := json.Unmarshal(resp.Body(), &categories); err != nil {
			return allCategories, &PartialError{
				Endpoint: "categories",
				Page:     page,
				Fetched:  len(allCategories),
				Err:      fmt.Errorf("failed to parse response: %w", err),
			}
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

		resp, err := c.fetchCollection("tags", fmt.Sprintf("page=%d&per_page=%d", page, perPage))
		if err != nil {
			return allTags, &PartialError{Endpoint: "tags", Page: page, Fetched: len(allTags), Err: err}
		}

		if resp.StatusCode() == 400 {
			break
		}

		if resp.StatusCode() != 200 {
			// A gap, not the end of the export: one unreadable page used to
			// discard every record already fetched — and, with media, the whole
			// run including the posts and pages already in hand (#57).
			return allTags, &PartialError{
				Endpoint: "tags",
				Page:     page,
				Fetched:  len(allTags),
				Err:      fmt.Errorf("API returned status %d", resp.StatusCode()),
			}
		}

		var tags []models.WordPressTag
		if err := json.Unmarshal(resp.Body(), &tags); err != nil {
			return allTags, &PartialError{
				Endpoint: "tags",
				Page:     page,
				Fetched:  len(allTags),
				Err:      fmt.Errorf("failed to parse response: %w", err),
			}
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

		resp, err := c.fetchCollection("users", fmt.Sprintf("page=%d&per_page=%d", page, perPage))
		if err != nil {
			return allUsers, &PartialError{Endpoint: "users", Page: page, Fetched: len(allUsers), Err: err}
		}

		if resp.StatusCode() == 400 {
			break
		}

		if resp.StatusCode() != 200 {
			// A gap, not the end of the export: one unreadable page used to
			// discard every record already fetched — and, with media, the whole
			// run including the posts and pages already in hand (#57).
			return allUsers, &PartialError{
				Endpoint: "users",
				Page:     page,
				Fetched:  len(allUsers),
				Err:      fmt.Errorf("API returned status %d", resp.StatusCode()),
			}
		}

		var users []models.WordPressUser
		if err := json.Unmarshal(resp.Body(), &users); err != nil {
			return allUsers, &PartialError{
				Endpoint: "users",
				Page:     page,
				Fetched:  len(allUsers),
				Err:      fmt.Errorf("failed to parse response: %w", err),
			}
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

	// How much this collection may take, and what a page should ask for: there
	// is no sense fetching a hundred records to keep five (#60).
	budget, proceed := c.takeBudget(endpoint)
	if !proceed {
		return nil, nil
	}

	page := 1
	perPage := pageSizes[0]
	if budget > 0 && budget < perPage {
		perPage = budget
	}
	// What the site says the collection holds, from its own header. A walk that
	// ends with fewer records than this has missed something (#43).
	stated := 0

	for {
		// Apply rate limiting between requests
		if page > 1 {
			c.applyRateLimit()
		}

		result := c.fetchContentPage(endpoint, page, perPage, len(allContent))

		switch {
		case result.err != nil:
			return allContent, result.err
		case result.retryWith > 0:
			// The site refused this page size; the same page is asked for again
			// at one it may accept (#43).
			perPage = result.retryWith

			continue
		case result.done:
			c.spendBudget(len(allContent))

			return c.checkedContent(endpoint, allContent, stated, page)
		}

		if stated == 0 {
			stated = result.total
			c.recordStated(endpoint, stated)
		}

		allContent = append(allContent, result.content...)

		// Stop as soon as the budget is spent. The records come newest first,
		// which is the REST default, so the first N are the N worth previewing.
		if budget > 0 && len(allContent) >= budget {
			allContent = allContent[:budget]
			c.spendBudget(len(allContent))

			return allContent, nil
		}

		page++

		// A site that never says stop is bounded here: without a ceiling one
		// that repeats its first page for ever is followed until memory runs
		// out (#37).
		if page > maxContentPages {
			return allContent, &PartialError{
				Endpoint: endpoint,
				Page:     page,
				Fetched:  len(allContent),
				Err:      fmt.Errorf("collection did not end within %d pages", maxContentPages),
			}
		}
	}
}

// fetchContentPage reads one page of a collection and says what the walk should
// do next. Every failure carries what the caller had already fetched, because
// an export missing one page and saying so beats no export at all (#37).
func (c *Client) fetchContentPage(endpoint string, page, perPage, fetched int) pageResult {
	resp, err := c.fetchCollection(endpoint, fmt.Sprintf("page=%d&per_page=%d", page, perPage))
	if err != nil {
		return pageResult{err: &PartialError{Endpoint: endpoint, Page: page, Fetched: fetched, Err: err}}
	}

	if resp.StatusCode() == 400 {
		// WordPress answers 400 both past the last page and for a parameter it
		// will not accept. Treating the two alike is what made a collection
		// that refuses per_page=100 export as zero records, silently (#43).
		decided := classifyRefusal(resp.Body(), perPage, fetched)

		switch {
		case decided.retryWith > 0:
			return pageResult{retryWith: decided.retryWith}
		case decided.done:
			return pageResult{done: true}
		default:
			return pageResult{err: &PartialError{
				Endpoint: endpoint,
				Page:     page,
				Fetched:  fetched,
				Err:      fmt.Errorf("the site refused the request (%s)", decided.code),
			}}
		}
	}

	if resp.StatusCode() != 200 {
		return pageResult{err: &PartialError{
			Endpoint: endpoint,
			Page:     page,
			Fetched:  fetched,
			Err:      statusReason(resp),
		}}
	}

	var content []models.WordPressPost
	if err := json.Unmarshal(resp.Body(), &content); err != nil {
		// A 200 carrying a page rather than a payload is a wall, or a site with
		// no REST API answering its own home page. Naming which turns a line
		// about an unexpected '<' into something an operator can act on (#73).
		return pageResult{err: &PartialError{
			Endpoint: endpoint,
			Page:     page,
			Fetched:  fetched,
			Err:      unreadableBody(resp, err),
		}}
	}

	if len(content) == 0 {
		return pageResult{done: true}
	}

	return pageResult{content: content, total: collectionTotal(resp.Header().Get(totalHeader))}
}

// checkedContent returns a finished walk, or the gap between what it read and
// what the site said it holds. Reporting the shortfall is the difference
// between an export that is wrong and one that knows it (#43).
func (c *Client) checkedContent(endpoint string, content []models.WordPressPost, stated, page int) ([]models.WordPressPost, error) {
	if stated > 0 && len(content) < stated {
		return content, &PartialError{
			Endpoint: endpoint,
			Page:     page,
			Fetched:  len(content),
			Err:      fmt.Errorf("the site lists %d records here", stated),
		}
	}

	return content, nil
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
			return allProducts, fmt.Errorf("failed to fetch WooCommerce products page %d: %w", page, err)
		}

		if resp.StatusCode() == 404 || resp.StatusCode() == 401 {
			// WooCommerce not installed or no access — legitimately empty, not an error.
			return allProducts, nil
		}

		if resp.StatusCode() == 400 {
			break
		}

		if resp.StatusCode() != 200 {
			return allProducts, fmt.Errorf("WooCommerce API returned status %d for products page %d", resp.StatusCode(), page)
		}

		var products []models.WooCommerceProduct
		if err := json.Unmarshal(resp.Body(), &products); err != nil {
			return allProducts, fmt.Errorf("failed to parse WooCommerce products page %d: %w", page, err)
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
			return allMedia, &PartialError{
				Endpoint: "media",
				Page:     page,
				Fetched:  len(allMedia),
				Err:      fmt.Errorf("failed to parse response: %w", err),
			}
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
