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

func TestNewStrapiExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "strapi",
	}

	exporter := NewStrapiExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
}

func TestStrapiExporter_ConvertAuthor(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

	user := models.WordPressUser{
		ID:          1,
		Name:        "John Doe",
		Slug:        "john-doe",
		Description: "Author bio",
		AvatarURLs:  map[string]string{"96": "https://example.com/avatar.jpg"},
	}

	strapiAuthor := exporter.convertAuthor(user)

	assert.Equal(t, 1, strapiAuthor.ID)
	assert.Equal(t, "John Doe", strapiAuthor.Name)
	assert.Equal(t, "john-doe", strapiAuthor.Slug)
	assert.Equal(t, "Author bio", strapiAuthor.Bio)
	assert.Equal(t, "https://example.com/avatar.jpg", strapiAuthor.Avatar)
}

func TestStrapiExporter_ConvertAuthorWithoutAvatar(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

	user := models.WordPressUser{
		ID:   1,
		Name: "No Avatar User",
		Slug: "no-avatar",
	}

	strapiAuthor := exporter.convertAuthor(user)

	assert.Equal(t, "", strapiAuthor.Avatar)
}

func TestStrapiExporter_ConvertCategory(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

	category := models.WordPressCategory{
		ID:          1,
		Name:        "Technology",
		Slug:        "technology",
		Description: "Tech posts",
	}

	strapiCategory := exporter.convertCategory(category)

	assert.Equal(t, 1, strapiCategory.ID)
	assert.Equal(t, "Technology", strapiCategory.Name)
	assert.Equal(t, "technology", strapiCategory.Slug)
	assert.Equal(t, "Tech posts", strapiCategory.Description)
}

func TestStrapiExporter_ConvertCategoryWithParent(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

	category := models.WordPressCategory{
		ID:     2,
		Name:   "Programming",
		Slug:   "programming",
		Parent: 1,
	}

	strapiCategory := exporter.convertCategory(category)

	assert.NotNil(t, strapiCategory.ParentID)
	assert.Equal(t, 1, *strapiCategory.ParentID)
}

func TestStrapiExporter_ConvertTag(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

	tag := models.WordPressTag{
		ID:   1,
		Name: "Go",
		Slug: "go",
	}

	strapiTag := exporter.convertTag(tag)

	assert.Equal(t, 1, strapiTag.ID)
	assert.Equal(t, "Go", strapiTag.Name)
	assert.Equal(t, "go", strapiTag.Slug)
}

func TestStrapiExporter_ConvertMedia(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

	tests := []struct {
		name     string
		media    models.WordPressMedia
		expected StrapiMedia
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
			expected: StrapiMedia{
				ID:     1,
				Name:   "Test Image",
				URL:    "https://example.com/image.jpg",
				Mime:   "image/jpeg",
				Width:  800,
				Height: 600,
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
			expected: StrapiMedia{
				ID:     2,
				Name:   "Test Image 2",
				URL:    "https://example.com/image2.jpg",
				Mime:   "image/png",
				Width:  1024,
				Height: 768,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strapiMedia := exporter.convertMedia(tt.media)
			assert.Equal(t, tt.expected.ID, strapiMedia.ID)
			assert.Equal(t, tt.expected.Name, strapiMedia.Name)
			assert.Equal(t, tt.expected.URL, strapiMedia.URL)
			assert.Equal(t, tt.expected.Mime, strapiMedia.Mime)
			assert.Equal(t, tt.expected.Width, strapiMedia.Width)
			assert.Equal(t, tt.expected.Height, strapiMedia.Height)
		})
	}
}

func TestStrapiExporter_ConvertArticle(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

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
			MetaKeywords:    "keyword1, keyword2",
		},
	}

	strapiArticle := exporter.convertArticle(post)

	assert.Equal(t, 1, strapiArticle.ID)
	assert.Equal(t, "Test Article", strapiArticle.Title)
	assert.Equal(t, "test-article", strapiArticle.Slug)
	assert.Equal(t, "<p>Article content</p>", strapiArticle.Content)
	assert.Equal(t, "Article excerpt", strapiArticle.Excerpt)
	assert.Equal(t, "published", strapiArticle.Status)
	assert.NotNil(t, strapiArticle.Author)
	assert.Equal(t, 1, strapiArticle.Author.ID)
	assert.NotNil(t, strapiArticle.FeaturedImage)
	assert.Equal(t, 10, strapiArticle.FeaturedImage.ID)
	assert.Len(t, strapiArticle.Categories, 1)
	assert.Len(t, strapiArticle.Tags, 2)
	assert.NotNil(t, strapiArticle.SEO)
	assert.Equal(t, "SEO Title", strapiArticle.SEO.MetaTitle)
	assert.Equal(t, "SEO Description", strapiArticle.SEO.MetaDescription)
}

