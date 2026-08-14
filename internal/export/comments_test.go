package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// commentedSiteFixture is a site with one commented post, one commented page
// and a comment whose post was not exported (#35).
func commentedSiteFixture() *models.ExportData {
	stamp := models.WordPressTime{Time: time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC)}

	return &models.ExportData{
		Posts: []models.WordPressPost{{
			ID: 7, Slug: "wms", Status: "publish",
			Link:  "https://hawanas.com/blog/wms/",
			Date:  stamp,
			Title: models.RenderedContent{Rendered: "WMS"},
		}},
		Pages: []models.WordPressPost{{
			ID: 9, Slug: "about", Status: "publish",
			Link:  "https://hawanas.com/about/",
			Date:  stamp,
			Title: models.RenderedContent{Rendered: "About"},
		}},
		Comments: []models.WordPressComment{
			{ID: 11, Post: 7, Author: "Jan", Date: stamp, Content: "<p>Dobry tekst</p>",
				Status: "approved", Link: "https://hawanas.com/blog/wms/#comment-11"},
			{ID: 12, Post: 7, Parent: 11, Author: "Ewa", Date: stamp, Content: "<p>Zgadzam się</p>",
				Status: "approved", Link: "https://hawanas.com/blog/wms/#comment-12"},
			{ID: 13, Post: 9, Author: "Ola", Date: stamp, Content: "<p>Hello</p>",
				Status: "approved", Link: "https://hawanas.com/about/#comment-13"},
			// Its post was excluded from this export — the permalink is all that
			// names the page.
			{ID: 14, Post: 999, Author: "Nieznany", Date: stamp, Content: "<p>?</p>",
				Status: "approved", Link: "https://hawanas.com/old-post/#comment-14"},
		},
	}
}

func readCommentsFile(t *testing.T, dir string) commentsFile {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(dir, commentsFileName)) // #nosec G304 -- test temp dir
	require.NoError(t, err, "expected comments.json to exist")

	var parsed commentsFile
	require.NoError(t, json.Unmarshal(body, &parsed))

	return parsed
}

// TestCommentsExportedByPageURL: comments leave the export addressed by the
// page they belong to, because a WordPress post ID means nothing on the other
// side of a migration.
func TestCommentsExportedByPageURL(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := ssgConfig(tmpDir)
	cfg.LinkStyle = "root"

	runSSGExport(t, cfg, commentedSiteFixture())

	parsed := readCommentsFile(t, tmpDir)
	assert.Equal(t, 4, parsed.Total)
	assert.Equal(t, 3, parsed.Pages, "three distinct commented pages")

	require.Len(t, parsed.Comments, 4)
	assert.Equal(t, "/blog/wms/", parsed.Comments[0].PostURL)
	assert.Equal(t, "/blog/wms/", parsed.Comments[1].PostURL)
	assert.Equal(t, 11, parsed.Comments[1].Parent, "the thread survives")
	assert.Equal(t, "/about/", parsed.Comments[2].PostURL)
	assert.Equal(t, "/old-post/", parsed.Comments[3].PostURL,
		"a comment whose post was not exported falls back to its own permalink, without the anchor")
	assert.Equal(t, "/blog/wms/#comment-11", parsed.Comments[0].Link)
}

// TestCommentsKeepAbsoluteAddresses: with the default link style the addresses
// stay absolute, exactly like the posts' own links.
func TestCommentsKeepAbsoluteAddresses(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := ssgConfig(tmpDir)
	cfg.LinkStyle = "absolute"

	runSSGExport(t, cfg, commentedSiteFixture())

	parsed := readCommentsFile(t, tmpDir)
	assert.Equal(t, "https://hawanas.com/blog/wms/", parsed.Comments[0].PostURL)
	assert.Equal(t, "https://hawanas.com/old-post/", parsed.Comments[3].PostURL)
}

// TestNoCommentsFileWithoutComments: an export with no comments writes no
// file — an empty comments.json would claim the site has none when the truth
// may be that they were never requested.
func TestNoCommentsFileWithoutComments(t *testing.T) {
	tmpDir := t.TempDir()

	runSSGExport(t, ssgConfig(tmpDir), &models.ExportData{
		Pages: []models.WordPressPost{{ID: 1, Slug: "x", Link: "https://hawanas.com/x/"}},
	})

	_, err := os.Stat(filepath.Join(tmpDir, commentsFileName))
	assert.True(t, os.IsNotExist(err), "comments.json must not be written")
}

// TestMarkdownExportWritesComments: the markdown writer — the one a migration
// uses — leaves comments.json beside metadata.json.
func TestMarkdownExportWritesComments(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		URL: "https://hawanas.com", Output: tmpDir, Format: "markdown",
		DownloadMedia: false, LinkStyle: "root",
	}

	require.NoError(t, NewExporter(cfg).Export(commentedSiteFixture()))

	parsed := readCommentsFile(t, tmpDir)
	assert.Equal(t, 4, parsed.Total)
	assert.Equal(t, "Jan", parsed.Comments[0].Author)
	assert.Equal(t, "/blog/wms/", parsed.Comments[0].PostURL)
	assert.FileExists(t, filepath.Join(tmpDir, "metadata.json"))
}

// TestCommentsFromCustomTypeEntries: a comment on a Services entry is addressed
// like any other — custom post types are content too (#28).
func TestCommentsFromCustomTypeEntries(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := ssgConfig(tmpDir)
	cfg.LinkStyle = "root"

	data := &models.ExportData{
		CustomTypes: []models.CustomTypeSet{{
			Slug: "cpt_services", Name: "Services", RestBase: "cpt_services",
			Posts: []models.WordPressPost{{
				ID: 21, Slug: "wms-implementation",
				Link:  "https://hawanas.com/services/wms-implementation/",
				Title: models.RenderedContent{Rendered: "WMS implementation"},
			}},
		}},
		Comments: []models.WordPressComment{{
			ID: 30, Post: 21, Author: "Piotr", Content: "<p>Świetne</p>", Status: "approved",
		}},
	}

	runSSGExport(t, cfg, data)

	parsed := readCommentsFile(t, tmpDir)
	assert.Equal(t, "/services/wms-implementation/", parsed.Comments[0].PostURL)
}

// TestExportCommentsUnwritableDirectory: a write failure is reported rather
// than swallowed.
func TestExportCommentsUnwritableDirectory(t *testing.T) {
	e := NewExporter(&config.Config{Output: filepath.Join(t.TempDir(), "missing")})

	err := e.exportComments([]models.WordPressComment{{ID: 1, Post: 1}})
	require.Error(t, err)
}

// TestCountCommentedPagesGroupsByPostWhenURLUnknown: comments with no address
// still group by their post, so the count is never inflated.
func TestCountCommentedPagesGroupsByPostWhenURLUnknown(t *testing.T) {
	assert.Equal(t, 2, countCommentedPages([]models.WordPressComment{
		{ID: 1, Post: 5}, {ID: 2, Post: 5}, {ID: 3, Post: 6},
	}))
}

// TestStripFragment covers the anchor-trimming helper directly, including the
// address that carries none.
func TestStripFragment(t *testing.T) {
	assert.Equal(t, "/a/", stripFragment("/a/#comment-1"))
	assert.Equal(t, "/a/", stripFragment("/a/"))
	assert.Empty(t, stripFragment(""))
}
