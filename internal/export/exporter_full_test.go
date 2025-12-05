package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestExportJSON(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	outputFile := filepath.Join(tmpDir, "export.json")

	cfg := &config.Config{
		Output:        outputFile,
		Format:        "json",
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
				ID:   1,
				Slug: "test-post",
				Title: models.RenderedContent{
					Rendered: "Test Post",
				},
			},
		},
		Stats: models.ExportStats{
			TotalPosts: 1,
		},
	}

	err = e.Export(data)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("Export() should create output file")
	}

	// Verify JSON content
	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var exported models.ExportData
	if err := json.Unmarshal(content, &exported); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if exported.Site.Name != "Test Site" {
		t.Errorf("Export() site name = %v, want %v", exported.Site.Name, "Test Site")
	}

	if len(exported.Posts) != 1 {
		t.Errorf("Export() posts count = %v, want %v", len(exported.Posts), 1)
	}
}

func TestExportMarkdown(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "markdown",
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
				ID:   1,
				Slug: "test-post",
				Title: models.RenderedContent{
					Rendered: "Test Post",
				},
				Content: models.RenderedContent{
					Rendered: "<p>Test content</p>",
				},
				Date:     models.WordPressTime{Time: time.Now()},
				Modified: models.WordPressTime{Time: time.Now()},
				Status:   "publish",
				Link:     "https://example.com/test-post",
			},
		},
		Categories: []models.WordPressCategory{
			{
				ID:   1,
				Name: "Technology",
				Slug: "technology",
			},
		},
		Stats: models.ExportStats{
			TotalPosts: 1,
		},
	}

	err = e.Export(data)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Verify directory structure was created
	postsDir := filepath.Join(tmpDir, "posts")
	if _, err := os.Stat(postsDir); os.IsNotExist(err) {
		t.Error("Export() should create posts directory")
	}
}

func TestExportUnsupportedFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output:        tmpDir,
		Format:        "xml", // Unsupported format
		DownloadMedia: false,
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
		},
	}

	err = e.Export(data)
	if err == nil {
		t.Error("Export() should return error for unsupported format")
	}
}

