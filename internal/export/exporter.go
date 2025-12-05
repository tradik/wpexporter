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

// Exporter handles data export functionality
type Exporter struct {
	config     *config.Config
	downloader *media.Downloader
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

	// Update media paths in content
	e.updateMediaPaths(data)

	// Export based on format
	switch e.config.Format {
	case "json":
		return e.exportJSON(data)
	case "markdown":
		return e.exportMarkdown(data)
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
		return "uncategorized"
	}

	// Use the first category for the primary path
	primaryCategoryID := post.Categories[0]

	if path, exists := hierarchy[primaryCategoryID]; exists && len(path) > 0 {
		categoryPath := filepath.Join(path...)

		// Skip generic "posts" category and use uncategorized instead
		if categoryPath == "posts" {
			return "uncategorized"
		}

		return categoryPath
	}

	// Fallback to category slug if hierarchy lookup fails
	if cat, exists := categoryMap[primaryCategoryID]; exists {
		slug := e.sanitizeDirectoryName(cat.Slug)

		// Skip generic "posts" category
		if slug == "posts" {
			return "uncategorized"
		}

		return slug
	}

	return "uncategorized"
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
		builder.WriteString(fmt.Sprintf("author: %d\n", post.Author))
	}

	if post.FeaturedMedia > 0 {
		builder.WriteString(fmt.Sprintf("featured_media: %d\n", post.FeaturedMedia))
	}

	if len(post.Categories) > 0 {
		builder.WriteString("categories:\n")
		for _, cat := range post.Categories {
			builder.WriteString(fmt.Sprintf("  - %d\n", cat))
		}
	}

	if len(post.Tags) > 0 {
		builder.WriteString("tags:\n")
		for _, tag := range post.Tags {
			builder.WriteString(fmt.Sprintf("  - %d\n", tag))
		}
	}

	builder.WriteString("---\n\n")

	// Title
	builder.WriteString(fmt.Sprintf("# %s\n\n", post.Title.Rendered))

	// Excerpt if available
	if post.Excerpt.Rendered != "" {
		builder.WriteString("## Excerpt\n\n")
		builder.WriteString(e.convertHTMLToMarkdown(post.Excerpt.Rendered))
		builder.WriteString("\n\n")
	}

	// Content
	builder.WriteString("## Content\n\n")
	builder.WriteString(e.convertHTMLToMarkdown(post.Content.Rendered))

	return builder.String()
}

// exportMetadata exports categories, tags, users, and media as JSON
func (e *Exporter) exportMetadata(data *models.ExportData) error {
	metadata := map[string]interface{}{
		"categories":  data.Categories,
		"tags":        data.Tags,
		"users":       data.Users,
		"media":       data.Media,
		"stats":       data.Stats,
		"exported_at": time.Now(),
	}

	jsonData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	filePath := filepath.Join(e.config.Output, "metadata.json")
	return os.WriteFile(filePath, jsonData, 0600)
}

// updateMediaPaths updates media URLs in all content
func (e *Exporter) updateMediaPaths(data *models.ExportData) {
	if !e.config.DownloadMedia {
		return
	}

	// Update posts
	for i := range data.Posts {
		data.Posts[i].Content.Rendered = e.downloader.UpdateMediaPaths(
			data.Posts[i].Content.Rendered, data.Media)
		data.Posts[i].Excerpt.Rendered = e.downloader.UpdateMediaPaths(
			data.Posts[i].Excerpt.Rendered, data.Media)
	}

	// Update pages
	for i := range data.Pages {
		data.Pages[i].Content.Rendered = e.downloader.UpdateMediaPaths(
			data.Pages[i].Content.Rendered, data.Media)
		data.Pages[i].Excerpt.Rendered = e.downloader.UpdateMediaPaths(
			data.Pages[i].Excerpt.Rendered, data.Media)
	}
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

	return md
}
