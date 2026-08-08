package export

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/media"
	"github.com/tradik/wpexporter/pkg/models"
)

// uncategorizedDir is where a post with no resolvable category lands. It is
// never empty, which is what keeps every post at least one directory below
// posts/ — a requirement of the ssg format.
const uncategorizedDir = "uncategorized"

// Exporter handles data export functionality
type Exporter struct {
	config      *config.Config
	downloader  *media.Downloader
	categoryMap map[int]string    // ID -> Name lookup
	tagMap      map[int]string    // ID -> Name lookup
	userMap     map[int]string    // ID -> Name lookup
	mediaMap    map[int]string    // ID -> media URL lookup (localized when rewriting is active)
	altMap      map[string]string // media URL -> alt text, for filling in missing alt attributes
	// rewriter localizes attachment URLs; nil when the export keeps original URLs.
	rewriter *media.URLRewriter
}

// NewExporter creates a new exporter instance
func NewExporter(cfg *config.Config) *Exporter {
	return &Exporter{
		config:     cfg,
		downloader: media.NewDownloader(cfg),
	}
}

// Export exports the data in the specified format
func (e *Exporter) Export(data *models.ExportData) error {
	// Ensure output directory exists
	if err := e.config.EnsureOutputDir(); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Download media files if enabled
	if e.config.DownloadMedia {
		downloaded, err := e.downloader.DownloadMedia(data.Media)
		if err != nil {
			return fmt.Errorf("failed to download media: %w", err)
		}
		data.Stats.MediaDownloaded = downloaded
	}

	// Update media paths in content only for local formats (json, markdown)
	// unless --keep-original-urls is set (for importing markdown to Shopify etc.)
	// Other formats (shopify, magento, etc.) need original URLs
	if e.config.LocalizesURLs() {
		e.updateMediaPaths(data)
		e.updateLinkPaths(data)
	}

	if err := e.reportAccessibility(data); err != nil {
		return fmt.Errorf("failed to write accessibility report: %w", err)
	}

	// Export based on format
	switch e.config.Format {
	case "json":
		return e.exportJSON(data)
	case "markdown":
		return e.exportMarkdown(data)
	case "shopify":
		return e.exportShopify(data)
	case "magento":
		return e.exportMagento(data)
	case "wordpress":
		return e.exportWordPress(data)
	case "drupal":
		return e.exportDrupal(data)
	case "wix":
		return e.exportWix(data)
	case "squarespace":
		return e.exportSquarespace(data)
	case "webflow":
		return e.exportWebflow(data)
	case "weebly":
		return e.exportWeebly(data)
	case "prestashop":
		return e.exportPrestaShop(data)
	case "ghost":
		return e.exportGhost(data)
	case "strapi":
		return e.exportStrapi(data)
	case "contentful":
		return e.exportContentful(data)
	case "ssg":
		return e.exportSSG(data)
	default:
		return fmt.Errorf("unsupported export format: %s", e.config.Format)
	}
}