func TestStrapiExporter_ConvertArticleDraft(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

	post := models.WordPressPost{
		ID:     1,
		Slug:   "draft-article",
		Status: "draft",
		Title:  models.RenderedContent{Rendered: "Draft Article"},
	}

	strapiArticle := exporter.convertArticle(post)

	assert.Equal(t, "draft", strapiArticle.Status)
	assert.Nil(t, strapiArticle.PublishedAt)
}

func TestStrapiExporter_ConvertPage(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

	page := models.WordPressPost{
		ID:       1,
		Slug:     "about-us",
		Status:   "publish",
		Title:    models.RenderedContent{Rendered: "About Us"},
		Content:  models.RenderedContent{Rendered: "<p>About page content</p>"},
		Modified: models.WordPressTime{Time: time.Now()},
		SEO: models.SEOData{
			Title:           "About - Company",
			MetaDescription: "Learn about our company",
		},
	}

	strapiPage := exporter.convertPage(page)

	assert.Equal(t, 1, strapiPage.ID)
	assert.Equal(t, "About Us", strapiPage.Title)
	assert.Equal(t, "about-us", strapiPage.Slug)
	assert.Equal(t, "<p>About page content</p>", strapiPage.Content)
	assert.Equal(t, "published", strapiPage.Status)
	assert.NotNil(t, strapiPage.SEO)
	assert.Equal(t, "About - Company", strapiPage.SEO.MetaTitle)
}

func TestStrapiExporter_BuildStrapiData(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
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

	strapiData := exporter.buildStrapiData(data)

	assert.Equal(t, "wpexporter", strapiData.Meta.Exporter)
	assert.Equal(t, "1.4.0", strapiData.Version)
	assert.Equal(t, "https://example.com", strapiData.Meta.SourceSite)

	assert.Len(t, strapiData.Articles, 1)
	assert.Len(t, strapiData.Pages, 1)
	assert.Len(t, strapiData.Categories, 1)
	assert.Len(t, strapiData.Tags, 1)
	assert.Len(t, strapiData.Authors, 1)
	assert.Len(t, strapiData.Media, 1)

	// Check counts
	assert.Equal(t, 1, strapiData.Meta.Counts.Articles)
	assert.Equal(t, 1, strapiData.Meta.Counts.Pages)
}

func TestStrapiExporter_Export(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "strapi-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "strapi",
	}
	exporter := NewStrapiExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
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

	// Check main export file
	mainFile := filepath.Join(tempDir, "strapi_export.json")
	assert.FileExists(t, mainFile)

	// Check separate files
	assert.FileExists(t, filepath.Join(tempDir, "strapi_articles.json"))
	assert.FileExists(t, filepath.Join(tempDir, "strapi_pages.json"))
	assert.FileExists(t, filepath.Join(tempDir, "strapi_categories.json"))
	assert.FileExists(t, filepath.Join(tempDir, "strapi_tags.json"))
	assert.FileExists(t, filepath.Join(tempDir, "strapi_authors.json"))
	assert.FileExists(t, filepath.Join(tempDir, "strapi_media.json"))

	// Verify main JSON structure
	content, err := os.ReadFile(mainFile)
	require.NoError(t, err)

	var strapiData StrapiExportData
	err = json.Unmarshal(content, &strapiData)
	require.NoError(t, err)

	assert.Equal(t, "wpexporter", strapiData.Meta.Exporter)
	assert.Len(t, strapiData.Articles, 1)
	assert.Len(t, strapiData.Pages, 1)
}

func TestStrapiExporter_ExportWithJSONExtension(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "strapi-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	outputFile := filepath.Join(tempDir, "custom_strapi.json")
	cfg := &config.Config{
		Output: outputFile,
		Format: "strapi",
	}
	exporter := NewStrapiExporter(cfg)

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

func TestStrapiExporter_EmptyData(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Empty Site",
			URL:  "https://example.com",
		},
	}

	strapiData := exporter.buildStrapiData(data)

	assert.Len(t, strapiData.Articles, 0)
	assert.Len(t, strapiData.Pages, 0)
	assert.Len(t, strapiData.Categories, 0)
	assert.Len(t, strapiData.Tags, 0)
	assert.Len(t, strapiData.Authors, 0)
	assert.Len(t, strapiData.Media, 0)
}

