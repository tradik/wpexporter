package media

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/pkg/models"
)

func TestNewFilter(t *testing.T) {
	f := NewFilter()
	assert.NotNil(t, f)
}

func TestFilterRelevantMedia_FeaturedImages(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, FeaturedMedia: 100, Content: models.RenderedContent{Rendered: ""}},
		{ID: 2, FeaturedMedia: 200, Content: models.RenderedContent{Rendered: ""}},
	}
	pages := []models.WordPressPost{
		{ID: 3, FeaturedMedia: 300, Content: models.RenderedContent{Rendered: ""}},
	}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/img100.jpg"},
		{ID: 200, SourceURL: "https://example.com/img200.jpg"},
		{ID: 300, SourceURL: "https://example.com/img300.jpg"},
		{ID: 400, SourceURL: "https://example.com/img400.jpg"}, // Not referenced
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 3)
	ids := make(map[int]bool)
	for _, m := range filtered {
		ids[m.ID] = true
	}
	assert.True(t, ids[100])
	assert.True(t, ids[200])
	assert.True(t, ids[300])
	assert.False(t, ids[400])
}

func TestFilterRelevantMedia_ContentImages(t *testing.T) {
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content: models.RenderedContent{
				Rendered: `<p>Some text</p><img src="https://example.com/content-img.jpg" /><p>More text</p>`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/content-img.jpg"},
		{ID: 200, SourceURL: "https://example.com/other-img.jpg"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestFilterRelevantMedia_ExcerptImages(t *testing.T) {
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content:       models.RenderedContent{Rendered: ""},
			Excerpt: models.RenderedContent{
				Rendered: `<img src="https://example.com/excerpt-img.jpg" />`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/excerpt-img.jpg"},
		{ID: 200, SourceURL: "https://example.com/other-img.jpg"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestFilterRelevantMedia_MediaSizes(t *testing.T) {
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content: models.RenderedContent{
				Rendered: `<img src="https://example.com/img-300x200.jpg" />`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{
			ID:        100,
			SourceURL: "https://example.com/img.jpg",
			MediaDetails: models.MediaDetails{
				Sizes: map[string]models.MediaSize{
					"medium": {SourceURL: "https://example.com/img-300x200.jpg"},
					"large":  {SourceURL: "https://example.com/img-1024x768.jpg"},
				},
			},
		},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestFilterRelevantMedia_EmptyInputs(t *testing.T) {
	f := NewFilter()

	// Empty posts and pages
	filtered := f.FilterRelevantMedia(nil, nil, []models.WordPressMedia{{ID: 1}})
	assert.Len(t, filtered, 0)

	// Empty media
	filtered = f.FilterRelevantMedia(
		[]models.WordPressPost{{ID: 1, FeaturedMedia: 100}},
		nil,
		nil,
	)
	assert.Len(t, filtered, 0)
}

func TestFilterRelevantMedia_DuplicateReferences(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, FeaturedMedia: 100, Content: models.RenderedContent{Rendered: ""}},
		{ID: 2, FeaturedMedia: 100, Content: models.RenderedContent{Rendered: ""}}, // Same featured image
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/img100.jpg"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	// Should only include the media item once
	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestFilterRelevantMedia_LinkedDocuments(t *testing.T) {
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content: models.RenderedContent{
				Rendered: `<p>Download our <a href="https://example.com/document.pdf">PDF guide</a></p>`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/document.pdf"},
		{ID: 200, SourceURL: "https://example.com/other.pdf"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestFilterRelevantMedia_LinkedVideos(t *testing.T) {
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content: models.RenderedContent{
				Rendered: `<p>Watch <a href="https://example.com/video.mp4">our video</a></p>`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/video.mp4"},
		{ID: 200, SourceURL: "https://example.com/other.mp4"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestFilterRelevantMedia_LinkedArchives(t *testing.T) {
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content: models.RenderedContent{
				Rendered: `<a href="https://example.com/files.zip">Download ZIP</a>`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/files.zip"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestFilterRelevantMedia_MixedContent(t *testing.T) {
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 100,
			Content: models.RenderedContent{
				Rendered: `<img src="https://example.com/image.jpg" />
				<p><a href="https://example.com/document.pdf">PDF</a></p>
				<p><a href="https://example.com/video.mp4">Video</a></p>
				<p><a href="https://example.com/page.html">Page link (ignored)</a></p>`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/featured.jpg"},
		{ID: 200, SourceURL: "https://example.com/image.jpg"},
		{ID: 300, SourceURL: "https://example.com/document.pdf"},
		{ID: 400, SourceURL: "https://example.com/video.mp4"},
		{ID: 500, SourceURL: "https://example.com/unreferenced.jpg"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 4)
	ids := make(map[int]bool)
	for _, m := range filtered {
		ids[m.ID] = true
	}
	assert.True(t, ids[100])  // Featured
	assert.True(t, ids[200])  // Image in content
	assert.True(t, ids[300])  // PDF link
	assert.True(t, ids[400])  // Video link
	assert.False(t, ids[500]) // Not referenced
}

func TestFilterRelevantMedia_LinkWithQueryString(t *testing.T) {
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content: models.RenderedContent{
				Rendered: `<a href="https://example.com/doc.pdf?v=1.0">Download</a>`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/doc.pdf?v=1.0"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestIsMediaURL(t *testing.T) {
	f := NewFilter()

	tests := []struct {
		url      string
		expected bool
	}{
		// Documents
		{"https://example.com/file.pdf", true},
		{"https://example.com/file.docx", true},
		{"https://example.com/file.xlsx", true},
		{"https://example.com/file.pptx", true},
		// Videos
		{"https://example.com/video.mp4", true},
		{"https://example.com/video.webm", true},
		{"https://example.com/video.avi", true},
		// Audio
		{"https://example.com/audio.mp3", true},
		{"https://example.com/audio.wav", true},
		// Archives
		{"https://example.com/files.zip", true},
		{"https://example.com/files.rar", true},
		// Images
		{"https://example.com/image.jpg", true},
		{"https://example.com/image.png", true},
		// With query string
		{"https://example.com/file.pdf?v=1", true},
		// Non-media URLs
		{"https://example.com/page", false},
		{"https://example.com/page.html", false},
		{"https://example.com/api/endpoint", false},
		{"https://example.com/", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := f.isMediaURL(tt.url)
			assert.Equal(t, tt.expected, result, "URL: %s", tt.url)
		})
	}
}

func TestExtractMediaLinks(t *testing.T) {
	f := NewFilter()
	pattern := `<a[^>]+href\s*=\s*["']([^"']+)["']`
	linkPattern := regexp.MustCompile(pattern)

	content := `
		<a href="https://example.com/doc.pdf">PDF</a>
		<a href="https://example.com/video.mp4">Video</a>
		<a href="https://example.com/page.html">Page</a>
		<a href="https://example.com/archive.zip">ZIP</a>
	`

	urls := f.extractMediaLinks(content, linkPattern)

	assert.Len(t, urls, 3) // PDF, MP4, ZIP (not HTML)
	assert.Contains(t, urls, "https://example.com/doc.pdf")
	assert.Contains(t, urls, "https://example.com/video.mp4")
	assert.Contains(t, urls, "https://example.com/archive.zip")
}

func TestFilterRelevantMedia_DifferentDomain(t *testing.T) {
	// Test case: content links to CDN, but Media API returns original domain
	// Both have same path suffix: "2024/01/document.pdf"
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content: models.RenderedContent{
				Rendered: `<a href="https://cdn.example.com/wp-content/uploads/2024/01/document.pdf">PDF</a>`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		// Media API returns different domain but same path after uploads/
		{ID: 100, SourceURL: "https://example.com/wp-content/uploads/2024/01/document.pdf"},
		// Different path - should NOT match even though filename is similar
		{ID: 200, SourceURL: "https://example.com/wp-content/uploads/2023/05/document.pdf"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	// Should match by path suffix (2024/01/document.pdf), not by filename alone
	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestFilterRelevantMedia_FilenameWithQueryString(t *testing.T) {
	// Test case: content has query string, API doesn't
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content: models.RenderedContent{
				Rendered: `<a href="https://example.com/report.pdf?ver=2.0&cache=1">PDF</a>`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/report.pdf"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestExtractPathSuffix(t *testing.T) {
	f := NewFilter()

	tests := []struct {
		url      string
		expected string
	}{
		// WordPress uploads paths - extracts after "uploads/"
		{"https://cdn.example.com/wp-content/uploads/2024/01/document.pdf", "2024/01/document.pdf"},
		{"https://example.com/wp-content/uploads/2023/05/image.jpg", "2023/05/image.jpg"},
		{"https://example.com/uploads/2024/file.pdf", "2024/file.pdf"},
		// With query string
		{"https://example.com/wp-content/uploads/2024/01/file.pdf?v=1", "2024/01/file.pdf"},
		// Fallback to last 3 segments when no "uploads/"
		{"https://example.com/path/to/file.pdf", "path/to/file.pdf"},
		{"https://example.com/a/b/c/d/file.pdf", "c/d/file.pdf"},
		// Lowercase
		{"https://example.com/wp-content/uploads/2024/File.PDF", "2024/file.pdf"},
		// Short paths (includes leading slash from URL split)
		{"https://example.com/file.pdf", "/example.com/file.pdf"},
		{"file.pdf", "file.pdf"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := f.extractPathSuffix(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}