func TestGetCategoryPath(t *testing.T) {
	e := NewExporter(&config.Config{})

	categoryMap := map[int]models.WordPressCategory{
		1: {ID: 1, Name: "Technology", Slug: "technology", Parent: 0},
		2: {ID: 2, Name: "Programming", Slug: "programming", Parent: 1},
		3: {ID: 3, Name: "Posts", Slug: "posts", Parent: 0},
	}

	hierarchy := map[int][]string{
		1: {"technology"},
		2: {"technology", "programming"},
		3: {"posts"},
	}

	tests := []struct {
		name     string
		post     models.WordPressPost
		expected string
	}{
		{
			name: "Post with category",
			post: models.WordPressPost{
				ID:         1,
				Categories: []int{1},
				Link:       "https://example.com/test-post",
			},
			expected: "technology",
		},
		{
			name: "Post with nested category",
			post: models.WordPressPost{
				ID:         2,
				Categories: []int{2},
				Link:       "https://example.com/test-post",
			},
			expected: "technology" + string(filepath.Separator) + "programming",
		},
		{
			name: "Post without categories",
			post: models.WordPressPost{
				ID:         3,
				Categories: []int{},
				Link:       "https://example.com/test-post",
			},
			expected: "uncategorized",
		},
		{
			name: "Post with posts category (should be uncategorized)",
			post: models.WordPressPost{
				ID:         4,
				Categories: []int{3},
				Link:       "https://example.com/test-post",
			},
			expected: "uncategorized",
		},
		{
			name: "Post with category in link",
			post: models.WordPressPost{
				ID:         5,
				Categories: []int{},
				Link:       "https://example.com/category/news/test-post",
			},
			expected: "news",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.getCategoryPath(tt.post, categoryMap, hierarchy)
			if result != tt.expected {
				t.Errorf("getCategoryPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExportSiteInfo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "markdown",
	}

	e := NewExporter(cfg)

	site := models.SiteInfo{
		Name:        "Test Site",
		Description: "A test site",
		URL:         "https://example.com",
	}

	err = e.exportSiteInfo(site)
	if err != nil {
		t.Fatalf("exportSiteInfo() error = %v", err)
	}

	// Verify README.md was created
	readmeFile := filepath.Join(tmpDir, "README.md")
	if _, err := os.Stat(readmeFile); os.IsNotExist(err) {
		t.Error("exportSiteInfo() should create README.md file")
	}
}

func TestExportMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "markdown",
	}

	e := NewExporter(cfg)

	data := &models.ExportData{
		Site: models.SiteInfo{
			Name: "Test Site",
		},
		Categories: []models.WordPressCategory{
			{ID: 1, Name: "Tech", Slug: "tech"},
		},
		Tags: []models.WordPressTag{
			{ID: 1, Name: "Go", Slug: "go"},
		},
		Users: []models.WordPressUser{
			{ID: 1, Name: "Admin", Slug: "admin"},
		},
		Stats: models.ExportStats{
			TotalPosts: 10,
		},
	}

	err = e.exportMetadata(data)
	if err != nil {
		t.Fatalf("exportMetadata() error = %v", err)
	}

	// Verify metadata.json was created
	metadataFile := filepath.Join(tmpDir, "metadata.json")
	if _, err := os.Stat(metadataFile); os.IsNotExist(err) {
		t.Error("exportMetadata() should create metadata.json file")
	}
}

func TestExportPostsWithCategories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "markdown",
	}

	e := NewExporter(cfg)

	posts := []models.WordPressPost{
		{
			ID:   1,
			Slug: "test-post",
			Title: models.RenderedContent{
				Rendered: "Test Post",
			},
			Content: models.RenderedContent{
				Rendered: "<p>Content</p>",
			},
			Date:       models.WordPressTime{Time: time.Now()},
			Modified:   models.WordPressTime{Time: time.Now()},
			Status:     "publish",
			Link:       "https://example.com/technology/test-post",
			Categories: []int{1},
		},
	}

	categories := []models.WordPressCategory{
		{ID: 1, Name: "Technology", Slug: "technology", Parent: 0},
	}

	err = e.exportPostsWithCategories(posts, categories, "post")
	if err != nil {
		t.Fatalf("exportPostsWithCategories() error = %v", err)
	}

	// Verify posts directory was created
	postsDir := filepath.Join(tmpDir, "posts")
	if _, err := os.Stat(postsDir); os.IsNotExist(err) {
		t.Error("exportPostsWithCategories() should create posts directory")
	}
}

func TestExportPostsMarkdown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "export_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	cfg := &config.Config{
		Output: tmpDir,
		Format: "markdown",
	}

	e := NewExporter(cfg)

	posts := []models.WordPressPost{
		{
			ID:   1,
			Slug: "test-post",
			Title: models.RenderedContent{
				Rendered: "Test Post",
			},
			Content: models.RenderedContent{
				Rendered: "<p>Content</p>",
			},
			Date:     models.WordPressTime{Time: time.Now()},
			Modified: models.WordPressTime{Time: time.Now()},
			Status:   "publish",
			Link:     "https://example.com/test-post",
		},
	}

	// Create the directory first
	postsDir := filepath.Join(tmpDir, "posts")
	if err := os.MkdirAll(postsDir, 0755); err != nil {
		t.Fatalf("Failed to create posts dir: %v", err)
	}

	err = e.exportPostsMarkdown(posts, postsDir, "post")
	if err != nil {
		t.Fatalf("exportPostsMarkdown() error = %v", err)
	}

	// Verify markdown file was created
	files, err := os.ReadDir(postsDir)
	if err != nil {
		t.Fatalf("Failed to read posts dir: %v", err)
	}

	if len(files) == 0 {
		t.Error("exportPostsMarkdown() should create markdown files")
	}
}

