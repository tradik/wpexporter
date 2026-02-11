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

// createProgressBar creates a progress bar that respects quiet mode
func (c *Crawler) createProgressBar(total int, description string) *progressbar.ProgressBar {
	opts := []progressbar.Option{
		progressbar.OptionSetDescription(description),
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
	}

	// Disable output in quiet mode
	if c.config.Quiet {
		opts = append(opts, progressbar.OptionSetVisibility(false))
	}

	return progressbar.NewOptions(total, opts...)
}

// logf prints a formatted message unless quiet mode is enabled
func (c *Crawler) logf(format string, args ...interface{}) {
	if !c.config.Quiet {
		fmt.Printf(format, args...)
	}
}

// EnrichPostsWithSEO crawls post URLs and extracts SEO data
func (c *Crawler) EnrichPostsWithSEO(posts []models.WordPressPost) []models.WordPressPost {
	if len(posts) == 0 {
		return posts
	}

	// Create progress bar
	progress := c.createProgressBar(len(posts), "Crawling SEO data")

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

	// Extract hreflang alternate links
	seo.Hreflangs = c.extractHreflangs(html)

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

// extractHreflangs extracts all hreflang alternate links from the HTML
func (c *Crawler) extractHreflangs(html string) []models.HreflangLink {
	var hreflangs []models.HreflangLink

	// Find all <link> tags that contain both rel="alternate" and hreflang
	linkPattern := regexp.MustCompile(`(?i)<link\s+[^>]*rel\s*=\s*["']alternate["'][^>]*>`)
	linkMatches := linkPattern.FindAllString(html, -1)

	// Also try pattern where rel comes later in the tag
	linkPattern2 := regexp.MustCompile(`(?i)<link\s+[^>]*hreflang\s*=\s*["'][^"']+["'][^>]*>`)
	linkMatches2 := linkPattern2.FindAllString(html, -1)

	// Combine and deduplicate link tags
	allLinks := append(linkMatches, linkMatches2...)
	seenLinks := make(map[string]bool)
	var uniqueLinks []string
	for _, link := range allLinks {
		if !seenLinks[link] {
			seenLinks[link] = true
			uniqueLinks = append(uniqueLinks, link)
		}
	}

	// Extract hreflang and href from each link tag
	hreflangPattern := regexp.MustCompile(`(?i)hreflang\s*=\s*["']([^"']+)["']`)
	hrefPattern := regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)
	relAlternatePattern := regexp.MustCompile(`(?i)rel\s*=\s*["']alternate["']`)

	for _, link := range uniqueLinks {
		// Must have rel="alternate"
		if !relAlternatePattern.MatchString(link) {
			continue
		}

		hreflangMatch := hreflangPattern.FindStringSubmatch(link)
		hrefMatch := hrefPattern.FindStringSubmatch(link)

		if len(hreflangMatch) > 1 && len(hrefMatch) > 1 {
			hreflangs = append(hreflangs, models.HreflangLink{
				Lang: strings.TrimSpace(hreflangMatch[1]),
				Href: strings.TrimSpace(hrefMatch[1]),
			})
		}
	}

	// Deduplicate results
	seen := make(map[string]bool)
	var unique []models.HreflangLink
	for _, h := range hreflangs {
		key := h.Lang + "|" + h.Href
		if !seen[key] {
			seen[key] = true
			unique = append(unique, h)
		}
	}

	return unique
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

// CrawlResult contains both SEO data and content extracted from a single page fetch
type CrawlResult struct {
	SEO     models.SEOData
	Content string
}

// EnrichPostsWithSEOAndContent crawls URLs once and extracts both SEO metadata and content
// This is more efficient than calling EnrichPostsWithSEO and EnrichPostsWithContent separately
func (c *Crawler) EnrichPostsWithSEOAndContent(posts []models.WordPressPost) []models.WordPressPost {
	if len(posts) == 0 {
		return posts
	}

	// Create progress bar
	progress := c.createProgressBar(len(posts), "Crawling SEO & content")

	// Create work queue
	type job struct {
		index        int
		url          string
		needsContent bool // true if content is empty and needs crawling
	}

	jobs := make(chan job, len(posts))
	results := make(chan struct {
		index    int
		result   CrawlResult
		hadEmpty bool
	}, len(posts))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < c.config.Concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				crawlResult := c.extractSEOAndContent(j.url, j.needsContent)
				results <- struct {
					index    int
					result   CrawlResult
					hadEmpty bool
				}{j.index, crawlResult, j.needsContent}
			}
		}()
	}

	// Send jobs - check which posts have empty content
	for i, post := range posts {
		needsContent := isContentEmpty(post.Content.Rendered)
		jobs <- job{index: i, url: post.Link, needsContent: needsContent}
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	enrichedContent := 0
	for result := range results {
		// Always set SEO data
		posts[result.index].SEO = result.result.SEO

		// Only set content if it was empty and we got content
		if result.hadEmpty && result.result.Content != "" {
			posts[result.index].Content.Rendered = result.result.Content
			enrichedContent++
		}
		_ = progress.Add(1)
	}

	_ = progress.Finish()
	c.logf("Enriched %d posts/pages with crawled content\n", enrichedContent)

	return posts
}

