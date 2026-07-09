package basichtml

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/pkg/models"
)

func TestNewSanitizer(t *testing.T) {
	s := NewSanitizer()
	assert.NotNil(t, s)
	assert.NotEmpty(t, s.allowedTags)
}

func TestSanitize_PreservesBasicTags(t *testing.T) {
	s := NewSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "paragraphs",
			input:    "<p>Hello world</p>",
			expected: "<p>Hello world</p>",
		},
		{
			name:     "headers",
			input:    "<h1>Title</h1><h2>Subtitle</h2>",
			expected: "<h1>Title</h1><h2>Subtitle</h2>",
		},
		{
			name:     "formatting",
			input:    "<p><strong>Bold</strong> and <em>italic</em></p>",
			expected: "<p><strong>Bold</strong> and <em>italic</em></p>",
		},
		{
			name:     "lists",
			input:    "<ul><li>Item 1</li><li>Item 2</li></ul>",
			expected: "<ul><li>Item 1</li><li>Item 2</li></ul>",
		},
		{
			name:     "tables",
			input:    "<table><tr><th>Header</th></tr><tr><td>Cell</td></tr></table>",
			expected: "<table><tr><th>Header</th></tr><tr><td>Cell</td></tr></table>",
		},
		{
			name:     "links with href",
			input:    `<a href="https://example.com">Link</a>`,
			expected: `<a href="https://example.com">Link</a>`,
		},
		{
			name:     "images with src and alt",
			input:    `<img src="image.jpg" alt="Image" />`,
			expected: `<img src="image.jpg" alt="Image" />`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitize_RemovesComplexTags(t *testing.T) {
	s := NewSanitizer()

	tests := []struct {
		name     string
		input    string
		contains string
		excludes string
	}{
		{
			name:     "removes div tags",
			input:    `<div class="brxe-container">Content</div>`,
			contains: "Content",
			excludes: "brxe",
		},
		{
			name:     "removes span tags",
			input:    `<span class="custom">Text</span>`,
			contains: "Text",
			excludes: "span",
		},
		{
			name:     "removes script tags completely",
			input:    `<p>Before</p><script>alert('xss')</script><p>After</p>`,
			contains: "After",
			excludes: "script",
		},
		{
			name:     "removes style tags completely",
			input:    `<p>Text</p><style>.foo{color:red}</style>`,
			contains: "Text",
			excludes: "style",
		},
		{
			name:     "removes iframe",
			input:    `<p>Video:</p><iframe src="youtube.com"></iframe>`,
			contains: "Video",
			excludes: "iframe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Contains(t, result, tt.contains)
			assert.NotContains(t, result, tt.excludes)
		})
	}
}

func TestSanitize_RemovesDangerousAttributes(t *testing.T) {
	s := NewSanitizer()

	tests := []struct {
		name     string
		input    string
		excludes string
	}{
		{
			name:     "removes style attribute",
			input:    `<p style="color:red">Text</p>`,
			excludes: "style",
		},
		{
			name:     "removes onclick",
			input:    `<a href="#" onclick="alert('xss')">Click</a>`,
			excludes: "onclick",
		},
		{
			name:     "removes class attribute",
			input:    `<p class="brxe-text elementor-widget">Text</p>`,
			excludes: "class",
		},
		{
			name:     "removes data attributes",
			input:    `<div data-id="123" data-widget="foo">Content</div>`,
			excludes: "data-",
		},
		{
			name:     "removes javascript href",
			input:    `<a href="javascript:alert('xss')">Click</a>`,
			excludes: "javascript",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.NotContains(t, result, tt.excludes)
		})
	}
}

func TestSanitize_PreservesTableAttributes(t *testing.T) {
	s := NewSanitizer()

	input := `<table border="1" cellpadding="5"><tr><td colspan="2" align="center">Cell</td></tr></table>`
	result := s.Sanitize(input)

	assert.Contains(t, result, `border="1"`)
	assert.Contains(t, result, `cellpadding="5"`)
	assert.Contains(t, result, `colspan="2"`)
	assert.Contains(t, result, `align="center"`)
}

