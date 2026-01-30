package media

import (
	"regexp"

	"github.com/tradik/wpexporter/pkg/models"
)

// Filter handles filtering of media items to only relevant ones
type Filter struct{}

// NewFilter creates a new media filter
func NewFilter() *Filter {
	return &Filter{}
}

// FilterRelevantMedia returns only media items that are:
// 1. Featured images of posts/pages
// 2. Images embedded in post/page content
func (f *Filter) FilterRelevantMedia(
	posts []models.WordPressPost,
	pages []models.WordPressPost,
	allMedia []models.WordPressMedia,
) []models.WordPressMedia {
	relevantIDs := make(map[int]bool)
	relevantURLs := make(map[string]bool)

	// Regex to extract image URLs from HTML content
	imgPattern := regexp.MustCompile(`<img[^>]+src\s*=\s*["']([^"']+)["']`)

	// Collect featured media IDs and content images from posts
	for _, post := range posts {
		if post.FeaturedMedia > 0 {
			relevantIDs[post.FeaturedMedia] = true
		}
		urls := f.extractImageURLs(post.Content.Rendered, imgPattern)
		for _, url := range urls {
			relevantURLs[url] = true
		}
		// Also check excerpt for images
		excerptURLs := f.extractImageURLs(post.Excerpt.Rendered, imgPattern)
		for _, url := range excerptURLs {
			relevantURLs[url] = true
		}
	}

	// Collect featured media IDs and content images from pages
	for _, page := range pages {
		if page.FeaturedMedia > 0 {
			relevantIDs[page.FeaturedMedia] = true
		}
		urls := f.extractImageURLs(page.Content.Rendered, imgPattern)
		for _, url := range urls {
			relevantURLs[url] = true
		}
		// Also check excerpt for images
		excerptURLs := f.extractImageURLs(page.Excerpt.Rendered, imgPattern)
		for _, url := range excerptURLs {
			relevantURLs[url] = true
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
		// Check if media URL is referenced in content
		if relevantURLs[media.SourceURL] {
			filtered = append(filtered, media)
			continue
		}
		// Also check different size URLs
		for _, size := range media.MediaDetails.Sizes {
			if relevantURLs[size.SourceURL] {
				filtered = append(filtered, media)
				break
			}
		}
	}

	return filtered
}

// extractImageURLs extracts image URLs from HTML content
func (f *Filter) extractImageURLs(content string, pattern *regexp.Regexp) []string {
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
