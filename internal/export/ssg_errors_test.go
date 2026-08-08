package export

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/pkg/models"
)

// TestSSGPostDirectoryUnwritable covers the mkdir failure path.
func TestSSGPostDirectoryUnwritable(t *testing.T) {
	tmpDir := t.TempDir()

	// A file where the posts directory must go makes MkdirAll fail.
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "posts"), []byte("blocked"), 0600))

	e := NewExporter(ssgConfig(tmpDir))
	err := e.exportSSG(ssgFixture())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export posts")
}

// TestSSGPageDirectoryUnwritable covers the same path for pages.
func TestSSGPageDirectoryUnwritable(t *testing.T) {
	tmpDir := t.TempDir()
	data := ssgFixture()
	data.Posts = nil

	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "pages"), []byte("blocked"), 0600))

	e := NewExporter(ssgConfig(tmpDir))
	err := e.exportSSG(data)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export pages")
}

// TestSSGDocumentWriteFails covers the write failure path: a directory already
// occupies the document's filename.
func TestSSGDocumentWriteFails(t *testing.T) {
	tmpDir := t.TempDir()
	data := ssgFixture()
	data.Posts = nil

	blocked := filepath.Join(tmpDir, "pages", "baby-water-instructor", "cost.md")
	require.NoError(t, os.MkdirAll(blocked, 0750))

	e := NewExporter(ssgConfig(tmpDir))
	err := e.exportSSG(data)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write page file")
}

// TestSSGMetadataWriteFails covers the metadata failure path.
func TestSSGMetadataWriteFails(t *testing.T) {
	tmpDir := t.TempDir()
	data := ssgFixture()
	data.Posts = nil
	data.Pages = nil

	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "metadata.json"), 0750))

	e := NewExporter(ssgConfig(tmpDir))
	err := e.exportSSG(data)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to export metadata")
}

// TestSSGPrintsCompletionWhenNotQuiet covers the non-quiet branch.
func TestSSGPrintsCompletionWhenNotQuiet(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := ssgConfig(tmpDir)
	cfg.Quiet = false

	data := ssgFixture()
	data.Posts = nil
	data.Pages = nil

	e := NewExporter(cfg)
	require.NoError(t, e.exportSSG(data))
}

// TestSSGQuietSuppressesCompletion covers the quiet branch.
func TestSSGQuietSuppressesCompletion(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := ssgConfig(tmpDir)
	cfg.Quiet = true

	data := ssgFixture()
	data.Posts = nil
	data.Pages = nil

	e := NewExporter(cfg)
	require.NoError(t, e.exportSSG(data))
}

// TestBuildLookupMapsSkipsMediaWithoutAlt covers the branch where an attachment
// has no alt text to index.
func TestBuildLookupMapsSkipsMediaWithoutAlt(t *testing.T) {
	e := NewExporter(ssgConfig(t.TempDir()))

	data := &models.ExportData{
		Tags: []models.WordPressTag{{ID: 9, Name: "swimming"}},
		Media: []models.WordPressMedia{
			{ID: 1, SourceURL: "https://x.test/a.jpg", AltText: ""},
			{ID: 2, SourceURL: "https://x.test/b.jpg", AltText: "Described"},
		},
	}

	e.buildLookupMaps(data)

	assert.Equal(t, "swimming", e.tagMap[9])
	assert.Len(t, e.mediaMap, 2, "every attachment is indexed by ID")
	assert.NotContains(t, e.altMap, "https://x.test/a.jpg")
	assert.Equal(t, "Described", e.altMap["https://x.test/b.jpg"])
}

// TestReportAccessibilityWriteFails covers the report write failure path.
func TestReportAccessibilityWriteFails(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "a11y-report.md"), 0750))

	e := NewExporter(a11yConfig(tmpDir, true))

	require.Error(t, e.reportAccessibility(&models.ExportData{}))
}

// TestReportAccessibilityNotQuiet covers both non-quiet summary branches.
func TestReportAccessibilityNotQuiet(t *testing.T) {
	clean := t.TempDir()
	cfg := a11yConfig(clean, true)
	cfg.Quiet = false

	require.NoError(t, NewExporter(cfg).reportAccessibility(&models.ExportData{}))

	withIssues := t.TempDir()
	cfgIssues := a11yConfig(withIssues, true)
	cfgIssues.Quiet = false

	data := &models.ExportData{
		Posts: []models.WordPressPost{
			{ID: 1, Content: models.RenderedContent{Rendered: `<img src="/a.jpg">`}},
		},
	}

	require.NoError(t, NewExporter(cfgIssues).reportAccessibility(data))
}

// TestAuditImageAltSkipsDescribedImages covers the has-alt branch.
func TestAuditImageAltSkipsDescribedImages(t *testing.T) {
	content := `<img src="/a.jpg" alt="Described"><img src="/b.jpg" alt="   ">`

	findings := auditImageAlt(content, "post 1")

	require.Len(t, findings, 1, "only the whitespace-only alt should be reported")
	assert.Contains(t, findings[0].Detail, "/b.jpg")
}

// TestAuditContrastIgnoresUnparseableColors covers the branches where a style
// carries no color or one this checker cannot read.
func TestAuditContrastIgnoresUnparseableColors(t *testing.T) {
	assert.Empty(t, auditContrast(`<span style="font-weight: bold">x</span>`, "post 1"))
	assert.Empty(t, auditContrast(`<span style="color: var(--brand)">x</span>`, "post 1"))
}

// TestAuditContrastIgnoresUnreadableBackground falls back to the white
// assumption when the declared background cannot be parsed.
func TestAuditContrastIgnoresUnreadableBackground(t *testing.T) {
	findings := auditContrast(`<span style="color: #ffff00; background-color: var(--bg)">x</span>`, "post 1")

	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Detail, "background assumed white")
}
