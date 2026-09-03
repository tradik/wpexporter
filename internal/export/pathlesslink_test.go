package export

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// pathlessFixture is a site whose gallery plugin registers a public post type
// with no rewrite rule, so WordPress publishes its entries at /?modula-gallery=N
// — an address whose path is the site root and whose meaning is all query (#78).
func pathlessFixture() *models.ExportData {
	stamp := models.WordPressTime{Time: time.Date(2025, 11, 4, 9, 0, 0, 0, time.UTC)}

	entry := func(id int, slug, title string) models.WordPressPost {
		return models.WordPressPost{
			ID:       id,
			Slug:     slug,
			Status:   "publish",
			Type:     "modula-gallery",
			Link:     "https://www.eadecology.co.uk/?modula-gallery=" + slug,
			Date:     stamp,
			Modified: stamp,
			Title:    models.RenderedContent{Rendered: title},
			Content:  models.RenderedContent{Rendered: "<p>Gallery.</p>"},
		}
	}

	return &models.ExportData{
		Pages: []models.WordPressPost{
			{
				ID:       2,
				Slug:     "home",
				Status:   "publish",
				Link:     "https://www.eadecology.co.uk/",
				Date:     stamp,
				Modified: stamp,
				Title:    models.RenderedContent{Rendered: "Home"},
				Content:  models.RenderedContent{Rendered: "<p>Ecology for the built environment.</p>"},
			},
		},
		CustomTypes: []models.CustomTypeSet{
			{
				Slug:     "modula-gallery",
				Name:     "Galleries",
				RestBase: "modula-gallery",
				Posts: []models.WordPressPost{
					entry(1289, "1289", "lifeatead"),
					entry(1876, "1876", "surveys"),
				},
			},
		},
	}
}

func pathlessConfig(output, format string) *config.Config {
	return &config.Config{
		URL:       "https://www.eadecology.co.uk",
		Output:    output,
		Format:    format,
		LinkStyle: "root",
	}
}

// TestPathlessCustomTypeIsGivenTheAddressItIsFiledAt: a permalink that carries
// no path is replaced by /<type>/<slug>/, so the document's front matter names
// the same address as its place in the tree instead of claiming the site root.
func TestPathlessCustomTypeIsGivenTheAddressItIsFiledAt(t *testing.T) {
	tmpDir := t.TempDir()

	runSSGExport(t, pathlessConfig(tmpDir, "ssg"), pathlessFixture())

	body := readFileString(t, filepath.Join(tmpDir, "pages", "modula-gallery", "1289.md"))
	assert.Contains(t, body, `link: "/modula-gallery/1289/"`)
	assert.NotContains(t, body, "?modula-gallery=")
}

// TestPathlessEntriesDoNotOverwriteTheFrontPage: two query-string entries used
// to reduce to the same address as the home page. Each now has one of its own,
// and the front page is still the front page.
func TestPathlessEntriesDoNotOverwriteTheFrontPage(t *testing.T) {
	tmpDir := t.TempDir()

	runSSGExport(t, pathlessConfig(tmpDir, "ssg"), pathlessFixture())

	first := readFileString(t, filepath.Join(tmpDir, "pages", "modula-gallery", "1289.md"))
	second := readFileString(t, filepath.Join(tmpDir, "pages", "modula-gallery", "1876.md"))
	home := readFileString(t, filepath.Join(tmpDir, "pages", "home.md"))

	assert.Contains(t, first, `link: "/modula-gallery/1289/"`)
	assert.Contains(t, second, `link: "/modula-gallery/1876/"`)
	assert.Contains(t, home, `link: "/"`)
	assert.Contains(t, home, "Ecology for the built environment.")
}

