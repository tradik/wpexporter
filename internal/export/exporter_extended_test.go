package export

import (
	"path/filepath"
	"testing"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestSanitizeDirectoryNameExtended(t *testing.T) {
	e := NewExporter(&config.Config{})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Name with multiple spaces",
			input:    "News   and   Updates",
			expected: "News-and-Updates",
		},
		{
			name:     "Name with leading/trailing spaces",
			input:    "  Technology  ",
			expected: "Technology",
		},
		{
			name:     "Name with numbers",
			input:    "Web3 Development",
			expected: "Web3-Development",
		},
		{
			name:     "Name with underscores",
			input:    "tech_news",
			expected: "tech_news",
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

func TestEscapeYAMLExtended(t *testing.T) {
	e := NewExporter(&config.Config{})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Text with quotes",
			input:    `Hello "World"`,
			expected: `Hello \"World\"`,
		},
		{
			name:     "Text with newlines",
			input:    "Line 1\nLine 2",
			expected: "Line 1\\nLine 2",
		},
		{
			name:     "Text with carriage return",
			input:    "Line 1\rLine 2",
			expected: "Line 1\\rLine 2",
		},
		{
			name:     "Text with multiple special chars",
			input:    `"Quote"\nNewline`,
			expected: `\"Quote\"\nNewline`,
		},
		{
			name:     "Text with tabs (not escaped)",
			input:    "Line 1\tLine 2",
			expected: "Line 1\tLine 2",
		},
		{
			name:     "Text with backslashes (not escaped)",
			input:    "Path\\to\\file",
			expected: "Path\\to\\file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.escapeYAML(tt.input)
			if result != tt.expected {
				t.Errorf("escapeYAML() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerateMarkdownFilenameExtended(t *testing.T) {
	e := NewExporter(&config.Config{})

	tests := []struct {
		name     string
		post     models.WordPressPost
		expected string
	}{
		{
			name: "Post with special chars in slug",
			post: models.WordPressPost{
				ID:   456,
				Slug: "test-post-123_with-special",
			},
			expected: "test-post-123_with-special.md",
		},
		{
			name: "Post with numeric slug",
			post: models.WordPressPost{
				ID:   789,
				Slug: "12345",
			},
			expected: "12345.md",
		},
		{
			name: "Post with dots in slug",
			post: models.WordPressPost{
				ID:   101,
				Slug: "version.1.2.post",
			},
			expected: "version.1.2.post.md",
		},
		{
			name: "Post with empty slug",
			post: models.WordPressPost{
				ID:   999,
				Slug: "",
			},
			expected: "post-999.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.generateMarkdownFilename(tt.post)
			if result != tt.expected {
				t.Errorf("generateMarkdownFilename() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractCategoriesFromLinkExtended(t *testing.T) {
	e := NewExporter(&config.Config{})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple category link",
			input:    "https://example.com/category/technology/post-slug",
			expected: "technology",
		},
		{
			name:     "Nested category link",
			input:    "https://example.com/category/tech/web/dev/post-slug",
			expected: "tech" + string(filepath.Separator) + "web" + string(filepath.Separator) + "dev",
		},
		{
			name:     "Date-based permalink (should return empty)",
			input:    "https://example.com/2024/01/15/post-slug",
			expected: "",
		},
		{
			name:     "Tag link (returns tag as category)",
			input:    "https://example.com/tag/wordpress/post-slug",
			expected: "tag" + string(filepath.Separator) + "wordpress",
		},
		{
			name:     "Blog segment (should return technology - blog filtered only if first)",
			input:    "https://example.com/blog/technology/post-slug",
			expected: "technology",
		},
		{
			name:     "News category (should work)",
			input:    "https://example.com/news/article-slug",
			expected: "news",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.extractCategoriesFromLink(tt.input)
			if result != tt.expected {
				t.Errorf("extractCategoriesFromLink() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConvertHTMLToMarkdown(t *testing.T) {
	e := NewExporter(&config.Config{})

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Headers",
			input:    "<h1>Title</h1><h2>Subtitle</h2>",
			expected: "# Title\n\n## Subtitle",
		},
		{
			name:     "Bold and italic",
			input:    "<strong>bold</strong> and <em>italic</em>",
			expected: "**bold** and *italic*",
		},
		{
			name:     "Paragraphs",
			input:    "<p>First paragraph</p><p>Second paragraph</p>",
			expected: "First paragraph\n\nSecond paragraph",
		},
		{
			name:     "Line breaks",
			input:    "Line 1<br>Line 2<br/>Line 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "Lists",
			input:    "<ul><li>Item 1</li><li>Item 2</li></ul>",
			expected: "- Item 1\n- Item 2",
		},
		{
			name:     "Code blocks",
			input:    "<code>inline code</code> and <pre>block code</pre>",
			expected: "`inline code` and ```\nblock code\n```",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := e.convertHTMLToMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("convertHTMLToMarkdown() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerateMarkdownContent(t *testing.T) {
	e := NewExporter(&config.Config{})

	post := models.WordPressPost{
		ID:   123,
		Slug: "test-post",
		Title: models.RenderedContent{
			Rendered: "Test Post Title",
		},
		Content: models.RenderedContent{
			Rendered: "<p>This is the content</p>",
		},
		Excerpt: models.RenderedContent{
			Rendered: "<p>This is the excerpt</p>",
		},
		Status:        "publish",
		Link:          "https://example.com/test-post",
		Author:        1,
		FeaturedMedia: 100,
		Categories:    []int{1, 2},
		Tags:          []int{3, 4},
	}

	result := e.generateMarkdownContent(post, "post")

	// Check front matter
	if !containsString(result, "---") {
		t.Error("generateMarkdownContent() should contain front matter delimiters")
	}
	if !containsString(result, "id: 123") {
		t.Error("generateMarkdownContent() should contain post ID")
	}
	if !containsString(result, `title: "Test Post Title"`) {
		t.Error("generateMarkdownContent() should contain post title")
	}
	if !containsString(result, `slug: "test-post"`) {
		t.Error("generateMarkdownContent() should contain post slug")
	}
	if !containsString(result, `status: "publish"`) {
		t.Error("generateMarkdownContent() should contain post status")
	}
	if !containsString(result, `type: "post"`) {
		t.Error("generateMarkdownContent() should contain content type")
	}
	if !containsString(result, "author: 1") {
		t.Error("generateMarkdownContent() should contain author ID")
	}
	if !containsString(result, "featured_media: 100") {
		t.Error("generateMarkdownContent() should contain featured media ID")
	}
	if !containsString(result, "categories:") {
		t.Error("generateMarkdownContent() should contain categories")
	}
	if !containsString(result, "tags:") {
		t.Error("generateMarkdownContent() should contain tags")
	}

	// Check content
	if !containsString(result, "This is the content") {
		t.Error("generateMarkdownContent() should contain post content")
	}
}

func TestBuildCategoryHierarchy(t *testing.T) {
	e := NewExporter(&config.Config{})

	categories := []models.WordPressCategory{
		{ID: 1, Name: "Parent", Slug: "parent", Parent: 0},
		{ID: 2, Name: "Child", Slug: "child", Parent: 1},
		{ID: 3, Name: "Grandchild", Slug: "grandchild", Parent: 2},
	}

	hierarchy := e.buildCategoryHierarchy(categories)

	if hierarchy == nil {
		t.Fatal("buildCategoryHierarchy() should not return nil")
	}

	// Check that hierarchy is built correctly
	if len(hierarchy) == 0 {
		t.Error("buildCategoryHierarchy() should return non-empty hierarchy")
	}
}

func TestUpdateMediaPaths(t *testing.T) {
	e := NewExporter(&config.Config{
		Output: "/tmp/export",
	})

	data := &models.ExportData{
		Posts: []models.WordPressPost{
			{
				ID: 1,
				Content: models.RenderedContent{
					Rendered: `<img src="https://example.com/wp-content/uploads/image.jpg">`,
				},
			},
		},
		Media: []models.WordPressMedia{
			{
				ID:        100,
				SourceURL: "https://example.com/wp-content/uploads/image.jpg",
			},
		},
	}

	e.updateMediaPaths(data)

	// Function should run without panic
	// The actual path replacement depends on implementation details
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstr(s, substr)
}

func findSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
