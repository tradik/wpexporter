package export

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestNewWordPressExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "wordpress",
	}

	exporter := NewWordPressExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
}

func TestWordPressExporter_WrapCDATA(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Plain text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "HTML content",
			input:    "<p>Hello World</p>",
			expected: "<![CDATA[<p>Hello World</p>]]>",
		},
		{
			name:     "Content with ampersand",
			input:    "Hello & World",
			expected: "<![CDATA[Hello & World]]>",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapCDATA(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWordPressExporter_BoolToInt(t *testing.T) {
	assert.Equal(t, 1, boolToInt(true))
	assert.Equal(t, 0, boolToInt(false))
}

func TestWordPressExporter_ExtractFilePath(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "WordPress uploads URL",
			url:      "https://example.com/wp-content/uploads/2024/01/image.jpg",
			expected: "2024/01/image.jpg",
		},
		{
			name:     "Simple URL",
			url:      "https://example.com/image.jpg",
			expected: "image.jpg",
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFilePath(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWordPressExporter_ConvertAuthors(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWordPressExporter(cfg)

	users := []models.WordPressUser{
		{ID: 1, Name: "John Doe", Slug: "john-doe"},
		{ID: 2, Name: "Jane Smith", Slug: "jane-smith"},
	}

	authors := exporter.convertAuthors(users)

	assert.Len(t, authors, 2)
	assert.Equal(t, 1, authors[0].ID)
	assert.Equal(t, "john-doe", authors[0].Login)
	assert.Equal(t, "John Doe", authors[0].DisplayName)
}

func TestWordPressExporter_ConvertCategories(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWordPressExporter(cfg)

	categories := []models.WordPressCategory{
		{ID: 1, Name: "Technology", Slug: "technology", Parent: 0, Description: "Tech posts"},
		{ID: 2, Name: "Programming", Slug: "programming", Parent: 1, Description: "Programming tutorials"},
	}

	wxrCategories := exporter.convertCategories(categories)

	assert.Len(t, wxrCategories, 2)
	assert.Equal(t, 1, wxrCategories[0].TermID)
	assert.Equal(t, "technology", wxrCategories[0].NiceName)
	assert.Equal(t, "Technology", wxrCategories[0].Name)
	assert.Equal(t, "", wxrCategories[0].Parent)

	assert.Equal(t, 2, wxrCategories[1].TermID)
	assert.Equal(t, "programming", wxrCategories[1].NiceName)
	assert.Equal(t, "technology", wxrCategories[1].Parent)
}

func TestWordPressExporter_ConvertTags(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWordPressExporter(cfg)

	tags := []models.WordPressTag{
		{ID: 1, Name: "Go", Slug: "go", Description: "Go language"},
		{ID: 2, Name: "Python", Slug: "python", Description: "Python language"},
	}

	wxrTags := exporter.convertTags(tags)

	assert.Len(t, wxrTags, 2)
	assert.Equal(t, 1, wxrTags[0].TermID)
	assert.Equal(t, "go", wxrTags[0].Slug)
	assert.Equal(t, "Go", wxrTags[0].Name)
}

func TestWordPressExporter_ConvertPostToItem(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWordPressExporter(cfg)

	categoryMap := map[int]models.WordPressCategory{
		1: {ID: 1, Name: "Technology", Slug: "technology"},
	}
	tagMap := map[int]models.WordPressTag{
		1: {ID: 1, Name: "Go", Slug: "go"},
	}
	userMap := map[int]models.WordPressUser{
		1: {ID: 1, Name: "Test User", Slug: "test-user"},
	}

	post := models.WordPressPost{
		ID:            1,
		Slug:          "test-post",
		Status:        "publish",
		Title:         models.RenderedContent{Rendered: "Test Post Title"},
		Content:       models.RenderedContent{Rendered: "<p>Test content</p>"},
		Excerpt:       models.RenderedContent{Rendered: "<p>Test excerpt</p>"},
		Author:        1,
		Date:          models.WordPressTime{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		DateGMT:       models.WordPressTime{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		Modified:      models.WordPressTime{Time: time.Date(2024, 1, 20, 12, 0, 0, 0, time.UTC)},
		Link:          "https://example.com/test-post",
		Categories:    []int{1},
		Tags:          []int{1},
		FeaturedMedia: 10,
		CommentStatus: "open",
		PingStatus:    "closed",
		Sticky:        false,
	}

	item := exporter.convertPostToItem(post, "post", categoryMap, tagMap, userMap)

	assert.Equal(t, "Test Post Title", item.Title)
	assert.Equal(t, "https://example.com/test-post", item.Link)
	assert.Equal(t, "test-user", item.Creator)
	assert.Equal(t, 1, item.PostID)
	assert.Equal(t, "test-post", item.PostName)
	assert.Equal(t, "publish", item.Status)
	assert.Equal(t, "post", item.PostType)
	assert.Contains(t, item.ContentEncoded, "<p>Test content</p>")
	assert.Len(t, item.Categories, 2) // 1 category + 1 tag
	assert.Equal(t, "category", item.Categories[0].Domain)
	assert.Equal(t, "post_tag", item.Categories[1].Domain)

	// Verify featured image meta
	hasThumbnail := false
	for _, meta := range item.PostMeta {
		if meta.MetaKey == "_thumbnail_id" {
			hasThumbnail = true
			assert.Equal(t, "10", meta.MetaValue)
		}
	}
	assert.True(t, hasThumbnail)
}

func TestWordPressExporter_ConvertMediaToItem(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWordPressExporter(cfg)

	userMap := map[int]models.WordPressUser{
		1: {ID: 1, Slug: "test-user"},
	}

	media := models.WordPressMedia{
		ID:          1,
		Slug:        "test-image",
		Status:      "inherit",
		Title:       models.RenderedContent{Rendered: "Test Image"},
		Description: models.RenderedContent{Rendered: "Image description"},
		Caption:     models.RenderedContent{Rendered: "Image caption"},
		Author:      1,
		Date:        models.WordPressTime{Time: time.Now()},
		DateGMT:     models.WordPressTime{Time: time.Now()},
		Modified:    models.WordPressTime{Time: time.Now()},
		Link:        "https://example.com/test-image",
		SourceURL:   "https://example.com/wp-content/uploads/2024/01/test-image.jpg",
		MimeType:    "image/jpeg",
		Post:        5,
	}

	item := exporter.convertMediaToItem(media, userMap)

	assert.Equal(t, "Test Image", item.Title)
	assert.Equal(t, 1, item.PostID)
	assert.Equal(t, "test-image", item.PostName)
	assert.Equal(t, "attachment", item.PostType)
	assert.Equal(t, "https://example.com/wp-content/uploads/2024/01/test-image.jpg", item.AttachmentURL)
	assert.Equal(t, 5, item.PostParent)
}

func TestWordPressExporter_BuildWXR(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWordPressExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
			HomeURL:     "https://example.com",
			Language:    "en-US",
		},
		Posts: []models.WordPressPost{
			{
				ID:     1,
				Slug:   "test-post",
				Status: "publish",
				Title:  models.RenderedContent{Rendered: "Test Post"},
			},
		},
		Pages: []models.WordPressPost{
			{
				ID:     2,
				Slug:   "test-page",
				Status: "publish",
				Title:  models.RenderedContent{Rendered: "Test Page"},
			},
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
			{ID: 1, Slug: "image-1", SourceURL: "https://example.com/image.jpg"},
		},
	}

	wxr := exporter.buildWXR(data)

	assert.Equal(t, "2.0", wxr.Version)
	assert.Equal(t, "Test Site", wxr.Channel.Title)
	assert.Equal(t, "https://example.com", wxr.Channel.Link)
	assert.Equal(t, "Test Description", wxr.Channel.Description)
	assert.Equal(t, "en-US", wxr.Channel.Language)
	assert.Equal(t, "1.2", wxr.Channel.WXRVersion)

	assert.Len(t, wxr.Channel.Authors, 1)
	assert.Len(t, wxr.Channel.Categories, 1)
	assert.Len(t, wxr.Channel.Tags, 1)
	// Items: 1 post + 1 page + 1 media = 3
	assert.Len(t, wxr.Channel.Items, 3)
}

func TestWordPressExporter_Export(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "wordpress-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "wordpress",
	}
	exporter := NewWordPressExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
			HomeURL:     "https://example.com",
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
				ID:      2,
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
			{ID: 1, SourceURL: "https://example.com/image.jpg"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify output file exists
	outputFile := filepath.Join(tempDir, "wordpress_export.xml")
	assert.FileExists(t, outputFile)

	// Read and verify XML structure
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	// Verify XML declaration
	assert.True(t, strings.HasPrefix(string(content), "<?xml"))

	// Verify basic structure
	assert.Contains(t, string(content), "<rss version=\"2.0\"")
	assert.Contains(t, string(content), "<channel>")
	assert.Contains(t, string(content), "<title>Test Site</title>")
	assert.Contains(t, string(content), "wp:wxr_version")
}

func TestWordPressExporter_ExportWithXMLExtension(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "wordpress-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	outputFile := filepath.Join(tempDir, "custom_export.xml")
	cfg := &config.Config{
		Output: outputFile,
		Format: "wordpress",
	}
	exporter := NewWordPressExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify custom output file exists
	assert.FileExists(t, outputFile)
}

func TestWordPressExporter_XMLValidation(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWordPressExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
			HomeURL:     "https://example.com",
			Language:    "en",
		},
		Posts: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "test-post",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "Test Post with <special> chars & symbols"},
				Content: models.RenderedContent{Rendered: "<p>Content with <strong>HTML</strong> & entities</p>"},
			},
		},
	}

	wxr := exporter.buildWXR(data)

	// Marshal and unmarshal to verify valid XML
	xmlData, err := xml.MarshalIndent(wxr, "", "  ")
	require.NoError(t, err)

	var decoded WXRExport
	err = xml.Unmarshal(xmlData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "Test Site", decoded.Channel.Title)
}

