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

func TestNewContentfulExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "contentful",
	}

	exporter := NewContentfulExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
}

func TestContentfulExporter_BuildContentTypes(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	contentTypes := exporter.buildContentTypes()

	assert.Len(t, contentTypes, 5)

	// Verify blogPost content type
	var blogPostType *ContentfulContentType
	for i, ct := range contentTypes {
		if ct.Sys.ID == "blogPost" {
			blogPostType = &contentTypes[i]
			break
		}
	}
	require.NotNil(t, blogPostType)
	assert.Equal(t, "Blog Post", blogPostType.Name)
	assert.Greater(t, len(blogPostType.Fields), 0)

	// Verify page content type exists
	var pageType *ContentfulContentType
	for i, ct := range contentTypes {
		if ct.Sys.ID == "page" {
			pageType = &contentTypes[i]
			break
		}
	}
	require.NotNil(t, pageType)
	assert.Equal(t, "Page", pageType.Name)
}

func TestContentfulExporter_ConvertPostToEntry(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	locale := "en-US"

	post := models.WordPressPost{
		ID:            1,
		Slug:          "test-article",
		Status:        "publish",
		Title:         models.RenderedContent{Rendered: "Test Article"},
		Content:       models.RenderedContent{Rendered: "<p>Article content</p>"},
		Excerpt:       models.RenderedContent{Rendered: "<p>Article excerpt</p>"},
		Author:        1,
		Date:          models.WordPressTime{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		Modified:      models.WordPressTime{Time: time.Date(2024, 1, 20, 12, 0, 0, 0, time.UTC)},
		Categories:    []int{1},
		Tags:          []int{1, 2},
		FeaturedMedia: 10,
		SEO: models.SEOData{
			Title:           "SEO Title",
			MetaDescription: "SEO Description",
		},
	}

	entry := exporter.convertPostToEntry(post, "blogPost", locale)

	assert.Equal(t, "blogPost-1", entry.Sys.ID)
	assert.Equal(t, "Entry", entry.Sys.Type)
	assert.Equal(t, "blogPost", entry.Sys.ContentType.Sys.ID)

	// Check fields
	titleField := entry.Fields["title"]
	assert.NotNil(t, titleField)
	assert.Equal(t, "Test Article", titleField[locale])

	slugField := entry.Fields["slug"]
	assert.NotNil(t, slugField)
	assert.Equal(t, "test-article", slugField[locale])

	contentField := entry.Fields["content"]
	assert.NotNil(t, contentField)
	assert.Equal(t, "<p>Article content</p>", contentField[locale])
}

func TestContentfulExporter_ConvertPostToEntryDraft(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	locale := "en-US"

	post := models.WordPressPost{
		ID:     1,
		Slug:   "draft-article",
		Status: "draft",
		Title:  models.RenderedContent{Rendered: "Draft Article"},
	}

	entry := exporter.convertPostToEntry(post, "blogPost", locale)

	// Draft posts should not have PublishedAt
	assert.Empty(t, entry.Sys.PublishedAt)
}

func TestContentfulExporter_ConvertCategoryToEntry(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	locale := "en-US"

	category := models.WordPressCategory{
		ID:          1,
		Name:        "Technology",
		Slug:        "technology",
		Description: "Tech posts",
	}

	entry := exporter.convertCategoryToEntry(category, locale)

	assert.Equal(t, "category-1", entry.Sys.ID)
	assert.Equal(t, "category", entry.Sys.ContentType.Sys.ID)
	assert.Equal(t, "Technology", entry.Fields["name"][locale])
	assert.Equal(t, "technology", entry.Fields["slug"][locale])
	assert.Equal(t, "Tech posts", entry.Fields["description"][locale])
}

func TestContentfulExporter_ConvertTagToEntry(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	locale := "en-US"

	tag := models.WordPressTag{
		ID:   1,
		Name: "Go",
		Slug: "go",
	}

	entry := exporter.convertTagToEntry(tag, locale)

	assert.Equal(t, "tag-1", entry.Sys.ID)
	assert.Equal(t, "tag", entry.Sys.ContentType.Sys.ID)
	assert.Equal(t, "Go", entry.Fields["name"][locale])
	assert.Equal(t, "go", entry.Fields["slug"][locale])
}

func TestContentfulExporter_ConvertAuthorToEntry(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	locale := "en-US"

	user := models.WordPressUser{
		ID:          1,
		Name:        "John Doe",
		Slug:        "john-doe",
		Description: "Author bio",
	}

	entry := exporter.convertAuthorToEntry(user, locale)

	assert.Equal(t, "author-1", entry.Sys.ID)
	assert.Equal(t, "author", entry.Sys.ContentType.Sys.ID)
	assert.Equal(t, "John Doe", entry.Fields["name"][locale])
	assert.Equal(t, "john-doe", entry.Fields["slug"][locale])
	assert.Equal(t, "Author bio", entry.Fields["bio"][locale])
}

func TestContentfulExporter_ConvertMediaToAsset(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	locale := "en-US"

	media := models.WordPressMedia{
		ID:        1,
		Title:     models.RenderedContent{Rendered: "Test Image"},
		SourceURL: "https://example.com/path/to/image.jpg",
		MimeType:  "image/jpeg",
		AltText:   "Test alt text",
	}

	asset := exporter.convertMediaToAsset(media, locale)

	assert.Equal(t, "asset-1", asset.Sys.ID)
	assert.Equal(t, "Asset", asset.Sys.Type)

	titleField := asset.Fields["title"]
	assert.Equal(t, "Test Image", titleField[locale])

	descField := asset.Fields["description"]
	assert.Equal(t, "Test alt text", descField[locale])

	fileField := asset.Fields["file"]
	fileMap := fileField[locale].(map[string]interface{})
	assert.Equal(t, "https://example.com/path/to/image.jpg", fileMap["url"])
	assert.Equal(t, "image/jpeg", fileMap["contentType"])
	assert.Equal(t, "image.jpg", fileMap["fileName"])
}

func TestContentfulExporter_BuildContentfulData(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
			Language:    "en-US",
		},
		Posts: []models.WordPressPost{
			{ID: 1, Slug: "post-1", Status: "publish", Title: models.RenderedContent{Rendered: "Post 1"}, Author: 1},
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

	contentfulData := exporter.buildContentfulData(data)

	assert.Len(t, contentfulData.ContentTypes, 5) // blogPost, page, category, tag, author
	assert.Len(t, contentfulData.Entries, 5)      // 1 post + 1 page + 1 category + 1 tag + 1 author
	assert.Len(t, contentfulData.Assets, 1)
	assert.Len(t, contentfulData.Locales, 1)
	assert.Equal(t, "en-US", contentfulData.Locales[0].Code)
}

func TestContentfulExporter_Export(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "contentful-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "contentful",
	}
	exporter := NewContentfulExporter(cfg)

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
				Author:  1,
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

	outputFile := filepath.Join(tempDir, "contentful_export.json")
	assert.FileExists(t, outputFile)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	var contentfulData ContentfulExportData
	err = json.Unmarshal(content, &contentfulData)
	require.NoError(t, err)

	assert.Len(t, contentfulData.ContentTypes, 5)
	assert.Greater(t, len(contentfulData.Entries), 0)
	assert.Len(t, contentfulData.Assets, 1)
}

