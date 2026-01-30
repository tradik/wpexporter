package export

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestExport_ShopifyFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_shopify_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "shopify",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "test-post",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "Test Post"},
				Content: models.RenderedContent{Rendered: "<p>Content</p>"},
			},
		},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err = e.Export(data)
	require.NoError(t, err)

	// Verify Shopify CSV files were created
	assert.FileExists(t, filepath.Join(tmpDir, "shopify_posts.csv"))
	assert.FileExists(t, filepath.Join(tmpDir, "shopify_products.csv"))
	assert.FileExists(t, filepath.Join(tmpDir, "shopify_metadata.csv"))
}

func TestExport_MagentoFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_magento_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "magento",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
			URL:  "https://example.com",
		},
		Posts: []models.WordPressPost{
			{
				ID:      1,
				Slug:    "test-post",
				Status:  "publish",
				Title:   models.RenderedContent{Rendered: "Test Post"},
				Content: models.RenderedContent{Rendered: "<p>Content</p>"},
			},
		},
		Stats: models.ExportStats{TotalPosts: 1},
	}

	err = e.Export(data)
	require.NoError(t, err)

	// Verify Magento CSV files were created
	assert.FileExists(t, filepath.Join(tmpDir, "magento_posts.csv"))
	assert.FileExists(t, filepath.Join(tmpDir, "magento_products.csv"))
	assert.FileExists(t, filepath.Join(tmpDir, "magento_metadata.csv"))
}

func TestUpdateMediaPaths_WithDownloadEnabled(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_media_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		DownloadMedia: true,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Posts: []models.WordPressPost{
			{
				ID: 1,
				Content: models.RenderedContent{
					Rendered: `<img src="https://example.com/image.jpg">`,
				},
				Excerpt: models.RenderedContent{
					Rendered: `<img src="https://example.com/thumb.jpg">`,
				},
			},
		},
		Pages: []models.WordPressPost{
			{
				ID: 2,
				Content: models.RenderedContent{
					Rendered: `<img src="https://example.com/page-img.jpg">`,
				},
				Excerpt: models.RenderedContent{
					Rendered: `Summary text`,
				},
			},
		},
		Media: []models.WordPressMedia{
			{ID: 100, SourceURL: "https://example.com/image.jpg"},
		},
	}

	// Should not panic
	e.updateMediaPaths(data)
}

func TestUpdateMediaPaths_Disabled(t *testing.T) {
	cfg := &config.Config{
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Posts: []models.WordPressPost{{ID: 1}},
	}

	// Should return early without modification
	e.updateMediaPaths(data)
}

func TestGetCategoryPath_UnknownCategoryID(t *testing.T) {
	e := NewExporter(&config.Config{})

	categoryMap := map[int]models.WordPressCategory{}
	hierarchy := map[int][]string{}

	post := models.WordPressPost{
		ID:         1,
		Categories: []int{999}, // Non-existent category
		Link:       "https://example.com/post",
	}

	result := e.getCategoryPath(post, categoryMap, hierarchy)
	assert.Equal(t, "uncategorized", result)
}

func TestGetCategoryPath_CategoryWithPostsSlug(t *testing.T) {
	e := NewExporter(&config.Config{})

	categoryMap := map[int]models.WordPressCategory{
		1: {ID: 1, Name: "Posts", Slug: "posts", Parent: 0},
	}
	hierarchy := map[int][]string{}

	post := models.WordPressPost{
		ID:         1,
		Categories: []int{1},
		Link:       "https://example.com/post",
	}

	result := e.getCategoryPath(post, categoryMap, hierarchy)
	assert.Equal(t, "uncategorized", result)
}

func TestGetCategoryPath_LinkBasedExtraction(t *testing.T) {
	e := NewExporter(&config.Config{})

	categoryMap := map[int]models.WordPressCategory{}
	hierarchy := map[int][]string{}

	post := models.WordPressPost{
		ID:         1,
		Categories: []int{},
		Link:       "https://example.com/category/tech/my-post",
	}

	result := e.getCategoryPath(post, categoryMap, hierarchy)
	assert.Equal(t, "tech", result)
}

