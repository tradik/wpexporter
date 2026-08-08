package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// localizeFixture mirrors the export shape reported in issue #13: an attachment
// referenced from the body, from featured_media, and from a scraped og:image.
func localizeFixture() *models.ExportData {
	return &models.ExportData{
		Posts: []models.WordPressPost{
			{
				ID:            1,
				Slug:          "swimming",
				Title:         models.RenderedContent{Rendered: "Swimming"},
				FeaturedMedia: 602,
				Content: models.RenderedContent{
					Rendered: `<img src="https://hawanas.com/wp-content/uploads/sites/2/2009/09/11.jpg">`,
				},
				SEO: models.SEOData{
					OGImage:      "https://hawanas.com/wp-content/uploads/sites/2/2009/09/11.jpg",
					CanonicalURL: "https://hawanas.com/2010/07/21/389/",
				},
			},
		},
		Media: []models.WordPressMedia{
			{
				ID:        602,
				SourceURL: "https://hawanas.com/wp-content/uploads/sites/2/2009/09/11.jpg",
				MimeType:  "image/jpeg",
			},
		},
	}
}

func localizeConfig(output string) *config.Config {
	return &config.Config{
		Output:        output,
		Format:        "markdown",
		DownloadMedia: true,
	}
}

// TestLocalizeOGImage covers the og_image half of issue #13.
func TestLocalizeOGImage(t *testing.T) {
	e := NewExporter(localizeConfig(t.TempDir()))
	data := localizeFixture()

	e.updateMediaPaths(data)

	assert.Equal(t, "/media/images/602_11.jpg", data.Posts[0].SEO.OGImage,
		"og_image should be localized like the body content")
}

// TestLocalizeCanonicalURLUntouched pins the deliberate exclusion: canonical_url
// is an address of the source site, not an asset.
func TestLocalizeCanonicalURLUntouched(t *testing.T) {
	e := NewExporter(localizeConfig(t.TempDir()))
	data := localizeFixture()

	e.updateMediaPaths(data)

	assert.Equal(t, "https://hawanas.com/2010/07/21/389/", data.Posts[0].SEO.CanonicalURL,
		"canonical_url must stay absolute")
}

// TestLocalizeOGImageOnForeignHost covers the case called out in #13: an og:image
// that is not a downloaded attachment has nothing to localize.
func TestLocalizeOGImageOnForeignHost(t *testing.T) {
	e := NewExporter(localizeConfig(t.TempDir()))
	data := localizeFixture()
	data.Posts[0].SEO.OGImage = "https://cdn.example.net/social/card.png"

	e.updateMediaPaths(data)

	assert.Equal(t, "https://cdn.example.net/social/card.png", data.Posts[0].SEO.OGImage,
		"an og:image outside the media library must stay absolute")
}

// TestLocalizeFeaturedImage covers the featured_image half of issue #13, end to
// end through the markdown writer.
func TestLocalizeFeaturedImage(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewExporter(localizeConfig(tmpDir))
	data := localizeFixture()

	written := exportMarkdownWithoutDownloads(t, e, data, tmpDir)

	assert.Contains(t, written, `featured_image: "/media/images/602_11.jpg"`,
		"featured_image should point at the exported file")
	assert.NotContains(t, written, "hawanas.com/wp-content",
		"no absolute wp-content URL should survive in the front matter")
}

// TestLocalizeKeepOriginalURLs pins that --keep-original-urls still opts out of
// localization for the front-matter fields too.
func TestLocalizeKeepOriginalURLs(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := localizeConfig(tmpDir)
	cfg.KeepOriginalURLs = true

	e := NewExporter(cfg)
	data := localizeFixture()

	written := exportMarkdownWithoutDownloads(t, e, data, tmpDir)

	assert.Contains(t, written, `featured_image: "https://hawanas.com/wp-content/uploads/sites/2/2009/09/11.jpg"`,
		"--keep-original-urls should leave featured_image absolute")
	assert.Contains(t, written, "https://hawanas.com/wp-content/uploads/sites/2/2009/09/11.jpg",
		"--keep-original-urls should leave og_image absolute")
}

// TestLocalizeMediaURLWithoutRewriter pins the nil-rewriter path used by formats
// that keep original URLs.
func TestLocalizeMediaURLWithoutRewriter(t *testing.T) {
	e := NewExporter(localizeConfig(t.TempDir()))

	raw := "https://hawanas.com/wp-content/uploads/sites/2/2009/09/11.jpg"
	assert.Equal(t, raw, e.localizeMediaURL(raw))
}

// exportMarkdownWithoutDownloads runs the markdown export path minus the media
// download step, which would otherwise reach the network. It mirrors the
// rewriting condition in Export.
func exportMarkdownWithoutDownloads(t *testing.T, e *Exporter, data *models.ExportData, root string) string {
	t.Helper()

	require.NoError(t, e.config.EnsureOutputDir())

	if !e.config.KeepOriginalURLs {
		e.updateMediaPaths(data)
	}

	require.NoError(t, e.exportMarkdown(data))

	return readExportedPost(t, root)
}

// readExportedPost returns the single markdown file written under posts/.
func readExportedPost(t *testing.T, root string) string {
	t.Helper()

	var found string
	err := filepath.Walk(filepath.Join(root, "posts"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			body, readErr := os.ReadFile(path) // #nosec G304 -- path comes from the test's own temp dir
			if readErr != nil {
				return readErr
			}
			found = string(body)
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, found, "expected a markdown post to be written")

	return found
}