func TestSanitize_BricksBuilderContent(t *testing.T) {
	s := NewSanitizer()

	// Simulated Bricks Builder HTML
	input := `
	<div class="brxe-container" data-script-id="abc123">
		<div class="brxe-block">
			<div class="brxe-heading" data-element-id="xyz">
				<h2>Welcome</h2>
			</div>
			<div class="brxe-text">
				<p>This is <strong>important</strong> content with a <a href="https://example.com">link</a>.</p>
			</div>
			<div class="brxe-image">
				<img src="photo.jpg" alt="Photo" class="attachment-full" />
			</div>
		</div>
	</div>
	`

	result := s.Sanitize(input)

	// Should preserve content
	assert.Contains(t, result, "<h2>Welcome</h2>")
	assert.Contains(t, result, "<p>This is <strong>important</strong> content")
	assert.Contains(t, result, `<a href="https://example.com">link</a>`)
	assert.Contains(t, result, `<img src="photo.jpg" alt="Photo" />`)

	// Should remove Bricks classes and data attributes
	assert.NotContains(t, result, "brxe-")
	assert.NotContains(t, result, "data-script-id")
	assert.NotContains(t, result, "data-element-id")
	assert.NotContains(t, result, "attachment-full")
}

func TestSanitize_RemovesComments(t *testing.T) {
	s := NewSanitizer()

	input := `<p>Before</p><!-- This is a comment --><p>After</p>`
	result := s.Sanitize(input)

	assert.Contains(t, result, "Before")
	assert.Contains(t, result, "After")
	assert.NotContains(t, result, "comment")
	assert.NotContains(t, result, "<!--")
}

func TestSanitizePosts(t *testing.T) {
	s := NewSanitizer()

	posts := []models.WordPressPost{
		{
			ID: 1,
			Content: models.RenderedContent{
				Rendered: `<div class="wrapper"><p>Content</p></div>`,
			},
			Excerpt: models.RenderedContent{
				Rendered: `<span class="excerpt">Summary</span>`,
			},
		},
	}

	result := s.SanitizePosts(posts)

	assert.Len(t, result, 1)
	assert.Contains(t, result[0].Content.Rendered, "<p>Content</p>")
	assert.NotContains(t, result[0].Content.Rendered, "wrapper")
	assert.Contains(t, result[0].Excerpt.Rendered, "Summary")
	assert.NotContains(t, result[0].Excerpt.Rendered, "span")
}

func TestSanitize_EmptyInput(t *testing.T) {
	s := NewSanitizer()

	assert.Equal(t, "", s.Sanitize(""))
}

func TestSanitize_PreservesBlockquoteAndCode(t *testing.T) {
	s := NewSanitizer()

	input := `<blockquote>Quote text</blockquote><pre><code>func main() {}</code></pre>`
	result := s.Sanitize(input)

	assert.Contains(t, result, "<blockquote>Quote text</blockquote>")
	assert.Contains(t, result, "<pre><code>func main() {}</code></pre>")
}

func TestNewSanitizerWithOptions(t *testing.T) {
	preserveOpts := &PreserveOptions{
		Classes: []string{"keep-me"},
		IDs:     []string{"preserved-id"},
	}

	s := NewSanitizerWithOptions(preserveOpts)
	assert.NotNil(t, s)
	assert.NotNil(t, s.preserveOptions)
	assert.Equal(t, []string{"keep-me"}, s.preserveOptions.Classes)
	assert.Equal(t, []string{"preserved-id"}, s.preserveOptions.IDs)
}

