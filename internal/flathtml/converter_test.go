package flathtml

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestNewConverter(t *testing.T) {
	c := NewConverter()
	assert.NotNil(t, c)
	assert.NotEmpty(t, c.rules)
}

func TestNewConverterWithRules(t *testing.T) {
	customRules := []config.FlatHTMLRule{
		{Class: "my-heading", Markdown: "# {content}\n\n"},
		{Class: "my-text", Tag: "div", Markdown: "{content}\n\n"},
	}

	c := NewConverterWithRules(customRules)
	assert.NotNil(t, c)
	assert.NotEmpty(t, c.rules)
}

func TestConvert_Headings(t *testing.T) {
	c := NewConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "h1 heading",
			input:    "<h1>Hello World</h1>",
			expected: "# Hello World",
		},
		{
			name:     "h2 heading",
			input:    "<h2>Section Title</h2>",
			expected: "## Section Title",
		},
		{
			name:     "h3 heading",
			input:    "<h3>Subsection</h3>",
			expected: "### Subsection",
		},
		{
			name:     "h4 heading",
			input:    "<h4>Minor Heading</h4>",
			expected: "#### Minor Heading",
		},
		{
			name:     "h5 heading",
			input:    "<h5>Small Heading</h5>",
			expected: "##### Small Heading",
		},
		{
			name:     "h6 heading",
			input:    "<h6>Tiny Heading</h6>",
			expected: "###### Tiny Heading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Convert(tt.input)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestConvert_Paragraphs(t *testing.T) {
	c := NewConverter()

	input := "<p>This is a paragraph.</p>"
	result := c.Convert(input)

	assert.Contains(t, result, "This is a paragraph.")
}

func TestConvert_BoldAndItalic(t *testing.T) {
	c := NewConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strong tag",
			input:    "<strong>bold text</strong>",
			expected: "**bold text**",
		},
		{
			name:     "b tag",
			input:    "<b>bold text</b>",
			expected: "**bold text**",
		},
		{
			name:     "em tag",
			input:    "<em>italic text</em>",
			expected: "*italic text*",
		},
		{
			name:     "i tag",
			input:    "<i>italic text</i>",
			expected: "*italic text*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Convert(tt.input)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestConvert_Links(t *testing.T) {
	c := NewConverter()

	input := `<a href="https://example.com">Example Link</a>`
	result := c.Convert(input)

	assert.Contains(t, result, "[Example Link](https://example.com)")
}

func TestConvert_Images(t *testing.T) {
	c := NewConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "image with alt",
			input:    `<img src="image.jpg" alt="My Image">`,
			expected: "![My Image](image.jpg)",
		},
		{
			name:     "image without alt",
			input:    `<img src="image.jpg">`,
			expected: "![](image.jpg)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Convert(tt.input)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestConvert_Lists(t *testing.T) {
	c := NewConverter()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "unordered list",
			input:    "<ul><li>Item 1</li><li>Item 2</li></ul>",
			expected: []string{"- Item 1", "- Item 2"},
		},
		{
			name:     "ordered list",
			input:    "<ol><li>First</li><li>Second</li></ol>",
			expected: []string{"1. First", "2. Second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Convert(tt.input)
			for _, exp := range tt.expected {
				assert.Contains(t, result, exp)
			}
		})
	}
}

func TestConvert_Blockquote(t *testing.T) {
	c := NewConverter()

	input := "<blockquote>This is a quote</blockquote>"
	result := c.Convert(input)

	assert.Contains(t, result, "> This is a quote")
}

func TestConvert_CodeBlocks(t *testing.T) {
	c := NewConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "inline code",
			input:    "<code>inline code</code>",
			expected: "`inline code`",
		},
		{
			name:     "code block",
			input:    "<pre><code>function test() {}</code></pre>",
			expected: "```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Convert(tt.input)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestConvert_HorizontalRule(t *testing.T) {
	c := NewConverter()

	tests := []struct {
		name  string
		input string
	}{
		{name: "hr tag", input: "<hr>"},
		{name: "hr self-closing", input: "<hr/>"},
		{name: "hr with space", input: "<hr />"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Convert(tt.input)
			assert.Contains(t, result, "---")
		})
	}
}

