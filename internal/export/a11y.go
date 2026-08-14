package export

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tradik/wpexporter/internal/wcag"
	"github.com/tradik/wpexporter/pkg/models"
)

// minContrastRatio is WCAG 2.2 SC 1.4.3 (Contrast Minimum) for normal-size text.
const minContrastRatio = wcag.MinContrastRatio

// defaultBackground is assumed when content sets a color but no background: a
// 2010-era WordPress theme almost always renders body copy on white, which is
// the worst case for the bright editor colors this check is aimed at.
var defaultBackground = wcag.Color{R: 255, G: 255, B: 255}

var (
	// styleAttrPattern matches an inline style attribute.
	styleAttrPattern = regexp.MustCompile(`(?is)style\s*=\s*"([^"]*)"|style\s*=\s*'([^']*)'`)
)

// a11yFinding is one accessibility problem found in exported content.
type a11yFinding struct {
	Location  string
	Criterion string
	Detail    string
}

// reportAccessibility writes an accessibility report for the exported content.
//
// It does not change the export. Any WordPress site of a certain age carries
// inline editor colors that fail WCAG 2.2 SC 1.4.3, and images with no alt text
// that fail SC 1.1.1; redesigning the content is not the exporter's job, but
// telling the operator before they publish is cheap and actionable.
func (e *Exporter) reportAccessibility(data *models.ExportData) error {
	if !e.config.ReportA11y {
		return nil
	}

	var findings []a11yFinding

	for _, post := range data.Posts {
		findings = append(findings, e.auditPost(post, "post")...)
	}

	for _, page := range data.Pages {
		findings = append(findings, e.auditPost(page, "page")...)
	}

	if err := e.writeA11yReport(findings); err != nil {
		return err
	}

	if !e.config.Quiet {
		if len(findings) == 0 {
			fmt.Println("Accessibility: no contrast or alt-text issues found")
		} else {
			fmt.Printf("Accessibility: %d issue(s) found, see a11y-report.md\n", len(findings))
		}
	}

	return nil
}

// auditPost collects the accessibility findings for one post or page.
func (e *Exporter) auditPost(post models.WordPressPost, kind string) []a11yFinding {
	location := fmt.Sprintf("%s %d (%s)", kind, post.ID, post.Slug)

	findings := auditContrast(post.Content.Rendered, location)
	findings = append(findings, auditImageAlt(post.Content.Rendered, location)...)

	return findings
}

// auditContrast reports foreground/background pairs below the WCAG minimum.
func auditContrast(content, location string) []a11yFinding {
	var findings []a11yFinding

	seen := make(map[string]bool)

	for _, style := range inlineStyles(content) {
		foreground, ok := wcag.Parse(declarationValue(style, "color"))
		if !ok {
			continue
		}

		background := defaultBackground
		assumed := true
		if parsed, ok := wcag.Parse(declarationValue(style, "background-color")); ok {
			background = parsed
			assumed = false
		}

		ratio := wcag.ContrastRatio(foreground, background)
		if ratio >= minContrastRatio {
			continue
		}

		detail := fmt.Sprintf("contrast %.2f:1 (minimum %.1f:1) for %s on %s",
			ratio, minContrastRatio, foreground.Hex(), background.Hex())
		if assumed {
			detail += " (background assumed white)"
		}

		if seen[detail] {
			continue
		}
		seen[detail] = true

		findings = append(findings, a11yFinding{
			Location:  location,
			Criterion: "WCAG 2.2 SC 1.4.3 Contrast (Minimum)",
			Detail:    detail,
		})
	}

	return findings
}

// auditImageAlt reports images that carry no alt text.
func auditImageAlt(content, location string) []a11yFinding {
	var findings []a11yFinding

	for _, tag := range imgTagPattern.FindAllString(content, -1) {
		var src string
		hasAlt := false

		for _, attr := range parseAttributes(tag) {
			switch attr.name {
			case "src":
				src = attr.value
			case "alt":
				hasAlt = strings.TrimSpace(attr.value) != ""
			}
		}

		if hasAlt {
			continue
		}

		findings = append(findings, a11yFinding{
			Location:  location,
			Criterion: "WCAG 2.2 SC 1.1.1 Non-text Content",
			Detail:    fmt.Sprintf("image has no alt text: %s", src),
		})
	}

	return findings
}

// writeA11yReport writes the findings as markdown next to the export.
func (e *Exporter) writeA11yReport(findings []a11yFinding) error {
	var builder strings.Builder

	builder.WriteString("# Accessibility Report\n\n")
	builder.WriteString(fmt.Sprintf("Source: %s\n\n", e.config.URL))

	if len(findings) == 0 {
		builder.WriteString("No contrast or alt-text issues found.\n")
	} else {
		builder.WriteString(fmt.Sprintf("%d issue(s) found.\n\n", len(findings)))
		builder.WriteString("| Location | Criterion | Detail |\n|---|---|---|\n")

		for _, finding := range findings {
			builder.WriteString(fmt.Sprintf("| %s | %s | %s |\n",
				finding.Location, finding.Criterion, finding.Detail))
		}
	}

	reportPath := filepath.Join(e.config.Output, "a11y-report.md")

	return os.WriteFile(reportPath, []byte(builder.String()), 0600)
}

// inlineStyles returns the body of every inline style attribute in content.
func inlineStyles(content string) []string {
	matches := styleAttrPattern.FindAllStringSubmatch(content, -1)
	styles := make([]string, 0, len(matches))

	for _, match := range matches {
		style := match[1]
		if style == "" {
			style = match[2]
		}

		styles = append(styles, style)
	}

	return styles
}

// declarationValue returns the value of one CSS property from a style attribute.
// Declarations are split before matching so that "background-color" is never
// mistaken for "color".
func declarationValue(style, property string) string {
	for _, declaration := range strings.Split(style, ";") {
		name, value, found := strings.Cut(declaration, ":")
		if !found {
			continue
		}

		if strings.EqualFold(strings.TrimSpace(name), property) {
			return strings.TrimSpace(value)
		}
	}

	return ""
}
