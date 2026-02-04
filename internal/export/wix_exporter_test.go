package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestNewWixExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "wix",
	}

	exporter := NewWixExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
}

func TestWixExporter_ConvertTag(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWixExporter(cfg)

	tag := models.WordPressTag{
		ID:          1,
		Name:        "Technology",
		Slug:        "technology",
		Description: "Tech posts",
	}

	wixTag := exporter.convertTag(tag)

	assert.Equal(t, "1", wixTag.ID)
	assert.Equal(t, "Technology", wixTag.Name)
	assert.Equal(t, "technology", wixTag.Slug)
}

func TestWixExporter_ConvertCategory(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWixExporter(cfg)

	tests := []struct {
		name     string
		category models.WordPressCategory
		expected WixCategory
	}{
		{
			name: "Root category",
			category: models.WordPressCategory{
				ID:          1,
				Name:        "Technology",
				Slug:        "technology",
				Description: "Tech posts",
				Parent:      0,
			},
			expected: WixCategory{
				ID:          "1",
				Name:        "Technology",
				Slug:        "technology",
				Description: "Tech posts",
				ParentID:    "",
			},
		},
		{
			name: "Child category",
			category: models.WordPressCategory{
				ID:          2,
				Name:        "Programming",
				Slug:        "programming",
				Description: "Code tutorials",
				Parent:      1,
			},
			expected: WixCategory{
				ID:          "2",
				Name:        "Programming",
				Slug:        "programming",
				Description: "Code tutorials",
				ParentID:    "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wixCat := exporter.convertCategory(tt.category)
			assert.Equal(t, tt.expected.ID, wixCat.ID)
			assert.Equal(t, tt.expected.Name, wixCat.Name)
			assert.Equal(t, tt.expected.Slug, wixCat.Slug)
			assert.Equal(t, tt.expected.Description, wixCat.Description)
			assert.Equal(t, tt.expected.ParentID, wixCat.ParentID)
		})
	}
}

func TestWixExporter_ConvertMedia(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWixExporter(cfg)

	tests := []struct {
		name     string
		media    models.WordPressMedia
		expected WixMedia
	}{
		{
			name: "Image with dimensions as float64",
			media: models.WordPressMedia{
				ID:        1,
				Title:     models.RenderedContent{Rendered: "Test Image"},
				SourceURL: "https://example.com/image.jpg",
				MimeType:  "image/jpeg",
				AltText:   "Test alt text",
				MediaDetails: models.MediaDetails{
					Width:  float64(800),
					Height: float64(600),
				},
			},
			expected: WixMedia{
				ID:       "1",
				Title:    "Test Image",
				URL:      "https://example.com/image.jpg",
				MimeType: "image/jpeg",
				AltText:  "Test alt text",
				Width:    800,
				Height:   600,
			},
		},
		{
			name: "Image with dimensions as int",
			media: models.WordPressMedia{
				ID:        2,
				Title:     models.RenderedContent{Rendered: "Test Image 2"},
				SourceURL: "https://example.com/image2.jpg",
				MimeType:  "image/png",
				AltText:   "Alt text 2",
				MediaDetails: models.MediaDetails{
					Width:  int(1024),
					Height: int(768),
				},
			},
			expected: WixMedia{
				ID:       "2",
				Title:    "Test Image 2",
				URL:      "https://example.com/image2.jpg",
				MimeType: "image/png",
				AltText:  "Alt text 2",
				Width:    1024,
				Height:   768,
			},
		},
		{
			name: "Image without dimensions",
			media: models.WordPressMedia{
				ID:        3,
				Title:     models.RenderedContent{Rendered: "No Dimensions"},
				SourceURL: "https://example.com/image3.jpg",
				MimeType:  "image/gif",
			},
			expected: WixMedia{
				ID:       "3",
				Title:    "No Dimensions",
				URL:      "https://example.com/image3.jpg",
				MimeType: "image/gif",
				Width:    0,
				Height:   0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wixMedia := exporter.convertMedia(tt.media)
			assert.Equal(t, tt.expected.ID, wixMedia.ID)
			assert.Equal(t, tt.expected.Title, wixMedia.Title)
			assert.Equal(t, tt.expected.URL, wixMedia.URL)
			assert.Equal(t, tt.expected.MimeType, wixMedia.MimeType)
			assert.Equal(t, tt.expected.Width, wixMedia.Width)
			assert.Equal(t, tt.expected.Height, wixMedia.Height)
		})
	}
}