func TestWordPressExporter_PostWithSEO(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWordPressExporter(cfg)

	categoryMap := map[int]models.WordPressCategory{}
	tagMap := map[int]models.WordPressTag{}
	userMap := map[int]models.WordPressUser{}

	post := models.WordPressPost{
		ID:     1,
		Slug:   "seo-test",
		Status: "publish",
		Title:  models.RenderedContent{Rendered: "SEO Test Post"},
		SEO: models.SEOData{
			Title:           "Custom SEO Title",
			MetaDescription: "Custom meta description",
		},
	}

	item := exporter.convertPostToItem(post, "post", categoryMap, tagMap, userMap)

	// Verify SEO meta is added
	hasSEOTitle := false
	hasSEODesc := false
	for _, meta := range item.PostMeta {
		if meta.MetaKey == "_yoast_wpseo_title" {
			hasSEOTitle = true
			assert.Equal(t, "Custom SEO Title", meta.MetaValue)
		}
		if meta.MetaKey == "_yoast_wpseo_metadesc" {
			hasSEODesc = true
			assert.Equal(t, "Custom meta description", meta.MetaValue)
		}
	}
	assert.True(t, hasSEOTitle)
	assert.True(t, hasSEODesc)
}

// Benchmark tests
func BenchmarkWordPressExporter_BuildWXR(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewWordPressExporter(cfg)

	// Create test data with multiple items
	posts := make([]models.WordPressPost, 100)
	for i := 0; i < 100; i++ {
		posts[i] = models.WordPressPost{
			ID:      i + 1,
			Slug:    "test-post",
			Status:  "publish",
			Title:   models.RenderedContent{Rendered: "Test Post"},
			Content: models.RenderedContent{Rendered: strings.Repeat("<p>Content</p>", 10)},
		}
	}

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test", URL: "https://example.com"},
		Posts: posts,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exporter.buildWXR(data)
	}
}