func TestBuildCategoryHierarchyDetailed(t *testing.T) {
	e := NewExporter(&config.Config{})

	tests := []struct {
		name       string
		categories []models.WordPressCategory
		wantLen    int
	}{
		{
			name: "Simple hierarchy",
			categories: []models.WordPressCategory{
				{ID: 1, Name: "Parent", Slug: "parent", Parent: 0},
				{ID: 2, Name: "Child", Slug: "child", Parent: 1},
			},
			wantLen: 2,
		},
		{
			name: "Deep hierarchy",
			categories: []models.WordPressCategory{
				{ID: 1, Name: "Level1", Slug: "level1", Parent: 0},
				{ID: 2, Name: "Level2", Slug: "level2", Parent: 1},
				{ID: 3, Name: "Level3", Slug: "level3", Parent: 2},
			},
			wantLen: 3,
		},
		{
			name:       "Empty categories",
			categories: []models.WordPressCategory{},
			wantLen:    0,
		},
		{
			name: "Single category",
			categories: []models.WordPressCategory{
				{ID: 1, Name: "Single", Slug: "single", Parent: 0},
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.buildCategoryHierarchy(tt.categories)
			if len(result) != tt.wantLen {
				t.Errorf("buildCategoryHierarchy() len = %v, want %v", len(result), tt.wantLen)
			}
		})
	}
}

func TestSanitizeDirectoryNameFull(t *testing.T) {
	e := NewExporter(&config.Config{})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Normal name",
			input:    "technology",
			expected: "technology",
		},
		{
			name:     "Name with spaces",
			input:    "web development",
			expected: "web-development",
		},
		{
			name:     "Name with special chars",
			input:    "C++/C#",
			expected: "C++-C#",
		},
		{
			name:     "Name with ampersand",
			input:    "News & Updates",
			expected: "News-&-Updates",
		},
		{
			name:     "Empty name",
			input:    "",
			expected: "category",
		},
		{
			name:     "Name with dots",
			input:    "version.1.0",
			expected: "version.1.0",
		},
		{
			name:     "Name with underscores",
			input:    "my_category",
			expected: "my_category",
		},
		{
			name:     "Name with hyphens",
			input:    "my-category",
			expected: "my-category",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.sanitizeDirectoryName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeDirectoryName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsNumericFull(t *testing.T) {
	e := NewExporter(&config.Config{})

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Numeric string",
			input:    "12345",
			expected: true,
		},
		{
			name:     "Single digit",
			input:    "5",
			expected: true,
		},
		{
			name:     "Zero",
			input:    "0",
			expected: true,
		},
		{
			name:     "Alphabetic string",
			input:    "abc",
			expected: false,
		},
		{
			name:     "Mixed string",
			input:    "123abc",
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "String with spaces",
			input:    "123 456",
			expected: false,
		},
		{
			name:     "Negative number (string)",
			input:    "-123",
			expected: false,
		},
		{
			name:     "Decimal number",
			input:    "12.34",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.isNumeric(tt.input)
			if result != tt.expected {
				t.Errorf("isNumeric() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestExportWithDownloadMedia is skipped because it requires network access
// and would cause timeouts in CI environments

func TestGenerateMarkdownContentWithoutOptionalFields(t *testing.T) {
	e := NewExporter(&config.Config{})

	// Post without optional fields
	post := models.WordPressPost{
		ID:   1,
		Slug: "minimal-post",
		Title: models.RenderedContent{
			Rendered: "Minimal Post",
		},
		Content: models.RenderedContent{
			Rendered: "<p>Content</p>",
		},
		Date:     models.WordPressTime{Time: time.Now()},
		Modified: models.WordPressTime{Time: time.Now()},
		Status:   "publish",
		Link:     "https://example.com/minimal-post",
		// No Author, FeaturedMedia, Categories, or Tags
	}

	result := e.generateMarkdownContent(post, "post")

	// Should not contain author or featured_media
	if containsStr(result, "author:") {
		t.Error("generateMarkdownContent() should not contain author when not set")
	}
	if containsStr(result, "featured_media:") {
		t.Error("generateMarkdownContent() should not contain featured_media when not set")
	}
	if containsStr(result, "categories:") {
		t.Error("generateMarkdownContent() should not contain categories when empty")
	}
	if containsStr(result, "tags:") {
		t.Error("generateMarkdownContent() should not contain tags when empty")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