func TestWixExporter_ConvertPost(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWixExporter(cfg)

	tagMap := map[int]string{1: "Go", 2: "Programming"}
	categoryMap := map[int]string{1: "Technology"}
	userMap := map[int]string{1: "John Doe"}
	mediaMap := map[int]string{10: "https://example.com/featured.jpg"}

	post := models.WordPressPost{
		ID:            1,
		Slug:          "test-post",
		Status:        "publish",
		Title:         models.RenderedContent{Rendered: "Test Post Title"},
		Content:       models.RenderedContent{Rendered: "<p>Test content</p>"},
		Excerpt:       models.RenderedContent{Rendered: "<p>Test excerpt</p>"},
		Author:        1,
		Date:          models.WordPressTime{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		Modified:      models.WordPressTime{Time: time.Date(2024, 1, 20, 12, 0, 0, 0, time.UTC)},
		Categories:    []int{1},
		Tags:          []int{1, 2},
		FeaturedMedia: 10,
		Sticky:        true,
		SEO: models.SEOData{
			Title:           "SEO Title",
			MetaDescription: "SEO Description",
		},
	}

	wixPost := exporter.convertPost(post, tagMap, categoryMap, userMap, mediaMap)

	assert.Equal(t, "1", wixPost.ID)
	assert.Equal(t, "Test Post Title", wixPost.Title)
	assert.Equal(t, "test-post", wixPost.Slug)
	assert.Equal(t, "<p>Test content</p>", wixPost.Content)
	assert.Equal(t, "Test excerpt", wixPost.Excerpt)
	assert.True(t, wixPost.Featured)
	assert.True(t, wixPost.Published)
	assert.Equal(t, "John Doe", wixPost.Author)
	assert.Equal(t, "https://example.com/featured.jpg", wixPost.CoverImage)
	assert.Len(t, wixPost.Tags, 2)
	assert.Contains(t, wixPost.Tags, "Go")
	assert.Contains(t, wixPost.Tags, "Programming")
	assert.Len(t, wixPost.Categories, 1)
	assert.Contains(t, wixPost.Categories, "Technology")
	assert.Equal(t, "SEO Title", wixPost.SEOTitle)
	assert.Equal(t, "SEO Description", wixPost.SEODescription)
}

func TestWixExporter_ConvertPostDraft(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWixExporter(cfg)

	post := models.WordPressPost{
		ID:     1,
		Slug:   "draft-post",
		Status: "draft",
		Title:  models.RenderedContent{Rendered: "Draft Post"},
	}

	wixPost := exporter.convertPost(post, nil, nil, nil, nil)

	assert.False(t, wixPost.Published)
	assert.True(t, wixPost.PublishedDate.IsZero())
}

func TestWixExporter_ConvertPage(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWixExporter(cfg)

	page := models.WordPressPost{
		ID:       1,
		Slug:     "about-us",
		Status:   "publish",
		Title:    models.RenderedContent{Rendered: "About Us"},
		Content:  models.RenderedContent{Rendered: "<p>About page content</p>"},
		Modified: models.WordPressTime{Time: time.Date(2024, 1, 20, 12, 0, 0, 0, time.UTC)},
		SEO: models.SEOData{
			Title:           "About - Company",
			MetaDescription: "Learn about our company",
		},
	}

	wixPage := exporter.convertPage(page)

	assert.Equal(t, "1", wixPage.ID)
	assert.Equal(t, "About Us", wixPage.Title)
	assert.Equal(t, "about-us", wixPage.Slug)
	assert.Equal(t, "<p>About page content</p>", wixPage.Content)
	assert.True(t, wixPage.Published)
	assert.Equal(t, "About - Company", wixPage.SEOTitle)
	assert.Equal(t, "Learn about our company", wixPage.SEODescription)
}

func TestWixExporter_BuildWixData(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWixExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
			Language:    "en-US",
		},
		Posts: []models.WordPressPost{
			{ID: 1, Slug: "post-1", Status: "publish", Title: models.RenderedContent{Rendered: "Post 1"}},
		},
		Pages: []models.WordPressPost{
			{ID: 1, Slug: "page-1", Status: "publish", Title: models.RenderedContent{Rendered: "Page 1"}},
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Category 1", Slug: "category-1"},
		},
		Tags: []models.WordPressTag{
			{ID: 1, Name: "Tag 1", Slug: "tag-1"},
		},
		Users: []models.WordPressUser{
			{ID: 1, Name: "User 1", Slug: "user-1"},
		},
		Media: []models.WordPressMedia{
			{ID: 1, Title: models.RenderedContent{Rendered: "Image 1"}, SourceURL: "https://example.com/image.jpg", MimeType: "image/jpeg"},
		},
	}

	wixData := exporter.buildWixData(data)

	assert.Equal(t, "1.4.0", wixData.Version)
	assert.Equal(t, "wpexporter", wixData.Meta.Exporter)
	assert.Equal(t, "https://example.com", wixData.Meta.SourceURL)
	assert.Equal(t, "Test Site", wixData.Site.Title)
	assert.Equal(t, "Test Description", wixData.Site.Description)
	assert.Equal(t, "en-US", wixData.Site.Language)

	assert.Len(t, wixData.Posts, 1)
	assert.Len(t, wixData.Pages, 1)
	assert.Len(t, wixData.Categories, 1)
	assert.Len(t, wixData.Tags, 1)
	assert.Len(t, wixData.Media, 1)
}

