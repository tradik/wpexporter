package export

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestExtractCategoriesFromLink(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewExporter(cfg)

	tests := []struct {
		name     string
		link     string
		expected string
	}{
		{
			name:     "Category in URL",
			link:     "https://example.com/technology/artificial-intelligence/my-post",
			expected: "technology/artificial-intelligence",
		},
		{
			name:     "Single category",
			link:     "https://example.com/news/breaking-news-today",
			expected: "news",
		},
		{
			name:     "No categories (direct post)",
			link:     "https://example.com/my-post-slug",
			expected: "",
		},
		{
			name:     "Date-based permalink",
			link:     "https://example.com/2024/01/15/my-post",
			expected: "",
		},
		{
			name:     "Mixed date and category",
			link:     "https://example.com/tech/2024/01/my-post",
			expected: "tech",
		},
		{
			name:     "Skip common segments",
			link:     "https://example.com/blog/technology/my-post",
			expected: "technology",
		},
		{
			name:     "Skip posts segment",
			link:     "https://example.com/posts/technology/my-post",
			expected: "technology",
		},
		{
			name:     "Deep category hierarchy",
			link:     "https://example.com/tech/web-development/frontend/react/my-tutorial",
			expected: "tech/web-development/frontend/react",
		},
		{
			name:     "Empty link",
			link:     "",
			expected: "",
		},
		{
			name:     "Invalid URL",
			link:     "not-a-valid-url",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := exporter.extractCategoriesFromLink(tt.link)
			if result != tt.expected {
				t.Errorf("extractCategoriesFromLink(%q) = %q, want %q", tt.link, result, tt.expected)
			}
		})
	}
}

func TestIsNumeric(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewExporter(cfg)

	tests := []struct {
		input    string
		expected bool
	}{
		{"2024", true},
		{"01", true},
		{"15", true},
		{"0", true},
		{"", false},
		{"abc", false},
		{"2024a", false},
		{"a2024", false},
		{"20-24", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := exporter.isNumeric(tt.input)
			if result != tt.expected {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// createTestData creates sample export data for testing
func createTestData() *models.ExportData {
	return &models.ExportData{
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
}

func TestExporter_ExportWix(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-wix-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cfg := &config.Config{
		Output: tempDir,
		Format: "wix",
	}
	exporter := NewExporter(cfg)

	err = exporter.Export(createTestData())
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, "wix_export.json"))
}

func TestExporter_ExportSquarespace(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-squarespace-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cfg := &config.Config{
		Output: tempDir,
		Format: "squarespace",
	}
	exporter := NewExporter(cfg)

	err = exporter.Export(createTestData())
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, "squarespace_export.xml"))
}

func TestExporter_ExportWebflow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-webflow-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cfg := &config.Config{
		Output: tempDir,
		Format: "webflow",
	}
	exporter := NewExporter(cfg)

	err = exporter.Export(createTestData())
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, "webflow_posts.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "webflow_export.json"))
}

func TestExporter_ExportWeebly(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-weebly-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cfg := &config.Config{
		Output: tempDir,
		Format: "weebly",
	}
	exporter := NewExporter(cfg)

	err = exporter.Export(createTestData())
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, "weebly_export.xml"))
	assert.FileExists(t, filepath.Join(tempDir, "weebly_export.json"))
}

func TestExporter_ExportPrestaShop(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-prestashop-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cfg := &config.Config{
		Output: tempDir,
		Format: "prestashop",
	}
	exporter := NewExporter(cfg)

	err = exporter.Export(createTestData())
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, "prestashop_products.csv"))
	assert.FileExists(t, filepath.Join(tempDir, "prestashop_export.json"))
}

func TestExporter_ExportGhost(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-ghost-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cfg := &config.Config{
		Output: tempDir,
		Format: "ghost",
	}
	exporter := NewExporter(cfg)

	err = exporter.Export(createTestData())
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, "ghost_export.json"))
}

func TestExporter_ExportStrapi(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-strapi-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cfg := &config.Config{
		Output: tempDir,
		Format: "strapi",
	}
	exporter := NewExporter(cfg)

	err = exporter.Export(createTestData())
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, "strapi_export.json"))
}

func TestExporter_ExportContentful(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-contentful-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cfg := &config.Config{
		Output: tempDir,
		Format: "contentful",
	}
	exporter := NewExporter(cfg)

	err = exporter.Export(createTestData())
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, "contentful_export.json"))
}

func TestExporter_ExportWordPress(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-wordpress-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cfg := &config.Config{
		Output: tempDir,
		Format: "wordpress",
	}
	exporter := NewExporter(cfg)

	err = exporter.Export(createTestData())
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, "wordpress_export.xml"))
}

func TestExporter_ExportDrupal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-drupal-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cfg := &config.Config{
		Output: tempDir,
		Format: "drupal",
	}
	exporter := NewExporter(cfg)

	err = exporter.Export(createTestData())
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, "drupal_export.json"))
}

func TestExporter_UnsupportedFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "exporter-unsupported-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	cfg := &config.Config{
		Output: tempDir,
		Format: "unsupported_format",
	}
	exporter := NewExporter(cfg)

	err = exporter.Export(createTestData())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported export format")
}