// exportJSON exports data as JSON
func (e *Exporter) exportJSON(data *models.ExportData) error {
	// Set export timestamp
	data.ExportedAt = time.Now()

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Determine output file path
	var outputPath string
	if filepath.Ext(e.config.Output) == ".json" {
		outputPath = e.config.Output
	} else {
		outputPath = filepath.Join(e.config.Output, "export.json")
	}

	// Write JSON file
	if err := os.WriteFile(outputPath, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	fmt.Printf("Export completed: %s\n", outputPath)
	return nil
}

// exportMarkdown exports data as Markdown files
func (e *Exporter) exportMarkdown(data *models.ExportData) error {
	e.buildLookupMaps(data)

	// Create base directory structure
	pagesDir := filepath.Join(e.config.Output, "pages")

	if err := os.MkdirAll(pagesDir, 0750); err != nil {
		return fmt.Errorf("failed to create pages directory: %w", err)
	}

	// Export site info
	if err := e.exportSiteInfo(data.Site); err != nil {
		return fmt.Errorf("failed to export site info: %w", err)
	}

	// Export posts with category-based folder structure
	if err := e.exportPostsWithCategories(data.Posts, data.Categories, "post"); err != nil {
		return fmt.Errorf("failed to export posts: %w", err)
	}

	// Export pages
	if err := e.exportPostsMarkdown(data.Pages, pagesDir, "page"); err != nil {
		return fmt.Errorf("failed to export pages: %w", err)
	}

	// Export metadata
	if err := e.exportMetadata(data); err != nil {
		return fmt.Errorf("failed to export metadata: %w", err)
	}

	fmt.Printf("Export completed: %s\n", e.config.Output)
	return nil
}

// exportSiteInfo exports site information as markdown
func (e *Exporter) exportSiteInfo(site models.SiteInfo) error {
	content := fmt.Sprintf(`# %s

**Description:** %s  
**URL:** %s  
**Home URL:** %s  
**Admin Email:** %s  
**Timezone:** %s  
**Language:** %s  

---

*Exported on %s*
`,
		site.Name,
		site.Description,
		site.URL,
		site.HomeURL,
		site.AdminEmail,
		site.Timezone,
		site.Language,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	filePath := filepath.Join(e.config.Output, "README.md")
	return os.WriteFile(filePath, []byte(content), 0600)
}

// exportPostsWithCategories exports posts organized by category folders
func (e *Exporter) exportPostsWithCategories(posts []models.WordPressPost, categories []models.WordPressCategory, contentType string) error {
	// Create category map for quick lookup
	categoryMap := make(map[int]models.WordPressCategory)
	for _, cat := range categories {
		categoryMap[cat.ID] = cat
	}

	// Create category hierarchy map
	categoryHierarchy := e.buildCategoryHierarchy(categories)

	for _, post := range posts {
		// Determine the category path for this post
		categoryPath := e.getCategoryPath(post, categoryMap, categoryHierarchy)

		// Create the full directory path
		postDir := filepath.Join(e.config.Output, "posts", categoryPath)
		if err := os.MkdirAll(postDir, 0750); err != nil {
			return fmt.Errorf("failed to create category directory %s: %w", postDir, err)
		}

		// Generate filename and content
		filename := e.generateMarkdownFilename(post)
		filePath := filepath.Join(postDir, filename)
		content := e.generateMarkdownContent(post, contentType)

		if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write %s file %s: %w", contentType, filename, err)
		}
	}

	return nil
}

// exportPostsMarkdown exports posts/pages as markdown files
func (e *Exporter) exportPostsMarkdown(posts []models.WordPressPost, dir, contentType string) error {
	for _, post := range posts {
		filename := e.generateMarkdownFilename(post)
		filePath := filepath.Join(dir, filename)

		content := e.generateMarkdownContent(post, contentType)

		if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write %s file %s: %w", contentType, filename, err)
		}
	}

	return nil
}

// generateMarkdownFilename generates a filename for a markdown file
func (e *Exporter) generateMarkdownFilename(post models.WordPressPost) string {
	// Use only slug for filename (no date)
	slug := post.Slug

	if slug == "" {
		slug = fmt.Sprintf("post-%d", post.ID)
	}

	// Sanitize slug
	slug = strings.ReplaceAll(slug, "/", "-")
	slug = strings.ReplaceAll(slug, "\\", "-")
	slug = strings.ReplaceAll(slug, ":", "-")

	return fmt.Sprintf("%s.md", slug)
}

// buildCategoryHierarchy creates a map of category ID to its parent path
func (e *Exporter) buildCategoryHierarchy(categories []models.WordPressCategory) map[int][]string {
	hierarchy := make(map[int][]string)
	categoryMap := make(map[int]models.WordPressCategory)

	// Create category lookup map
	for _, cat := range categories {
		categoryMap[cat.ID] = cat
	}

	// Build hierarchy paths
	var buildPath func(int) []string
	buildPath = func(catID int) []string {
		if path, exists := hierarchy[catID]; exists {
			return path
		}

		cat, exists := categoryMap[catID]
		if !exists {
			return []string{}
		}

		var path []string
		if cat.Parent > 0 {
			parentPath := buildPath(cat.Parent)
			path = append(parentPath, e.sanitizeDirectoryName(cat.Slug))
		} else {
			path = []string{e.sanitizeDirectoryName(cat.Slug)}
		}

		hierarchy[catID] = path
		return path
	}

	// Build paths for all categories
	for _, cat := range categories {
		buildPath(cat.ID)
	}

	return hierarchy
}

