package media

import (
	"path"
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
	refs := newMediaRefs()

	for _, post := range posts {
		f.collectRefs(refs, post)
	}
	for _, page := range pages {
		f.collectRefs(refs, page)
	}

	relevantIDs, relevantURLs, relevantPaths, relevantKeys := refs.ids, refs.urls, refs.paths, refs.keys

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
		// Size/-scaled-insensitive match: content may embed photo-1024x768.jpg while
		// the attachment's source_url is photo-scaled.jpg (#22).
		if key := f.canonicalMediaKey(media.SourceURL); key != "" && relevantKeys[key] {
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

// mediaRefs accumulates every way a post can point at an attachment.
type mediaRefs struct {
	ids   map[int]bool
	urls  map[string]bool
	paths map[string]bool
	keys  map[string]bool // size/-scaled-insensitive attachment keys
}

func newMediaRefs() *mediaRefs {
	return &mediaRefs{
		ids:   make(map[int]bool),
		urls:  make(map[string]bool),
		paths: make(map[string]bool),
		keys:  make(map[string]bool),
	}
}

var (
	// refImgSrcPattern matches an <img> src. Gutenberg also emits srcset and, with
	// lazy-loading plugins, data-src — both are collected so an image embedded in a
	// figure block is recognized however the theme wrote it (#22).
	refImgSrcPattern  = regexp.MustCompile(`(?i)<img[^>]+\bsrc\s*=\s*["']([^"']+)["']`)
	refDataSrcPattern = regexp.MustCompile(`(?i)<img[^>]+\bdata-src\s*=\s*["']([^"']+)["']`)
	refSrcsetPattern  = regexp.MustCompile(`(?i)\b(?:data-)?srcset\s*=\s*["']([^"']+)["']`)
	refLinkPattern    = regexp.MustCompile(`(?i)<a[^>]+href\s*=\s*["']([^"']+)["']`)
)

// collectRefs records every attachment reference a post carries: its featured
// media id, plus the images and media links in its content and excerpt.
func (f *Filter) collectRefs(refs *mediaRefs, post models.WordPressPost) {
	if post.FeaturedMedia > 0 {
		refs.ids[post.FeaturedMedia] = true
	}

	for _, html := range []string{post.Content.Rendered, post.Excerpt.Rendered} {
		if html == "" {
			continue
		}
		for _, pattern := range []*regexp.Regexp{refImgSrcPattern, refDataSrcPattern} {
			f.addRefs(refs, f.extractURLs(html, pattern))
		}
		f.addRefs(refs, f.extractSrcsetURLs(html))
		f.addRefs(refs, f.extractMediaLinks(html, refLinkPattern))
	}
}

// addRefs registers each URL under every form the matcher checks.
func (f *Filter) addRefs(refs *mediaRefs, urls []string) {
	for _, url := range urls {
		refs.urls[url] = true
		if suffix := f.extractPathSuffix(url); suffix != "" {
			refs.paths[suffix] = true
		}
		if key := f.canonicalMediaKey(url); key != "" {
			refs.keys[key] = true
		}
	}
}

// extractSrcsetURLs pulls every candidate URL out of the srcset attributes in
// html. A srcset is a comma-separated list of "<url> <descriptor>" pairs.
func (f *Filter) extractSrcsetURLs(html string) []string {
	var urls []string
	seen := make(map[string]bool)

	for _, match := range refSrcsetPattern.FindAllStringSubmatch(html, -1) {
		for _, candidate := range strings.Split(match[1], ",") {
			fields := strings.Fields(candidate)
			if len(fields) == 0 {
				continue
			}
			if url := fields[0]; !seen[url] {
				seen[url] = true
				urls = append(urls, url)
			}
		}
	}

	return urls
}

// sizeOrScaledSuffix matches WordPress' generated-variant suffixes: "-1024x768"
// for a registered size and "-scaled" for the large-upload rescale.
var sizeOrScaledSuffix = regexp.MustCompile(`(?i)-(?:\d{1,5}x\d{1,5}|scaled)$`)

// canonicalMediaKey reduces a URL to the attachment it belongs to: the uploads-path
// suffix with any size or "-scaled" marker removed. `photo-1024x768.jpg`,
// `photo-scaled.jpg` and `photo.jpg` therefore share one key, so an image embedded
// at a size the media registry does not list still matches its attachment (#22).
func (f *Filter) canonicalMediaKey(urlStr string) string {
	suffix := f.extractPathSuffix(urlStr)
	if suffix == "" {
		return ""
	}

	ext := path.Ext(suffix)
	base := strings.TrimSuffix(suffix, ext)

	return sizeOrScaledSuffix.ReplaceAllString(base, "") + ext
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
