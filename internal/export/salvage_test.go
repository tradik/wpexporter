package export

import (
	"strings"
	"testing"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// TestLocalizesEverySEOMediaField covers the second half of #30: og:image was the
// only metadata field rewritten, so twitter:image, the meta map and the JSON-LD
// blocks kept absolute source-host URLs even for files that had been downloaded.
func TestLocalizesEverySEOMediaField(t *testing.T) {
	const site = "https://example.com"
	const asset = site + "/wp-content/uploads/2024/03/logo.png"

	cfg := &config.Config{
		URL:            site,
		DownloadMedia:  true,
		MediaPathStyle: "root",
		Output:         t.TempDir(),
	}
	e := NewExporter(cfg)

	data := &models.ExportData{
		Media: []models.WordPressMedia{{ID: 1, SourceURL: asset, MimeType: "image/png"}},
		Posts: []models.WordPressPost{{
			ID: 7,
			SEO: models.SEOData{
				OGImage:      asset,
				TwitterImage: asset,
				Meta:         map[string]string{"msapplication-tileimage": asset},
				JSONLD:       []string{`{"@type":"Article","image":"` + asset + `","url":"` + site + `/a-post/"}`},
			},
		}},
		Marketing: &models.SiteMarketing{
			OGImage:        asset,
			Favicon:        asset,
			AppleTouchIcon: asset,
			Logo:           asset,
		},
	}

	e.updateMediaPaths(data)

	seo := data.Posts[0].SEO
	checks := map[string]string{
		"og_image":              seo.OGImage,
		"twitter_image":         seo.TwitterImage,
		"meta[msapplication]":   seo.Meta["msapplication-tileimage"],
		"json_ld":               seo.JSONLD[0],
		"marketing.og_image":    data.Marketing.OGImage,
		"marketing.favicon":     data.Marketing.Favicon,
		"marketing.apple_touch": data.Marketing.AppleTouchIcon,
		"marketing.logo":        data.Marketing.Logo,
	}

	for name, value := range checks {
		if strings.Contains(value, "example.com/wp-content") {
			t.Errorf("%s still points at the source host: %s", name, value)
		}
		if !strings.Contains(value, "/media/images/") {
			t.Errorf("%s was not localized: %s", name, value)
		}
	}

	// A page address inside JSON-LD is not an attachment and must survive intact —
	// the rewriter replaces only what resolves to an exported file.
	if !strings.Contains(seo.JSONLD[0], site+"/a-post/") {
		t.Errorf("json_ld page address should be untouched: %s", seo.JSONLD[0])
	}
}

// TestSalvageSkipsWhenNothingIsMissing guards the common case: an export whose
// every reference is already in the library must not attempt any network fetch.
func TestSalvageSkipsWhenNothingIsMissing(t *testing.T) {
	const site = "https://example.com"
	const asset = site + "/wp-content/uploads/2024/03/known.jpg"

	cfg := &config.Config{
		URL:            site,
		DownloadMedia:  true,
		MediaPathStyle: "root",
		Quiet:          true,
		Output:         t.TempDir(),
	}
	e := NewExporter(cfg)

	data := &models.ExportData{
		Media: []models.WordPressMedia{{ID: 1, SourceURL: asset, MimeType: "image/jpeg"}},
		Posts: []models.WordPressPost{{
			ID:      1,
			Content: models.RenderedContent{Rendered: `<img src="` + asset + `">`},
		}},
	}

	e.updateMediaPaths(data)

	if data.Stats.MediaDownloaded != 0 {
		t.Errorf("nothing should have been salvaged, got %d", data.Stats.MediaDownloaded)
	}
	if !strings.Contains(data.Posts[0].Content.Rendered, "/media/images/") {
		t.Errorf("known attachment was not localized: %s", data.Posts[0].Content.Rendered)
	}
}
