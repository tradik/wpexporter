package export

// Two pages sharing a slug (#38). bociany.pl has 124 pages and exported 111
// files: a child of one branch and an unrelated top-level page both wrote
// pages/znaczenie-zerowisk-bociana-bialego.md, and whichever came second won.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// sameSlugPages are the two records from the issue: a child page and a
// top-level page whose slugs are identical and whose URLs are not.
func sameSlugPages() []models.WordPressPost {
	child := models.WordPressPost{
		ID:     123,
		Slug:   "znaczenie-zerowisk",
		Parent: 117,
		Link:   "https://bociany.pl/zerowisko-i-pokarm/znaczenie-zerowisk/",
	}
	child.Title.Rendered = "Znaczenie żerowisk (dziecko)"

	top := models.WordPressPost{
		ID:   7056,
		Slug: "znaczenie-zerowisk",
		Link: "https://bociany.pl/znaczenie-zerowisk/",
	}
	top.Title.Rendered = "Znaczenie żerowisk"

	parent := models.WordPressPost{ID: 117, Slug: "zerowisko-i-pokarm", Link: "https://bociany.pl/zerowisko-i-pokarm/"}
	parent.Title.Rendered = "Żerowisko i pokarm"

	return []models.WordPressPost{child, top, parent}
}

func newMarkdownExporter(t *testing.T) (*Exporter, string) {
	t.Helper()

	output := t.TempDir()

	return NewExporter(&config.Config{Output: output, Format: "markdown", Quiet: true}), output
}

// TestPagesKeepTheirOwnAddress: the fix. Each page lands under the path its URL
// states, so neither can overwrite the other and every live address has a
// document of its own.
func TestPagesKeepTheirOwnAddress(t *testing.T) {
	exporter, output := newMarkdownExporter(t)
	data := &models.ExportData{Pages: sameSlugPages()}
	exporter.buildLookupMaps(data)

	require.NoError(t, os.MkdirAll(filepath.Join(output, "pages"), 0750))
	require.NoError(t, exporter.exportPagesMarkdown(data))

	assert.FileExists(t, filepath.Join(output, "pages", "zerowisko-i-pokarm", "znaczenie-zerowisk.md"))
	assert.FileExists(t, filepath.Join(output, "pages", "znaczenie-zerowisk.md"))
	assert.Equal(t, 3, data.Stats.PagesWritten, "every page fetched is a page written")
}

// TestChildPageNamesItsParent: an ID means nothing after a migration, so the
// parent travels as a slug too, and a consumer can rebuild the tree without
// re-deriving it from paths.
func TestChildPageNamesItsParent(t *testing.T) {
	exporter, output := newMarkdownExporter(t)
	data := &models.ExportData{Pages: sameSlugPages()}
	exporter.buildLookupMaps(data)

	require.NoError(t, os.MkdirAll(filepath.Join(output, "pages"), 0750))
	require.NoError(t, exporter.exportPagesMarkdown(data))

	body, err := os.ReadFile(filepath.Join(output, "pages", "zerowisko-i-pokarm", "znaczenie-zerowisk.md"))
	require.NoError(t, err)

	assert.Contains(t, string(body), "parent: 117")
	assert.Contains(t, string(body), `parent_slug: "zerowisko-i-pokarm"`)
	assert.Contains(t, string(body), `link: "https://bociany.pl/zerowisko-i-pokarm/znaczenie-zerowisk/"`,
		"the address WordPress published, not one rebuilt from the slug")

	top, err := os.ReadFile(filepath.Join(output, "pages", "znaczenie-zerowisk.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(top), "parent:", "a top-level page has no parent to name")
}

// TestCollidingPagesAreBothWritten: when two documents still want one file —
// a site whose links are missing, so both fall back to the same slug — the
// second is renamed rather than allowed to replace the first.
func TestCollidingPagesAreBothWritten(t *testing.T) {
	exporter, output := newMarkdownExporter(t)

	first := models.WordPressPost{ID: 1, Slug: "kontakt"}
	first.Title.Rendered = "Kontakt"
	second := models.WordPressPost{ID: 2, Slug: "kontakt"}
	second.Title.Rendered = "Kontakt (inny)"

	data := &models.ExportData{Pages: []models.WordPressPost{first, second}}
	exporter.buildLookupMaps(data)

	require.NoError(t, os.MkdirAll(filepath.Join(output, "pages"), 0750))
	require.NoError(t, exporter.exportPagesMarkdown(data))

	assert.FileExists(t, filepath.Join(output, "pages", "kontakt.md"))
	assert.FileExists(t, filepath.Join(output, "pages", "kontakt-2.md"))
	assert.Equal(t, 2, data.Stats.PagesWritten)
}

// TestPagePlacementReportsCollisions: the rename is stated, because a page that
// quietly becomes a different page is the defect, not the remedy.
func TestPagePlacementReportsCollisions(t *testing.T) {
	placement := newPagePlacement()

	assert.Equal(t, "kontakt.md", placement.claim("pages", "kontakt.md", 1))
	assert.Equal(t, "kontakt-2.md", placement.claim("pages", "kontakt.md", 2))
	assert.Equal(t, "kontakt.md", placement.claim("pages", "kontakt.md", 1),
		"the same document offered twice keeps its file")

	report := placement.report()
	require.Len(t, report, 1)
	assert.Contains(t, report[0], "ids 1 and 2")
	assert.Contains(t, report[0], "kontakt-2.md")

	assert.Nil(t, newPagePlacement().report(), "a clean export reports nothing")
}