// getCategoryPath determines the directory path for a post based on its categories
func (e *Exporter) getCategoryPath(post models.WordPressPost, categoryMap map[int]models.WordPressCategory, hierarchy map[int][]string) string {
	// First, try to extract categories from the post link
	if linkCategories := e.extractCategoriesFromLink(post.Link); linkCategories != "" {
		return linkCategories
	}

	if len(post.Categories) == 0 {
		return uncategorizedDir
	}

	// Use the first category for the primary path
	primaryCategoryID := post.Categories[0]

	if path, exists := hierarchy[primaryCategoryID]; exists && len(path) > 0 {
		categoryPath := filepath.Join(path...)

		// Skip generic "posts" category and use uncategorized instead
		if categoryPath == "posts" {
			return uncategorizedDir
		}

		return categoryPath
	}

	// Fallback to category slug if hierarchy lookup fails
	if cat, exists := categoryMap[primaryCategoryID]; exists {
		slug := e.sanitizeDirectoryName(cat.Slug)

		// Skip generic "posts" category
		if slug == "posts" {
			return uncategorizedDir
		}

		return slug
	}

	return uncategorizedDir
}

// sanitizeDirectoryName sanitizes a string for use as a directory name
func (e *Exporter) sanitizeDirectoryName(name string) string {
	// Replace invalid characters with hyphens
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", " "}
	sanitized := name

	for _, char := range invalid {
		sanitized = strings.ReplaceAll(sanitized, char, "-")
	}

	// Remove multiple consecutive hyphens
	for strings.Contains(sanitized, "--") {
		sanitized = strings.ReplaceAll(sanitized, "--", "-")
	}

	// Trim hyphens from start and end
	sanitized = strings.Trim(sanitized, "-")

	// Ensure it's not empty
	if sanitized == "" {
		sanitized = "category"
	}

	return sanitized
}

// extractCategoriesFromLink extracts category path from WordPress permalink structure
func (e *Exporter) extractCategoriesFromLink(link string) string {
	if link == "" {
		return ""
	}

	// Parse the URL to get the path
	parsedURL, err := url.Parse(link)
	if err != nil {
		return ""
	}

	path := strings.Trim(parsedURL.Path, "/")
	if path == "" {
		return ""
	}

	// Split the path into segments
	segments := strings.Split(path, "/")

	// Common WordPress permalink structures:
	// 1. /%category%/%postname%/
	// 2. /%category%/%subcategory%/%postname%/
	// 3. /%year%/%monthnum%/%day%/%postname%/
	// 4. /%postname%/ (no categories)

	// If there's only one segment, it's likely just the post slug
	if len(segments) <= 1 {
		return ""
	}

	// Check if the last segment looks like a post slug (no file extension, reasonable length)
	_ = segments[len(segments)-1] // Last segment is the post slug

	// Skip if it looks like a date-based permalink (YYYY/MM/DD structure)
	if len(segments) >= 3 {
		// Check if first three segments are numeric (year/month/day)
		if e.isNumeric(segments[0]) && e.isNumeric(segments[1]) && e.isNumeric(segments[2]) {
			return ""
		}
	}

	// Extract category segments (all but the last one, which should be the post slug)
	categorySegments := segments[:len(segments)-1]

	// Filter out common non-category segments
	var validCategories []string
	for _, segment := range categorySegments {
		// Skip numeric segments (likely dates)
		if e.isNumeric(segment) {
			continue
		}

		// Skip common WordPress segments that aren't categories (but keep 'news' as it's often a valid category)
		if segment == "blog" || segment == "posts" || segment == "archives" {
			continue
		}

		// Sanitize and add valid category segments
		sanitized := e.sanitizeDirectoryName(segment)
		if sanitized != "" && sanitized != "category" {
			validCategories = append(validCategories, sanitized)
		}
	}

	// Return the category path
	if len(validCategories) > 0 {
		return filepath.Join(validCategories...)
	}

	return ""
}

