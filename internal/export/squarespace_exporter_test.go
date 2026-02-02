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

func TestNewSquarespaceExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "squarespace",
	}

	exporter := NewSquarespaceExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
}

func TestSquarespaceExporter_ConvertPostToItem(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewSquarespaceExporter(cfg)

	categoryMap := map[int]models.WordPressCategory{
		1: {ID: 1, Name: "Technology", Slug: "technology"},
	}
	tagMap := map[int]models.WordPressTag{
		1: {ID: 1, Name: "Go", Slug: "go"},
	}
	userMap := map[int]string{
		1: "test-user",
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
}

func TestSquarespaceExporter_ConvertPostToItemDefaultCreator(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewSquarespaceExporter(cfg)

	post := models.WordPressPost{
		ID:     1,
		Slug:   "test-post",
		Status: "publish",
		Title:  models.RenderedContent{Rendered: "Test Post"},
		Author: 999, // Non-existent author
	}

	item := exporter.convertPostToItem(post, "post", nil, nil, nil)

	assert.Equal(t, "admin", item.Creator) // Should default to admin
}

func TestSquarespaceExporter_ConvertMediaToItem(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewSquarespaceExporter(cfg)

	userMap := map[int]string{
		1: "test-user",
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
}

func TestSquarespaceExporter_BuildSquarespaceData(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewSquarespaceExporter(cfg)

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

	sqData := exporter.buildSquarespaceData(data)

	assert.Equal(t, "2.0", sqData.Version)
	assert.Equal(t, "Test Site", sqData.Channel.Title)
	assert.Equal(t, "https://example.com", sqData.Channel.Link)
	assert.Equal(t, "Test Description", sqData.Channel.Description)
	assert.Equal(t, "en-US", sqData.Channel.Language)
	assert.Equal(t, "1.2", sqData.Channel.WXRVersion)

	assert.Len(t, sqData.Channel.Categories, 1)
	assert.Len(t, sqData.Channel.Tags, 1)
	assert.Len(t, sqData.Channel.Items, 3) // 1 post + 1 page + 1 media
}

func TestSquarespaceExporter_Export(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "squarespace-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "squarespace",
	}
	exporter := NewSquarespaceExporter(cfg)

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

	outputFile := filepath.Join(tempDir, "squarespace_export.xml")
	assert.FileExists(t, outputFile)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(string(content), "<?xml"))
	assert.Contains(t, string(content), "<rss version=\"2.0\"")
	assert.Contains(t, string(content), "<channel>")
	assert.Contains(t, string(content), "<title>Test Site</title>")
}

func TestSquarespaceExporter_ExportWithXMLExtension(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "squarespace-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	outputFile := filepath.Join(tempDir, "custom_export.xml")
	cfg := &config.Config{
		Output: outputFile,
		Format: "squarespace",
	}
	exporter := NewSquarespaceExporter(cfg)

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

func TestSquarespaceExporter_XMLValidation(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewSquarespaceExporter(cfg)

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

	sqData := exporter.buildSquarespaceData(data)

	// Marshal and unmarshal to verify valid XML
	xmlData, err := xml.MarshalIndent(sqData, "", "  ")
	require.NoError(t, err)

	var decoded SquarespaceExport
	err = xml.Unmarshal(xmlData, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "Test Site", decoded.Channel.Title)
}

func TestSquarespaceExporter_CategoryWithParent(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewSquarespaceExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Parent", Slug: "parent", Parent: 0},
			{ID: 2, Name: "Child", Slug: "child", Parent: 1},
		},
	}

	sqData := exporter.buildSquarespaceData(data)

	assert.Len(t, sqData.Channel.Categories, 2)
	assert.Equal(t, "", sqData.Channel.Categories[0].Parent)
	assert.Equal(t, "parent", sqData.Channel.Categories[1].Parent)
}

func TestSquarespaceExporter_EmptyData(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewSquarespaceExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Empty Site",
			URL:  "https://example.com",
		},
	}

	sqData := exporter.buildSquarespaceData(data)

	assert.Equal(t, "Empty Site", sqData.Channel.Title)
	assert.Len(t, sqData.Channel.Items, 0)
	assert.Len(t, sqData.Channel.Categories, 0)
	assert.Len(t, sqData.Channel.Tags, 0)
}

func TestSquarespaceExporter_FullExport(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "squarespace-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "squarespace",
	}
	exporter := NewSquarespaceExporter(cfg)

	testData := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Full Test Site",
			Description: "Full description",
			URL:         "https://example.com",
			HomeURL:     "https://example.com",
			Language:    "en-US",
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
				DateGMT:       models.WordPressTime{Time: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
				Modified:      models.WordPressTime{Time: time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)},
				Link:          "https://example.com/test-post",
				Categories:    []int{1},
				Tags:          []int{1},
				FeaturedMedia: 1,
				CommentStatus: "open",
				PingStatus:    "closed",
			},
		},
		Pages: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "about",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "About"},
				Content: models.RenderedContent{Rendered: "<p>About content</p>"},
				Author:  1,
			},
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Category", Slug: "category", Description: "Cat desc"},
		},
		Tags: []models.WordPressTag{
			{ID: 1, Name: "Tag", Slug: "tag", Description: "Tag desc"},
		},
		Users: []models.WordPressUser{
			{ID: 1, Name: "Author", Slug: "author"},
		},
		Media: []models.WordPressMedia{
			{
				ID:          1,
				Slug:        "image",
				Title:       models.RenderedContent{Rendered: "Image"},
				SourceURL:   "https://example.com/image.jpg",
				MimeType:    "image/jpeg",
				Author:      1,
				Date:        models.WordPressTime{Time: time.Now()},
				DateGMT:     models.WordPressTime{Time: time.Now()},
				Modified:    models.WordPressTime{Time: time.Now()},
				Link:        "https://example.com/image",
				Description: models.RenderedContent{Rendered: "Image desc"},
				Caption:     models.RenderedContent{Rendered: "Caption"},
				Post:        1,
			},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tempDir, "squarespace_export.xml"))

	// Verify XML content
	content, err := os.ReadFile(filepath.Join(tempDir, "squarespace_export.xml"))
	require.NoError(t, err)

	assert.Contains(t, string(content), "<title>Full Test Site</title>")
	assert.Contains(t, string(content), "test-post")
	assert.Contains(t, string(content), "attachment")
}

// Benchmark tests
func BenchmarkSquarespaceExporter_BuildSquarespaceData(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewSquarespaceExporter(cfg)

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
		_ = exporter.buildSquarespaceData(data)
	}
}
