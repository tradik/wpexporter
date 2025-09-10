package export

import (
	"testing"

	"github.com/tradik/wpexporter/internal/config"
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