func TestContentfulExporter_ExportWithJSONExtension(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "contentful-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	outputFile := filepath.Join(tempDir, "custom_contentful.json")
	cfg := &config.Config{
		Output: outputFile,
		Format: "contentful",
	}
	exporter := NewContentfulExporter(cfg)

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

func TestContentfulExporter_EmptyData(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Empty Site",
			URL:  "https://example.com",
		},
	}

	contentfulData := exporter.buildContentfulData(data)

	assert.Len(t, contentfulData.ContentTypes, 5) // Content types are always created
	assert.Len(t, contentfulData.Entries, 0)
	assert.Len(t, contentfulData.Assets, 0)
}

func TestContentfulExporter_CategoryWithParent(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	locale := "en-US"

	category := models.WordPressCategory{
		ID:     2,
		Name:   "Programming",
		Slug:   "programming",
		Parent: 1,
	}

	entry := exporter.convertCategoryToEntry(category, locale)

	// Verify parent link is set
	parentField := entry.Fields["parent"]
	assert.NotNil(t, parentField)
	parentLink := parentField[locale].(ContentfulLink)
	assert.Equal(t, "category-1", parentLink.Sys.ID)
}

func TestContentfulExporter_PostWithCategories(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	locale := "en-US"

	post := models.WordPressPost{
		ID:         1,
		Slug:       "post-with-cats",
		Status:     "publish",
		Title:      models.RenderedContent{Rendered: "Post with Categories"},
		Categories: []int{1, 2, 3},
	}

	entry := exporter.convertPostToEntry(post, "blogPost", locale)

	catField := entry.Fields["categories"]
	assert.NotNil(t, catField)
	catLinks := catField[locale].([]ContentfulLink)
	assert.Len(t, catLinks, 3)
}

// Benchmark tests
func BenchmarkContentfulExporter_BuildContentfulData(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewContentfulExporter(cfg)

	posts := make([]models.WordPressPost, 100)
	for i := 0; i < 100; i++ {
		posts[i] = models.WordPressPost{
			ID:      i + 1,
			Slug:    "test-post",
			Status:  "publish",
			Title:   models.RenderedContent{Rendered: "Test Post"},
			Content: models.RenderedContent{Rendered: "<p>Content</p>"},
			Author:  1,
		}
	}

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test", URL: "https://example.com"},
		Posts: posts,
		Users: []models.WordPressUser{{ID: 1, Name: "User", Slug: "user"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exporter.buildContentfulData(data)
	}
}
