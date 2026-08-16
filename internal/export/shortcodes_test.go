package export

// Shortcodes shown to readers (#47) and pages the API never served (#46).
// Both are the same failure in different clothes: an export that is not wrong
// so much as silently incomplete.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// TestUnexpandedShortcodesAreRemoved: what a reader would otherwise see, taken
// from the six migrations in the issue.
func TestUnexpandedShortcodesAreRemoved(t *testing.T) {
	for _, body := range []string{
		`<p>[osm_map_v3 map_center="35.849,14.571" zoom="17" width="100%"]</p>`,
		`<p>[eo_events numberposts=5]</p>`,
		`<p>[img_assist fid=12|width=300]</p>`,
		`<p>[postimages]</p>`,
	} {
		cleaned, removed := stripShortcodes(body)

		assert.NotContains(t, cleaned, "[", "body %q", body)
		assert.Len(t, removed, 1, "body %q", body)
	}
}

// TestPairedShortcodeTakesItsBody: the text between [gallery] and [/gallery] is
// the plugin's arguments, not the page's prose.
func TestPairedShortcodeTakesItsBody(t *testing.T) {
	cleaned, removed := stripShortcodes(
		`<p>Before</p>[gallery ids="1,2"]<p>caption</p>[/gallery]<p>After</p>`)

	assert.Contains(t, cleaned, "Before")
	assert.Contains(t, cleaned, "After")
	assert.NotContains(t, cleaned, "caption")
	assert.Equal(t, []string{"gallery"}, removed)
}

// TestProseAndLinksAreNotShortcodes: the fix must not become the worse bug.
// A Markdown link's label and an editorial bracket both look like a shortcode
// and are neither.
func TestProseAndLinksAreNotShortcodes(t *testing.T) {
	for _, body := range []string{
		`See [the recipe](https://x.test/recipe/) for details.`,
		`The quotation reads "an unusual spelling" [sic] in the original.`,
		`As discussed [note] later in this chapter.`,
		`Footnote marker [1] and range [10] stay put.`,
		`An [em]phasis[/em] pair is markup, but nobody writes it in prose.`,
	} {
		cleaned, removed := stripShortcodes(body)

		if strings.Contains(body, "[em]") {
			continue // paired: legitimately removed, and covered above
		}

		assert.Equal(t, body, cleaned, "body %q", body)
		assert.Empty(t, removed, "body %q", body)
	}
}

// TestContentWithoutShortcodesIsUntouched: the common path changes nothing,
// which is what makes the report meaningful when it does appear.
func TestContentWithoutShortcodesIsUntouched(t *testing.T) {
	body := `<h2>Recipe</h2><p>Mix the flour and <strong>water</strong>.</p>`

	cleaned, removed := stripShortcodes(body)
	assert.Equal(t, body, cleaned)
	assert.Empty(t, removed)
}

// TestShortcodeReportCountsByPlugin: "45 [eo_events]" tells an operator a
// calendar has to be rebuilt; silence tells them nothing until a visitor finds
// the gap.
func TestShortcodeReportCountsByPlugin(t *testing.T) {
	exporter, _ := newMarkdownExporter(t)
	exporter.config.Quiet = true

	withMap := models.WordPressPost{ID: 1, Slug: "contact"}
	withMap.Content.Rendered = `<p>[osm_map_v3 zoom="17"]</p>`
	withEvents := models.WordPressPost{ID: 2, Slug: "events"}
	withEvents.Content.Rendered = `<p>[eo_events a=1]</p><p>[eo_events a=2]</p>`

	data := &models.ExportData{Pages: []models.WordPressPost{withMap, withEvents}}
	exporter.stripShortcodesFromContent(data)

	report := strings.Join(data.Stats.RemovedShortcodes, "\n")
	assert.Contains(t, report, "Removed 3 unexpanded shortcode(s)")
	assert.Contains(t, report, "[eo_events]")
	assert.Contains(t, report, "[osm_map_v3]")
	assert.Less(t, strings.Index(report, "eo_events"), strings.Index(report, "osm_map_v3"),
		"the plugin that leaked most is named first")

	assert.NotContains(t, data.Pages[0].Content.Rendered, "[osm_map_v3",
		"the document itself is cleaned, which is the point")
}

