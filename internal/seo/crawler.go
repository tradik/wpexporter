package seo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// Crawler extracts SEO metadata from URLs
type Crawler struct {
	config     *config.Config
	httpClient *http.Client
}

// NewCrawler creates a new SEO crawler
func NewCrawler(cfg *config.Config) *Crawler {
	return &Crawler{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}
}

// EnrichPostsWithSEO crawls post URLs and extracts SEO data
func (c *Crawler) EnrichPostsWithSEO(posts []models.WordPressPost) []models.WordPressPost {
	if len(posts) == 0 {
		return posts
	}

	// Create progress bar
	progress := progressbar.NewOptions(len(posts),
		progressbar.OptionSetDescription("Crawling SEO data"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	// Create work queue
	type job struct {
		index int
		url   string
	}

	jobs := make(chan job, len(posts))
	results := make(chan struct {
		index int
		seo   models.SEOData
	}, len(posts))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < c.config.Concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				seoData := c.extractSEO(j.url)
				results <- struct {
					index int
					seo   models.SEOData
				}{j.index, seoData}
			}
		}()
	}

	// Send jobs
	for i, post := range posts {
		jobs <- job{index: i, url: post.Link}
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for result := range results {
		posts[result.index].SEO = result.seo
		_ = progress.Add(1)
	}

	_ = progress.Finish()
	return posts
}

// extractSEO fetches a URL and extracts SEO metadata from HTML
func (c *Crawler) extractSEO(pageURL string) models.SEOData {
	seo := models.SEOData{}

	if pageURL == "" {
		return seo
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return seo
	}

	// Set user agent
	req.Header.Set("User-Agent", c.config.UserAgent)

	// Apply authentication if configured
	if c.config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.AuthToken)
	} else if c.config.AuthUser != "" && c.config.AuthPass != "" {
		req.SetBasicAuth(c.config.AuthUser, c.config.AuthPass)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return seo
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return seo
	}

	// Read response body (limit to 2MB to prevent memory issues)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return seo
	}

	html := string(body)

	// Extract <title> tag
	seo.Title = c.extractTitle(html)

	// Extract meta description
	seo.MetaDescription = c.extractMetaContent(html, "description")

	// Extract meta keywords
	seo.MetaKeywords = c.extractMetaContent(html, "keywords")

	// Extract Open Graph tags
	seo.OGTitle = c.extractOGContent(html, "og:title")
	seo.OGDescription = c.extractOGContent(html, "og:description")
	seo.OGImage = c.extractOGContent(html, "og:image")

	// Extract canonical URL
	seo.CanonicalURL = c.extractCanonical(html)

	return seo
}

// extractTitle extracts the content of the <title> tag
func (c *Crawler) extractTitle(html string) string {
	// Case-insensitive pattern for title tag
	pattern := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	matches := pattern.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(c.decodeHTMLEntities(matches[1]))
	}
	return ""
}

// extractMetaContent extracts content from a meta tag by name
func (c *Crawler) extractMetaContent(html, name string) string {
	// Try name="..." content="..." format
	patternStr := `(?i)<meta[^>]+name\s*=\s*["']%s["'][^>]+content\s*=\s*["']([^"']+)["']`
	pattern1 := regexp.MustCompile(fmt.Sprintf(patternStr, regexp.QuoteMeta(name)))
	matches := pattern1.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(c.decodeHTMLEntities(matches[1]))
	}

	// Try content="..." name="..." format (reverse order)
	patternStr2 := `(?i)<meta[^>]+content\s*=\s*["']([^"']+)["'][^>]+name\s*=\s*["']%s["']`
	pattern2 := regexp.MustCompile(fmt.Sprintf(patternStr2, regexp.QuoteMeta(name)))
	matches = pattern2.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(c.decodeHTMLEntities(matches[1]))
	}

	return ""
}

// extractOGContent extracts content from an Open Graph meta tag
func (c *Crawler) extractOGContent(html, property string) string {
	// Try property="..." content="..." format
	patternStr := `(?i)<meta[^>]+property\s*=\s*["']%s["'][^>]+content\s*=\s*["']([^"']+)["']`
	pattern1 := regexp.MustCompile(fmt.Sprintf(patternStr, regexp.QuoteMeta(property)))
	matches := pattern1.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(c.decodeHTMLEntities(matches[1]))
	}

	// Try content="..." property="..." format (reverse order)
	patternStr2 := `(?i)<meta[^>]+content\s*=\s*["']([^"']+)["'][^>]+property\s*=\s*["']%s["']`
	pattern2 := regexp.MustCompile(fmt.Sprintf(patternStr2, regexp.QuoteMeta(property)))
	matches = pattern2.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(c.decodeHTMLEntities(matches[1]))
	}

	return ""
}

