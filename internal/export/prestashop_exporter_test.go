package export

import (
	"encoding/csv"
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

func TestNewPrestaShopExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "prestashop",
	}

	exporter := NewPrestaShopExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
}

func TestPrestaShopExporter_ConvertPostToProduct(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewPrestaShopExporter(cfg)

	categoryMap := map[int]string{1: "Electronics", 2: "Gadgets"}
	tagMap := map[int]string{1: "New", 2: "Sale"}
	mediaMap := map[int]string{10: "https://example.com/product.jpg"}

	post := models.WordPressPost{
		ID:            1,
		Slug:          "test-product",
		Status:        "publish",
		Title:         models.RenderedContent{Rendered: "Test Product"},
		Content:       models.RenderedContent{Rendered: "<p>Product description</p>"},
		Excerpt:       models.RenderedContent{Rendered: "<p>Short description</p>"},
		Categories:    []int{1, 2},
		Tags:          []int{1, 2},
		FeaturedMedia: 10,
		SEO: models.SEOData{
			Title:           "SEO Title",
			MetaKeywords:    "keyword1, keyword2",
			MetaDescription: "SEO Description",
		},
	}

	product := exporter.convertPostToProduct(post, categoryMap, tagMap, mediaMap)

	assert.Equal(t, 1, product.ID)
	assert.Equal(t, 1, product.Active)
	assert.Equal(t, "Test Product", product.Name)
	assert.Contains(t, product.Categories, "Electronics")
	assert.Contains(t, product.Categories, "Gadgets")
	assert.Equal(t, "0.00", product.Price)
	assert.Equal(t, 999, product.Quantity)
	assert.Equal(t, 1, product.MinimalQuantity)
	assert.Equal(t, "WP-1", product.Reference)
	assert.Equal(t, "<p>Product description</p>", product.Description)
	assert.Equal(t, "Short description", product.DescriptionShort)
	assert.Contains(t, product.Tags, "New")
	assert.Contains(t, product.Tags, "Sale")
	assert.Equal(t, "SEO Title", product.MetaTitle)
	assert.Equal(t, "keyword1, keyword2", product.MetaKeywords)
	assert.Equal(t, "SEO Description", product.MetaDescription)
	assert.Equal(t, "test-product", product.LinkRewrite)
	assert.Equal(t, "https://example.com/product.jpg", product.ImageURLs)
}

func TestPrestaShopExporter_ConvertPostToProductDraft(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewPrestaShopExporter(cfg)

	post := models.WordPressPost{
		ID:     1,
		Slug:   "draft-product",
		Status: "draft",
		Title:  models.RenderedContent{Rendered: "Draft Product"},
	}

	product := exporter.convertPostToProduct(post, nil, nil, nil)

	assert.Equal(t, 0, product.Active)
}

func TestPrestaShopExporter_ConvertPostToProductDefaultCategory(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewPrestaShopExporter(cfg)

	post := models.WordPressPost{
		ID:         1,
		Slug:       "no-category",
		Status:     "publish",
		Title:      models.RenderedContent{Rendered: "No Category Product"},
		Categories: []int{}, // Empty categories
	}

	product := exporter.convertPostToProduct(post, nil, nil, nil)

	assert.Equal(t, "Default", product.Categories)
}

func TestPrestaShopExporter_Export(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prestashop-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "prestashop",
	}
	exporter := NewPrestaShopExporter(cfg)

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

	// Check all files exist
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_products.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_posts.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_pages.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_categories.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_metadata.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_export.json"))

	// Verify products CSV structure (semicolon delimited)
	productsFile, err := os.Open(filepath.Join(tempDir, "prestashop_products.csv"))
	require.NoError(t, err)
	defer productsFile.Close()

	reader := csv.NewReader(productsFile)
	reader.Comma = ';'
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// First row should be header
	assert.Equal(t, "ID", records[0][0])
	assert.Equal(t, "Active", records[0][1])
	assert.Equal(t, "Name", records[0][2])

	// Verify JSON export
	jsonFile := filepath.Join(tempDir, "prestashop_export.json")
	content, err := os.ReadFile(jsonFile)
	require.NoError(t, err)

	var products []PrestaShopProduct
	err = json.Unmarshal(content, &products)
	require.NoError(t, err)

	assert.Greater(t, len(products), 0)
}

