package filter

import (
	"net/url"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// PathFilter filters content by URL path patterns
type PathFilter struct {
	Pattern string
}

// NewPathFilter creates a new path filter with the given pattern
func NewPathFilter(pattern string) *PathFilter {
	return &PathFilter{
		Pattern: pattern,
	}
}

// FilterPosts filters posts whose Link field path contains the pattern
func (f *PathFilter) FilterPosts(posts []models.WordPressPost) []models.WordPressPost {
	if f.Pattern == "" {
		return posts
	}

	var filtered []models.WordPressPost
	for _, post := range posts {
		if f.matchesPattern(post.Link) {
			filtered = append(filtered, post)
		}
	}
	return filtered
}

// matchesPattern checks if the URL path contains the filter pattern
func (f *PathFilter) matchesPattern(link string) bool {
	if link == "" {
		return false
	}

	parsedURL, err := url.Parse(link)
	if err != nil {
		return false
	}

	// Check if the path contains the pattern (simple substring matching)
	return strings.Contains(parsedURL.Path, f.Pattern)
}