// extractCanonical extracts the canonical URL from a link tag
func (c *Crawler) extractCanonical(html string) string {
	// Try rel="canonical" href="..." format
	pattern1 := regexp.MustCompile(
		`(?i)<link[^>]+rel\s*=\s*["']canonical["'][^>]+href\s*=\s*["']([^"']+)["']`)
	matches := pattern1.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try href="..." rel="canonical" format (reverse order)
	pattern2 := regexp.MustCompile(
		`(?i)<link[^>]+href\s*=\s*["']([^"']+)["'][^>]+rel\s*=\s*["']canonical["']`)
	matches = pattern2.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return ""
}

// decodeHTMLEntities decodes common HTML entities
func (c *Crawler) decodeHTMLEntities(s string) string {
	replacements := map[string]string{
		"&amp;":   "&",
		"&lt;":    "<",
		"&gt;":    ">",
		"&quot;":  "\"",
		"&#39;":   "'",
		"&apos;":  "'",
		"&nbsp;":  " ",
		"&#x27;":  "'",
		"&#x2F;":  "/",
		"&ndash;": "-",
		"&mdash;": "-",
		"&lsquo;": "'",
		"&rsquo;": "'",
		"&ldquo;": "\"",
		"&rdquo;": "\"",
	}

	result := s
	for entity, char := range replacements {
		result = strings.ReplaceAll(result, entity, char)
	}
	return result
}

// EnrichPostsWithContent crawls posts with empty content and extracts HTML body content
func (c *Crawler) EnrichPostsWithContent(posts []models.WordPressPost) []models.WordPressPost {
	// Find posts with empty content
	emptyPosts := make([]int, 0)
	for i, post := range posts {
		if isContentEmpty(post.Content.Rendered) {
			emptyPosts = append(emptyPosts, i)
		}
	}

	if len(emptyPosts) == 0 {
		return posts
	}

	fmt.Printf("Found %d posts/pages with empty content, crawling...\n", len(emptyPosts))

	// Create progress bar
	progress := progressbar.NewOptions(len(emptyPosts),
		progressbar.OptionSetDescription("Crawling content"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	// Create work queue
	type job struct {
		postIndex int
		url       string
	}

	jobs := make(chan job, len(emptyPosts))
	results := make(chan struct {
		postIndex int
		content   string
	}, len(emptyPosts))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < c.config.Concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				content := c.extractPageContent(j.url)
				results <- struct {
					postIndex int
					content   string
				}{j.postIndex, content}
			}
		}()
	}

	// Send jobs
	for _, idx := range emptyPosts {
		jobs <- job{postIndex: idx, url: posts[idx].Link}
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	enriched := 0
	for result := range results {
		if result.content != "" {
			posts[result.postIndex].Content.Rendered = result.content
			enriched++
		}
		_ = progress.Add(1)
	}

	_ = progress.Finish()
	fmt.Printf("Enriched %d posts/pages with crawled content\n", enriched)

	return posts
}

// extractPageContent fetches a URL and extracts the main content from HTML
func (c *Crawler) extractPageContent(pageURL string) string {
	if pageURL == "" {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return ""
	}

	// Set user agent
	req.Header.Set("User-Agent", c.config.UserAgent)

	// Apply authentication if configured
	if c.config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.AuthToken)
	} else if c.config.AuthUser != "" && c.config.AuthPass != "" {
		req.SetBasicAuth(c.config.AuthUser, c.config.AuthPass)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ""
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	// Read response body (limit to 5MB for full page content)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return ""
	}

	html := string(body)

	// Extract main content area
	content := c.extractMainContent(html)

	return content
}