// TestEmptyFrontPageIsReported: the case from #46 — the export is correct and
// useless at the same time, and only saying so helps.
func TestEmptyFrontPageIsReported(t *testing.T) {
	exporter, _ := newMarkdownExporter(t)
	exporter.config.Quiet = true

	front := models.WordPressPost{ID: 5, Slug: "home", Link: "https://x.test/"}
	front.Title.Rendered = "Home"
	front.Content.Rendered = `<div class="generate-sections-container"></div>`

	data := &models.ExportData{
		Site:  models.SiteInfo{URL: "https://x.test", HomeURL: "https://x.test/"},
		Pages: []models.WordPressPost{front},
	}
	exporter.reportEmptyPages(data)

	require.Len(t, data.Stats.EmptyPages, 1)
	notice := data.Stats.EmptyPages[0]
	assert.Contains(t, notice, "front page https://x.test/")
	assert.Contains(t, notice, "--assisted-crawl --crawl-content",
		"the report names the remedy, since a re-run alone will not find the sections")
}

// TestPagesWithContentAreNotReported: a page that says something is not a
// finding, or the warning becomes noise nobody reads.
func TestPagesWithContentAreNotReported(t *testing.T) {
	exporter, _ := newMarkdownExporter(t)
	exporter.config.Quiet = true

	page := models.WordPressPost{ID: 6, Slug: "about", Link: "https://x.test/about/"}
	page.Content.Rendered = `<p>` + strings.Repeat("We have been making things since 2004. ", 5) + `</p>`

	data := &models.ExportData{Pages: []models.WordPressPost{page}}
	exporter.reportEmptyPages(data)

	assert.Empty(t, data.Stats.EmptyPages)
}

// TestAPostLoopPageIsExplainedOnce: #41 already explains that page, and by the
// more specific of the two explanations.
func TestAPostLoopPageIsExplainedOnce(t *testing.T) {
	exporter, _ := newMarkdownExporter(t)
	exporter.config.Quiet = true

	data := &models.ExportData{Pages: []models.WordPressPost{fusionBlogPage()}}
	exporter.reportEmptyPages(data)

	assert.Empty(t, data.Stats.EmptyPages)
}

// TestFrontPageRecognition: the API does not say which page is the front one in
// a public read, so it is recognized by address — and a trailing slash or a
// changed host must not decide it.
func TestFrontPageRecognition(t *testing.T) {
	home := normalizeSiteURL("https://x.test/", "")

	assert.True(t, isFrontPage(models.WordPressPost{Link: "https://x.test"}, home))
	assert.True(t, isFrontPage(models.WordPressPost{Link: "https://X.test/"}, home))
	assert.False(t, isFrontPage(models.WordPressPost{Link: "https://x.test/about/"}, home))
	assert.False(t, isFrontPage(models.WordPressPost{}, home))
	assert.False(t, isFrontPage(models.WordPressPost{Link: "https://x.test/"}, ""))
}

// TestPageAddressNamesWhatItCan: a page with no link is still worth naming.
func TestPageAddressNamesWhatItCan(t *testing.T) {
	assert.Equal(t, "https://x.test/a/", pageAddress(models.WordPressPost{Link: "https://x.test/a/"}))
	assert.Equal(t, "/a/", pageAddress(models.WordPressPost{Slug: "a"}))
	assert.Equal(t, "#7", pageAddress(models.WordPressPost{ID: 7}))
}

// TestShortcodesAreStrippedOnlyForLocalFormats: a platform importer takes the
// source site's own markup, and rewriting what it receives is not this pass's
// business.
func TestShortcodesAreStrippedOnlyForLocalFormats(t *testing.T) {
	post := models.WordPressPost{ID: 1, Slug: "contact"}
	post.Content.Rendered = `<p>[osm_map_v3 zoom="17"]</p>`

	shopify := NewExporter(&config.Config{Output: t.TempDir(), Format: "shopify", Quiet: true})
	data := &models.ExportData{Posts: []models.WordPressPost{post}}
	shopify.localizeAddresses(data)

	assert.Contains(t, data.Posts[0].Content.Rendered, "[osm_map_v3",
		"a platform format keeps the source markup it was asked for")
}