func TestStrapiExporter_ExportSeparateFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "strapi-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "strapi",
	}
	exporter := NewStrapiExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
		},
		Posts: []models.WordPressPost{
			{ID: 1, Slug: "post-1", Status: "publish", Title: models.RenderedContent{Rendered: "Post 1"}, Author: 1},
			{ID: 2, Slug: "post-2", Status: "publish", Title: models.RenderedContent{Rendered: "Post 2"}, Author: 1},
		},
		Pages: []models.WordPressPost{
			{ID: 1, Slug: "page-1", Status: "publish", Title: models.RenderedContent{Rendered: "Page 1"}},
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Cat 1", Slug: "cat-1"},
			{ID: 2, Name: "Cat 2", Slug: "cat-2"},
		},
		Tags: []models.WordPressTag{
			{ID: 1, Name: "Tag 1", Slug: "tag-1"},
		},
		Users: []models.WordPressUser{
			{ID: 1, Name: "User 1", Slug: "user-1"},
		},
		Media: []models.WordPressMedia{
			{ID: 1, Title: models.RenderedContent{Rendered: "Media 1"}, SourceURL: "https://example.com/m.jpg", MimeType: "image/jpeg"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify all separate files are created
	assert.FileExists(t, filepath.Join(tempDir, "strapi_articles.json"))
	assert.FileExists(t, filepath.Join(tempDir, "strapi_pages.json"))
	assert.FileExists(t, filepath.Join(tempDir, "strapi_categories.json"))
	assert.FileExists(t, filepath.Join(tempDir, "strapi_tags.json"))
	assert.FileExists(t, filepath.Join(tempDir, "strapi_authors.json"))
	assert.FileExists(t, filepath.Join(tempDir, "strapi_media.json"))

	// Verify content of separate files
	articlesJSON, err := os.ReadFile(filepath.Join(tempDir, "strapi_articles.json"))
	require.NoError(t, err)
	var articles []StrapiArticle
	err = json.Unmarshal(articlesJSON, &articles)
	require.NoError(t, err)
	assert.Len(t, articles, 2)

	categoriesJSON, err := os.ReadFile(filepath.Join(tempDir, "strapi_categories.json"))
	require.NoError(t, err)
	var categories []StrapiCategory
	err = json.Unmarshal(categoriesJSON, &categories)
	require.NoError(t, err)
	assert.Len(t, categories, 2)
}

func TestStrapiExporter_ConvertArticleWithAllFields(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

	post := models.WordPressPost{
		ID:            1,
		Slug:          "full-post",
		Status:        "publish",
		Title:         models.RenderedContent{Rendered: "Full Post"},
		Content:       models.RenderedContent{Rendered: "<p>Full content</p>"},
		Excerpt:       models.RenderedContent{Rendered: "<p>Excerpt</p>"},
		Author:        1,
		Date:          models.WordPressTime{Time: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		Modified:      models.WordPressTime{Time: time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)},
		Categories:    []int{1, 2, 3},
		Tags:          []int{1, 2},
		FeaturedMedia: 10,
		SEO: models.SEOData{
			Title:           "SEO Title",
			MetaDescription: "SEO Description",
			MetaKeywords:    "key1, key2",
		},
	}

	article := exporter.convertArticle(post)

	assert.Equal(t, 1, article.ID)
	assert.Equal(t, "Full Post", article.Title)
	assert.Equal(t, "full-post", article.Slug)
	assert.Equal(t, "<p>Full content</p>", article.Content)
	assert.Equal(t, "Excerpt", article.Excerpt)
	assert.Equal(t, "published", article.Status)
	assert.NotNil(t, article.Author)
	assert.NotNil(t, article.FeaturedImage)
	assert.Len(t, article.Categories, 3)
	assert.Len(t, article.Tags, 2)
	assert.NotNil(t, article.SEO)
	assert.Equal(t, "SEO Title", article.SEO.MetaTitle)
}

// Benchmark tests
func BenchmarkStrapiExporter_BuildStrapiData(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewStrapiExporter(cfg)

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
		_ = exporter.buildStrapiData(data)
	}
}
