package export

import (
	"encoding/json"
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

func TestNewWeeblyExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "weebly",
	}

	exporter := NewWeeblyExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
}

func TestWeeblyExporter_ConvertPostToItem(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWeeblyExporter(cfg)

	userMap := map[int]string{
		1: "test-user",
	}

	post := models.WordPressPost{
		ID:      1,
		Slug:    "test-post",
		Status:  "publish",
		Title:   models.RenderedContent{Rendered: "Test Post Title"},
		Content: models.RenderedContent{Rendered: "<p>Test content</p>"},
		Author:  1,
		Date:    models.WordPressTime{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		Link:    "https://example.com/test-post",
	}

	item := exporter.convertPostToItem(post, "post", userMap)

	assert.Equal(t, "Test Post Title", item.Title)
	assert.Equal(t, "https://example.com/test-post", item.Link)
	assert.Equal(t, "test-user", item.Creator)
	assert.Equal(t, 1, item.PostID)
	assert.Equal(t, "test-post", item.PostName)
	assert.Equal(t, "publish", item.Status)
	assert.Equal(t, "post", item.PostType)
	assert.Contains(t, item.ContentEncoded, "<p>Test content</p>")
}

func TestWeeblyExporter_ConvertPostToItemDefaultCreator(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewWeeblyExporter(cfg)

	post := models.WordPressPost{
		ID:     1,
		Slug:   "test-post",
		Status: "publish",
		Title:  models.RenderedContent{Rendered: "Test Post"},
		Author: 999, // Non-existent author
	}

	item := exporter.convertPostToItem(post, "post", nil)

	assert.Equal(t, "admin", item.Creator) // Should default to admin
}

func TestWeeblyExporter_Export(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "weebly-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "weebly",
	}
	exporter := NewWeeblyExporter(cfg)

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
				Author:  1,
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
			{ID: 1, Title: models.RenderedContent{Rendered: "Test Image"}, SourceURL: "https://example.com/image.jpg", MimeType: "image/jpeg"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Check XML file
	xmlFile := filepath.Join(tempDir, "weebly_export.xml")
	assert.FileExists(t, xmlFile)

	xmlContent, err := os.ReadFile(xmlFile)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(xmlContent), "<?xml"))
	assert.Contains(t, string(xmlContent), "<rss version=\"2.0\"")

	// Check JSON file
	jsonFile := filepath.Join(tempDir, "weebly_export.json")
	assert.FileExists(t, jsonFile)

	jsonContent, err := os.ReadFile(jsonFile)
	require.NoError(t, err)

	var weeblyData WeeblyJSONExport
	err = json.Unmarshal(jsonContent, &weeblyData)
	require.NoError(t, err)

	assert.Equal(t, "wpexporter", weeblyData.Meta.Exporter)
	assert.Len(t, weeblyData.Posts, 1)
	assert.Len(t, weeblyData.Pages, 1)
}

func TestWeeblyExporter_ExportWithXMLExtension(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "weebly-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	outputFile := filepath.Join(tempDir, "custom_export.xml")
	cfg := &config.Config{
		Output: outputFile,
		Format: "weebly",
	}
	exporter := NewWeeblyExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Check files in parent dir
	assert.FileExists(t, filepath.Join(tempDir, "weebly_export.xml"))
	assert.FileExists(t, filepath.Join(tempDir, "weebly_export.json"))
}

func TestWeeblyExporter_XMLValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "weebly-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "weebly",
	}
	exporter := NewWeeblyExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
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

	err = exporter.Export(data)
	require.NoError(t, err)

	// Read and parse XML to verify it's valid
	xmlContent, err := os.ReadFile(filepath.Join(tempDir, "weebly_export.xml"))
	require.NoError(t, err)

	// Remove XML declaration for unmarshaling
	xmlStr := string(xmlContent)
	if idx := strings.Index(xmlStr, "<rss"); idx != -1 {
		xmlStr = xmlStr[idx:]
	}

	var decoded WeeblyExport
	err = xml.Unmarshal([]byte(xmlStr), &decoded)
	require.NoError(t, err)

	assert.Equal(t, "Test Site", decoded.Channel.Title)
}

func TestWeeblyExporter_EmptyData(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "weebly-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "weebly",
	}
	exporter := NewWeeblyExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Empty Site",
			URL:  "https://example.com",
		},
	}

	err = exporter.Export(data)
	require.NoError(t, err)

	// Read JSON and verify
	jsonContent, err := os.ReadFile(filepath.Join(tempDir, "weebly_export.json"))
	require.NoError(t, err)

	var weeblyData WeeblyJSONExport
	err = json.Unmarshal(jsonContent, &weeblyData)
	require.NoError(t, err)

	assert.Len(t, weeblyData.Posts, 0)
	assert.Len(t, weeblyData.Pages, 0)
}