func TestGenerateMarkdownContent_WithSEOFields(t *testing.T) {
	e := NewExporter(&config.Config{})

	post := models.WordPressPost{
		ID:       1,
		Slug:     "seo-post",
		Title:    models.RenderedContent{Rendered: "SEO Post"},
		Status:   "publish",
		Date:     models.WordPressTime{Time: time.Now()},
		Modified: models.WordPressTime{Time: time.Now()},
		Link:     "https://example.com/seo-post",
		Content:  models.RenderedContent{Rendered: "<p>Content</p>"},
		SEO: models.SEOData{
			Title:           "Custom SEO Title",
			MetaDescription: "Meta description here",
			MetaKeywords:    "keyword1, keyword2",
			OGTitle:         "OG Title",
			OGDescription:   "OG Description",
			OGImage:         "https://example.com/og.jpg",
			CanonicalURL:    "https://example.com/canonical",
		},
	}

	result := e.generateMarkdownContent(post, "post")

	assert.Contains(t, result, `seo_title: "Custom SEO Title"`)
	assert.Contains(t, result, `meta_description: "Meta description here"`)
	assert.Contains(t, result, `meta_keywords: "keyword1, keyword2"`)
	assert.Contains(t, result, `og_title: "OG Title"`)
	assert.Contains(t, result, `og_description: "OG Description"`)
	assert.Contains(t, result, `og_image: "https://example.com/og.jpg"`)
	assert.Contains(t, result, `canonical_url: "https://example.com/canonical"`)
}

func TestGenerateMarkdownFilename_WithSlashes(t *testing.T) {
	e := NewExporter(&config.Config{})

	post := models.WordPressPost{
		ID:   1,
		Slug: "path/to/post",
	}

	result := e.generateMarkdownFilename(post)
	assert.Equal(t, "path-to-post.md", result)
}

func TestGenerateMarkdownFilename_WithBackslashes(t *testing.T) {
	e := NewExporter(&config.Config{})

	post := models.WordPressPost{
		ID:   1,
		Slug: "path\\to\\post",
	}

	result := e.generateMarkdownFilename(post)
	assert.Equal(t, "path-to-post.md", result)
}

func TestGenerateMarkdownFilename_WithColons(t *testing.T) {
	e := NewExporter(&config.Config{})

	post := models.WordPressPost{
		ID:   1,
		Slug: "post:title",
	}

	result := e.generateMarkdownFilename(post)
	assert.Equal(t, "post-title.md", result)
}

func TestExportJSON_ToDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_json_dir_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "json",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Stats: models.ExportStats{},
	}

	err = e.Export(data)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(tmpDir, "export.json"))
}

func TestExportJSON_ToFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_json_file_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	outputFile := filepath.Join(tmpDir, "custom.json")

	cfg := &config.Config{
		Output:        outputFile,
		Format:        "json",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Stats: models.ExportStats{},
	}

	err = e.Export(data)
	require.NoError(t, err)

	assert.FileExists(t, outputFile)
}

func TestExportMarkdown_WithAllContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_md_full_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "markdown",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name:        "Test Site",
			Description: "Description",
			URL:         "https://example.com",
			HomeURL:     "https://example.com",
			AdminEmail:  "admin@example.com",
			Timezone:    "UTC",
			Language:    "en",
		},
		Posts: []models.WordPressPost{
			{
				ID:         1,
				Slug:       "post-1",
				Title:      models.RenderedContent{Rendered: "Post 1"},
				Content:    models.RenderedContent{Rendered: "<p>Content</p>"},
				Status:     "publish",
				Date:       models.WordPressTime{Time: time.Now()},
				Modified:   models.WordPressTime{Time: time.Now()},
				Link:       "https://example.com/tech/post-1",
				Categories: []int{1},
			},
		},
		Pages: []models.WordPressPost{
			{
				ID:       2,
				Slug:     "page-1",
				Title:    models.RenderedContent{Rendered: "Page 1"},
				Content:  models.RenderedContent{Rendered: "<p>Page content</p>"},
				Status:   "publish",
				Date:     models.WordPressTime{Time: time.Now()},
				Modified: models.WordPressTime{Time: time.Now()},
				Link:     "https://example.com/page-1",
			},
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Tech", Slug: "tech", Parent: 0},
		},
		Tags:  []models.WordPressTag{},
		Users: []models.WordPressUser{},
		Media: []models.WordPressMedia{},
		Stats: models.ExportStats{
			TotalPosts: 1,
			TotalPages: 1,
		},
	}

	err = e.Export(data)
	require.NoError(t, err)

	// Verify files were created
	assert.FileExists(t, filepath.Join(tmpDir, "README.md"))
	assert.FileExists(t, filepath.Join(tmpDir, "metadata.json"))
	assert.DirExists(t, filepath.Join(tmpDir, "pages"))
}