// isNumeric checks if a string contains only digits
func (e *Exporter) isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, char := range s {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// generateMarkdownContent generates markdown content for a post
func (e *Exporter) generateMarkdownContent(post models.WordPressPost, contentType string) string {
	var builder strings.Builder

	// Front matter
	builder.WriteString("---\n")
	builder.WriteString(fmt.Sprintf("id: %d\n", post.ID))
	builder.WriteString(fmt.Sprintf("title: \"%s\"\n", e.escapeYAML(post.Title.Rendered)))
	builder.WriteString(fmt.Sprintf("slug: \"%s\"\n", post.Slug))
	builder.WriteString(fmt.Sprintf("date: %s\n", post.Date.Format("2006-01-02T15:04:05Z07:00")))
	builder.WriteString(fmt.Sprintf("modified: %s\n", post.Modified.Format("2006-01-02T15:04:05Z07:00")))
	builder.WriteString(fmt.Sprintf("status: \"%s\"\n", post.Status))
	builder.WriteString(fmt.Sprintf("type: \"%s\"\n", contentType))
	builder.WriteString(fmt.Sprintf("link: \"%s\"\n", post.Link))

	if post.Author > 0 {
		if name, ok := e.userMap[post.Author]; ok && name != "" {
			builder.WriteString(fmt.Sprintf("author: \"%s\"\n", e.escapeYAML(name)))
		}
		if !e.config.NoIDs {
			builder.WriteString(fmt.Sprintf("author_id: %d\n", post.Author))
		}
	}

	if post.FeaturedMedia > 0 {
		if url, ok := e.mediaMap[post.FeaturedMedia]; ok && url != "" {
			builder.WriteString(fmt.Sprintf("featured_image: \"%s\"\n", e.escapeYAML(url)))
		}
		if !e.config.NoIDs {
			builder.WriteString(fmt.Sprintf("featured_image_id: %d\n", post.FeaturedMedia))
		}
	}

	if len(post.Categories) > 0 {
		// Collect category names
		var categoryNames []string
		for _, catID := range post.Categories {
			if name, ok := e.categoryMap[catID]; ok && name != "" {
				categoryNames = append(categoryNames, name)
			}
		}
		// Output names only if we have any
		if len(categoryNames) > 0 {
			builder.WriteString("categories:\n")
			for _, name := range categoryNames {
				builder.WriteString(fmt.Sprintf("  - \"%s\"\n", e.escapeYAML(name)))
			}
		}
		// Output IDs unless --no-ids
		if !e.config.NoIDs {
			builder.WriteString("category_ids:\n")
			for _, catID := range post.Categories {
				builder.WriteString(fmt.Sprintf("  - %d\n", catID))
			}
		}
	}

	if len(post.Tags) > 0 {
		// Collect tag names
		var tagNames []string
		for _, tagID := range post.Tags {
			if name, ok := e.tagMap[tagID]; ok && name != "" {
				tagNames = append(tagNames, name)
			}
		}
		// Output names only if we have any
		if len(tagNames) > 0 {
			builder.WriteString("tags:\n")
			for _, name := range tagNames {
				builder.WriteString(fmt.Sprintf("  - \"%s\"\n", e.escapeYAML(name)))
			}
		}
		// Output IDs unless --no-ids
		if !e.config.NoIDs {
			builder.WriteString("tag_ids:\n")
			for _, tagID := range post.Tags {
				builder.WriteString(fmt.Sprintf("  - %d\n", tagID))
			}
		}
	}

	// SEO fields (if crawled via --assisted-crawl)
	if post.SEO.Title != "" {
		builder.WriteString(fmt.Sprintf("seo_title: \"%s\"\n", e.escapeYAML(post.SEO.Title)))
	}
	if post.SEO.MetaDescription != "" {
		builder.WriteString(fmt.Sprintf("meta_description: \"%s\"\n", e.escapeYAML(post.SEO.MetaDescription)))
	}
	if post.SEO.MetaKeywords != "" {
		builder.WriteString(fmt.Sprintf("meta_keywords: \"%s\"\n", e.escapeYAML(post.SEO.MetaKeywords)))
	}
	if post.SEO.OGTitle != "" {
		builder.WriteString(fmt.Sprintf("og_title: \"%s\"\n", e.escapeYAML(post.SEO.OGTitle)))
	}
	if post.SEO.OGDescription != "" {
		builder.WriteString(fmt.Sprintf("og_description: \"%s\"\n", e.escapeYAML(post.SEO.OGDescription)))
	}
	if post.SEO.OGImage != "" {
		builder.WriteString(fmt.Sprintf("og_image: \"%s\"\n", e.escapeYAML(post.SEO.OGImage)))
	}
	if post.SEO.CanonicalURL != "" {
		builder.WriteString(fmt.Sprintf("canonical_url: \"%s\"\n", e.escapeYAML(post.SEO.CanonicalURL)))
	}
	if post.SEO.Lang != "" {
		builder.WriteString(fmt.Sprintf("lang: \"%s\"\n", post.SEO.Lang))
	}
	if len(post.SEO.Hreflangs) > 0 {
		builder.WriteString("hreflangs:\n")
		for _, h := range post.SEO.Hreflangs {
			builder.WriteString(fmt.Sprintf("  - lang: \"%s\"\n", h.Lang))
			builder.WriteString(fmt.Sprintf("    href: \"%s\"\n", e.escapeYAML(h.Href)))
		}
	}

	// Excerpt in frontmatter, reduced to plain text: the theme's "Continue
	// reading" anchor is navigation rather than content and otherwise ends up in
	// <meta name="description">.
	if excerptText := plainTextExcerpt(post.Excerpt.Rendered); excerptText != "" {
		builder.WriteString(fmt.Sprintf("excerpt: \"%s\"\n", e.escapeYAML(excerptText)))
	}

	builder.WriteString("---\n\n")

	// Title
	builder.WriteString(fmt.Sprintf("# %s\n\n", post.Title.Rendered))

	// Content
	builder.WriteString(e.convertHTMLToMarkdown(post.Content.Rendered))

	return builder.String()
}

// exportMetadata exports categories, tags, users, and media as JSON
func (e *Exporter) exportMetadata(data *models.ExportData) error {
	jsonData, err := exportMetadataJSON(data)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	filePath := filepath.Join(e.config.Output, "metadata.json")
	return os.WriteFile(filePath, jsonData, 0600)
}

// updateMediaPaths localizes attachment URLs across all exported content.
//
// It runs before the format switch, so the rewriter it builds is also what
// exportMarkdown later uses for featured_image.
func (e *Exporter) updateMediaPaths(data *models.ExportData) {
	if !e.config.DownloadMedia {
		return
	}

	// Built once and reused for every field: indexing the attachment list is
	// O(media) and would otherwise be repeated for each post.
	e.rewriter = e.downloader.NewURLRewriter(data.Media)

	for i := range data.Posts {
		e.localizePostMedia(&data.Posts[i])
	}

	for i := range data.Pages {
		e.localizePostMedia(&data.Pages[i])
	}
}

// localizePostMedia localizes every attachment reference a post carries: body
// content, excerpt, and the og:image scraped from the live page.
//
// canonical_url, link and hreflangs are deliberately left absolute — they are
// addresses of the source site rather than assets, and a consumer needs them to
// derive the target URL. og:image is scraped rather than read from the media
// library, so one pointing at a CDN or a third-party host resolves to nothing in
// the index and correctly stays absolute.
func (e *Exporter) localizePostMedia(post *models.WordPressPost) {
	post.Content.Rendered = e.rewriter.Rewrite(post.Content.Rendered)
	post.Excerpt.Rendered = e.rewriter.Rewrite(post.Excerpt.Rendered)
	post.SEO.OGImage = e.rewriter.Rewrite(post.SEO.OGImage)
}

// localizeMediaURL localizes a single attachment URL, leaving it untouched when
// the export is configured to keep original URLs.
func (e *Exporter) localizeMediaURL(rawURL string) string {
	if e.rewriter == nil {
		return rawURL
	}

	return e.rewriter.Rewrite(rawURL)
}

// updateLinkPaths converts same-host address fields to root-relative paths when
// --link-style root is set.
//
// These are addresses of the source site rather than assets, so the default
// keeps them absolute: a consumer needs the original URL to derive the target
// one. A site rebuilt at the same paths wants the root-relative form instead,
// because that preserves each URL (and its search ranking) on the new host
// without pinning the content to the old one.
func (e *Exporter) updateLinkPaths(data *models.ExportData) {
	if e.config.EffectiveLinkStyle() != "root" {
		return
	}

	for i := range data.Posts {
		e.rootRelativizeAddresses(&data.Posts[i])
	}

	for i := range data.Pages {
		e.rootRelativizeAddresses(&data.Pages[i])
	}
}

// rootRelativizeAddresses converts every address field a post carries.
func (e *Exporter) rootRelativizeAddresses(post *models.WordPressPost) {
	post.Link = e.rootRelativeURL(post.Link)
	post.SEO.CanonicalURL = e.rootRelativeURL(post.SEO.CanonicalURL)

	for i := range post.SEO.Hreflangs {
		post.SEO.Hreflangs[i].Href = e.rootRelativeURL(post.SEO.Hreflangs[i].Href)
	}
}

// rootRelativeURL strips scheme and host from a same-host address, preserving
// query and fragment. A URL on a foreign host is returned unchanged — an
// external canonical or hreflang alternate must keep pointing where it points.
func (e *Exporter) rootRelativeURL(rawURL string) string {
	if rawURL == "" || !e.config.IsSameHost(rawURL) {
		return rawURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Path == "" {
		return rawURL
	}

	relative := parsed.Path
	if parsed.RawQuery != "" {
		relative += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		relative += "#" + parsed.Fragment
	}

	return relative
}

// escapeYAML escapes special characters for YAML
func (e *Exporter) escapeYAML(s string) string {
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

// convertHTMLToMarkdown performs basic HTML to Markdown conversion
func (e *Exporter) convertHTMLToMarkdown(html string) string {
	// Basic HTML to Markdown conversion
	// This is a simplified version - for production use, consider using a proper HTML to Markdown library

	md := html

	// Headers
	md = strings.ReplaceAll(md, "<h1>", "# ")
	md = strings.ReplaceAll(md, "</h1>", "\n\n")
	md = strings.ReplaceAll(md, "<h2>", "## ")
	md = strings.ReplaceAll(md, "</h2>", "\n\n")
	md = strings.ReplaceAll(md, "<h3>", "### ")
	md = strings.ReplaceAll(md, "</h3>", "\n\n")
	md = strings.ReplaceAll(md, "<h4>", "#### ")
	md = strings.ReplaceAll(md, "</h4>", "\n\n")
	md = strings.ReplaceAll(md, "<h5>", "##### ")
	md = strings.ReplaceAll(md, "</h5>", "\n\n")
	md = strings.ReplaceAll(md, "<h6>", "###### ")
	md = strings.ReplaceAll(md, "</h6>", "\n\n")

	// Bold and italic
	md = strings.ReplaceAll(md, "<strong>", "**")
	md = strings.ReplaceAll(md, "</strong>", "**")
	md = strings.ReplaceAll(md, "<b>", "**")
	md = strings.ReplaceAll(md, "</b>", "**")
	md = strings.ReplaceAll(md, "<em>", "*")
	md = strings.ReplaceAll(md, "</em>", "*")
	md = strings.ReplaceAll(md, "<i>", "*")
	md = strings.ReplaceAll(md, "</i>", "*")

	// Paragraphs
	md = strings.ReplaceAll(md, "<p>", "")
	md = strings.ReplaceAll(md, "</p>", "\n\n")

	// Line breaks
	md = strings.ReplaceAll(md, "<br>", "\n")
	md = strings.ReplaceAll(md, "<br/>", "\n")
	md = strings.ReplaceAll(md, "<br />", "\n")

	// Lists
	md = strings.ReplaceAll(md, "<ul>", "")
	md = strings.ReplaceAll(md, "</ul>", "\n")
	md = strings.ReplaceAll(md, "<ol>", "")
	md = strings.ReplaceAll(md, "</ol>", "\n")
	md = strings.ReplaceAll(md, "<li>", "- ")
	md = strings.ReplaceAll(md, "</li>", "\n")

	// Code
	md = strings.ReplaceAll(md, "<code>", "`")
	md = strings.ReplaceAll(md, "</code>", "`")
	md = strings.ReplaceAll(md, "<pre>", "```\n")
	md = strings.ReplaceAll(md, "</pre>", "\n```")

	// Clean up extra whitespace
	md = strings.ReplaceAll(md, "\n\n\n", "\n\n")
	md = strings.TrimSpace(md)

	// The exported file is UTF-8, so typographic entities are noise that would
	// otherwise survive into the rendered page. The HTML-significant ones stay
	// encoded — decoding them would turn escaped markup into live markup.
	return decodeTypographicEntities(md)
}

// exportShopify exports data as Shopify-compatible CSV
func (e *Exporter) exportShopify(data *models.ExportData) error {
	shopifyExporter := NewShopifyExporter(e.config)

	// Export products to CSV
	if err := shopifyExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Shopify products: %w", err)
	}

	// Export metadata
	if err := shopifyExporter.ExportMetadata(data); err != nil {
		return fmt.Errorf("failed to export Shopify metadata: %w", err)
	}

	return nil
}

// exportMagento exports data as Magento-compatible CSV
func (e *Exporter) exportMagento(data *models.ExportData) error {
	magentoExporter := NewMagentoExporter(e.config)

	// Export products to CSV
	if err := magentoExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Magento products: %w", err)
	}

	// Export metadata
	if err := magentoExporter.ExportMetadata(data); err != nil {
		return fmt.Errorf("failed to export Magento metadata: %w", err)
	}

	return nil
}

// exportWordPress exports data as WordPress WXR (WordPress eXtended RSS) XML
func (e *Exporter) exportWordPress(data *models.ExportData) error {
	wpExporter := NewWordPressExporter(e.config)

	if err := wpExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export WordPress WXR: %w", err)
	}

	return nil
}

