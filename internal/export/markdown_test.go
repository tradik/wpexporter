package export

import (
	"strings"
	"testing"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// TestHTMLToMarkdown_GutenbergBlocks covers issue #21: block tags that carry
// attributes must be converted, and no orphaned opening tag may survive.
func TestHTMLToMarkdown_GutenbergBlocks(t *testing.T) {
	in := `<h3 class="wp-block-heading">Ingredients</h3>` +
		`<p class="wp-block-paragraph"><strong>Mixture (room temperature):</strong></p>` +
		`<ul class="wp-block-list"><li>200g dark chocolate</li><li>200g unsalted butter</li></ul>` +
		`<ol class="wp-block-list"><li>Melt</li><li>Whisk</li></ol>`

	out := htmlToMarkdown(in)

	if strings.Contains(out, "<") || strings.Contains(out, ">") {
		t.Errorf("orphaned HTML tag survived conversion:\n%s", out)
	}
	for _, want := range []string{
		"### Ingredients",
		"**Mixture (room temperature):**",
		"- 200g dark chocolate",
		"- 200g unsalted butter",
		"- Melt",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
	// The literal markers must not sit next to a raw block tag.
	if strings.Contains(out, "wp-block") {
		t.Errorf("block class leaked into output:\n%s", out)
	}
}

// TestHTMLToMarkdown_KeepsSelfContainedHTML confirms images and anchors are passed
// through as complete HTML — valid inside Markdown, and the form the SSG format and
// the media URL rewriter both expect. Only block tags are converted.
func TestHTMLToMarkdown_KeepsSelfContainedHTML(t *testing.T) {
	in := `<figure class="wp-block-image"><img src="/media/images/1_cake.jpg" alt="A cake"/></figure>` +
		`<p class="wp-block-paragraph">See <a href="/recipe">the recipe</a>.</p>`
	out := htmlToMarkdown(in)

	if !strings.Contains(out, `<img src="/media/images/1_cake.jpg" alt="A cake"/>`) {
		t.Errorf("image tag should be preserved:\n%s", out)
	}
	if !strings.Contains(out, `<a href="/recipe">the recipe</a>`) {
		t.Errorf("anchor should be preserved:\n%s", out)
	}
	// The paragraph block tag must still be converted, not left orphaned.
	if strings.Contains(out, "wp-block-paragraph") {
		t.Errorf("block tag survived:\n%s", out)
	}
}

// TestGenerateMarkdownContent_DecodesEntities covers issue #23: entities in the
// flattened frontmatter text fields (title, excerpt) must be decoded.
func TestGenerateMarkdownContent_DecodesEntities(t *testing.T) {
	e := NewExporter(&config.Config{})
	post := models.WordPressPost{
		ID:      7,
		Title:   models.RenderedContent{Rendered: "Domowe Kino &#8211; Warszawa"},
		Excerpt: models.RenderedContent{Rendered: "<p>Akustyka &amp; design &#8211; krok po kroku.</p>"},
	}

	out := e.generateMarkdownContent(post, "post")

	if !strings.Contains(out, `title: "Domowe Kino – Warszawa"`) {
		t.Errorf("title entity not decoded:\n%s", firstLines(out, 12))
	}
	if strings.Contains(out, "&#8211;") || strings.Contains(out, "&amp;") {
		t.Errorf("raw entity survived in frontmatter:\n%s", firstLines(out, 12))
	}
	if !strings.Contains(out, "Akustyka & design – krok po kroku.") {
		t.Errorf("excerpt entity not decoded:\n%s", firstLines(out, 12))
	}
}

func firstLines(s string, n int) string {
	parts := strings.SplitN(s, "\n", n+1)
	if len(parts) > n {
		parts = parts[:n]
	}
	return strings.Join(parts, "\n")
}

// TestGenerateMarkdownContent_SSGSections covers issue #20: with SSGSections the
// body carries ## Excerpt / ## Content markers and no duplicate # Title H1, while
// the frontmatter excerpt key is preserved.
func TestGenerateMarkdownContent_SSGSections(t *testing.T) {
	e := NewExporter(&config.Config{SSGSections: true})
	post := models.WordPressPost{
		ID:      1,
		Title:   models.RenderedContent{Rendered: "My Post"},
		Excerpt: models.RenderedContent{Rendered: "<p>Short summary.</p>"},
		Content: models.RenderedContent{Rendered: "<p>Body text.</p>"},
	}

	out := e.generateMarkdownContent(post, "post")

	if !strings.Contains(out, "excerpt: \"Short summary.\"") {
		t.Errorf("frontmatter excerpt missing:\n%s", out)
	}
	if !strings.Contains(out, "## Excerpt\n\nShort summary.") {
		t.Errorf("## Excerpt section missing:\n%s", out)
	}
	if !strings.Contains(out, "## Content\n\nBody text.") {
		t.Errorf("## Content section missing:\n%s", out)
	}
	// No duplicate H1 in the body.
	if strings.Contains(out, "\n# My Post") {
		t.Errorf("duplicate # Title H1 should be omitted with SSGSections:\n%s", out)
	}
}

// TestGenerateMarkdownContent_DefaultKeepsH1 confirms the default markdown output
// still emits the # Title H1 (only --ssg-sections drops it).
func TestGenerateMarkdownContent_DefaultKeepsH1(t *testing.T) {
	e := NewExporter(&config.Config{})
	post := models.WordPressPost{ID: 1, Title: models.RenderedContent{Rendered: "My Post"}}
	out := e.generateMarkdownContent(post, "post")
	if !strings.Contains(out, "# My Post") {
		t.Errorf("default markdown should keep the H1:\n%s", out)
	}
	if strings.Contains(out, "## Content") {
		t.Errorf("default markdown must not emit ## Content:\n%s", out)
	}
}