func TestWeeblyExporter_JSONPostContent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "weebly-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "weebly",
	}
	exporter := NewWeeblyExporter(cfg)

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
				Modified:      models.WordPressTime{Time: time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)},
				Categories:    []int{1},
				Tags:          []int{1},
				FeaturedMedia: 10,
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

	// Read and verify JSON
	jsonContent, err := os.ReadFile(filepath.Join(tempDir, "weebly_export.json"))
	require.NoError(t, err)

	var weeblyData WeeblyJSONExport
	err = json.Unmarshal(jsonContent, &weeblyData)
	require.NoError(t, err)

	assert.Len(t, weeblyData.Posts, 1)
	post := weeblyData.Posts[0]
	assert.Equal(t, 1, post.ID)
	assert.Equal(t, "Test Post", post.Title)
	assert.Equal(t, "test-post", post.Slug)
	assert.Equal(t, "<p>Content</p>", post.Content)
	assert.Equal(t, "Excerpt", post.Excerpt) // HTML stripped
	assert.Equal(t, "John Doe", post.Author)
	assert.True(t, post.Published)
	assert.Equal(t, "https://example.com/featured.jpg", post.FeaturedImage)
	assert.Len(t, post.Categories, 1)
	assert.Len(t, post.Tags, 1)
}

func TestWeeblyExporter_DraftPost(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "weebly-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "weebly",
	}
	exporter := NewWeeblyExporter(cfg)

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

	// Read and verify JSON
	jsonContent, err := os.ReadFile(filepath.Join(tempDir, "weebly_export.json"))
	require.NoError(t, err)

	var weeblyData WeeblyJSONExport
	err = json.Unmarshal(jsonContent, &weeblyData)
	require.NoError(t, err)

	assert.Len(t, weeblyData.Posts, 1)
	assert.False(t, weeblyData.Posts[0].Published)
}

func TestWeeblyExporter_FullExportWithAllTypes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "weebly-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "weebly",
	}
	exporter := NewWeeblyExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Full Test Site",
			Description: "Complete test description",
			URL:         "https://example.com",
			HomeURL:     "https://example.com",
			Language:    "en-US",
		},
		Posts: []models.WordPressPost{
			{
				ID:            1,
				Slug:          "test-post-1",
				Status:        "publish",
				Title:         models.RenderedContent{Rendered: "Test Post 1"},
				Content:       models.RenderedContent{Rendered: "<p>Content 1</p>"},
				Excerpt:       models.RenderedContent{Rendered: "<p>Excerpt</p>"},
				Author:        1,
				Date:          models.WordPressTime{Time: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
				Modified:      models.WordPressTime{Time: time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)},
				Categories:    []int{1},
				Tags:          []int{1},
				FeaturedMedia: 1,
			},
			{
				ID:     2,
				Slug:   "draft-post",
				Status: "draft",
				Title:  models.RenderedContent{Rendered: "Draft Post"},
				Author: 1,
			},
		},
		Pages: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "about",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "About Page"},
				Content: models.RenderedContent{Rendered: "<p>About content</p>"},
			},
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Category 1", Slug: "category-1", Description: "Cat desc"},
		},
		Tags: []models.WordPressTag{
			{ID: 1, Name: "Tag 1", Slug: "tag-1"},
		},
		Users: []models.WordPressUser{
			{ID: 1, Name: "Test Author", Slug: "test-author"},
		},
		Media: []models.WordPressMedia{
			{ID: 1, Title: models.RenderedContent{Rendered: "Image"}, SourceURL: "https://example.com/image.jpg", MimeType: "image/jpeg"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify both files exist
	assert.FileExists(t, filepath.Join(tempDir, "weebly_export.xml"))
	assert.FileExists(t, filepath.Join(tempDir, "weebly_export.json"))

	// Verify JSON content
	jsonContent, err := os.ReadFile(filepath.Join(tempDir, "weebly_export.json"))
	require.NoError(t, err)

	var weeblyData WeeblyJSONExport
	err = json.Unmarshal(jsonContent, &weeblyData)
	require.NoError(t, err)

	assert.Equal(t, "wpexporter", weeblyData.Meta.Exporter)
	assert.Len(t, weeblyData.Posts, 2)
	assert.Len(t, weeblyData.Pages, 1)

	// Verify XML structure
	xmlContent, err := os.ReadFile(filepath.Join(tempDir, "weebly_export.xml"))
	require.NoError(t, err)
	assert.Contains(t, string(xmlContent), "<item>")
	assert.Contains(t, string(xmlContent), "test-post-1")
}

// Benchmark tests
func BenchmarkWeeblyExporter_Export(b *testing.B) {
	cfg := &config.Config{}

	posts := make([]models.WordPressPost, 100)
	for i := 0; i < 100; i++ {
		posts[i] = models.WordPressPost{
			ID:      i + 1,
			Slug:    "test-post",
			Status:  "publish",
			Title:   models.RenderedContent{Rendered: "Test Post"},
			Content: models.RenderedContent{Rendered: strings.Repeat("<p>Content</p>", 10)},
			Author:  1,
		}
	}

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test", URL: "https://example.com"},
		Posts: posts,
		Users: []models.WordPressUser{{ID: 1, Name: "User", Slug: "user"}},
	}

	tempDir, _ := os.MkdirTemp("", "weebly-bench")
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg.Output = tempDir
	exporter := NewWeeblyExporter(cfg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = exporter.Export(data)
	}
}