// exportDrupal exports data as Drupal-compatible JSON
func (e *Exporter) exportDrupal(data *models.ExportData) error {
	drupalExporter := NewDrupalExporter(e.config)

	if err := drupalExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Drupal JSON: %w", err)
	}

	return nil
}

// exportWix exports data as Wix-compatible JSON
func (e *Exporter) exportWix(data *models.ExportData) error {
	wixExporter := NewWixExporter(e.config)

	if err := wixExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Wix JSON: %w", err)
	}

	return nil
}

// exportSquarespace exports data as Squarespace-compatible XML
func (e *Exporter) exportSquarespace(data *models.ExportData) error {
	sqExporter := NewSquarespaceExporter(e.config)

	if err := sqExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Squarespace XML: %w", err)
	}

	return nil
}

// exportWebflow exports data as Webflow-compatible CSV
func (e *Exporter) exportWebflow(data *models.ExportData) error {
	webflowExporter := NewWebflowExporter(e.config)

	if err := webflowExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Webflow CSV: %w", err)
	}

	return nil
}

// exportWeebly exports data as Weebly-compatible XML/JSON
func (e *Exporter) exportWeebly(data *models.ExportData) error {
	weeblyExporter := NewWeeblyExporter(e.config)

	if err := weeblyExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Weebly: %w", err)
	}

	return nil
}

// exportPrestaShop exports data as PrestaShop-compatible CSV
func (e *Exporter) exportPrestaShop(data *models.ExportData) error {
	psExporter := NewPrestaShopExporter(e.config)

	if err := psExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export PrestaShop CSV: %w", err)
	}

	return nil
}

// exportGhost exports data as Ghost-compatible JSON
func (e *Exporter) exportGhost(data *models.ExportData) error {
	ghostExporter := NewGhostExporter(e.config)

	if err := ghostExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Ghost JSON: %w", err)
	}

	return nil
}

// exportStrapi exports data as Strapi-compatible JSON
func (e *Exporter) exportStrapi(data *models.ExportData) error {
	strapiExporter := NewStrapiExporter(e.config)

	if err := strapiExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Strapi JSON: %w", err)
	}

	return nil
}

// exportContentful exports data as Contentful-compatible JSON
func (e *Exporter) exportContentful(data *models.ExportData) error {
	contentfulExporter := NewContentfulExporter(e.config)

	if err := contentfulExporter.Export(data); err != nil {
		return fmt.Errorf("failed to export Contentful JSON: %w", err)
	}

	return nil
}
