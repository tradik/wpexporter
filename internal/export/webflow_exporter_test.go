package export

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestNewWebflowExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "webflow",
	}

	exporter := NewWebflowExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
}

func TestWebflowExporter_Export(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webflow-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "webflow",
	}
	exporter := NewWebflowExporter(cfg)

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
				Date:    models.WordPressTime{Time: time.Now()},
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
			{ID: 1, SourceURL: "https://example.com/image.jpg"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Check all CSV files exist
	assert.FileExists(t, filepath.Join(tempDir, "webflow_posts.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "webflow_pages.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "webflow_categories.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "webflow_authors.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "webflow_export.json"))

	// Verify posts CSV structure
	postsFile, err := os.Open(filepath.Join(tempDir, "webflow_posts.csv"))
	require.NoError(t, err)
	defer postsFile.Close()

	reader := csv.NewReader(postsFile)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// First row should be header
	assert.Equal(t, "Name", records[0][0])
	assert.Equal(t, "Slug", records[0][1])
	// Second row should be data
	assert.Equal(t, "Test Post", records[1][0])
	assert.Equal(t, "test-post", records[1][1])
}

func TestWebflowExporter_ExportEmptyData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webflow-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "webflow",
	}
	exporter := NewWebflowExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Empty Site",
			URL:  "https://example.com",
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// JSON file should still be created
	assert.FileExists(t, filepath.Join(tempDir, "webflow_export.json"))
}

func TestWebflowExporter_ExportWithCSVExtension(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webflow-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	outputFile := filepath.Join(tempDir, "custom_export.csv")
	cfg := &config.Config{
		Output: outputFile,
		Format: "webflow",
	}
	exporter := NewWebflowExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{ID: 1, Slug: "test", Status: "publish", Title: models.RenderedContent{Rendered: "Test"}},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Should create files in the parent directory
	assert.FileExists(t, filepath.Join(tempDir, "webflow_posts.csv"))
}

func TestWebflowExporter_PostsCSVContent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webflow-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "webflow",
	}
	exporter := NewWebflowExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{
				ID:            1,
				Slug:          "test-post",
				Status:        "publish",
				Title:         models.RenderedContent{Rendered: "Test Post"},
				Content:       models.RenderedContent{Rendered: "<p>Content</p>"},
				Excerpt:       models.RenderedContent{Rendered: "<p>Excerpt</p>"},
				Author:        1,
				Date:          models.WordPressTime{Time: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
				Categories:    []int{1},
				Tags:          []int{1},
				FeaturedMedia: 10,
				SEO: models.SEOData{
					Title:           "SEO Title",
					MetaDescription: "SEO Description",
				},
			},
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Tech", Slug: "tech"},
		},
		Tags: []models.WordPressTag{
			{ID: 1, Name: "Go", Slug: "go"},
		},
		Users: []models.WordPressUser{
			{ID: 1, Name: "John Doe", Slug: "john-doe"},
		},
		Media: []models.WordPressMedia{
			{ID: 10, SourceURL: "https://example.com/featured.jpg"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Read and verify posts CSV
	postsFile, err := os.Open(filepath.Join(tempDir, "webflow_posts.csv"))
	require.NoError(t, err)
	defer postsFile.Close()

	reader := csv.NewReader(postsFile)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, records, 2) // Header + 1 post

	// Check header
	assert.Equal(t, "Name", records[0][0])
	assert.Equal(t, "Slug", records[0][1])

	// Check data
	assert.Equal(t, "Test Post", records[1][0])
	assert.Equal(t, "test-post", records[1][1])
	assert.Equal(t, "John Doe", records[1][5]) // Author
	assert.Equal(t, "Tech", records[1][7])     // Categories
	assert.Equal(t, "Go", records[1][8])       // Tags
}

func TestWebflowExporter_DraftPost(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webflow-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "webflow",
	}
	exporter := NewWebflowExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{
				ID:     1,
				Slug:   "draft-post",
				Status: "draft",
				Title:  models.RenderedContent{Rendered: "Draft Post"},
			},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Read and verify posts CSV
	postsFile, err := os.Open(filepath.Join(tempDir, "webflow_posts.csv"))
	require.NoError(t, err)
	defer postsFile.Close()

	reader := csv.NewReader(postsFile)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// Last column should be "true" for draft
	assert.Equal(t, "true", records[1][11])
}

