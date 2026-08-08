package export

import (
	"math"
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

func TestContrastRatio(t *testing.T) {
	tests := []struct {
		name string
		a    rgb
		b    rgb
		want float64
	}{
		{"black on white", rgb{0, 0, 0}, rgb{255, 255, 255}, 21},
		{"white on white", rgb{255, 255, 255}, rgb{255, 255, 255}, 1},
		{"yellow on white", rgb{255, 255, 0}, rgb{255, 255, 255}, 1.074},
		{"order does not matter", rgb{255, 255, 255}, rgb{0, 0, 0}, 21},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contrastRatio(tt.a, tt.b)
			assert.Less(t, math.Abs(got-tt.want), 0.01, "got %.3f, want %.3f", got, tt.want)
		})
	}
}

func TestParseColor(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		want  rgb
		valid bool
	}{
		{"six digit hex", "#ff0000", rgb{255, 0, 0}, true},
		{"three digit hex", "#f00", rgb{255, 0, 0}, true},
		{"uppercase hex", "#FF00FF", rgb{255, 0, 255}, true},
		{"named", "yellow", rgb{255, 255, 0}, true},
		{"named uppercase", "BLACK", rgb{0, 0, 0}, true},
		{"rgb", "rgb(1, 2, 3)", rgb{1, 2, 3}, true},
		{"rgba", "rgba(1, 2, 3, 0.5)", rgb{1, 2, 3}, true},
		{"padded", "  #ff0000  ", rgb{255, 0, 0}, true},
		{"empty", "", rgb{}, false},
		{"unknown keyword", "chartreuse-ish", rgb{}, false},
		{"bad hex length", "#ff00", rgb{}, false},
		{"rgb out of range", "rgb(1, 2, 300)", rgb{}, false},
		{"rgb non numeric", "rgb(a, b, c)", rgb{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseColor(tt.in)
			assert.Equal(t, tt.valid, ok)
			if tt.valid {
				assert.Equal(t, tt.want, got)
			}
		})
	}
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

// TestParseHexChannelRejectsNonHex pins the defensive branch: a channel that is
// not valid hex reads as zero rather than panicking.
func TestParseHexChannelRejectsNonHex(t *testing.T) {
	assert.Equal(t, uint8(0), parseHexChannel("zz"))
	assert.Equal(t, uint8(255), parseHexChannel("ff"))
	assert.Equal(t, uint8(0), parseHexChannel("00"))
}

func TestFormatColor(t *testing.T) {
	assert.Equal(t, "#ffff00", formatColor(rgb{255, 255, 0}))
	assert.Equal(t, "#000000", formatColor(rgb{0, 0, 0}))
}
