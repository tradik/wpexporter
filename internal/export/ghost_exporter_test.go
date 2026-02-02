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

func TestNewGhostExporter(t *testing.T) {
	cfg := &config.Config{
		Output: "/tmp/test-output",
		Format: "ghost",
	}

	exporter := NewGhostExporter(cfg)

	assert.NotNil(t, exporter)
	assert.NotNil(t, exporter.config)
}

func TestGhostExporter_ConvertUser(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	user := models.WordPressUser{
		ID:          1,
		Name:        "John Doe",
		Slug:        "john-doe",
		Description: "Author bio",
	}

	ghostUser := exporter.convertUser(user)

	assert.Equal(t, "1", ghostUser.ID)
	assert.Equal(t, "John Doe", ghostUser.Name)
	assert.Equal(t, "john-doe", ghostUser.Slug)
	assert.Contains(t, ghostUser.Email, "john-doe@")
}

func TestGhostExporter_ConvertTag(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	tag := models.WordPressTag{
		ID:          1,
		Name:        "Technology",
		Slug:        "technology",
		Description: "Tech posts",
	}

	ghostTag := exporter.convertTag(tag)

	assert.Equal(t, "tag-1", ghostTag.ID)
	assert.Equal(t, "Technology", ghostTag.Name)
	assert.Equal(t, "technology", ghostTag.Slug)
	assert.Equal(t, "Tech posts", ghostTag.Description)
}

func TestGhostExporter_ConvertCategoryToTag(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	category := models.WordPressCategory{
		ID:          1,
		Name:        "News",
		Slug:        "news",
		Description: "News articles",
	}

	ghostTag := exporter.convertCategoryToTag(category)

	assert.Equal(t, "cat-1", ghostTag.ID)
	assert.Equal(t, "News", ghostTag.Name)
	assert.Equal(t, "news", ghostTag.Slug)
	assert.Equal(t, "News articles", ghostTag.Description)
}

func TestGhostExporter_ConvertPost(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	userMap := map[int]string{1: "1"}

	post := models.WordPressPost{
		ID:       1,
		Slug:     "test-post",
		Status:   "publish",
		Title:    models.RenderedContent{Rendered: "Test Post Title"},
		Content:  models.RenderedContent{Rendered: "<p>Test content</p>"},
		Excerpt:  models.RenderedContent{Rendered: "<p>Test excerpt</p>"},
		Author:   1,
		Date:     models.WordPressTime{Time: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)},
		Modified: models.WordPressTime{Time: time.Date(2024, 1, 20, 12, 0, 0, 0, time.UTC)},
		Sticky:   true,
		SEO: models.SEOData{
			Title:           "SEO Title",
			MetaDescription: "SEO Description",
		},
	}

	ghostPost := exporter.convertPost(post, userMap, "post")

	assert.Equal(t, "1", ghostPost.ID)
	assert.Equal(t, "Test Post Title", ghostPost.Title)
	assert.Equal(t, "test-post", ghostPost.Slug)
	assert.Equal(t, "<p>Test content</p>", ghostPost.HTML)
	assert.Equal(t, "Test excerpt", ghostPost.CustomExcerpt)
	assert.Equal(t, "published", ghostPost.Status)
	assert.Equal(t, 1, ghostPost.Featured)
	assert.Equal(t, "1", ghostPost.AuthorID)
	assert.Equal(t, "SEO Title", ghostPost.MetaTitle)
	assert.Equal(t, "SEO Description", ghostPost.MetaDescription)
	assert.Equal(t, "post", ghostPost.Type)
	assert.Greater(t, ghostPost.PublishedAt, int64(0))
}

func TestGhostExporter_ConvertPostDraft(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	post := models.WordPressPost{
		ID:     1,
		Slug:   "draft-post",
		Status: "draft",
		Title:  models.RenderedContent{Rendered: "Draft Post"},
	}

	ghostPost := exporter.convertPost(post, nil, "post")

	assert.Equal(t, "draft", ghostPost.Status)
	assert.Equal(t, int64(0), ghostPost.PublishedAt)
}

func TestGhostExporter_ConvertPage(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	userMap := map[int]string{1: "1"}

	page := models.WordPressPost{
		ID:       1,
		Slug:     "about-us",
		Status:   "publish",
		Title:    models.RenderedContent{Rendered: "About Us"},
		Content:  models.RenderedContent{Rendered: "<p>About content</p>"},
		Author:   1,
		Modified: models.WordPressTime{Time: time.Now()},
	}

	ghostPost := exporter.convertPost(page, userMap, "page")

	assert.Equal(t, "About Us", ghostPost.Title)
	assert.Equal(t, "page", ghostPost.Type)
	assert.Equal(t, "published", ghostPost.Status)
}

func TestGhostExporter_BuildGhostData(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

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
			{ID: 1, Slug: "page-1", Status: "publish", Title: models.RenderedContent{Rendered: "Page 1"}, Author: 1},
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
	}

	ghostData := exporter.buildGhostData(data)

	assert.Len(t, ghostData.DB, 1)
	assert.Equal(t, "5.0.0", ghostData.DB[0].Meta.Version)

	assert.Len(t, ghostData.DB[0].Data.Users, 1)
	assert.Len(t, ghostData.DB[0].Data.Tags, 2)  // 1 tag + 1 category
	assert.Len(t, ghostData.DB[0].Data.Posts, 2) // 1 post + 1 page
	assert.Len(t, ghostData.DB[0].Data.PostsAuthors, 2)
}

