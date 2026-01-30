package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/pkg/models"
)

func TestNewPathFilter(t *testing.T) {
	f := NewPathFilter("/fr/arts/")
	assert.NotNil(t, f)
	assert.Equal(t, "/fr/arts/", f.Pattern)
}

func TestFilterPosts_EmptyPattern(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Link: "https://example.com/fr/arts/post1"},
		{ID: 2, Link: "https://example.com/en/blog/post2"},
	}

	f := NewPathFilter("")
	filtered := f.FilterPosts(posts)

	// Empty pattern should return all posts
	assert.Len(t, filtered, 2)
}

func TestFilterPosts_MatchingPattern(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Link: "https://example.com/fr/arts/post1"},
		{ID: 2, Link: "https://example.com/fr/arts/post2"},
		{ID: 3, Link: "https://example.com/en/blog/post3"},
		{ID: 4, Link: "https://example.com/de/news/post4"},
	}

	f := NewPathFilter("/fr/arts/")
	filtered := f.FilterPosts(posts)

	assert.Len(t, filtered, 2)
	assert.Equal(t, 1, filtered[0].ID)
	assert.Equal(t, 2, filtered[1].ID)
}

func TestFilterPosts_PartialPathMatch(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Link: "https://example.com/articles/tech/post1"},
		{ID: 2, Link: "https://example.com/art/post2"},
		{ID: 3, Link: "https://example.com/marketing/post3"},
	}

	f := NewPathFilter("/art")
	filtered := f.FilterPosts(posts)

	// Should match both /articles and /art
	assert.Len(t, filtered, 2)
}

func TestFilterPosts_NoMatches(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Link: "https://example.com/en/blog/post1"},
		{ID: 2, Link: "https://example.com/fr/news/post2"},
	}

	f := NewPathFilter("/fr/arts/")
	filtered := f.FilterPosts(posts)

	assert.Len(t, filtered, 0)
}

func TestFilterPosts_EmptyPosts(t *testing.T) {
	f := NewPathFilter("/fr/arts/")
	filtered := f.FilterPosts(nil)
	assert.Len(t, filtered, 0)

	filtered = f.FilterPosts([]models.WordPressPost{})
	assert.Len(t, filtered, 0)
}

func TestFilterPosts_InvalidURL(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Link: "not-a-valid-url"},
		{ID: 2, Link: "https://example.com/fr/arts/valid"},
	}

	f := NewPathFilter("/fr/arts/")
	filtered := f.FilterPosts(posts)

	// Should only match the valid URL
	assert.Len(t, filtered, 1)
	assert.Equal(t, 2, filtered[0].ID)
}

func TestFilterPosts_EmptyLink(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Link: ""},
		{ID: 2, Link: "https://example.com/fr/arts/post"},
	}

	f := NewPathFilter("/fr/arts/")
	filtered := f.FilterPosts(posts)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 2, filtered[0].ID)
}

func TestFilterPosts_CaseSensitive(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Link: "https://example.com/FR/ARTS/post1"},
		{ID: 2, Link: "https://example.com/fr/arts/post2"},
	}

	// Substring match is case-sensitive
	f := NewPathFilter("/fr/arts/")
	filtered := f.FilterPosts(posts)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 2, filtered[0].ID)
}

func TestMatchesPattern_URLWithQueryParams(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Link: "https://example.com/fr/arts/post?utm_source=twitter"},
	}

	f := NewPathFilter("/fr/arts/")
	filtered := f.FilterPosts(posts)

	// Query params should not affect path matching
	assert.Len(t, filtered, 1)
}

func TestMatchesPattern_URLWithFragment(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Link: "https://example.com/fr/arts/post#section1"},
	}

	f := NewPathFilter("/fr/arts/")
	filtered := f.FilterPosts(posts)

	// Fragment should not affect path matching
	assert.Len(t, filtered, 1)
}

func TestMatchesPattern_MalformedURL(t *testing.T) {
	// Test with a URL that causes url.Parse to fail
	// A URL with control characters will fail
	posts := []models.WordPressPost{
		{ID: 1, Link: "https://example.com/fr/arts/post"},
		{ID: 2, Link: "http://[::1]:namedport/path"}, // Invalid port
	}

	f := NewPathFilter("/fr/arts/")
	filtered := f.FilterPosts(posts)

	// Only the first valid URL should match
	assert.Len(t, filtered, 1)
	assert.Equal(t, 1, filtered[0].ID)
}

func TestMatchesPattern_DirectCall(t *testing.T) {
	f := NewPathFilter("/test/")

	// Test direct call with empty link
	result := f.matchesPattern("")
	assert.False(t, result, "Empty link should not match")

	// Test direct call with valid matching URL
	result = f.matchesPattern("https://example.com/test/page")
	assert.True(t, result, "Valid URL with matching path should match")

	// Test direct call with non-matching URL
	result = f.matchesPattern("https://example.com/other/page")
	assert.False(t, result, "Non-matching URL should not match")
}

func TestFilterPosts_MultiplePatternsMatches(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, Link: "https://example.com/blog/tech/post1"},
		{ID: 2, Link: "https://example.com/blog/news/post2"},
		{ID: 3, Link: "https://example.com/shop/product"},
		{ID: 4, Link: "https://example.com/blog/tech/post3"},
	}

	f := NewPathFilter("/blog/tech/")
	filtered := f.FilterPosts(posts)

	assert.Len(t, filtered, 2)
	assert.Equal(t, 1, filtered[0].ID)
	assert.Equal(t, 4, filtered[1].ID)
}

func TestFilterPosts_NilSlice(t *testing.T) {
	f := NewPathFilter("/test/")
	filtered := f.FilterPosts(nil)

	// nil input should return nil
	assert.Nil(t, filtered)
}

func TestNewPathFilter_EmptyPattern(t *testing.T) {
	f := NewPathFilter("")
	assert.NotNil(t, f)
	assert.Equal(t, "", f.Pattern)
}
