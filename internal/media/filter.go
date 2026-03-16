package media

import (
	"regexp"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// Filter handles filtering of media items to only relevant ones
type Filter struct{}

// NewFilter creates a new media filter
func NewFilter() *Filter {
	return &Filter{}
}

// mediaExtensions lists file extensions that should be extracted from <a href> links
var mediaExtensions = []string{
	// Documents
	".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
	".odt", ".ods", ".odp", ".txt", ".rtf", ".epub",
	// Videos
	".mp4", ".avi", ".webm", ".mkv", ".mov", ".wmv", ".flv",
	".m4v", ".3gp", ".3g2", ".ogv", ".mpeg", ".mpg",
	// Audio
	".mp3", ".wav", ".ogg", ".flac", ".aac", ".m4a", ".wma", ".aiff",
	// Archives
	".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz",
	// Images (also extract linked images, not just <img>)
	".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico", ".avif", ".heic",
}

// FilterRelevantMedia returns only media items that are:
// 1. Featured images of posts/pages
// 2. Images embedded in post/page content (<img src>)
// 3. Documents, videos, and other media linked in content (<a href>)
func (f *Filter) FilterRelevantMedia(
	posts []models.WordPressPost,
	pages []models.WordPressPost,
	allMedia []models.WordPressMedia,
) []models.WordPressMedia {
	relevantIDs := make(map[int]bool)
	relevantURLs := make(map[string]bool)
	relevantPaths := make(map[string]bool) // Fallback matching by path suffix (e.g., "2024/01/file.pdf")

	// Regex to extract image URLs from HTML content
	imgPattern := regexp.MustCompile(`<img[^>]+src\s*=\s*["']([^"']+)["']`)
	// Regex to extract link URLs from HTML content
	linkPattern := regexp.MustCompile(`<a[^>]+href\s*=\s*["']([^"']+)["']`)

	// Collect featured media IDs and content media from posts
	for _, post := range posts {
		if post.FeaturedMedia > 0 {
			relevantIDs[post.FeaturedMedia] = true
		}
		// Extract images from <img> tags
		urls := f.extractURLs(post.Content.Rendered, imgPattern)
		for _, url := range urls {
			relevantURLs[url] = true
			relevantPaths[f.extractPathSuffix(url)] = true
		}
		// Extract media links from <a> tags
		linkURLs := f.extractMediaLinks(post.Content.Rendered, linkPattern)
		for _, url := range linkURLs {
			relevantURLs[url] = true
			relevantPaths[f.extractPathSuffix(url)] = true
		}
		// Also check excerpt
		excerptURLs := f.extractURLs(post.Excerpt.Rendered, imgPattern)
		for _, url := range excerptURLs {
			relevantURLs[url] = true
			relevantPaths[f.extractPathSuffix(url)] = true
		}
		excerptLinkURLs := f.extractMediaLinks(post.Excerpt.Rendered, linkPattern)
		for _, url := range excerptLinkURLs {
			relevantURLs[url] = true
			relevantPaths[f.extractPathSuffix(url)] = true
		}
	}

	// Collect featured media IDs and content media from pages
	for _, page := range pages {
		if page.FeaturedMedia > 0 {
			relevantIDs[page.FeaturedMedia] = true
		}
		// Extract images from <img> tags
		urls := f.extractURLs(page.Content.Rendered, imgPattern)
		for _, url := range urls {
			relevantURLs[url] = true
			relevantPaths[f.extractPathSuffix(url)] = true
		}
		// Extract media links from <a> tags
		linkURLs := f.extractMediaLinks(page.Content.Rendered, linkPattern)
		for _, url := range linkURLs {
			relevantURLs[url] = true
			relevantPaths[f.extractPathSuffix(url)] = true
		}
		// Also check excerpt
		excerptURLs := f.extractURLs(page.Excerpt.Rendered, imgPattern)
		for _, url := range excerptURLs {
			relevantURLs[url] = true
			relevantPaths[f.extractPathSuffix(url)] = true
		}
		excerptLinkURLs := f.extractMediaLinks(page.Excerpt.Rendered, linkPattern)
		for _, url := range excerptLinkURLs {
			relevantURLs[url] = true
			relevantPaths[f.extractPathSuffix(url)] = true
		}
	}

	// Filter media to only include relevant items
	var filtered []models.WordPressMedia
	for _, media := range allMedia {
		// Check if media ID is a featured image
		if relevantIDs[media.ID] {
			filtered = append(filtered, media)
			continue
		}
		// Check if media URL is referenced in content (exact match)
		if relevantURLs[media.SourceURL] {
			filtered = append(filtered, media)
			continue
		}
		// Check if media path suffix matches (handles CDN/different domains)
		mediaPath := f.extractPathSuffix(media.SourceURL)
		if mediaPath != "" && relevantPaths[mediaPath] {
			filtered = append(filtered, media)
			continue
		}
		// Also check different size URLs
		matched := false
		for _, size := range media.MediaDetails.Sizes {
			if relevantURLs[size.SourceURL] {
				filtered = append(filtered, media)
				matched = true
				break
			}
			sizePath := f.extractPathSuffix(size.SourceURL)
			if sizePath != "" && relevantPaths[sizePath] {
				filtered = append(filtered, media)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
	}

	return filtered
}

// extractPathSuffix extracts the path after "uploads/" for WordPress-style matching
// e.g., "https://cdn.example.com/wp-content/uploads/2024/01/file.pdf" -> "2024/01/file.pdf"
// This is more specific than filename-only matching to avoid false positives
func (f *Filter) extractPathSuffix(urlStr string) string {
	// Remove query string
	if idx := strings.Index(urlStr, "?"); idx > 0 {
		urlStr = urlStr[:idx]
	}
	urlStr = strings.ToLower(urlStr)

	// Try to extract path after "uploads/"
	if idx := strings.Index(urlStr, "uploads/"); idx >= 0 {
		return urlStr[idx+8:] // len("uploads/") = 8
	}

	// Fallback: extract last 3 path segments (e.g., "2024/01/file.pdf")
	parts := strings.Split(urlStr, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[len(parts)-3:], "/")
	}
	if len(parts) >= 1 {
		return parts[len(parts)-1]
	}
	return ""
}

// extractURLs extracts all URLs from HTML content matching the pattern
func (f *Filter) extractURLs(content string, pattern *regexp.Regexp) []string {
	matches := pattern.FindAllStringSubmatch(content, -1)
	var urls []string
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			url := match[1]
			if !seen[url] {
				urls = append(urls, url)
				seen[url] = true
			}
		}
	}
	return urls
}

// extractMediaLinks extracts media file URLs from <a href> links
func (f *Filter) extractMediaLinks(content string, pattern *regexp.Regexp) []string {
	matches := pattern.FindAllStringSubmatch(content, -1)
	var urls []string
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			url := match[1]
			if !seen[url] && f.isMediaURL(url) {
				urls = append(urls, url)
				seen[url] = true
			}
		}
	}
	return urls
}

// isMediaURL checks if a URL points to a media file based on extension
func (f *Filter) isMediaURL(url string) bool {
	lowerURL := strings.ToLower(url)
	// Remove query string for extension check
	if idx := strings.Index(lowerURL, "?"); idx > 0 {
		lowerURL = lowerURL[:idx]
	}
	for _, ext := range mediaExtensions {
		if strings.HasSuffix(lowerURL, ext) {
			return true
		}
	}
	return false
}