func TestPrestaShopExporter_ExportCategories(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prestashop-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "prestashop",
	}
	exporter := NewPrestaShopExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Root Category", Slug: "root-category", Parent: 0, Description: "Root"},
			{ID: 2, Name: "Child Category", Slug: "child-category", Parent: 1, Description: "Child"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify categories CSV
	catFile, err := os.Open(filepath.Join(tempDir, "prestashop_categories.csv"))
	require.NoError(t, err)
	defer catFile.Close()

	reader := csv.NewReader(catFile)
	reader.Comma = ';'
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// Header + 2 categories
	assert.Len(t, records, 3)

	// First category (root)
	assert.Equal(t, "1", records[1][0]) // ID
	assert.Equal(t, "1", records[1][1]) // Active
	assert.Equal(t, "", records[1][3])  // Parent (empty for root)
	assert.Equal(t, "1", records[1][4]) // Root category

	// Second category (child)
	assert.Equal(t, "2", records[2][0]) // ID
	assert.Equal(t, "1", records[2][3]) // Parent ID
	assert.Equal(t, "0", records[2][4]) // Not root
}

func TestPrestaShopExporter_ExportMetadata(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prestashop-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "prestashop",
	}
	exporter := NewPrestaShopExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts:      make([]models.WordPressPost, 5),
		Pages:      make([]models.WordPressPost, 3),
		Categories: make([]models.WordPressCategory, 2),
		Tags:       make([]models.WordPressTag, 4),
		Media:      make([]models.WordPressMedia, 10),
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify metadata CSV
	metaFile, err := os.Open(filepath.Join(tempDir, "prestashop_metadata.csv"))
	require.NoError(t, err)
	defer metaFile.Close()

	reader := csv.NewReader(metaFile)
	reader.Comma = ';'
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// Find total products row
	var totalProducts string
	for _, record := range records {
		if record[0] == "Total Products" {
			totalProducts = record[1]
			break
		}
	}
	assert.Equal(t, "8", totalProducts) // 5 posts + 3 pages
}

func TestPrestaShopExporter_ExportWithCSVExtension(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prestashop-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	outputFile := filepath.Join(tempDir, "custom_export.csv")
	cfg := &config.Config{
		Output: outputFile,
		Format: "prestashop",
	}
	exporter := NewPrestaShopExporter(cfg)

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
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_products.csv"))
}

func TestPrestaShopExporter_ExportEmptyData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prestashop-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "prestashop",
	}
	exporter := NewPrestaShopExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Empty Site",
			URL:  "https://example.com",
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Metadata file should still be created
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_metadata.csv"))
}

func TestPrestaShopExporter_CSVDelimiter(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prestashop-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "prestashop",
	}
	exporter := NewPrestaShopExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "test-product",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "Test Product"},
				Content: models.RenderedContent{Rendered: "Description with; semicolon"},
			},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Read raw file to verify semicolon delimiter
	content, err := os.ReadFile(filepath.Join(tempDir, "prestashop_products.csv"))
	require.NoError(t, err)

	// Should contain semicolons as delimiters
	assert.Contains(t, string(content), ";")
}

func TestPrestaShopExporter_ProductReference(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewPrestaShopExporter(cfg)

	tests := []struct {
		postID      int
		expectedRef string
	}{
		{1, "WP-1"},
		{100, "WP-100"},
		{12345, "WP-12345"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedRef, func(t *testing.T) {
			post := models.WordPressPost{
				ID:     tt.postID,
				Status: "publish",
			}

			product := exporter.convertPostToProduct(post, nil, nil, nil)
			assert.Equal(t, tt.expectedRef, product.Reference)
		})
	}
}

func TestPrestaShopExporter_JSONPageOffset(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prestashop-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "prestashop",
	}
	exporter := NewPrestaShopExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{ID: 1, Slug: "post", Status: "publish", Title: models.RenderedContent{Rendered: "Post"}},
		},
		Pages: []models.WordPressPost{
			{ID: 1, Slug: "page", Status: "publish", Title: models.RenderedContent{Rendered: "Page"}},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify JSON export has offset for pages
	content, err := os.ReadFile(filepath.Join(tempDir, "prestashop_export.json"))
	require.NoError(t, err)

	var products []PrestaShopProduct
	err = json.Unmarshal(content, &products)
	require.NoError(t, err)

	// Find page product (should have ID > 1000000)
	var pageID int
	for _, p := range products {
		if p.Name == "Page" {
			pageID = p.ID
			break
		}
	}
	assert.Equal(t, 1000001, pageID)
}