// TestMarkdownPathlessCustomTypeMatchesItsDirectory: the markdown format already
// filed these under pages/<type>/, and the address now agrees with it.
func TestMarkdownPathlessCustomTypeMatchesItsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := pathlessConfig(tmpDir, "markdown")
	require.NoError(t, cfg.EnsureOutputDir())

	e := NewExporter(cfg)
	data := pathlessFixture()
	e.localizeAddresses(data)
	require.NoError(t, e.exportMarkdown(data))

	body := readFileString(t, filepath.Join(tmpDir, "pages", "modula-gallery", "1289.md"))
	assert.Contains(t, body, `link: "/modula-gallery/1289/"`)
}

// TestPathlessPageKeepsItsSlug: a WordPress left on plain permalinks publishes
// every page as /?page_id=N. A page is not a custom type, so it takes its slug
// alone rather than growing a "page" directory in front of it.
func TestPathlessPageKeepsItsSlug(t *testing.T) {
	tmpDir := t.TempDir()
	data := pathlessFixture()
	data.Pages = append(data.Pages, models.WordPressPost{
		ID:     45,
		Slug:   "about",
		Status: "publish",
		Type:   "page",
		Link:   "https://www.eadecology.co.uk/?page_id=45",
		Title:  models.RenderedContent{Rendered: "About"},
	})

	runSSGExport(t, pathlessConfig(tmpDir, "ssg"), data)

	body := readFileString(t, filepath.Join(tmpDir, "pages", "about.md"))
	assert.Contains(t, body, `link: "/about/"`)
}

// TestCanonicalKeepsItsQuery: the canonical names a document on the source site,
// where the query really is the address, so it is left as WordPress wrote it.
func TestCanonicalKeepsItsQuery(t *testing.T) {
	post := &models.WordPressPost{
		Slug: "1289",
		Link: "https://www.eadecology.co.uk/?modula-gallery=1289",
		SEO: models.SEOData{
			CanonicalURL: "https://www.eadecology.co.uk/?modula-gallery=1289",
		},
	}

	e := NewExporter(pathlessConfig(t.TempDir(), "ssg"))
	e.rootRelativizeAddresses(post, "modula-gallery")

	assert.Equal(t, "/modula-gallery/1289/", post.Link)
	assert.Equal(t, "/?modula-gallery=1289", post.SEO.CanonicalURL)
}

// TestSynthesizePath pins when an address is invented and when the one in hand
// is kept.
func TestSynthesizePath(t *testing.T) {
	tests := []struct {
		name     string
		link     string
		slug     string
		typeSlug string
		want     string
		wantOK   bool
	}{
		{
			name:     "query-only link takes the type and slug",
			link:     "/?modula-gallery=1289",
			slug:     "1289",
			typeSlug: "modula-gallery",
			want:     "/modula-gallery/1289/",
			wantOK:   true,
		},
		{
			name:   "query-only page takes its slug alone",
			link:   "/?page_id=45",
			slug:   "about",
			want:   "/about/",
			wantOK: true,
		},
		{
			name:     "a pretty permalink is left alone",
			link:     "/services/wms-implementation/",
			slug:     "wms-implementation",
			typeSlug: "cpt_services",
			wantOK:   false,
		},
		{
			name:   "a foreign host is not ours to rewrite",
			link:   "https://example.org/?p=7",
			slug:   "elsewhere",
			wantOK: false,
		},
		{
			name:     "the front page's own root is a real address",
			link:     "/",
			slug:     "home",
			typeSlug: "",
			wantOK:   false,
		},
		{
			name:     "a missing link has no query to have hidden an address in",
			slug:     "1289",
			typeSlug: "modula-gallery",
			wantOK:   false,
		},
		{
			name:   "nothing to build from",
			link:   "/?p=7",
			wantOK: false,
		},
		{
			name:   "an unparsable link is left as it is",
			link:   "://not a url",
			slug:   "1289",
			wantOK: false,
		},
		{
			name:     "traversal segments never reach the address",
			link:     "/?p=7",
			slug:     "../../etc/passwd",
			typeSlug: "..",
			want:     "/etc/passwd/",
			wantOK:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := synthesizePath(tc.link, tc.slug, tc.typeSlug)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