func TestWebflowExporter_CategoriesCSV(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webflow-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "webflow",
	}
	exporter := NewWebflowExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Tech", Slug: "tech", Description: "Technology articles"},
			{ID: 2, Name: "News", Slug: "news", Description: "Latest news"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Read and verify categories CSV
	catFile, err := os.Open(filepath.Join(tempDir, "webflow_categories.csv"))
	require.NoError(t, err)
	defer catFile.Close()

	reader := csv.NewReader(catFile)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, records, 3) // Header + 2 categories
	assert.Equal(t, "Name", records[0][0])
	assert.Equal(t, "Slug", records[0][1])
	assert.Equal(t, "Description", records[0][2])
	assert.Equal(t, "Tech", records[1][0])
	assert.Equal(t, "tech", records[1][1])
}

func TestWebflowExporter_AuthorsCSV(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webflow-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "webflow",
	}
	exporter := NewWebflowExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Users: []models.WordPressUser{
			{
				ID:          1,
				Name:        "John Doe",
				Slug:        "john-doe",
				Description: "Author bio",
				AvatarURLs:  map[string]string{"96": "https://example.com/avatar.jpg"},
			},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Read and verify authors CSV
	authFile, err := os.Open(filepath.Join(tempDir, "webflow_authors.csv"))
	require.NoError(t, err)
	defer authFile.Close()

	reader := csv.NewReader(authFile)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, records, 2) // Header + 1 author
	assert.Equal(t, "John Doe", records[1][0])
	assert.Equal(t, "john-doe", records[1][1])
	assert.Equal(t, "Author bio", records[1][2])
	assert.Equal(t, "https://example.com/avatar.jpg", records[1][3])
}

func TestWebflowExporter_PagesCSV(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webflow-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "webflow",
	}
	exporter := NewWebflowExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Pages: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "about-us",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "About Us"},
				Content: models.RenderedContent{Rendered: "<p>About page</p>"},
				SEO: models.SEOData{
					Title:           "About - Company",
					MetaDescription: "Learn about us",
				},
			},
			{
				ID:     2,
				Slug:   "draft-page",
				Status: "draft",
				Title:  models.RenderedContent{Rendered: "Draft Page"},
			},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify pages CSV content
	pagesFile, err := os.Open(filepath.Join(tempDir, "webflow_pages.csv"))
	require.NoError(t, err)
	defer pagesFile.Close()

	reader := csv.NewReader(pagesFile)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, records, 3) // Header + 2 pages
	assert.Equal(t, "About Us", records[1][0])
	assert.Equal(t, "false", records[1][5]) // Not draft
	assert.Equal(t, "true", records[2][5])  // Is draft
}

func TestWebflowExporter_ExportWithSEOData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webflow-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "webflow",
	}
	exporter := NewWebflowExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "seo-post",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "SEO Test Post"},
				Content: models.RenderedContent{Rendered: "<p>Content</p>"},
				Author:  1,
				SEO: models.SEOData{
					Title:           "Custom SEO Title",
					MetaDescription: "Custom meta description",
				},
			},
		},
		Users: []models.WordPressUser{
			{ID: 1, Name: "Author Name", Slug: "author-name", Description: "Bio"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Check posts have SEO data
	postsFile, err := os.Open(filepath.Join(tempDir, "webflow_posts.csv"))
	require.NoError(t, err)
	defer postsFile.Close()

	reader := csv.NewReader(postsFile)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, records, 2)
	assert.Equal(t, "Custom SEO Title", records[1][9])
	assert.Equal(t, "Custom meta description", records[1][10])
}

// Benchmark tests
func BenchmarkWebflowExporter_Export(b *testing.B) {
	cfg := &config.Config{}

	posts := make([]models.WordPressPost, 100)
	for i := 0; i < 100; i++ {
		posts[i] = models.WordPressPost{
			ID:      i + 1,
			Slug:    "test-post",
			Status:  "publish",
			Title:   models.RenderedContent{Rendered: "Test Post"},
			Content: models.RenderedContent{Rendered: "<p>Content</p>"},
			Author:  1,
			Date:    models.WordPressTime{Time: time.Now()},
		}
	}

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test", URL: "https://example.com"},
		Posts: posts,
		Users: []models.WordPressUser{{ID: 1, Name: "User", Slug: "user"}},
	}

	tempDir, _ := os.MkdirTemp("", "webflow-bench")
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg.Output = tempDir
	exporter := NewWebflowExporter(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exporter.Export(data)
	}
}
