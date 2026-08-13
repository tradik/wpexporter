package export

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// customTypeFixture is a site whose theme publishes Services: real content with
// its own URL, which an export of posts and pages alone would drop (#28).
func customTypeFixture() *models.ExportData {
	stamp := models.WordPressTime{Time: time.Date(2024, 3, 24, 12, 0, 0, 0, time.UTC)}

	return &models.ExportData{
		CustomTypes: []models.CustomTypeSet{
			{
				Slug:     "cpt_services",
				Name:     "Services",
				RestBase: "cpt_services",
				Posts: []models.WordPressPost{
					{
						ID:       35133,
						Slug:     "wms-implementation",
						Status:   "publish",
						Link:     "https://magnavalor.eu/services/wms-implementation/",
						Date:     stamp,
						Modified: stamp,
						Title:    models.RenderedContent{Rendered: "WMS implementation"},
						Content:  models.RenderedContent{Rendered: "<p>We implement warehouse systems.</p>"},
					},
				},
			},
		},
	}
}

// TestSSGCustomTypesFollowTheirURL: a Services entry lands where its published
// address says it does, so /services/wms-implementation/ survives the move.
func TestSSGCustomTypesFollowTheirURL(t *testing.T) {
	tmpDir := t.TempDir()

	runSSGExport(t, ssgConfig(tmpDir), customTypeFixture())

	path := filepath.Join(tmpDir, "pages", "services", "wms-implementation.md")
	assert.FileExists(t, path)

	body := readFileString(t, path)
	// The WordPress type travels with the document, so a theme can still tell a
	// Service from a Page.
	assert.Contains(t, body, `type: "cpt_services"`)
	assert.Contains(t, body, `title: "WMS implementation"`)
	assert.Contains(t, body, "We implement warehouse systems.")
}

// TestMarkdownCustomTypesGetTheirOwnDirectory: the markdown format keeps each
// type visible as itself, in its own directory below pages/ — where a consumer
// that walks pages/ recursively will actually find it.
func TestMarkdownCustomTypesGetTheirOwnDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{URL: "https://magnavalor.eu", Output: tmpDir, Format: "markdown"}
	require.NoError(t, cfg.EnsureOutputDir())

	e := NewExporter(cfg)
	require.NoError(t, e.exportMarkdown(customTypeFixture()))

	body := readFileString(t, filepath.Join(tmpDir, "pages", "cpt_services", "wms-implementation.md"))
	assert.Contains(t, body, "type: \"cpt_services\"")
}

// TestCustomTypesReachMetadata: a consumer that reads metadata.json learns which
// types exist and how many entries they hold.
func TestCustomTypesReachMetadata(t *testing.T) {
	data := customTypeFixture()
	data.Stats.TotalCustomPosts = 1

	raw, err := exportMetadataJSON(data)
	require.NoError(t, err)

	var decoded struct {
		CustomTypes []models.CustomTypeSet `json:"custom_types"`
		Stats       models.ExportStats     `json:"stats"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded))

	require.Len(t, decoded.CustomTypes, 1)
	assert.Equal(t, "Services", decoded.CustomTypes[0].Name)
	assert.Equal(t, 1, decoded.Stats.TotalCustomPosts)
}

// TestCustomTypesAreLocalizedLikePages: media and addresses inside a custom
// type's entries go through the same rewriting, or a Services entry keeps
// pointing at the old host.
func TestCustomTypesAreLocalizedLikePages(t *testing.T) {
	tmpDir := t.TempDir()
	data := customTypeFixture()
	data.Media = []models.WordPressMedia{{
		ID:        391,
		SourceURL: "https://magnavalor.eu/wp-content/uploads/2024/03/wms.jpg",
		MimeType:  "image/jpeg",
	}}
	data.CustomTypes[0].Posts[0].Content.Rendered =
		`<img src="https://magnavalor.eu/wp-content/uploads/2024/03/wms.jpg">`

	// The export's own host, so the address fields are recognised as same-site.
	cfg := &config.Config{
		URL: "https://magnavalor.eu", Output: tmpDir, Format: "ssg",
		DownloadMedia: true, LinkStyle: "root",
	}
	runSSGExport(t, cfg, data)

	body := readFileString(t, filepath.Join(tmpDir, "pages", "services", "wms-implementation.md"))
	assert.NotContains(t, body, "https://magnavalor.eu/wp-content/uploads")
	assert.Contains(t, body, "/media/images/391_wms.jpg")
	// The address fields follow --link-style root too.
	assert.Contains(t, body, `link: "/services/wms-implementation/"`)
	assert.False(t, strings.Contains(body, `link: "https://magnavalor.eu`))
}

// TestCustomTypesEmptyIsANoOp: a site with no custom types writes no extra
// directories and no empty JSON key.
func TestCustomTypesEmptyIsANoOp(t *testing.T) {
	tmpDir := t.TempDir()
	runSSGExport(t, ssgConfig(tmpDir), ssgFixture())

	assert.NoDirExists(t, filepath.Join(tmpDir, "pages", "cpt_services"))

	raw, err := exportMetadataJSON(ssgFixture())
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "custom_types")
}