// extractMainContent extracts the main content from HTML
func (c *Crawler) extractMainContent(html string) string {
	// Try to find main content area in order of preference
	contentPatterns := []string{
		// WordPress main content areas
		`(?is)<main[^>]*id\s*=\s*["']?content["']?[^>]*>(.*?)</main>`,
		`(?is)<main[^>]*>(.*?)</main>`,
		`(?is)<article[^>]*class\s*=\s*["'][^"']*(?:post|page|entry)[^"']*["'][^>]*>(.*?)</article>`,
		`(?is)<article[^>]*>(.*?)</article>`,
		// Bricks Builder specific
		`(?is)<div[^>]*id\s*=\s*["']?brx-content["']?[^>]*>(.*?)</div>`,
		`(?is)<section[^>]*class\s*=\s*["'][^"']*brxe-[^"']*["'][^>]*>(.*?)</section>`,
		// Elementor specific
		`(?is)<div[^>]*class\s*=\s*["'][^"']*elementor-widget-container[^"']*["'][^>]*>(.*?)</div>`,
		// Generic content areas
		`(?is)<div[^>]*id\s*=\s*["']?content["']?[^>]*>(.*?)</div>`,
		`(?is)<div[^>]*class\s*=\s*["'][^"']*(?:content|entry-content|post-content|page-content)[^"']*["'][^>]*>(.*?)</div>`,
	}

	for _, patternStr := range contentPatterns {
		pattern := regexp.MustCompile(patternStr)
		matches := pattern.FindStringSubmatch(html)
		if len(matches) > 1 && strings.TrimSpace(matches[1]) != "" {
			content := c.cleanHTMLContent(matches[1])
			if len(content) > 50 { // Only return if we got substantial content
				return content
			}
		}
	}

	// Fallback: try to extract all text from Bricks text elements
	bricksPattern := regexp.MustCompile(`(?is)<[^>]*class\s*=\s*["'][^"']*brxe-text[^"']*["'][^>]*>([^<]+)</[^>]+>`)
	bricksMatches := bricksPattern.FindAllStringSubmatch(html, -1)
	if len(bricksMatches) > 0 {
		var texts []string
		for _, match := range bricksMatches {
			if len(match) > 1 {
				text := strings.TrimSpace(c.decodeHTMLEntities(match[1]))
				if text != "" {
					texts = append(texts, "<p>"+text+"</p>")
				}
			}
		}
		if len(texts) > 0 {
			return strings.Join(texts, "\n")
		}
	}

	return ""
}

// cleanHTMLContent cleans HTML content, removing scripts, styles, and normalizing whitespace
func (c *Crawler) cleanHTMLContent(html string) string {
	// Remove script tags
	scriptPattern := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptPattern.ReplaceAllString(html, "")

	// Remove style tags
	stylePattern := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = stylePattern.ReplaceAllString(html, "")

	// Remove comments
	commentPattern := regexp.MustCompile(`(?is)<!--.*?-->`)
	html = commentPattern.ReplaceAllString(html, "")

	// Remove noscript tags
	noscriptPattern := regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	html = noscriptPattern.ReplaceAllString(html, "")

	// Remove empty tags
	emptyTagPattern := regexp.MustCompile(`(?i)<[^/>][^>]*>\s*</[^>]+>`)
	for i := 0; i < 3; i++ { // Multiple passes to handle nested empty tags
		html = emptyTagPattern.ReplaceAllString(html, "")
	}

	// Decode HTML entities
	html = c.decodeHTMLEntities(html)

	// Normalize whitespace
	whitespacePattern := regexp.MustCompile(`\s+`)
	html = whitespacePattern.ReplaceAllString(html, " ")

	return strings.TrimSpace(html)
}

// isContentEmpty checks if content is effectively empty
func isContentEmpty(content string) bool {
	if content == "" {
		return true
	}

	// Strip HTML tags
	tagPattern := regexp.MustCompile(`<[^>]*>`)
	stripped := tagPattern.ReplaceAllString(content, "")

	// Remove whitespace and common empty content patterns
	stripped = strings.TrimSpace(stripped)
	stripped = strings.ReplaceAll(stripped, "&nbsp;", "")
	stripped = strings.ReplaceAll(stripped, "\n", "")
	stripped = strings.ReplaceAll(stripped, "\t", "")
	stripped = strings.ReplaceAll(stripped, " ", "")

	return len(stripped) < 10 // Less than 10 characters = empty
}

// FilterEmptyContent filters out posts with empty content
func FilterEmptyContent(posts []models.WordPressPost) []models.WordPressPost {
	filtered := make([]models.WordPressPost, 0, len(posts))
	for _, post := range posts {
		if !isContentEmpty(post.Content.Rendered) {
			filtered = append(filtered, post)
		}
	}
	return filtered
}
