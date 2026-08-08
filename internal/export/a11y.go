package export

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// minContrastRatio is WCAG 2.2 SC 1.4.3 (Contrast Minimum) for normal-size text.
const minContrastRatio = 4.5

// defaultBackground is assumed when content sets a color but no background: a
// 2010-era WordPress theme almost always renders body copy on white, which is
// the worst case for the bright editor colors this check is aimed at.
var defaultBackground = rgb{255, 255, 255}

var (
	// styleAttrPattern matches an inline style attribute.
	styleAttrPattern = regexp.MustCompile(`(?is)style\s*=\s*"([^"]*)"|style\s*=\s*'([^']*)'`)
	// hexColorPattern matches #rgb and #rrggbb.
	hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	// rgbColorPattern matches rgb(r, g, b) and rgba(r, g, b, a).
	rgbColorPattern = regexp.MustCompile(`^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,[^)]*)?\)$`)
)

// namedColors covers the palette the classic WordPress editor offered, which is
// where these contrast problems come from.
var namedColors = map[string]rgb{
	"black": {0, 0, 0}, "white": {255, 255, 255}, "red": {255, 0, 0},
	"lime": {0, 255, 0}, "blue": {0, 0, 255}, "yellow": {255, 255, 0},
	"cyan": {0, 255, 255}, "aqua": {0, 255, 255}, "magenta": {255, 0, 255},
	"fuchsia": {255, 0, 255}, "green": {0, 128, 0}, "silver": {192, 192, 192},
	"gray": {128, 128, 128}, "grey": {128, 128, 128}, "maroon": {128, 0, 0},
	"olive": {128, 128, 0}, "navy": {0, 0, 128}, "purple": {128, 0, 128},
	"teal": {0, 128, 128},
}

// rgb is an 8-bit-per-channel color.
type rgb struct {
	r, g, b uint8
}

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
		foreground, ok := parseColor(declarationValue(style, "color"))
		if !ok {
			continue
		}

		background := defaultBackground
		assumed := true
		if parsed, ok := parseColor(declarationValue(style, "background-color")); ok {
			background = parsed
			assumed = false
		}

		ratio := contrastRatio(foreground, background)
		if ratio >= minContrastRatio {
			continue
		}

		detail := fmt.Sprintf("contrast %.2f:1 (minimum %.1f:1) for %s on %s",
			ratio, minContrastRatio, formatColor(foreground), formatColor(background))
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

// parseColor reads a CSS color in hex, rgb()/rgba() or named form.
func parseColor(value string) (rgb, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return rgb{}, false
	}

	if named, ok := namedColors[value]; ok {
		return named, true
	}

	if match := hexColorPattern.FindStringSubmatch(value); match != nil {
		return parseHexColor(match[1]), true
	}

	if match := rgbColorPattern.FindStringSubmatch(value); match != nil {
		return parseRGBColor(match[1:4])
	}

	return rgb{}, false
}

// parseHexColor expands a 3- or 6-digit hex body into a color.
//
// Each channel is parsed on its own rather than shifted out of a wider integer,
// so every value provably fits the 8 bits it is stored in.
func parseHexColor(digits string) rgb {
	if len(digits) == 3 {
		digits = string([]byte{
			digits[0], digits[0],
			digits[1], digits[1],
			digits[2], digits[2],
		})
	}

	return rgb{
		r: parseHexChannel(digits[0:2]),
		g: parseHexChannel(digits[2:4]),
		b: parseHexChannel(digits[4:6]),
	}
}

// parseHexChannel reads one two-digit hex channel. The caller's pattern has
// already established that the digits are valid hex.
func parseHexChannel(pair string) uint8 {
	value, err := strconv.ParseUint(pair, 16, 8)
	if err != nil {
		return 0
	}

	return uint8(value)
}

// parseRGBColor reads the three channels of an rgb()/rgba() color.
func parseRGBColor(channels []string) (rgb, bool) {
	values := make([]uint8, 0, 3)

	for _, channel := range channels {
		parsed, err := strconv.Atoi(channel)
		if err != nil || parsed < 0 || parsed > 255 {
			return rgb{}, false
		}

		values = append(values, uint8(parsed))
	}

	return rgb{r: values[0], g: values[1], b: values[2]}, true
}

// formatColor renders a color back to hex for the report.
func formatColor(c rgb) string {
	return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b)
}

// contrastRatio computes the WCAG contrast ratio between two colors.
func contrastRatio(a, b rgb) float64 {
	lighter, darker := relativeLuminance(a), relativeLuminance(b)
	if lighter < darker {
		lighter, darker = darker, lighter
	}

	return (lighter + 0.05) / (darker + 0.05)
}

// relativeLuminance implements the WCAG 2.2 relative luminance formula.
func relativeLuminance(c rgb) float64 {
	return 0.2126*channelLuminance(c.r) + 0.7152*channelLuminance(c.g) + 0.0722*channelLuminance(c.b)
}

// channelLuminance linearises one sRGB channel.
func channelLuminance(value uint8) float64 {
	scaled := float64(value) / 255.0
	if scaled <= 0.04045 {
		return scaled / 12.92
	}

	return math.Pow((scaled+0.055)/1.055, 2.4)
}
