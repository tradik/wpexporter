package export

// Emphasis that closes (#50) and a post that stays pinned (#51).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/pkg/models"
)

// TestEmphasisSpaceMovesOutsideTheDelimiters: the case from the issue. In
// CommonMark a closing run preceded by whitespace is not right-flanking, so
// `**text **` closes nothing and the reader is shown the asterisks.
func TestEmphasisSpaceMovesOutsideTheDelimiters(t *testing.T) {
	out := htmlToMarkdown(`<p>Projekt <em><strong>bociany.pl </strong></em>realizowany jest</p>`)

	assert.Contains(t, out, "***bociany.pl*** realizowany")
	assert.NotContains(t, out, "bociany.pl *", "no delimiter may sit behind a space")
}

// TestEmphasisSpaceInEveryPosition: inside-leading, inside-trailing, both, and
// a run with nothing but space in it — the four shapes the issue asks to pin.
func TestEmphasisSpaceInEveryPosition(t *testing.T) {
	for _, testCase := range []struct {
		name string
		in   string
		want string
	}{
		{"trailing", `<p>a <strong>bold </strong>b</p>`, "a **bold** b"},
		{"leading", `<p>a<strong> bold</strong> b</p>`, "a **bold** b"},
		{"both", `<p>a<strong> bold </strong>b</p>`, "a **bold** b"},
		{"nested", `<p>x <em><strong>y </strong></em>z</p>`, "x ***y*** z"},
		{"emphasis", `<p>a <em>slanted </em>b</p>`, "a *slanted* b"},
		{"italic tag", `<p>a <i>slanted </i>b</p>`, "a *slanted* b"},
		{"bold tag", `<p>a <b>heavy </b>b</p>`, "a **heavy** b"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.want, strings.TrimSpace(htmlToMarkdown(testCase.in)))
		})
	}
}

// TestEmptyEmphasisRunIsDropped: `** **` is emphasis around nothing, and means
// nothing in either language. The space the page showed stays.
func TestEmptyEmphasisRunIsDropped(t *testing.T) {
	out := htmlToMarkdown(`<p>before<strong> </strong>after</p>`)

	assert.Equal(t, "before after", strings.TrimSpace(out))
	assert.NotContains(t, out, "*")
}

// TestEmphasisWithoutStraySpacesIsUnchanged: the common case must convert
// exactly as it did before, or every existing export is a diff.
func TestEmphasisWithoutStraySpacesIsUnchanged(t *testing.T) {
	assert.Equal(t, "a **bold** and *slanted* b",
		strings.TrimSpace(htmlToMarkdown(`<p>a <strong>bold</strong> and <em>slanted</em> b</p>`)))
}

// TestEmphasisNormalisationKeepsAttributes: the tag may carry classes — the
// block editor's do — and the content is what matters, not the attributes.
func TestEmphasisNormalisationKeepsAttributes(t *testing.T) {
	out := htmlToMarkdown(`<p>a <strong class="has-text-color">bold </strong>b</p>`)

	assert.Equal(t, "a **bold** b", strings.TrimSpace(out))
}

// TestStickyPostSaysSo: WordPress lets an editor pin a post to the top of the
// blog. A listing sorted by date alone drops it wherever its date falls —
// sixth, on the site that reported this (#51).
func TestStickyPostSaysSo(t *testing.T) {
	exporter, _ := newMarkdownExporter(t)

	pinned := models.WordPressPost{ID: 100, Slug: "zonqor-point", Sticky: true}
	pinned.Title.Rendered = "Zonqor Point aerial footage"
	ordinary := models.WordPressPost{ID: 218, Slug: "azzure-like-window"}
	ordinary.Title.Rendered = "Azzure like Window"

	data := &models.ExportData{Posts: []models.WordPressPost{pinned, ordinary}}
	exporter.buildLookupMaps(data)

	assert.Contains(t, exporter.generateMarkdownContent(pinned, "post"), "\nsticky: true\n")
	assert.NotContains(t, exporter.generateMarkdownContent(ordinary, "post"), "sticky:",
		"omitted when false, so it appears only where the editor asked for it")

	assert.Contains(t, exporter.generateSSGContent(pinned, "post"), "\nsticky: true\n")
	assert.NotContains(t, exporter.generateSSGContent(ordinary, "post"), "sticky:")
}

// TestStickySurvivesTheExport: the flag has to reach the file on disk, not just
// the string builder.
func TestStickySurvivesTheExport(t *testing.T) {
	exporter, output := newMarkdownExporter(t)

	pinned := models.WordPressPost{ID: 100, Slug: "pinned", Link: "https://x.test/pinned/", Sticky: true}
	pinned.Title.Rendered = "Pinned"

	data := &models.ExportData{Pages: []models.WordPressPost{pinned}}
	exporter.buildLookupMaps(data)

	require.NoError(t, exporter.exportPagesMarkdown(data))

	body := readExportedFile(t, output, "pages", "pinned.md")
	assert.Contains(t, body, "sticky: true")
}

// readExportedFile reads one document out of an export tree.
func readExportedFile(t *testing.T, output string, parts ...string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(append([]string{output}, parts...)...))
	require.NoError(t, err)

	return string(body)
}

// TestPageTemplateSurvivesTheExport: a theme is often two designs rather than
// one — a page opening with a photographic banner and a page opening with a
// plain title band, a profile laid out unlike anything else on the site. Which
// of them a page gets is decided by its page template, and nothing else in an
// export says so: a consumer rebuilding the theme was left guessing from the
// content, or fetching every page again to look (#81).
func TestPageTemplateSurvivesTheExport(t *testing.T) {
	exporter, output := newMarkdownExporter(t)

	profile := models.WordPressPost{
		ID: 69, Slug: "matt-jones", Link: "https://x.test/people/matt-jones/",
		Template: "people-content-page.php",
	}
	profile.Title.Rendered = "Matt Jones"
	ordinary := models.WordPressPost{ID: 10, Slug: "services", Link: "https://x.test/services/"}
	ordinary.Title.Rendered = "Services"

	data := &models.ExportData{Pages: []models.WordPressPost{profile, ordinary}}
	exporter.buildLookupMaps(data)

	for _, generated := range []string{
		exporter.generateMarkdownContent(profile, "page"),
		exporter.generateSSGContent(profile, "page"),
	} {
		assert.Contains(t, generated, "source_template: \"people-content-page.php\"")
		// Not `template`: that names the template a generator should render
		// this document with, and a WordPress file name there would send the
		// build looking for one it does not have.
		assert.NotContains(t, generated, "\ntemplate:")
	}

	// WordPress reports an empty template for the default one, and an empty
	// value here would say "no template" where the truth is "the ordinary one".
	for _, generated := range []string{
		exporter.generateMarkdownContent(ordinary, "page"),
		exporter.generateSSGContent(ordinary, "page"),
	} {
		assert.NotContains(t, generated, "source_template:",
			"omitted for the default template, which WordPress reports as empty")
	}

	// And it has to reach the file on disk, not just the string builder.
	require.NoError(t, exporter.exportPagesMarkdown(data))
	// Under the path its link implies: a child page is written where the site
	// published it.
	assert.Contains(t, readExportedFile(t, output, "pages", "people", "matt-jones.md"),
		"source_template: \"people-content-page.php\"")
}
