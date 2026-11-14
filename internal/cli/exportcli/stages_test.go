package exportcli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/export"
	"github.com/tradik/wpexporter/pkg/models"
)

// brokenSource fails the named collection with an error that is not a gap:
// the source broke, rather than leaving a hole.
type brokenSource struct {
	broken string
	site   *models.SiteInfo
}

func (b brokenSource) fail(name string) error {
	if b.broken == name {
		return errors.New("source broke")
	}
	return nil
}
func (b brokenSource) GetSiteInfo() (*models.SiteInfo, error) { return b.site, b.fail("site") }
func (b brokenSource) GetPosts() ([]models.WordPressPost, error) {
	return nil, b.fail("posts")
}
func (b brokenSource) GetPages() ([]models.WordPressPost, error) {
	return nil, b.fail("pages")
}
func (b brokenSource) GetMedia() ([]models.WordPressMedia, error) { return nil, b.fail("media") }
func (b brokenSource) GetCategories() ([]models.WordPressCategory, error) {
	return nil, b.fail("categories")
}
func (b brokenSource) GetTags() ([]models.WordPressTag, error)   { return nil, b.fail("tags") }
func (b brokenSource) GetUsers() ([]models.WordPressUser, error) { return nil, b.fail("users") }
func (b brokenSource) GetMenus() ([]models.WordPressMenu, error) { return nil, b.fail("menus") }
func (b brokenSource) GetComments() ([]models.WordPressComment, error) {
	return nil, b.fail("comments")
}
func (b brokenSource) GetPostTypes() ([]api.PostType, error) { return nil, b.fail("types") }
func (b brokenSource) GetCustomPosts(string) ([]models.WordPressPost, error) {
	return nil, b.fail("custom")
}

// A source that breaks — not one that leaves a gap — ends the run, naming the
// collection.
func TestBrokenSourceEndsTheRun(t *testing.T) {
	quietMode = true
	t.Cleanup(func() { quietMode = false })

	for _, broken := range []string{"site", "posts", "pages", "media", "categories", "tags"} {
		cfg := config.DefaultConfig()
		cfg.URL = "https://example.com"
		r := &exportRun{cfg: cfg, src: brokenSource{broken: broken, site: &models.SiteInfo{}}}

		var err error
		switch broken {
		case "site", "posts", "pages":
			err = r.fetchRemoteDocuments()
		case "media":
			err = r.fetchRemoteMedia()
		default:
			err = r.fetchRemoteRest()
		}
		require.Error(t, err, broken)
		assert.Contains(t, err.Error(), "source broke", broken)
	}
}

// The summary names every kind of thing a run can have to report.
func TestSummaryReportsEverything(t *testing.T) {
	out := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Output, cfg.BruteForce, cfg.CreateZip = out, true, true
	require.NoError(t, os.WriteFile(out+".zip", []byte("zip"), 0o600))

	r := &exportRun{cfg: cfg, stated: zeroCounts{}, c: collected{
		siteInfo: &models.SiteInfo{Name: "S"}, pages: make([]models.WordPressPost, 3),
		customTypes:    []models.CustomTypeSet{{Name: "Services", Slug: "service", Posts: make([]models.WordPressPost, 2)}},
		productsNotice: "Products: read from the public route", gaps: []string{"tags page 2"},
		notices: []string{"a fact"}, bruteForceFound: 4,
	}}
	data := &models.ExportData{Stats: models.ExportStats{PagesWritten: 2, Uncovered: []string{"1 address not exported"}}}

	log := captureStdout(t, func() { r.summarize(data, out+".zip") })
	for _, want := range []string{"Pages: 3 fetched, 2 written", "Services (service): 2", "Products: read from",
		"Incomplete: tags page 2", "Note: a fact", "1 address not exported", "Brute force found: 4", "ZIP size:", "Output:"} {
		assert.Contains(t, log, want)
	}
}

// Writing fails cleanly when the archive or the cleanup cannot be done.
func TestWriteFailures(t *testing.T) {
	quietMode = true
	t.Cleanup(func() { quietMode = false })

	dir := t.TempDir()
	cfg := config.DefaultConfig()
	cfg.Format, cfg.DownloadMedia, cfg.CreateZip = "json", false, true
	cfg.Output = filepath.Join(dir, "out")
	require.NoError(t, os.MkdirAll(cfg.Output+".zip", 0o750)) // the archive's path is taken

	r := &exportRun{cfg: cfg, exporter: export.NewExporter(cfg)}
	_, err := r.write(&models.ExportData{})
	assert.ErrorContains(t, err, "ZIP")

	// --no-files removes the export after zipping it.
	cfg.Output, cfg.NoFiles = filepath.Join(dir, "second"), true
	zipPath, err := r.write(&models.ExportData{})
	require.NoError(t, err)
	assert.FileExists(t, zipPath)
	assert.NoDirExists(t, cfg.Output)
}

// zeroCounts is a source that states no totals.
type zeroCounts struct{}

func (zeroCounts) StatedTotal(string) int { return 0 }
func (zeroCounts) Limited() bool          { return false }
