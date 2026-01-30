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
