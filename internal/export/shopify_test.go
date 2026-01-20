package export

import (
	"encoding/csv"
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

func TestNewShopifyExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "shopify",
	}

	exporter := NewShopifyExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
	assert.NotNil(t, exporter.categoryMap)
	assert.NotNil(t, exporter.tagMap)
	assert.NotNil(t, exporter.userMap)
	assert.NotNil(t, exporter.mediaMap)
}

func TestShopifyExporter_GenerateHandle(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	tests := []struct {
		name     string
		slug     string
		id       int
		expected string
	}{
		{
			name:     "Simple slug",
			slug:     "my-post-title",
			id:       1,
			expected: "my-post-title",
		},
		{
			name:     "Empty slug",
			slug:     "",
			id:       123,
			expected: "product-123",
		},
		{
			name:     "Slug with uppercase",
			slug:     "My-Post-Title",
			id:       1,
			expected: "my-post-title",
		},
		{
			name:     "Slug with special characters",
			slug:     "my_post_title!@#",
			id:       1,
			expected: "my-post-title",
		},
		{
			name:     "Slug with consecutive hyphens",
			slug:     "my--post--title",
			id:       1,
			expected: "my-post-title",
		},
		{
			name:     "Slug with leading/trailing hyphens",
			slug:     "-my-post-title-",
			id:       1,
			expected: "my-post-title",
		},
		{
			name:     "Slug with Polish characters",
			slug:     "mój-post-tytuł",
			id:       1,
			expected: "m-j-post-tytu",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.generateHandle(tt.slug, tt.id)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShopifyExporter_GetVendorFromAuthor(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	// Populate user map
	exporter.userMap = map[int]models.WordPressUser{
		1: {ID: 1, Name: "John Doe", Slug: "john-doe"},
		2: {ID: 2, Name: "", Slug: "jane-doe"},
	}

	tests := []struct {
		name     string
		authorID int
		expected string
	}{
		{
			name:     "Existing author with name",
			authorID: 1,
			expected: "John Doe",
		},
		{
			name:     "Existing author without name",
			authorID: 2,
			expected: "jane-doe",
		},
		{
			name:     "Non-existing author",
			authorID: 99,
			expected: "WordPress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.getVendorFromAuthor(tt.authorID)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShopifyExporter_GetProductType(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	// Populate category map
	exporter.categoryMap = map[int]models.WordPressCategory{
		1: {ID: 1, Name: "Technology", Slug: "technology"},
		2: {ID: 2, Name: "Travel", Slug: "travel"},
	}

	tests := []struct {
		name        string
		categoryIDs []int
		expected    string
	}{
		{
			name:        "Single category",
			categoryIDs: []int{1},
			expected:    "Technology",
		},
		{
			name:        "Multiple categories (uses first)",
			categoryIDs: []int{2, 1},
			expected:    "Travel",
		},
		{
			name:        "Empty categories",
			categoryIDs: []int{},
			expected:    "Content",
		},
		{
			name:        "Non-existing category",
			categoryIDs: []int{99},
			expected:    "Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.getProductType(tt.categoryIDs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShopifyExporter_GetTagsString(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	// Populate tag map
	exporter.tagMap = map[int]models.WordPressTag{
		1: {ID: 1, Name: "Go", Slug: "go"},
		2: {ID: 2, Name: "Programming", Slug: "programming"},
		3: {ID: 3, Name: "Web Development", Slug: "web-development"},
	}

	tests := []struct {
		name     string
		tagIDs   []int
		expected string
	}{
		{
			name:     "Single tag",
			tagIDs:   []int{1},
			expected: "Go",
		},
		{
			name:     "Multiple tags",
			tagIDs:   []int{1, 2, 3},
			expected: "Go, Programming, Web Development",
		},
		{
			name:     "Empty tags",
			tagIDs:   []int{},
			expected: "",
		},
		{
			name:     "Non-existing tags",
			tagIDs:   []int{99},
			expected: "",
		},
		{
			name:     "Mixed existing and non-existing tags",
			tagIDs:   []int{1, 99, 2},
			expected: "Go, Programming",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.getTagsString(tt.tagIDs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShopifyExporter_IsPublished(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{
			name:     "Publish status",
			status:   "publish",
			expected: "TRUE",
		},
		{
			name:     "Draft status",
			status:   "draft",
			expected: "FALSE",
		},
		{
			name:     "Pending status",
			status:   "pending",
			expected: "FALSE",
		},
		{
			name:     "Private status",
			status:   "private",
			expected: "FALSE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.isPublished(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShopifyExporter_GetStatus(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	tests := []struct {
		name     string
		wpStatus string
		expected string
	}{
		{
			name:     "Publish to active",
			wpStatus: "publish",
			expected: "active",
		},
		{
			name:     "Draft remains draft",
			wpStatus: "draft",
			expected: "draft",
		},
		{
			name:     "Pending to draft",
			wpStatus: "pending",
			expected: "draft",
		},
		{
			name:     "Private to draft",
			wpStatus: "private",
			expected: "draft",
		},
		{
			name:     "Unknown to draft",
			wpStatus: "unknown",
			expected: "draft",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.getStatus(tt.wpStatus)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShopifyExporter_CleanHTMLForShopify(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "Empty string",
			html:     "",
			expected: "",
		},
		{
			name:     "Clean HTML",
			html:     "<p>Hello, World!</p>",
			expected: "<p>Hello, World!</p>",
		},
		{
			name:     "WordPress shortcodes",
			html:     "<p>Hello [gallery ids=\"1,2,3\"] World</p>",
			expected: "<p>Hello  World</p>",
		},
		{
			name:     "WordPress block comments",
			html:     "<!-- wp:paragraph --><p>Content</p><!-- /wp:paragraph -->",
			expected: "<p>Content</p>",
		},
		{
			name:     "Empty paragraphs",
			html:     "<p>Hello</p><p>   </p><p>World</p>",
			expected: "<p>Hello</p><p>World</p>",
		},
		{
			name:     "Multiple newlines",
			html:     "<p>Hello</p>\n\n\n\n<p>World</p>",
			expected: "<p>Hello</p>\n\n<p>World</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.cleanHTMLForShopify(tt.html)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShopifyExporter_ExtractImagesFromContent(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	tests := []struct {
		name     string
		html     string
		expected []string
	}{
		{
			name:     "Empty string",
			html:     "",
			expected: nil,
		},
		{
			name:     "No images",
			html:     "<p>Hello World</p>",
			expected: nil,
		},
		{
			name:     "Single image",
			html:     "<p><img src=\"https://example.com/image.jpg\" alt=\"Test\"></p>",
			expected: []string{"https://example.com/image.jpg"},
		},
		{
			name:     "Multiple images",
			html:     "<img src=\"https://example.com/img1.jpg\"><img src=\"https://example.com/img2.png\">",
			expected: []string{"https://example.com/img1.jpg", "https://example.com/img2.png"},
		},
		{
			name:     "Duplicate images",
			html:     "<img src=\"https://example.com/img.jpg\"><img src=\"https://example.com/img.jpg\">",
			expected: []string{"https://example.com/img.jpg"},
		},
		{
			name:     "Image with single quotes",
			html:     "<img src='https://example.com/image.jpg'>",
			expected: []string{"https://example.com/image.jpg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.extractImagesFromContent(tt.html)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShopifyExporter_TruncateString(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	tests := []struct {
		name     string
		str      string
		maxLen   int
		expected string
	}{
		{
			name:     "Short string",
			str:      "Hello",
			maxLen:   10,
			expected: "Hello",
		},
		{
			name:     "Exact length",
			str:      "Hello",
			maxLen:   5,
			expected: "Hello",
		},
		{
			name:     "Long string with truncation",
			str:      "This is a very long string that needs to be truncated",
			maxLen:   25,
			expected: "This is a very long...",
		},
		{
			name:     "String with HTML stripped",
			str:      "<p>Hello <strong>World</strong></p>",
			maxLen:   20,
			expected: "Hello World",
		},
		{
			name:     "String with HTML entities",
			str:      "&amp; &lt; &gt; &quot;",
			maxLen:   15,
			expected: "& < > \"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.truncateString(tt.str, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShopifyExporter_GetFeaturedImage(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	// Populate media map
	exporter.mediaMap = map[int]models.WordPressMedia{
		1: {
			ID:        1,
			SourceURL: "https://example.com/image1.jpg",
			AltText:   "Image 1 Alt",
			Title:     models.RenderedContent{Rendered: "Image 1 Title"},
		},
		2: {
			ID:        2,
			SourceURL: "https://example.com/image2.jpg",
			AltText:   "",
			Title:     models.RenderedContent{Rendered: "Image 2 Title"},
		},
	}

	tests := []struct {
		name        string
		mediaID     int
		expectedURL string
		expectedAlt string
	}{
		{
			name:        "Existing media with alt text",
			mediaID:     1,
			expectedURL: "https://example.com/image1.jpg",
			expectedAlt: "Image 1 Alt",
		},
		{
			name:        "Existing media without alt text (uses title)",
			mediaID:     2,
			expectedURL: "https://example.com/image2.jpg",
			expectedAlt: "Image 2 Title",
		},
		{
			name:        "Non-existing media",
			mediaID:     99,
			expectedURL: "",
			expectedAlt: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, alt := exporter.getFeaturedImage(tt.mediaID)
			assert.Equal(t, tt.expectedURL, url)
			assert.Equal(t, tt.expectedAlt, alt)
		})
	}
}

func TestShopifyExporter_ConvertPostToShopifyProduct(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	// Populate lookup maps
	exporter.userMap = map[int]models.WordPressUser{
		1: {ID: 1, Name: "Test Author", Slug: "test-author"},
	}
	exporter.categoryMap = map[int]models.WordPressCategory{
		1: {ID: 1, Name: "Technology", Slug: "technology"},
	}
	exporter.tagMap = map[int]models.WordPressTag{
		1: {ID: 1, Name: "Go", Slug: "go"},
		2: {ID: 2, Name: "Programming", Slug: "programming"},
	}
	exporter.mediaMap = map[int]models.WordPressMedia{
		1: {
			ID:        1,
			SourceURL: "https://example.com/featured.jpg",
			AltText:   "Featured Image",
		},
	}

	post := models.WordPressPost{
		ID:            1,
		Slug:          "test-post",
		Status:        "publish",
		Title:         models.RenderedContent{Rendered: "Test Post Title"},
		Content:       models.RenderedContent{Rendered: "<p>Test content</p>"},
		Excerpt:       models.RenderedContent{Rendered: "<p>Test excerpt</p>"},
		Author:        1,
		Categories:    []int{1},
		Tags:          []int{1, 2},
		FeaturedMedia: 1,
	}

	product := exporter.convertPostToShopifyProduct(post)

	assert.Equal(t, "test-post", product.Handle)
	assert.Equal(t, "Test Post Title", product.Title)
	assert.Equal(t, "<p>Test content</p>", product.BodyHTML)
	assert.Equal(t, "Test Author", product.Vendor)
	assert.Equal(t, "Technology", product.Type)
	assert.Equal(t, "Go, Programming", product.Tags)
	assert.Equal(t, "TRUE", product.Published)
	assert.Equal(t, "WP-1", product.VariantSKU)
	assert.Equal(t, "https://example.com/featured.jpg", product.ImageSrc)
	assert.Equal(t, "Featured Image", product.ImageAltText)
	assert.Equal(t, "active", product.Status)
	assert.Equal(t, "Test Post Title", product.SEOTitle)
}

func TestShopifyExporter_GetCSVHeaders(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	headers := exporter.getCSVHeaders()

	// Verify essential headers are present
	assert.Contains(t, headers, "Handle")
	assert.Contains(t, headers, "Title")
	assert.Contains(t, headers, "Body (HTML)")
	assert.Contains(t, headers, "Vendor")
	assert.Contains(t, headers, "Type")
	assert.Contains(t, headers, "Tags")
	assert.Contains(t, headers, "Published")
	assert.Contains(t, headers, "Variant SKU")
	assert.Contains(t, headers, "Variant Price")
	assert.Contains(t, headers, "Image Src")
	assert.Contains(t, headers, "SEO Title")
	assert.Contains(t, headers, "SEO Description")
	assert.Contains(t, headers, "Status")

	// Verify header count matches expected Shopify format (46 columns)
	assert.Equal(t, 46, len(headers))
}

func TestShopifyExporter_ProductToCSVRow(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	product := ShopifyProduct{
		Handle:     "test-handle",
		Title:      "Test Title",
		BodyHTML:   "<p>Body content</p>",
		Vendor:     "Test Vendor",
		Type:       "Test Type",
		Tags:       "tag1, tag2",
		Published:  "TRUE",
		VariantSKU: "SKU-123",
		ImageSrc:   "https://example.com/image.jpg",
		Status:     "active",
	}

	row := exporter.productToCSVRow(product)

	// Verify row length matches header length
	assert.Equal(t, len(exporter.getCSVHeaders()), len(row))

	// Verify key fields
	assert.Equal(t, "test-handle", row[0])         // Handle
	assert.Equal(t, "Test Title", row[1])          // Title
	assert.Equal(t, "<p>Body content</p>", row[2]) // Body (HTML)
	assert.Equal(t, "Test Vendor", row[3])         // Vendor
	assert.Equal(t, "Test Type", row[4])           // Type
	assert.Equal(t, "tag1, tag2", row[5])          // Tags
	assert.Equal(t, "TRUE", row[6])                // Published
}

func TestShopifyExporter_CreateImageRow(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	row := exporter.createImageRow("test-handle", "https://example.com/image.jpg", 2)

	// Verify row length matches header length
	assert.Equal(t, len(exporter.getCSVHeaders()), len(row))

	// Verify handle and image fields
	assert.Equal(t, "test-handle", row[0])                    // Handle
	assert.Equal(t, "https://example.com/image.jpg", row[24]) // Image Src
	assert.Equal(t, "2", row[25])                             // Image Position
}

func TestShopifyExporter_Export(t *testing.T) {
	// Create temporary directory for test output
	tempDir, err := os.MkdirTemp("", "shopify-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "shopify",
		URL:    "https://example.com",
	}
	exporter := NewShopifyExporter(cfg)

	// Create test data
	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
			HomeURL:     "https://example.com",
			Language:    "en",
			Timezone:    "UTC",
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
			{ID: 1, SourceURL: "https://example.com/image.jpg", AltText: "Test Image"},
		},
		Stats: models.ExportStats{
			TotalPosts:      1,
			TotalPages:      1,
			TotalMedia:      1,
			TotalCategories: 1,
			TotalTags:       1,
		},
	}

	// Execute export
	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify output files exist
	postsCSV := filepath.Join(tempDir, "shopify_posts.csv")
	pagesCSV := filepath.Join(tempDir, "shopify_pages.csv")
	productsCSV := filepath.Join(tempDir, "shopify_products.csv")

	assert.FileExists(t, postsCSV)
	assert.FileExists(t, pagesCSV)
	assert.FileExists(t, productsCSV)

	// Verify posts CSV content
	file, err := os.Open(postsCSV)
	require.NoError(t, err)
	defer func() {
		_ = file.Close()
	}()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// Verify header row
	assert.Equal(t, "Handle", records[0][0])
	assert.Equal(t, "Title", records[0][1])

	// Verify data row
	assert.Equal(t, "test-post", records[1][0])
	assert.Equal(t, "Test Post", records[1][1])
}

func TestShopifyExporter_ExportMetadata(t *testing.T) {
	// Create temporary directory for test output
	tempDir, err := os.MkdirTemp("", "shopify-metadata-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "shopify",
	}
	exporter := NewShopifyExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
			HomeURL:     "https://example.com",
			Language:    "en",
			Timezone:    "UTC",
		},
		Stats: models.ExportStats{
			TotalPosts:      10,
			TotalPages:      5,
			TotalMedia:      20,
			TotalCategories: 3,
			TotalTags:       15,
		},
	}

	err = exporter.ExportMetadata(testData)
	require.NoError(t, err)

	// Verify metadata file exists
	metadataCSV := filepath.Join(tempDir, "shopify_metadata.csv")
	assert.FileExists(t, metadataCSV)

	// Read and verify content
	file, err := os.Open(metadataCSV)
	require.NoError(t, err)
	defer func() {
		_ = file.Close()
	}()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	require.NoError(t, err)

	// Verify header
	assert.Equal(t, "Field", records[0][0])
	assert.Equal(t, "Value", records[0][1])

	// Verify some data rows
	assert.Equal(t, "Site Name", records[1][0])
	assert.Equal(t, "Test Site", records[1][1])

	assert.Equal(t, "Total Posts", records[7][0])
	assert.Equal(t, "10", records[7][1])
}

func TestShopifyExporter_BuildLookupMaps(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	testData := &models.ExportData{
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Category 1"},
			{ID: 2, Name: "Category 2"},
		},
		Tags: []models.WordPressTag{
			{ID: 1, Name: "Tag 1"},
			{ID: 2, Name: "Tag 2"},
		},
		Users: []models.WordPressUser{
			{ID: 1, Name: "User 1"},
			{ID: 2, Name: "User 2"},
		},
		Media: []models.WordPressMedia{
			{ID: 1, SourceURL: "https://example.com/img1.jpg"},
			{ID: 2, SourceURL: "https://example.com/img2.jpg"},
		},
	}

	exporter.buildLookupMaps(testData)

	// Verify category map
	assert.Len(t, exporter.categoryMap, 2)
	assert.Equal(t, "Category 1", exporter.categoryMap[1].Name)

	// Verify tag map
	assert.Len(t, exporter.tagMap, 2)
	assert.Equal(t, "Tag 1", exporter.tagMap[1].Name)

	// Verify user map
	assert.Len(t, exporter.userMap, 2)
	assert.Equal(t, "User 1", exporter.userMap[1].Name)

	// Verify media map
	assert.Len(t, exporter.mediaMap, 2)
	assert.Equal(t, "https://example.com/img1.jpg", exporter.mediaMap[1].SourceURL)
}

func TestShopifyExporter_GenerateSEODescription(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	tests := []struct {
		name     string
		excerpt  string
		maxLen   int
		expected string
	}{
		{
			name:     "Empty excerpt",
			excerpt:  "",
			maxLen:   160,
			expected: "",
		},
		{
			name:     "Short excerpt",
			excerpt:  "A short excerpt.",
			maxLen:   160,
			expected: "A short excerpt.",
		},
		{
			name:     "Long excerpt",
			excerpt:  "This is a very long excerpt that exceeds the maximum length allowed for SEO descriptions and needs to be truncated properly.",
			maxLen:   50,
			expected: "This is a very long excerpt that exceeds the...",
		},
		{
			name:     "Excerpt with HTML",
			excerpt:  "<p>An excerpt with <strong>HTML</strong> tags.</p>",
			maxLen:   160,
			expected: "An excerpt with HTML tags.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.generateSEODescription(tt.excerpt, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// BenchmarkShopifyExporter_ConvertPost benchmarks post conversion.
func BenchmarkShopifyExporter_ConvertPost(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	exporter.userMap = map[int]models.WordPressUser{
		1: {ID: 1, Name: "Test Author"},
	}
	exporter.categoryMap = map[int]models.WordPressCategory{
		1: {ID: 1, Name: "Technology"},
	}
	exporter.tagMap = map[int]models.WordPressTag{
		1: {ID: 1, Name: "Go"},
	}

	post := models.WordPressPost{
		ID:         1,
		Slug:       "benchmark-post",
		Status:     "publish",
		Title:      models.RenderedContent{Rendered: "Benchmark Post"},
		Content:    models.RenderedContent{Rendered: strings.Repeat("<p>Lorem ipsum dolor sit amet.</p>", 50)},
		Excerpt:    models.RenderedContent{Rendered: "<p>Benchmark excerpt</p>"},
		Author:     1,
		Categories: []int{1},
		Tags:       []int{1},
		Date:       models.WordPressTime{Time: time.Now()},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exporter.convertPostToShopifyProduct(post)
	}
}

// BenchmarkShopifyExporter_CleanHTML benchmarks HTML cleaning.
func BenchmarkShopifyExporter_CleanHTML(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewShopifyExporter(cfg)

	html := `<!-- wp:paragraph --><p>Hello [gallery ids="1,2,3"] World</p><!-- /wp:paragraph -->
		<p>   </p>
		<p>More content with [shortcode] and <!-- wp:block --> block comments</p>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exporter.cleanHTMLForShopify(html)
	}
}