func TestWixExporter_Export(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wix-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "wix",
	}
	exporter := NewWixExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
			Language:    "en-US",
		},
		Posts: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "test-post",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "Test Post"},
				Content: models.RenderedContent{Rendered: "<p>Test content</p>"},
			},
		},
		Pages: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "test-page",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "Test Page"},
				Content: models.RenderedContent{Rendered: "<p>Page content</p>"},
			},
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Test Category", Slug: "test-category"},
		},
		Tags: []models.WordPressTag{
			{ID: 1, Name: "Test Tag", Slug: "test-tag"},
		},
		Users: []models.WordPressUser{
			{ID: 1, Name: "Test User", Slug: "test-user"},
		},
		Media: []models.WordPressMedia{
			{ID: 1, Title: models.RenderedContent{Rendered: "Test Image"}, SourceURL: "https://example.com/image.jpg", MimeType: "image/jpeg"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	outputFile := filepath.Join(tempDir, "wix_export.json")
	assert.FileExists(t, outputFile)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	var wixData WixExportData
	err = json.Unmarshal(content, &wixData)
	require.NoError(t, err)

	assert.Equal(t, "wpexporter", wixData.Meta.Exporter)
	assert.Len(t, wixData.Posts, 1)
	assert.Len(t, wixData.Pages, 1)
	assert.Len(t, wixData.Categories, 1)
	assert.Len(t, wixData.Tags, 1)
	assert.Len(t, wixData.Media, 1)
}

func TestWixExporter_ExportWithJSONExtension(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wix-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	outputFile := filepath.Join(tempDir, "custom_wix.json")
	cfg := &config.Config{
		Output: outputFile,
		Format: "wix",
	}
	exporter := NewWixExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	assert.FileExists(t, outputFile)
}

func TestWixExporter_EmptyData(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWixExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Empty Site",
			URL:  "https://example.com",
		},
	}

	wixData := exporter.buildWixData(data)

	assert.Equal(t, "Empty Site", wixData.Site.Title)
	assert.Len(t, wixData.Posts, 0)
	assert.Len(t, wixData.Pages, 0)
	assert.Len(t, wixData.Categories, 0)
	assert.Len(t, wixData.Tags, 0)
	assert.Len(t, wixData.Media, 0)
}

// Benchmark tests
func BenchmarkWixExporter_BuildWixData(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewWixExporter(cfg)

	posts := make([]models.WordPressPost, 100)
	for i := 0; i < 100; i++ {
		posts[i] = models.WordPressPost{
			ID:      i + 1,
			Slug:    "test-post",
			Status:  "publish",
			Title:   models.RenderedContent{Rendered: "Test Post"},
			Content: models.RenderedContent{Rendered: "<p>Content</p>"},
		}
	}

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test", URL: "https://example.com"},
		Posts: posts,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exporter.buildWixData(data)
	}
}
