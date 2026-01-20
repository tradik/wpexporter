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

func TestNewMagentoExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "magento",
	}

	exporter := NewMagentoExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
	assert.NotNil(t, exporter.categoryMap)
	assert.NotNil(t, exporter.tagMap)
	assert.NotNil(t, exporter.userMap)
	assert.NotNil(t, exporter.mediaMap)
}

func TestMagentoExporter_GenerateSKU(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

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
			expected: "MY-POST-TITLE",
		},
		{
			name:     "Empty slug",
			slug:     "",
			id:       123,
			expected: "WP-123",
		},
		{
			name:     "Slug with lowercase",
			slug:     "my-product",
			id:       1,
			expected: "MY-PRODUCT",
		},
		{
			name:     "Slug with special characters",
			slug:     "my_post_title!@#",
			id:       1,
			expected: "MY_POST_TITLE",
		},
		{
			name:     "Slug with consecutive hyphens",
			slug:     "my--post--title",
			id:       1,
			expected: "MY-POST-TITLE",
		},
		{
			name:     "Slug with leading/trailing hyphens",
			slug:     "-my-post-title-",
			id:       1,
			expected: "MY-POST-TITLE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.generateSKU(tt.slug, tt.id)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMagentoExporter_GenerateURLKey(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.generateURLKey(tt.slug, tt.id)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMagentoExporter_GetCategoriesPath(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	// Populate category map
	exporter.categoryMap = map[int]models.WordPressCategory{
		1: {ID: 1, Name: "Electronics", Slug: "electronics"},
		2: {ID: 2, Name: "Clothing", Slug: "clothing"},
	}

	tests := []struct {
		name        string
		categoryIDs []int
		expected    string
	}{
		{
			name:        "Single category",
			categoryIDs: []int{1},
			expected:    "Default Category/Electronics",
		},
		{
			name:        "Multiple categories",
			categoryIDs: []int{1, 2},
			expected:    "Default Category/Electronics,Default Category/Clothing",
		},
		{
			name:        "Empty categories",
			categoryIDs: []int{},
			expected:    "Default Category",
		},
		{
			name:        "Non-existing category",
			categoryIDs: []int{99},
			expected:    "Default Category",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.getCategoriesPath(tt.categoryIDs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMagentoExporter_GetMetaKeywords(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

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
			expected: "Go,Programming,Web Development",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.getMetaKeywords(tt.tagIDs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMagentoExporter_IsOnline(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{
			name:     "Publish status",
			status:   "publish",
			expected: "1",
		},
		{
			name:     "Draft status",
			status:   "draft",
			expected: "0",
		},
		{
			name:     "Pending status",
			status:   "pending",
			expected: "0",
		},
		{
			name:     "Private status",
			status:   "private",
			expected: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.isOnline(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMagentoExporter_ExtractFilename(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Simple URL",
			url:      "https://example.com/images/product.jpg",
			expected: "product.jpg",
		},
		{
			name:     "URL with query params",
			url:      "https://example.com/images/product.jpg?width=100",
			expected: "product.jpg",
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: "",
		},
		{
			name:     "URL with multiple segments",
			url:      "https://example.com/wp-content/uploads/2024/01/image.png",
			expected: "image.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.extractFilename(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMagentoExporter_CleanHTMLForMagento(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.cleanHTMLForMagento(tt.html)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMagentoExporter_StripHTML(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

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
			name:     "Simple HTML",
			html:     "<p>Hello World</p>",
			expected: "Hello World",
		},
		{
			name:     "Nested HTML",
			html:     "<p><strong>Bold</strong> and <em>italic</em></p>",
			expected: "Bold and italic",
		},
		{
			name:     "HTML entities",
			html:     "&amp; &lt; &gt; &quot;",
			expected: "& < > \"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.stripHTML(tt.html)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMagentoExporter_TruncateString(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.truncateString(tt.str, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMagentoExporter_GetFeaturedImage(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	// Populate media map
	exporter.mediaMap = map[int]models.WordPressMedia{
		1: {
			ID:        1,
			SourceURL: "https://example.com/images/image1.jpg",
			AltText:   "Image 1 Alt",
			Title:     models.RenderedContent{Rendered: "Image 1 Title"},
		},
		2: {
			ID:        2,
			SourceURL: "https://example.com/images/image2.jpg",
			AltText:   "",
			Title:     models.RenderedContent{Rendered: "Image 2 Title"},
		},
	}

	tests := []struct {
		name          string
		mediaID       int
		expectedPath  string
		expectedLabel string
	}{
		{
			name:          "Existing media with alt text",
			mediaID:       1,
			expectedPath:  "image1.jpg",
			expectedLabel: "Image 1 Alt",
		},
		{
			name:          "Existing media without alt text (uses title)",
			mediaID:       2,
			expectedPath:  "image2.jpg",
			expectedLabel: "Image 2 Title",
		},
		{
			name:          "Non-existing media",
			mediaID:       99,
			expectedPath:  "",
			expectedLabel: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, label := exporter.getFeaturedImage(tt.mediaID)
			assert.Equal(t, tt.expectedPath, path)
			assert.Equal(t, tt.expectedLabel, label)
		})
	}
}

func TestMagentoExporter_GetAdditionalImages(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	tests := []struct {
		name         string
		html         string
		excludeImage string
		expected     string
	}{
		{
			name:         "Empty string",
			html:         "",
			excludeImage: "",
			expected:     "",
		},
		{
			name:         "No images",
			html:         "<p>Hello World</p>",
			excludeImage: "",
			expected:     "",
		},
		{
			name:         "Single image",
			html:         "<img src=\"https://example.com/img1.jpg\">",
			excludeImage: "",
			expected:     "img1.jpg",
		},
		{
			name:         "Multiple images with exclusion",
			html:         "<img src=\"https://example.com/main.jpg\"><img src=\"https://example.com/side.jpg\">",
			excludeImage: "https://example.com/main.jpg",
			expected:     "side.jpg",
		},
		{
			name:         "Duplicate images",
			html:         "<img src=\"https://example.com/img.jpg\"><img src=\"https://example.com/img.jpg\">",
			excludeImage: "",
			expected:     "img.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.getAdditionalImages(tt.html, tt.excludeImage)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMagentoExporter_ConvertPostToMagentoProduct(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

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

	product := exporter.convertPostToMagentoProduct(post)

	assert.Equal(t, "TEST-POST", product.SKU)
	assert.Equal(t, "test-post", product.URLKey)
	assert.Equal(t, "Test Post Title", product.Name)
	assert.Equal(t, "<p>Test content</p>", product.Description)
	assert.Equal(t, "Default Category/Technology", product.Categories)
	assert.Equal(t, "Go,Programming", product.MetaKeywords)
	assert.Equal(t, "1", product.ProductOnline)
	assert.Equal(t, "featured.jpg", product.BaseImage)
	assert.Equal(t, "Featured Image", product.BaseImageLabel)
	assert.Equal(t, "simple", product.ProductType)
	assert.Equal(t, "Default", product.AttributeSetCode)
}

func TestMagentoExporter_GetCSVHeaders(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	headers := exporter.getCSVHeaders()

	// Verify essential headers are present
	assert.Contains(t, headers, "sku")
	assert.Contains(t, headers, "name")
	assert.Contains(t, headers, "description")
	assert.Contains(t, headers, "short_description")
	assert.Contains(t, headers, "price")
	assert.Contains(t, headers, "categories")
	assert.Contains(t, headers, "product_type")
	assert.Contains(t, headers, "attribute_set_code")
	assert.Contains(t, headers, "visibility")
	assert.Contains(t, headers, "url_key")
	assert.Contains(t, headers, "meta_title")
	assert.Contains(t, headers, "meta_description")
	assert.Contains(t, headers, "base_image")

	// Verify header count matches expected Magento format (57 columns)
	assert.Equal(t, 57, len(headers))
}

func TestMagentoExporter_ProductToCSVRow(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	product := MagentoProduct{
		SKU:              "TEST-SKU",
		Name:             "Test Product",
		Description:      "<p>Description</p>",
		ShortDescription: "Short desc",
		Price:            "19.99",
		Categories:       "Default Category/Test",
		ProductType:      "simple",
		URLKey:           "test-product",
		ProductOnline:    "1",
	}

	row := exporter.productToCSVRow(product)

	// Verify row length matches header length
	assert.Equal(t, len(exporter.getCSVHeaders()), len(row))

	// Verify key fields
	assert.Equal(t, "TEST-SKU", row[0])              // sku
	assert.Equal(t, "simple", row[3])                // product_type
	assert.Equal(t, "Default Category/Test", row[4]) // categories
	assert.Equal(t, "Test Product", row[6])          // name
	assert.Equal(t, "<p>Description</p>", row[7])    // description
}

func TestMagentoExporter_Export(t *testing.T) {
	// Create temporary directory for test output
	tempDir, err := os.MkdirTemp("", "magento-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "magento",
		URL:    "https://example.com",
	}
	exporter := NewMagentoExporter(cfg)

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
	postsCSV := filepath.Join(tempDir, "magento_posts.csv")
	pagesCSV := filepath.Join(tempDir, "magento_pages.csv")
	productsCSV := filepath.Join(tempDir, "magento_products.csv")

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
	assert.Equal(t, "sku", records[0][0])
	assert.Equal(t, "name", records[0][6])

	// Verify data row
	assert.Equal(t, "TEST-POST", records[1][0])
	assert.Equal(t, "Test Post", records[1][6])
}

func TestMagentoExporter_ExportMetadata(t *testing.T) {
	// Create temporary directory for test output
	tempDir, err := os.MkdirTemp("", "magento-metadata-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "magento",
	}
	exporter := NewMagentoExporter(cfg)

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
	metadataCSV := filepath.Join(tempDir, "magento_metadata.csv")
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

	assert.Equal(t, "Export Format", records[12][0])
	assert.Equal(t, "Magento 2", records[12][1])
}

func TestMagentoExporter_BuildLookupMaps(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

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

// BenchmarkMagentoExporter_ConvertPost benchmarks post conversion.
func BenchmarkMagentoExporter_ConvertPost(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

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
		_ = exporter.convertPostToMagentoProduct(post)
	}
}

// BenchmarkMagentoExporter_CleanHTML benchmarks HTML cleaning.
func BenchmarkMagentoExporter_CleanHTML(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewMagentoExporter(cfg)

	html := `<!-- wp:paragraph --><p>Hello [gallery ids="1,2,3"] World</p><!-- /wp:paragraph -->
		<p>   </p>
		<p>More content with [shortcode] and <!-- wp:block --> block comments</p>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exporter.cleanHTMLForMagento(html)
	}
}
