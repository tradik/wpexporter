package export

import (
	"encoding/json"
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

func TestNewDrupalExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "drupal",
	}

	exporter := NewDrupalExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
}

func TestDrupalExporter_GenerateUUID(t *testing.T) {
	tests := []struct {
		name   string
		id     int
		prefix string
	}{
		{
			name:   "User UUID",
			id:     1,
			prefix: "user",
		},
		{
			name:   "Node UUID",
			id:     123,
			prefix: "article",
		},
		{
			name:   "Media UUID",
			id:     456,
			prefix: "media",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uuid := generateUUID(tt.id, tt.prefix)
			// UUID should have a specific format
			assert.NotEmpty(t, uuid)
			assert.Contains(t, uuid, "-")
		})
	}
}

func TestDrupalExporter_StatusToInt(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected int
	}{
		{
			name:     "Published",
			status:   "publish",
			expected: 1,
		},
		{
			name:     "Draft",
			status:   "draft",
			expected: 0,
		},
		{
			name:     "Pending",
			status:   "pending",
			expected: 0,
		},
		{
			name:     "Private",
			status:   "private",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := statusToInt(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDrupalExporter_StripTags(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "Plain text",
			html:     "Hello World",
			expected: "Hello World",
		},
		{
			name:     "With paragraph tags",
			html:     "<p>Hello World</p>",
			expected: "Hello World",
		},
		{
			name:     "Nested tags",
			html:     "<div><p>Hello <strong>World</strong></p></div>",
			expected: "Hello World",
		},
		{
			name:     "Empty string",
			html:     "",
			expected: "",
		},
		{
			name:     "Only tags",
			html:     "<br><hr>",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripTags(tt.html)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDrupalExporter_ConvertToPublicURI(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "WordPress uploads URL",
			url:      "https://example.com/wp-content/uploads/2024/01/image.jpg",
			expected: "public://2024/01/image.jpg",
		},
		{
			name:     "Simple URL",
			url:      "https://example.com/image.jpg",
			expected: "public://image.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToPublicURI(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDrupalExporter_ExtractFilename(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Full URL",
			url:      "https://example.com/path/to/image.jpg",
			expected: "image.jpg",
		},
		{
			name:     "Simple filename",
			url:      "image.jpg",
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
			result := extractFilename(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDrupalExporter_ConvertUsers(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewDrupalExporter(cfg)

	users := []models.WordPressUser{
		{ID: 1, Name: "John Doe", Slug: "john-doe"},
		{ID: 2, Name: "Jane Smith", Slug: "jane-smith"},
	}

	drupalUsers := exporter.convertUsers(users)

	assert.Len(t, drupalUsers, 2)
	assert.Equal(t, 1, drupalUsers[0].UID)
	assert.Equal(t, "john-doe", drupalUsers[0].Name)
	assert.Equal(t, 1, drupalUsers[0].Status)
	assert.Equal(t, "en", drupalUsers[0].Langcode)
}

func TestDrupalExporter_ConvertTerms(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewDrupalExporter(cfg)

	categories := []models.WordPressCategory{
		{ID: 1, Name: "Technology", Slug: "technology", Parent: 0, Description: "Tech posts"},
		{ID: 2, Name: "Programming", Slug: "programming", Parent: 1, Description: "Code tutorials"},
	}

	tags := []models.WordPressTag{
		{ID: 1, Name: "Go", Slug: "go", Description: "Go language"},
		{ID: 2, Name: "Python", Slug: "python", Description: "Python language"},
	}

	terms := exporter.convertTerms(categories, tags)

	// Should have 2 categories + 2 tags = 4 terms
	assert.Len(t, terms, 4)

	// Check category terms
	assert.Equal(t, 1, terms[0].TID)
	assert.Equal(t, "categories", terms[0].VID)
	assert.Equal(t, "Technology", terms[0].Name)
	assert.Equal(t, 0, terms[0].Parent)

	assert.Equal(t, 2, terms[1].TID)
	assert.Equal(t, 1, terms[1].Parent)

	// Check tag terms (with offset)
	assert.Equal(t, 100001, terms[2].TID)
	assert.Equal(t, "tags", terms[2].VID)
	assert.Equal(t, "Go", terms[2].Name)
}

func TestDrupalExporter_ConvertMedia(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewDrupalExporter(cfg)

	media := []models.WordPressMedia{
		{
			ID:        1,
			Slug:      "test-image",
			Status:    "inherit",
			Title:     models.RenderedContent{Rendered: "Test Image"},
			Author:    1,
			Date:      models.WordPressTime{Time: time.Now()},
			Modified:  models.WordPressTime{Time: time.Now()},
			SourceURL: "https://example.com/wp-content/uploads/2024/01/test.jpg",
			MimeType:  "image/jpeg",
		},
		{
			ID:        2,
			Slug:      "test-video",
			Status:    "inherit",
			Title:     models.RenderedContent{Rendered: "Test Video"},
			Author:    1,
			Date:      models.WordPressTime{Time: time.Now()},
			Modified:  models.WordPressTime{Time: time.Now()},
			SourceURL: "https://example.com/wp-content/uploads/2024/01/video.mp4",
			MimeType:  "video/mp4",
		},
	}

	drupalMedia := exporter.convertMedia(media)

	assert.Len(t, drupalMedia, 2)

	// Check image
	assert.Equal(t, 1, drupalMedia[0].MID)
	assert.Equal(t, "image", drupalMedia[0].Bundle)
	assert.Equal(t, "Test Image", drupalMedia[0].Name)
	assert.Equal(t, "public://2024/01/test.jpg", drupalMedia[0].File.URI)
	assert.Equal(t, "image/jpeg", drupalMedia[0].File.Filemime)

	// Check video
	assert.Equal(t, 2, drupalMedia[1].MID)
	assert.Equal(t, "video", drupalMedia[1].Bundle)
}

func TestDrupalExporter_ConvertPostToNode(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewDrupalExporter(cfg)

	mediaMap := map[int]models.WordPressMedia{
		10: {
			ID:      10,
			AltText: "Featured image alt",
			Title:   models.RenderedContent{Rendered: "Featured Image"},
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
		Date:          models.WordPressTime{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		Modified:      models.WordPressTime{Time: time.Date(2024, 1, 20, 12, 0, 0, 0, time.UTC)},
		Categories:    []int{1, 2},
		Tags:          []int{1, 2},
		FeaturedMedia: 10,
		SEO: models.SEOData{
			Title:           "SEO Title",
			MetaDescription: "SEO Description",
		},
	}

	node := exporter.convertPostToNode(post, "article", mediaMap)

	assert.Equal(t, 1, node.NID)
	assert.Equal(t, "article", node.Type)
	assert.Equal(t, "Test Post Title", node.Title)
	assert.Equal(t, "en", node.Langcode)
	assert.Equal(t, 1, node.Status)
	assert.Equal(t, int64(1705314600), node.Created)
	assert.Equal(t, 1, node.UID)
	assert.Equal(t, "<p>Test content</p>", node.Body.Value)
	assert.Equal(t, "Test excerpt", node.Body.Summary)
	assert.Equal(t, "full_html", node.Body.Format)
	assert.Equal(t, "/test-post", node.Path.Alias)

	// Check categories
	assert.Len(t, node.Categories, 2)

	// Check tags with offset
	assert.Len(t, node.Tags, 2)
	assert.Equal(t, 100001, node.Tags[0])
	assert.Equal(t, 100002, node.Tags[1])

	// Check featured image
	assert.NotNil(t, node.Image)
	assert.Equal(t, 10, node.Image.TargetID)
	assert.Equal(t, "Featured image alt", node.Image.Alt)

	// Check SEO meta
	assert.Equal(t, "SEO Title", node.MetaTags.Title)
	assert.Equal(t, "SEO Description", node.MetaTags.Description)
}

func TestDrupalExporter_ConvertNodes(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewDrupalExporter(cfg)

	posts := []models.WordPressPost{
		{ID: 1, Slug: "post-1", Status: "publish", Title: models.RenderedContent{Rendered: "Post 1"}},
		{ID: 2, Slug: "post-2", Status: "publish", Title: models.RenderedContent{Rendered: "Post 2"}},
	}

	pages := []models.WordPressPost{
		{ID: 1, Slug: "page-1", Status: "publish", Title: models.RenderedContent{Rendered: "Page 1"}},
	}

	media := []models.WordPressMedia{}

	nodes := exporter.convertNodes(posts, pages, media)

	// Should have 2 posts + 1 page = 3 nodes
	assert.Len(t, nodes, 3)

	// Check post nodes
	assert.Equal(t, 1, nodes[0].NID)
	assert.Equal(t, "article", nodes[0].Type)

	assert.Equal(t, 2, nodes[1].NID)
	assert.Equal(t, "article", nodes[1].Type)

	// Check page node (with offset)
	assert.Equal(t, 1000001, nodes[2].NID)
	assert.Equal(t, "page", nodes[2].Type)
}

func TestDrupalExporter_BuildDrupalData(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewDrupalExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Test Description",
			URL:         "https://example.com",
		},
		Posts: []models.WordPressPost{
			{ID: 1, Slug: "post-1", Status: "publish", Title: models.RenderedContent{Rendered: "Post 1"}},
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
			{ID: 1, Slug: "image-1", SourceURL: "https://example.com/image.jpg", MimeType: "image/jpeg"},
		},
	}

	drupalData := exporter.buildDrupalData(data)

	assert.Equal(t, "wpexporter", drupalData.Meta.Exporter)
	assert.Equal(t, "1.3.7", drupalData.Meta.Version)
	assert.Equal(t, "https://example.com", drupalData.Meta.SourceSite)
	assert.Equal(t, "Test Site", drupalData.Meta.SourceName)

	assert.Equal(t, 2, drupalData.Meta.TotalNodes) // 1 post + 1 page
	assert.Equal(t, 2, drupalData.Meta.TotalTerms) // 1 category + 1 tag
	assert.Equal(t, 1, drupalData.Meta.TotalUsers)
	assert.Equal(t, 1, drupalData.Meta.TotalMedia)
}

func TestDrupalExporter_Export(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "drupal-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "drupal",
	}
	exporter := NewDrupalExporter(cfg)

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
			{ID: 1, SourceURL: "https://example.com/image.jpg", MimeType: "image/jpeg"},
		},
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	// Verify main output file exists
	mainFile := filepath.Join(tempDir, "drupal_export.json")
	assert.FileExists(t, mainFile)

	// Verify separate files exist
	assert.FileExists(t, filepath.Join(tempDir, "drupal_nodes.json"))
	assert.FileExists(t, filepath.Join(tempDir, "drupal_terms.json"))
	assert.FileExists(t, filepath.Join(tempDir, "drupal_users.json"))
	assert.FileExists(t, filepath.Join(tempDir, "drupal_media.json"))

	// Read and verify main JSON structure
	content, err := os.ReadFile(mainFile)
	require.NoError(t, err)

	var drupalData DrupalExportData
	err = json.Unmarshal(content, &drupalData)
	require.NoError(t, err)

	assert.Equal(t, "wpexporter", drupalData.Meta.Exporter)
	assert.Equal(t, 2, drupalData.Meta.TotalNodes)
	assert.Len(t, drupalData.Nodes, 2)
	assert.Len(t, drupalData.Terms, 2)
	assert.Len(t, drupalData.Users, 1)
	assert.Len(t, drupalData.Media, 1)
}

func TestDrupalExporter_ExportWithJSONExtension(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "drupal-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	outputFile := filepath.Join(tempDir, "custom_export.json")
	cfg := &config.Config{
		Output: outputFile,
		Format: "drupal",
	}
	exporter := NewDrupalExporter(cfg)

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

func TestDrupalExporter_NodeFieldsValidation(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewDrupalExporter(cfg)

	post := models.WordPressPost{
		ID:      1,
		Slug:    "test-post",
		Status:  "publish",
		Title:   models.RenderedContent{Rendered: "Test Post"},
		Content: models.RenderedContent{Rendered: "<p>Content</p>"},
		Excerpt: models.RenderedContent{Rendered: "<p>Excerpt with <strong>HTML</strong></p>"},
		Author:  1,
		Date:    models.WordPressTime{Time: time.Now()},
	}

	node := exporter.convertPostToNode(post, "article", nil)

	// Validate body fields
	assert.Equal(t, "<p>Content</p>", node.Body.Value)
	assert.Equal(t, "Excerpt with HTML", node.Body.Summary) // HTML should be stripped
	assert.Equal(t, "full_html", node.Body.Format)

	// Validate path
	assert.Equal(t, "/test-post", node.Path.Alias)
	assert.Equal(t, "en", node.Path.Langcode)
}

func TestDrupalExporter_MediaBundleDetection(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewDrupalExporter(cfg)

	tests := []struct {
		mimeType       string
		expectedBundle string
	}{
		{"image/jpeg", "image"},
		{"image/png", "image"},
		{"image/gif", "image"},
		{"video/mp4", "video"},
		{"video/webm", "video"},
		{"audio/mpeg", "audio"},
		{"audio/ogg", "audio"},
		{"application/pdf", "file"},
		{"text/plain", "file"},
	}

	for _, tt := range tests {
		t.Run(tt.mimeType, func(t *testing.T) {
			media := []models.WordPressMedia{
				{ID: 1, MimeType: tt.mimeType, SourceURL: "https://example.com/file"},
			}

			drupalMedia := exporter.convertMedia(media)

			assert.Len(t, drupalMedia, 1)
			assert.Equal(t, tt.expectedBundle, drupalMedia[0].Bundle)
		})
	}
}

// Benchmark tests
func BenchmarkDrupalExporter_BuildDrupalData(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewDrupalExporter(cfg)

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
		_ = exporter.buildDrupalData(data)
	}
}

func BenchmarkDrupalExporter_StripTags(b *testing.B) {
	html := "<div><p>Hello <strong>World</strong> with <a href='#'>link</a> and <em>emphasis</em></p></div>"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stripTags(html)
	}
}
