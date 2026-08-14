package export

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

func a11yConfig(output string, enabled bool) *config.Config {
	return &config.Config{
		URL:        "https://hawanas.com",
		Output:     output,
		Format:     "markdown",
		ReportA11y: enabled,
		Quiet:      true,
	}
}

// TestReportAccessibilityFlagsLowContrast covers the case from #11: 2010-era
// editor colors that fail WCAG 2.2 SC 1.4.3 on white.
func TestReportAccessibilityFlagsLowContrast(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewExporter(a11yConfig(tmpDir, true))

	data := &models.ExportData{
		Posts: []models.WordPressPost{
			{
				ID:   1,
				Slug: "bright",
				Content: models.RenderedContent{
					Rendered: `<span style="color: #ffff00">yellow on white</span>`,
				},
			},
		},
	}

	require.NoError(t, e.reportAccessibility(data))

	report := readFileString(t, filepath.Join(tmpDir, "a11y-report.md"))

	assert.Contains(t, report, "SC 1.4.3")
	assert.Contains(t, report, "#ffff00")
	assert.Contains(t, report, "background assumed white")
	assert.Contains(t, report, "post 1 (bright)")
}

// TestReportAccessibilityRespectsDeclaredBackground pins that a declared
// background is used instead of the white assumption.
func TestReportAccessibilityRespectsDeclaredBackground(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewExporter(a11yConfig(tmpDir, true))

	data := &models.ExportData{
		Posts: []models.WordPressPost{
			{
				ID:   1,
				Slug: "ok",
				Content: models.RenderedContent{
					Rendered: `<span style="color: #ffff00; background-color: #000000">yellow on black</span>`,
				},
			},
		},
	}

	require.NoError(t, e.reportAccessibility(data))

	report := readFileString(t, filepath.Join(tmpDir, "a11y-report.md"))
	assert.Contains(t, report, "No contrast or alt-text issues found")
}

// TestReportAccessibilityFlagsMissingAlt covers SC 1.1.1.
func TestReportAccessibilityFlagsMissingAlt(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewExporter(a11yConfig(tmpDir, true))

	data := &models.ExportData{
		Pages: []models.WordPressPost{
			{
				ID:      7,
				Slug:    "about",
				Content: models.RenderedContent{Rendered: `<img src="/media/images/1_a.jpg">`},
			},
		},
	}

	require.NoError(t, e.reportAccessibility(data))

	report := readFileString(t, filepath.Join(tmpDir, "a11y-report.md"))

	assert.Contains(t, report, "SC 1.1.1")
	assert.Contains(t, report, "/media/images/1_a.jpg")
	assert.Contains(t, report, "page 7 (about)")
}

// TestReportAccessibilityDisabled pins that no file is written without the flag.
func TestReportAccessibilityDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	e := NewExporter(a11yConfig(tmpDir, false))

	data := &models.ExportData{
		Posts: []models.WordPressPost{
			{ID: 1, Content: models.RenderedContent{Rendered: `<img src="/a.jpg">`}},
		},
	}

	require.NoError(t, e.reportAccessibility(data))
	assert.NoFileExists(t, filepath.Join(tmpDir, "a11y-report.md"))
}

// TestAuditContrastDeduplicates pins that the same color pair is reported once
// per document, not once per occurrence.
func TestAuditContrastDeduplicates(t *testing.T) {
	content := `<span style="color: #ffff00">a</span><span style="color: #ffff00">b</span>`

	assert.Len(t, auditContrast(content, "post 1"), 1)
}

// TestDeclarationValue pins that "background-color" is never read as "color".
func TestDeclarationValue(t *testing.T) {
	style := "background-color: #000; color: #fff; font-weight: bold"

	assert.Equal(t, "#fff", declarationValue(style, "color"))
	assert.Equal(t, "#000", declarationValue(style, "background-color"))
	assert.Equal(t, "", declarationValue(style, "border"))
	assert.Equal(t, "", declarationValue("no-colon-here", "color"))
}

func TestInlineStyles(t *testing.T) {
	content := `<a style="color: red">x</a><b style='color: blue'>y</b><i>z</i>`

	assert.Equal(t, []string{"color: red", "color: blue"}, inlineStyles(content))
}