func TestConvert_BricksBuilder(t *testing.T) {
	c := NewConverter()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "brxe-heading",
			input:    `<div class="brxe-heading">Bricks Heading</div>`,
			expected: "## Bricks Heading",
		},
		{
			name:     "brxe-text",
			input:    `<div class="brxe-text">Bricks paragraph text</div>`,
			expected: "Bricks paragraph text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Convert(tt.input)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestConvert_RemovesScriptsAndStyles(t *testing.T) {
	c := NewConverter()

	input := `<p>Content</p><script>alert('bad');</script><style>.foo{color:red}</style><p>More content</p>`
	result := c.Convert(input)

	assert.Contains(t, result, "Content")
	assert.Contains(t, result, "More content")
	assert.NotContains(t, result, "alert")
	assert.NotContains(t, result, "color")
}

func TestConvert_RemovesEmptyDivs(t *testing.T) {
	c := NewConverter()

	input := `<div></div><p>Content</p><div>   </div>`
	result := c.Convert(input)

	assert.Contains(t, result, "Content")
}

func TestConvert_DecodesHTMLEntities(t *testing.T) {
	c := NewConverter()

	input := `<p>Tom &amp; Jerry &lt;3 &quot;cartoons&quot;</p>`
	result := c.Convert(input)

	assert.Contains(t, result, "Tom & Jerry")
	assert.Contains(t, result, "<3")
	assert.Contains(t, result, `"cartoons"`)
}

func TestConvertPosts(t *testing.T) {
	c := NewConverter()

	posts := []models.WordPressPost{
		{
			ID:      1,
			Content: models.RenderedContent{Rendered: "<h1>Title</h1><p>Paragraph content here.</p>"},
		},
		{
			ID:      2,
			Content: models.RenderedContent{Rendered: "<h2>Section</h2><ul><li>Item 1</li><li>Item 2</li></ul>"},
		},
	}

	result := c.ConvertPosts(posts)

	assert.Len(t, result, 2)
	assert.Contains(t, result[0].Content.Rendered, "# Title")
	assert.Contains(t, result[0].Content.Rendered, "Paragraph content here.")
	assert.Contains(t, result[1].Content.Rendered, "## Section")
	assert.Contains(t, result[1].Content.Rendered, "- Item 1")
}

func TestCustomRules(t *testing.T) {
	customRules := []config.FlatHTMLRule{
		{
			Class:    "custom-heading",
			Markdown: "### {content}\n\n",
		},
		{
			Class:    "custom-para",
			Tag:      "span",
			Markdown: "{content}\n\n",
		},
	}

	c := NewConverterWithRules(customRules)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "custom heading class",
			input:    `<div class="custom-heading">Custom Title</div>`,
			expected: "### Custom Title",
		},
		{
			name:     "custom span class",
			input:    `<span class="custom-para">Custom paragraph</span>`,
			expected: "Custom paragraph",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Convert(tt.input)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"1", 1},
		{"2", 2},
		{"3", 3},
		{"4", 4},
		{"5", 5},
		{"6", 6},
		{"0", 2}, // out of range, default
		{"7", 2}, // out of range, default
		{"", 2},  // empty, default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseInt(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDecodeHTMLEntities(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"&amp;", "&"},
		{"&lt;", "<"},
		{"&gt;", ">"},
		{"&quot;", "\""},
		{"&#39;", "'"},
		{"&nbsp;", " "},
		{"&ndash;", "-"},
		{"&mdash;", "-"},
		{"&hellip;", "..."},
		{"&copy;", "(c)"},
		{"&reg;", "(R)"},
		{"&trade;", "(TM)"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := decodeHTMLEntities(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertList(t *testing.T) {
	input := "<li>First item</li><li>Second item</li><li>Third item</li>"
	result := convertList(input)

	assert.Contains(t, result, "- First item")
	assert.Contains(t, result, "- Second item")
	assert.Contains(t, result, "- Third item")
}

func TestConvertOrderedList(t *testing.T) {
	input := "<li>First</li><li>Second</li><li>Third</li>"
	result := convertOrderedList(input)

	assert.Contains(t, result, "1. First")
	assert.Contains(t, result, "2. Second")
	assert.Contains(t, result, "3. Third")
}