func TestPrestaShopExporter_ExportPostsAndPagesCSV(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prestashop-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "prestashop",
	}
	exporter := NewPrestaShopExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "post-1",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "Post 1"},
				Content: models.RenderedContent{Rendered: "<p>Content</p>"},
				Excerpt: models.RenderedContent{Rendered: "<p>Excerpt</p>"},
				Date:    models.WordPressTime{Time: time.Now()},
			},
			{
				ID:     2,
				Slug:   "draft-post",
				Status: "draft",
				Title:  models.RenderedContent{Rendered: "Draft Post"},
			},
		},
		Pages: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "page-1",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "Page 1"},
				Content: models.RenderedContent{Rendered: "<p>Page content</p>"},
			},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify posts CSV
	postsFile, err := os.Open(filepath.Join(tempDir, "prestashop_posts.csv"))
	require.NoError(t, err)
	defer postsFile.Close()

	reader := csv.NewReader(postsFile)
	reader.Comma = ';'
	records, err := reader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, records, 3) // Header + 2 posts

	// Verify pages CSV
	pagesFile, err := os.Open(filepath.Join(tempDir, "prestashop_pages.csv"))
	require.NoError(t, err)
	defer pagesFile.Close()

	pageReader := csv.NewReader(pagesFile)
	pageReader.Comma = ';'
	pageRecords, err := pageReader.ReadAll()
	require.NoError(t, err)

	assert.Len(t, pageRecords, 2) // Header + 1 page
}

func TestPrestaShopExporter_FullExport(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "prestashop-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "prestashop",
	}
	exporter := NewPrestaShopExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Site description",
			URL:         "https://example.com",
		},
		Posts: []models.WordPressPost{
			{
				ID:            1,
				Slug:          "product-post",
				Status:        "publish",
				Title:         models.RenderedContent{Rendered: "Product"},
				Content:       models.RenderedContent{Rendered: "<p>Description</p>"},
				Categories:    []int{1},
				Tags:          []int{1},
				FeaturedMedia: 1,
				SEO: models.SEOData{
					Title:           "SEO Title",
					MetaKeywords:    "keywords",
					MetaDescription: "SEO desc",
				},
			},
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Category", Slug: "category", Description: "Cat desc"},
			{ID: 2, Name: "Child", Slug: "child", Parent: 1},
		},
		Tags: []models.WordPressTag{
			{ID: 1, Name: "Tag", Slug: "tag"},
		},
		Media: []models.WordPressMedia{
			{ID: 1, SourceURL: "https://example.com/image.jpg"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify all files exist
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_products.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_categories.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_metadata.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_export.json"))
}

// Benchmark tests
func BenchmarkPrestaShopExporter_ConvertPostToProduct(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewPrestaShopExporter(cfg)

	categoryMap := map[int]string{1: "Category"}
	tagMap := map[int]string{1: "Tag"}
	mediaMap := map[int]string{1: "https://example.com/image.jpg"}

	post := models.WordPressPost{
		ID:            1,
		Slug:          "test-product",
		Status:        "publish",
		Title:         models.RenderedContent{Rendered: "Test Product"},
		Content:       models.RenderedContent{Rendered: "<p>Description</p>"},
		Categories:    []int{1},
		Tags:          []int{1},
		FeaturedMedia: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exporter.convertPostToProduct(post, categoryMap, tagMap, mediaMap)
	}
}

func BenchmarkPrestaShopExporter_Export(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewPrestaShopExporter(cfg)

	posts := make([]models.WordPressPost, 100)
	for i := 0; i < 100; i++ {
		posts[i] = models.WordPressPost{
			ID:      i + 1,
			Slug:    "test-product",
			Status:  "publish",
			Title:   models.RenderedContent{Rendered: "Test Product"},
			Content: models.RenderedContent{Rendered: "<p>Description</p>"},
			Date:    models.WordPressTime{Time: time.Now()},
		}
	}

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test", URL: "https://example.com"},
		Posts: posts,
	}

	tempDir, _ := os.MkdirTemp("", "prestashop-bench")
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg.Output = tempDir

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exporter.Export(data)
	}
}