func TestGhostExporter_BuildGhostDataDefaultAuthor(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{ID: 1, Slug: "post-1", Status: "publish", Title: models.RenderedContent{Rendered: "Post 1"}},
		},
		// No users - should create default author
	}

	ghostData := exporter.buildGhostData(data)

	assert.Len(t, ghostData.DB[0].Data.Users, 1)
	assert.Equal(t, "Admin", ghostData.DB[0].Data.Users[0].Name)
}

func TestGhostExporter_Export(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ghost-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	cfg := &config.Config{
		Output: tempDir,
		Format: "ghost",
	}
	exporter := NewGhostExporter(cfg)

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
				Author:  1,
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
	}

	err = exporter.Export(testData)
	require.NoError(t, err)

	outputFile := filepath.Join(tempDir, "ghost_export.json")
	assert.FileExists(t, outputFile)

	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	var ghostData GhostExportData
	err = json.Unmarshal(content, &ghostData)
	require.NoError(t, err)

	assert.Len(t, ghostData.DB, 1)
	assert.Len(t, ghostData.DB[0].Data.Posts, 2)
	assert.Len(t, ghostData.DB[0].Data.Users, 1)
}

func TestGhostExporter_ExportWithJSONExtension(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "ghost-export-test")
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	outputFile := filepath.Join(tempDir, "custom_ghost.json")
	cfg := &config.Config{
		Output: outputFile,
		Format: "ghost",
	}
	exporter := NewGhostExporter(cfg)

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

func TestGhostExporter_EmptyData(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Empty Site",
			URL:  "https://example.com",
		},
	}

	ghostData := exporter.buildGhostData(data)

	assert.Len(t, ghostData.DB[0].Data.Posts, 0)
	assert.Len(t, ghostData.DB[0].Data.Tags, 0)
	// Should have default user
	assert.Len(t, ghostData.DB[0].Data.Users, 1)
}

func TestGhostExporter_PostTagRelationships(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{
				ID:         1,
				Slug:       "post-with-tags",
				Status:     "publish",
				Title:      models.RenderedContent{Rendered: "Post with Tags"},
				Tags:       []int{1, 2},
				Categories: []int{1},
				Author:     1,
			},
		},
		Tags: []models.WordPressTag{
			{ID: 1, Name: "Tag 1", Slug: "tag-1"},
			{ID: 2, Name: "Tag 2", Slug: "tag-2"},
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Cat 1", Slug: "cat-1"},
		},
		Users: []models.WordPressUser{
			{ID: 1, Name: "User 1", Slug: "user-1"},
		},
	}

	ghostData := exporter.buildGhostData(data)

	// Should have 2 tags + 1 category = 3 tags
	assert.Len(t, ghostData.DB[0].Data.Tags, 3)

	// Should have 3 post-tag relationships (2 tags + 1 category)
	assert.Len(t, ghostData.DB[0].Data.PostsTags, 3)
}

func TestGhostExporter_GenerateGhostUUID(t *testing.T) {
	uuid := generateGhostUUID(123)

	assert.NotEmpty(t, uuid)
	assert.Contains(t, uuid, "-")
}

func TestGhostExporter_ConvertPostWithManyTags(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	userMap := map[int]string{1: "1"}

	post := models.WordPressPost{
		ID:         1,
		Slug:       "tagged-post",
		Status:     "publish",
		Title:      models.RenderedContent{Rendered: "Post with Tags"},
		Tags:       []int{1, 2, 3},
		Categories: []int{1, 2},
		Author:     1,
	}

	ghostPost := exporter.convertPost(post, userMap, "post")

	assert.Equal(t, "Post with Tags", ghostPost.Title)
	assert.Equal(t, "1", ghostPost.AuthorID)
}

func TestGhostExporter_BuildGhostDataWithRelationships(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{
				ID:         1,
				Slug:       "post-1",
				Status:     "publish",
				Title:      models.RenderedContent{Rendered: "Post 1"},
				Author:     1,
				Tags:       []int{1},
				Categories: []int{1},
			},
		},
		Tags: []models.WordPressTag{
			{ID: 1, Name: "Tag 1", Slug: "tag-1"},
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Cat 1", Slug: "cat-1"},
		},
		Users: []models.WordPressUser{
			{ID: 1, Name: "User 1", Slug: "user-1"},
		},
	}

	ghostData := exporter.buildGhostData(data)

	// Check post-tag relationships
	assert.Greater(t, len(ghostData.DB[0].Data.PostsTags), 0)
	assert.Greater(t, len(ghostData.DB[0].Data.PostsAuthors), 0)
}

func TestGhostExporter_ConvertUserWithEmail(t *testing.T) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

	user := models.WordPressUser{
		ID:          1,
		Name:        "Test User",
		Slug:        "test-user",
		Description: "A test user",
	}

	ghostUser := exporter.convertUser(user)

	assert.Equal(t, "Test User", ghostUser.Name)
	assert.Contains(t, ghostUser.Email, "@")
	assert.Equal(t, "test-user", ghostUser.Slug)
}

// Benchmark tests
func BenchmarkGhostExporter_BuildGhostData(b *testing.B) {
	cfg := &config.Config{}
	exporter := NewGhostExporter(cfg)

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
		_ = exporter.buildGhostData(data)
	}
}