func TestBuildCategoryHierarchy_WithMissingParent(t *testing.T) {
	e := NewExporter(&config.Config{})

	categories := []models.WordPressCategory{
		{ID: 2, Name: "Child", Slug: "child", Parent: 999}, // Parent doesn't exist
	}

	hierarchy := e.buildCategoryHierarchy(categories)

	// Should handle missing parent gracefully
	assert.NotNil(t, hierarchy)
}

func TestExtractCategoriesFromLink_RootPath(t *testing.T) {
	e := NewExporter(&config.Config{})

	result := e.extractCategoriesFromLink("https://example.com/")
	assert.Equal(t, "", result)
}

func TestExtractCategoriesFromLink_PathOnly(t *testing.T) {
	e := NewExporter(&config.Config{})

	result := e.extractCategoriesFromLink("https://example.com")
	assert.Equal(t, "", result)
}

func TestExtractCategoriesFromLink_ArchivesSegment(t *testing.T) {
	e := NewExporter(&config.Config{})

	result := e.extractCategoriesFromLink("https://example.com/archives/tech/post")
	assert.Equal(t, "tech", result)
}

func TestSanitizeDirectoryName_MultipleInvalidChars(t *testing.T) {
	e := NewExporter(&config.Config{})

	tests := []struct {
		input    string
		expected string
	}{
		{`file<>:"/\|?*name`, "file-name"},
		{"--leading--trailing--", "leading-trailing"},
		{"   only spaces   ", "only-spaces"},
		{"///", "category"},
	}

	for _, tt := range tests {
		result := e.sanitizeDirectoryName(tt.input)
		assert.Equal(t, tt.expected, result, "input: %s", tt.input)
	}
}

func TestExport_InvalidOutputDir(t *testing.T) {
	cfg := &config.Config{
		Output:        "/nonexistent/path/that/should/fail",
		Format:        "json",
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site:  models.SiteInfo{Name: "Test"},
		Stats: models.ExportStats{},
	}

	err := e.Export(data)
	assert.Error(t, err)
}

func TestExportPostsMarkdown_WriteError(t *testing.T) {
	e := NewExporter(&config.Config{})

	posts := []models.WordPressPost{
		{
			ID:       1,
			Slug:     "test",
			Title:    models.RenderedContent{Rendered: "Test"},
			Content:  models.RenderedContent{Rendered: "<p>Content</p>"},
			Status:   "publish",
			Date:     models.WordPressTime{Time: time.Now()},
			Modified: models.WordPressTime{Time: time.Now()},
		},
	}

	// Try to write to a non-existent directory
	err := e.exportPostsMarkdown(posts, "/nonexistent/path", "post")
	assert.Error(t, err)
}

func TestExportPostsWithCategories_WriteError(t *testing.T) {
	cfg := &config.Config{
		Output: "/nonexistent/path",
	}
	e := NewExporter(cfg)

	posts := []models.WordPressPost{
		{
			ID:       1,
			Slug:     "test",
			Title:    models.RenderedContent{Rendered: "Test"},
			Content:  models.RenderedContent{Rendered: "<p>Content</p>"},
			Status:   "publish",
			Date:     models.WordPressTime{Time: time.Now()},
			Modified: models.WordPressTime{Time: time.Now()},
			Link:     "https://example.com/test",
		},
	}

	err := e.exportPostsWithCategories(posts, nil, "post")
	assert.Error(t, err)
}

func TestConvertHTMLToMarkdown_AllTags(t *testing.T) {
	e := NewExporter(&config.Config{})

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"h4", "<h4>Heading 4</h4>", "#### Heading 4"},
		{"h5", "<h5>Heading 5</h5>", "##### Heading 5"},
		{"h6", "<h6>Heading 6</h6>", "###### Heading 6"},
		{"b", "<b>bold</b>", "**bold**"},
		{"i", "<i>italic</i>", "*italic*"},
		{"br with slash", "line<br/>break", "line\nbreak"},
		{"br with space", "line<br />break", "line\nbreak"},
		{"ol", "<ol><li>item</li></ol>", "- item"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.convertHTMLToMarkdown(tt.input)
			assert.Contains(t, result, tt.contains)
		})
	}
}
