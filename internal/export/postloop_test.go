package export

// A /blog/ that renders a post loop (#41). The page exports as a few hundred
// bytes that say nothing, and then sits at the address the target's own listing
// wants — so the migrated site has a blog that lists nothing, with no error
// anywhere.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/pkg/models"
)

// fusionBlogPage is the case from the issue: a heading, a line of introduction,
// and a builder element where the archive would be.
func fusionBlogPage() models.WordPressPost {
	page := models.WordPressPost{ID: 12, Slug: "blog", Link: "https://x.test/blog/"}
	page.Title.Rendered = "Blog"
	page.Content.Rendered = `<h1>Blog</h1><p>What we have been writing.</p>` +
		`<div class="fusion-blog-shortcode fusion_blog" data-columns="3"></div>`

	return page
}

// TestPostLoopHintNamesWhatItMatched: the detection says which element gave the
// page away, so a wrong guess is visible rather than silently changing the
// export.
func TestPostLoopHintNamesWhatItMatched(t *testing.T) {
	assert.Equal(t, "fusion_blog", postLoopHint(fusionBlogPage().Content.Rendered))

	for marker, body := range map[string]string{
		"elementor-widget-posts": `<div class="elementor-widget elementor-widget-posts"></div>`,
		"wp:query":               `<!-- wp:query {"queryId":1} --><!-- /wp:query -->`,
		"[blog":                  `<p>[blog columns="2"]</p>`,
		"[posts":                 `[posts per_page="10"]`,
		"et_pb_blog":             `<div class="et_pb_blog_0 et_pb_blog"></div>`,
	} {
		assert.Equal(t, marker, postLoopHint(body), "body %q", body)
	}
}

// TestPostLoopHintLeavesRealContentAlone: an article about Elementor that
// mentions the Posts widget is an article. The marker alone is not enough —
// the page also has to be empty of its own content.
func TestPostLoopHintLeavesRealContentAlone(t *testing.T) {
	article := `<h1>How the Posts widget works</h1>` +
		`<p>The elementor-widget-posts element renders an archive at render time, which is why ` +
		`an export of a page built with it carries almost nothing: the REST API serves what is ` +
		`stored, and what is stored is the element. This article explains what that means for a ` +
		`migration, how to recognize such a page, and what to do with it on the other side. It ` +
		`also covers the other builders that do the same thing, and why a count-based check ` +
		`cannot notice any of it. There is quite a lot of prose here, which is the point.</p>`

	assert.Equal(t, "", postLoopHint(article))
	assert.Equal(t, "", postLoopHint(`<p>An ordinary page.</p>`))
	assert.Equal(t, "", postLoopHint(""))
}

// TestPostLoopPageSaysSoInFrontMatter: the export states it in the document,
// where a generator can read it.
func TestPostLoopPageSaysSoInFrontMatter(t *testing.T) {
	exporter, output := newMarkdownExporter(t)
	data := &models.ExportData{Pages: []models.WordPressPost{fusionBlogPage()}}
	exporter.buildLookupMaps(data)

	require.NoError(t, os.MkdirAll(filepath.Join(output, "pages"), 0750))
	require.NoError(t, exporter.exportPagesMarkdown(data))

	body, err := os.ReadFile(filepath.Join(output, "pages", "blog.md"))
	require.NoError(t, err)

	assert.Contains(t, string(body), "lists: posts")
	assert.Contains(t, string(body), `lists_hint: "fusion_blog"`)
	assert.Contains(t, string(body), "What we have been writing.",
		"the introduction above the loop is real content and still travels")
}

// TestPostLoopPageIsReported: the report line is what changes the operator's
// next move, and it outlives the console in metadata.json.
func TestPostLoopPageIsReported(t *testing.T) {
	exporter, output := newMarkdownExporter(t)
	data := &models.ExportData{Pages: []models.WordPressPost{fusionBlogPage()}}
	exporter.buildLookupMaps(data)

	require.NoError(t, os.MkdirAll(filepath.Join(output, "pages"), 0750))
	require.NoError(t, exporter.exportPagesMarkdown(data))

	require.Len(t, data.Stats.PostLoopPages, 1)
	notice := data.Stats.PostLoopPages[0]
	assert.Contains(t, notice, "https://x.test/blog/")
	assert.Contains(t, notice, "fusion_blog")
	assert.Contains(t, notice, "Point your generator's listing at this URL")
}

// TestOrdinaryPageIsNotLabelled: nothing is said about a page that says
// something itself.
func TestOrdinaryPageIsNotLabelled(t *testing.T) {
	exporter, output := newMarkdownExporter(t)

	page := models.WordPressPost{ID: 3, Slug: "about", Link: "https://x.test/about/"}
	page.Title.Rendered = "About"
	page.Content.Rendered = `<p>We have been making things since 2004.</p>`

	data := &models.ExportData{Pages: []models.WordPressPost{page}}
	exporter.buildLookupMaps(data)

	require.NoError(t, os.MkdirAll(filepath.Join(output, "pages"), 0750))
	require.NoError(t, exporter.exportPagesMarkdown(data))

	body, err := os.ReadFile(filepath.Join(output, "pages", "about.md"))
	require.NoError(t, err)

	assert.NotContains(t, string(body), "lists:")
	assert.Empty(t, data.Stats.PostLoopPages)
}

// TestVisibleTextIgnoresMarkupAndShortcodes: the budget measures what a reader
// would see, so a page padded with builder markup does not read as content.
func TestVisibleTextIgnoresMarkupAndShortcodes(t *testing.T) {
	assert.Equal(t, "Hello there", visibleText(
		`<div class="x"><script>var a = "lots of text in here";</script>`+
			`<p>Hello   there</p>[fusion_blog columns="3"]</div>`))
}

// TestSiteNamesItsOwnListingElement: the marker list is identifiers rather than
// words — a shortcode name reads the same on a Polish site as on an English one
// — so the gap was never language. It is coverage: every theme that ships a
// listing element writes its own name for it, and the next one is on nobody's
// list.
func TestSiteNamesItsOwnListingElement(t *testing.T) {
	body := `<div class="zzt-posts-grid" data-count="6"></div>`

	assert.Empty(t, postLoopHint(body), "nothing recognizes it yet")
	assert.Equal(t, "zzt-posts-grid", postLoopHintWith(body, []string{"zzt-posts-grid", "  "}))

	// Additive: a site running Elementor and its own shortcode has both.
	assert.Equal(t, "elementor-widget-posts",
		postLoopHintWith(`<div class="elementor-widget-posts"></div>`, []string{"zzt-posts-grid"}))
}

// TestTextBudgetCountsCharactersNotBytes: a budget counted in bytes is a
// different budget on every alphabet. 400 bytes is 400 letters of English, 300
// of Polish and 133 of Japanese, so the same listing page was judged three ways
// depending on the language it was written in — and never against the English
// one.
func TestTextBudgetCountsCharactersNotBytes(t *testing.T) {
	// A listing element with a Japanese introduction above it: 200 characters,
	// 600 bytes. Counted in bytes this page was over the budget and read as
	// ordinary content, so its listing was never reported.
	intro := strings.Repeat("お", 200)
	body := `<p>` + intro + `</p><div class="elementor-widget-posts"></div>`

	assert.Greater(t, len(visibleText(body)), postLoopTextBudget, "over budget in bytes")
	assert.Equal(t, "elementor-widget-posts", postLoopHint(body), "within budget in characters")
}