func TestSanitize_PreserveClasses(t *testing.T) {
	preserveOpts := &PreserveOptions{
		Classes: []string{"klaviyo-form", "keep-intact"},
	}
	s := NewSanitizerWithOptions(preserveOpts)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "preserves element with exact class",
			input:    `<div class="klaviyo-form">Subscribe form</div><div class="remove-me">Regular</div>`,
			expected: `<div class="klaviyo-form">Subscribe form</div>`,
		},
		{
			name:     "preserves element with class among others",
			input:    `<div class="wrapper klaviyo-form signup">Form content</div>`,
			expected: `<div class="wrapper klaviyo-form signup">Form content</div>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestSanitize_PreserveIDs(t *testing.T) {
	preserveOpts := &PreserveOptions{
		IDs: []string{"newsletter-form"},
	}
	s := NewSanitizerWithOptions(preserveOpts)

	input := `<div id="newsletter-form" class="widget">Subscribe</div><div class="other">Text</div>`
	result := s.Sanitize(input)

	assert.Contains(t, result, `<div id="newsletter-form" class="widget">Subscribe</div>`)
}

func TestSanitize_PreserveWildcard(t *testing.T) {
	preserveOpts := &PreserveOptions{
		Classes: []string{"klaviyo-form-*"},
	}
	s := NewSanitizerWithOptions(preserveOpts)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "preserves element with wildcard suffix",
			input:    `<div class="klaviyo-form-XL7uTf">Form</div><div class="other">Text</div>`,
			expected: `<div class="klaviyo-form-XL7uTf">Form</div>`,
		},
		{
			name:     "preserves various wildcard matches",
			input:    `<div class="klaviyo-form-ABC123">First</div><div class="klaviyo-form-XYZ789">Second</div>`,
			expected: `klaviyo-form-ABC123`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestWildcardToRegex(t *testing.T) {
	tests := []struct {
		pattern  string
		expected string
	}{
		{"exact-class", `exact-class`},
		{"prefix-*", `prefix-[^"'\s]*`},
		{"*-suffix", `[^"'\s]*-suffix`},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			result := wildcardToRegex(tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsSafeURLValue covers scheme allow-listing and obfuscation defeats (SEC-003).
func TestIsSafeURLValue(t *testing.T) {
	safe := []string{
		"https://example.com/x",
		"http://example.com",
		"mailto:a@b.com",
		"tel:+123",
		"/relative/path",
		"../also/relative",
		"#anchor",
		"path/with:colon/after/slash",
		"",
	}
	for _, u := range safe {
		if !isSafeURLValue(u) {
			t.Errorf("isSafeURLValue(%q) = false, want true", u)
		}
	}

	unsafe := []string{
		"javascript:alert(1)",
		"JAVASCRIPT:alert(1)",
		"java\tscript:alert(1)",     // literal tab in scheme
		"java\nscript:alert(1)",     // literal newline
		"  javascript:alert(1)",     // leading whitespace
		"java&#9;script:alert(1)",   // entity tab
		"javascript&#58;alert(1)",   // entity colon
		"javascript&colon;alert(1)", // named-entity colon
		"vbscript:msgbox(1)",        // other dangerous scheme
		"data:text/html,<script>x</script>",
	}
	for _, u := range unsafe {
		if isSafeURLValue(u) {
			t.Errorf("isSafeURLValue(%q) = true, want false", u)
		}
	}
}

// TestSanitizeStripsDangerousHref confirms the sanitizer drops a bypass href end-to-end.
func TestSanitizeStripsDangerousHref(t *testing.T) {
	s := NewSanitizer()
	out := s.Sanitize(`<a href="java&#9;script:alert(1)">click</a>`)
	if strings.Contains(strings.ToLower(out), "javascript") || strings.Contains(out, "&#9;") {
		t.Errorf("Sanitize kept a dangerous href: %q", out)
	}
	// A normal link is preserved.
	out2 := s.Sanitize(`<a href="https://example.com">ok</a>`)
	if !strings.Contains(out2, "https://example.com") {
		t.Errorf("Sanitize dropped a safe href: %q", out2)
	}
}