// extractSEOAndContent fetches a URL once and extracts both SEO metadata and content
func (c *Crawler) extractSEOAndContent(pageURL string, extractContent bool) CrawlResult {
	result := CrawlResult{}

	if pageURL == "" {
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.config.Timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", pageURL, nil)
	if err != nil {
		return result
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
		return result
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return result
	}

	// Read response body (limit to 5MB for full page content)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return result
	}

	html := string(body)

	// Extract SEO data
	result.SEO.Title = c.extractTitle(html)
	result.SEO.MetaDescription = c.extractMetaContent(html, "description")
	result.SEO.MetaKeywords = c.extractMetaContent(html, "keywords")
	result.SEO.OGTitle = c.extractOGContent(html, "og:title")
	result.SEO.OGDescription = c.extractOGContent(html, "og:description")
	result.SEO.OGImage = c.extractOGContent(html, "og:image")
	result.SEO.CanonicalURL = c.extractCanonical(html)

	// Extract content only if needed
	if extractContent {
		result.Content = c.extractMainContent(html)
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

	c.logf("Found %d posts/pages with empty content, crawling...\n", len(emptyPosts))

	// Create progress bar
	progress := c.createProgressBar(len(emptyPosts), "Crawling content")

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
	c.logf("Enriched %d posts/pages with crawled content\n", enriched)

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
	// First, remove header, footer, nav, aside elements to isolate main content
	cleanedHTML := c.removeNonContentElements(html)

	// Try to find main content area using balanced tag extraction
	contentSelectors := []struct {
		tag   string
		attrs string
	}{
		{"main", `id\s*=\s*["']?content["']?`},
		{"main", ""},
		{"article", `class\s*=\s*["'][^"']*(?:post|page|entry)[^"']*["']`},
		{"article", ""},
		{"div", `id\s*=\s*["']?brx-content["']?`},
		{"section", `class\s*=\s*["'][^"']*brxe-[^"']*["']`},
		{"div", `class\s*=\s*["'][^"']*elementor-widget-container[^"']*["']`},
		{"div", `id\s*=\s*["']?content["']?`},
		{"div", `class\s*=\s*["'][^"']*(?:content|entry-content|post-content|page-content)[^"']*["']`},
	}

	for _, sel := range contentSelectors {
		content := c.extractBalancedTag(cleanedHTML, sel.tag, sel.attrs)
		if content != "" {
			cleaned := c.cleanHTMLContent(content)
			if len(cleaned) > 50 {
				return cleaned
			}
		}
	}

	// Fallback: try to extract all Bricks Builder elements
	bricksContent := c.extractBricksContent(cleanedHTML)
	if bricksContent != "" {
		return bricksContent
	}

	return ""
}

// removeNonContentElements removes header, footer, nav, aside, and other non-content elements
func (c *Crawler) removeNonContentElements(html string) string {
	// Tags to remove completely
	tagsToRemove := []string{"header", "footer", "nav", "aside", "script", "style", "noscript"}

	result := html
	for _, tag := range tagsToRemove {
		result = c.removeBalancedTag(result, tag)
	}

	// Remove HTML comments
	commentPattern := regexp.MustCompile(`(?is)<!--.*?-->`)
	result = commentPattern.ReplaceAllString(result, "")

	return result
}

// removeBalancedTag removes all occurrences of a tag with its content, handling nesting
func (c *Crawler) removeBalancedTag(html string, tag string) string {
	result := html
	tagLower := strings.ToLower(tag)

	for {
		// Find opening tag
		openPattern := regexp.MustCompile(`(?i)<` + tag + `[^>]*>`)
		openLoc := openPattern.FindStringIndex(result)
		if openLoc == nil {
			break
		}

		// Find the matching closing tag by counting nested tags
		depth := 1
		pos := openLoc[1]
		closeTagPattern := regexp.MustCompile(`(?i)<(/?)` + tag + `[^>]*>`)

		for depth > 0 && pos < len(result) {
			remaining := result[pos:]
			match := closeTagPattern.FindStringSubmatchIndex(remaining)
			if match == nil {
				break
			}

			isClose := remaining[match[2]:match[3]] == "/"
			if isClose {
				depth--
			} else {
				depth++
			}

			if depth == 0 {
				// Found matching close tag
				closeEnd := pos + match[1]
				result = result[:openLoc[0]] + result[closeEnd:]
				break
			}
			pos += match[1]
		}

		// Safety check to avoid infinite loop
		if depth > 0 {
			// Couldn't find matching close tag, remove just the open tag
			result = result[:openLoc[0]] + result[openLoc[1]:]
		}
	}

	_ = tagLower // avoid unused variable warning
	return result
}

// extractBalancedTag extracts content from a tag, handling nested tags properly
func (c *Crawler) extractBalancedTag(html string, tag string, attrPattern string) string {
	// Build pattern for opening tag
	var openPatternStr string
	if attrPattern != "" {
		openPatternStr = `(?i)<` + tag + `[^>]*` + attrPattern + `[^>]*>`
	} else {
		openPatternStr = `(?i)<` + tag + `[^>]*>`
	}

	openPattern := regexp.MustCompile(openPatternStr)
	openLoc := openPattern.FindStringIndex(html)
	if openLoc == nil {
		return ""
	}

	// Find the matching closing tag by counting nested tags
	depth := 1
	pos := openLoc[1]
	closeTagPattern := regexp.MustCompile(`(?i)<(/?)` + tag + `[^>]*>`)

	for depth > 0 && pos < len(html) {
		remaining := html[pos:]
		match := closeTagPattern.FindStringSubmatchIndex(remaining)
		if match == nil {
			break
		}

		isClose := remaining[match[2]:match[3]] == "/"
		if isClose {
			depth--
		} else {
			depth++
		}

		if depth == 0 {
			// Found matching close tag - extract content between tags
			contentStart := openLoc[1]
			contentEnd := pos + match[0]
			return html[contentStart:contentEnd]
		}
		pos += match[1]
	}

	return ""
}

// extractBricksContent extracts content from Bricks Builder elements
func (c *Crawler) extractBricksContent(html string) string {
	var parts []string

	// Extract headings with their level
	headingPattern := regexp.MustCompile(`(?is)<[^>]*class\s*=\s*["'][^"']*brxe-heading[^"']*["'][^>]*>(.+?)</[^>]+>`)
	headingMatches := headingPattern.FindAllStringSubmatch(html, -1)
	for _, match := range headingMatches {
		if len(match) > 1 {
			text := strings.TrimSpace(c.decodeHTMLEntities(match[1]))
			// Strip any inner tags but keep the text
			text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, "")
			if text != "" {
				parts = append(parts, "<h2>"+text+"</h2>")
			}
		}
	}

	// Extract text elements
	textPattern := regexp.MustCompile(`(?is)<[^>]*class\s*=\s*["'][^"']*brxe-text[^"']*["'][^>]*>(.+?)</[^>]+>`)
	textMatches := textPattern.FindAllStringSubmatch(html, -1)
	for _, match := range textMatches {
		if len(match) > 1 {
			text := strings.TrimSpace(match[1])
			if text != "" {
				parts = append(parts, text)
			}
		}
	}

	// Extract rich text elements (may contain HTML)
	richTextPattern := regexp.MustCompile(`(?is)<[^>]*class\s*=\s*["'][^"']*brxe-rich-text[^"']*["'][^>]*>(.+?)</[^>]+>`)
	richTextMatches := richTextPattern.FindAllStringSubmatch(html, -1)
	for _, match := range richTextMatches {
		if len(match) > 1 {
			text := strings.TrimSpace(match[1])
			if text != "" {
				parts = append(parts, text)
			}
		}
	}

	if len(parts) > 0 {
		return strings.Join(parts, "\n")
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
