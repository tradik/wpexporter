package export

// Backward compatibility of the export format.
//
// Every consumer of this tool reads the files it writes: a generator, an
// importer, a script somebody wrote once and forgot. A key that changes name, a
// document that moves, a marker that leaks into the text — each is a silent
// break on the far side, discovered by someone reading built HTML rather than
// by an error. These pin what 1.8.7 must keep doing exactly as before.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/pkg/models"
)

// TestFrontMatterKeysAreStable: the keys a consumer reads today keep their
// names and their spelling. New keys may appear beside them; none of these may
// disappear or change.
func TestFrontMatterKeysAreStable(t *testing.T) {
	exporter, output := newMarkdownExporter(t)

	page := models.WordPressPost{ID: 7, Slug: "about", Link: "https://x.test/about/", Status: "publish"}
	page.Title.Rendered = "About"
	page.Content.Rendered = `<p>We make things.</p>`

	data := &models.ExportData{Pages: []models.WordPressPost{page}}
	exporter.buildLookupMaps(data)

	require.NoError(t, os.MkdirAll(filepath.Join(output, "pages"), 0750))
	require.NoError(t, exporter.exportPagesMarkdown(data))

	body, err := os.ReadFile(filepath.Join(output, "pages", "about.md"))
	require.NoError(t, err)

	for _, key := range []string{"id:", "title:", "slug:", "date:", "modified:", "status:", "type:", "link:"} {
		assert.Contains(t, string(body), "\n"+key, "front-matter key %q", key)
	}

	assert.True(t, strings.HasPrefix(string(body), "---\n"), "the document still opens with front matter")
}

// TestNewStatsFieldsAreOmittedWhenEmpty: a clean export's metadata is what it
// always was. Fields added for a warning must not appear as empty arrays in
// every file a consumer already parses.
func TestNewStatsFieldsAreOmittedWhenEmpty(t *testing.T) {
	encoded, err := json.Marshal(models.ExportStats{TotalPosts: 3})
	require.NoError(t, err)

	for _, added := range []string{"uncovered", "post_loop_pages", "incomplete", "pages_written"} {
		assert.NotContains(t, string(encoded), added,
			"%q is new information, absent from an export that has none of it", added)
	}

	// The counts a consumer has always read are still spelled the same way.
	for _, kept := range []string{"total_posts", "total_pages", "total_media", "total_categories"} {
		assert.Contains(t, string(encoded), kept)
	}
}

// TestListConversionLeaksNoMarker: the placeholder that carries an unnumberable
// list through the pipeline is an internal device. A NUL byte reaching a
// document would corrupt it for every reader.
func TestListConversionLeaksNoMarker(t *testing.T) {
	for _, body := range []string{
		`<ol type="a"><li>alpha</li></ol>`,
		`<ol reversed><li>last</li></ol><ul><li>plain</li></ul>`,
		`<ol type="i"><li>one</li></ol><ol><li>two</li></ol><ol type="a"><li>three</li></ol>`,
	} {
		out := htmlToMarkdown(body)

		assert.NotContains(t, out, "\x00", "body %q", body)
		assert.NotContains(t, out, "wpx-list", "body %q", body)
	}
}

// TestUnorderedListOutputIsUnchanged: the fix is about ordered lists. A bullet
// list must convert exactly as it did before, or every consumer that reads
// bullets has to be re-tested for no reason.
func TestUnorderedListOutputIsUnchanged(t *testing.T) {
	assert.Equal(t, "- one\n- two",
		strings.TrimSpace(htmlToMarkdown(`<ul><li>one</li><li>two</li></ul>`)))
}
