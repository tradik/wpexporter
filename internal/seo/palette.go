package seo

// Theme palette extraction (#27).
//
// A migration carries the content and loses the look. The colours, though, are
// not hidden in a design file: every modern WordPress theme declares them as CSS
// custom properties in the page's own <style> blocks — Elementor as
// --e-global-color-*, block themes as --wp--preset--color--*, classic themes
// under their own prefix. Reading them is the difference between a migrated site
// that arrives in its own colours and one that arrives in the generator's
// defaults with a note to "rebuild the theme".
//
// Only declarations inside <style> are read, and only values that are literally
// colours: the palette travels to another system's stylesheet, so a var()
// reference or an expression would be meaningless there and is dropped.

import (
	"regexp"
	"strings"
)

var (
	// styleBlockPattern isolates the document's own stylesheets. Custom
	// properties elsewhere (a code sample, an inline style attribute) are not
	// the theme's palette.
	styleBlockPattern = regexp.MustCompile(`(?is)<style\b[^>]*>(.*?)</style\s*>`)
	// customPropertyPattern matches one `--name: value;` declaration.
	customPropertyPattern = regexp.MustCompile(`--([a-zA-Z0-9_-]+)\s*:\s*([^;{}]+)`)
	// hexColorPattern matches #rgb, #rgba, #rrggbb and #rrggbbaa.
	hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	// functionalColorPattern matches rgb()/rgba()/hsl()/hsla() forms.
	functionalColorPattern = regexp.MustCompile(`^(?i)(rgb|rgba|hsl|hsla)\([0-9.,%\s/deg-]+\)$`)
)

// paletteNamedColors is the handful of CSS colour keywords a theme variable is
// realistically set to. It only has to RECOGNISE a colour — the contrast
// checker in internal/export needs their channel values and keeps its own,
// larger table for that.
var paletteNamedColors = map[string]bool{
	"black": true, "white": true, "transparent": true, "currentcolor": true,
	"red": true, "blue": true, "green": true, "yellow": true, "orange": true,
	"purple": true, "gray": true, "grey": true, "silver": true, "navy": true,
	"teal": true, "olive": true, "maroon": true, "lime": true, "aqua": true,
	"cyan": true, "magenta": true, "fuchsia": true,
}

// paletteRoles maps each role the export records to the custom-property names
// that carry it, most theme-specific first. The first name that resolves to a
// real colour wins, so a theme's own variable beats a page builder's default —
// which is usually still the builder's factory blue nobody chose.
var paletteRoles = []struct {
	role  string
	names []string
}{
	{"primary", []string{
		"theme-color-accent1", "wp--preset--color--primary", "color-primary", "primary-color",
		"brand-primary", "e-global-color-primary",
	}},
	{"secondary", []string{
		"theme-color-accent2", "wp--preset--color--secondary", "color-secondary", "secondary-color",
		"brand-secondary", "e-global-color-secondary",
	}},
	{"accent", []string{
		"theme-color-accent3", "wp--preset--color--accent", "color-accent", "accent-color",
		"e-global-color-accent",
	}},
	{"text", []string{
		"theme-color-text", "wp--preset--color--foreground", "wp--preset--color--text-dark",
		"color-text", "text-color", "e-global-color-text",
	}},
	{"background", []string{
		"theme-color-bg_color", "wp--preset--color--background", "wp--preset--color--bg-color",
		"color-background", "background-color", "e-global-color-bg",
	}},
	{"link", []string{
		"theme-color-text_link", "wp--preset--color--text-link", "color-link", "link-color",
	}},
}

// ExtractPalette reads the theme's palette from a page's stylesheets, keyed by
// role. Returns nil when the page declares nothing usable — an export records
// what the site says about itself and never invents a colour scheme.
func ExtractPalette(html string) map[string]string {
	declared := customProperties(html)
	if len(declared) == 0 {
		return nil
	}

	palette := make(map[string]string, len(paletteRoles))
	for _, role := range paletteRoles {
		for _, name := range role.names {
			if value, ok := declared[strings.ToLower(name)]; ok {
				palette[role.role] = value
				break
			}
		}
	}
	if len(palette) == 0 {
		return nil
	}
	return palette
}

// customProperties collects every custom property declared in the document's
// <style> blocks whose value is a literal colour. The first declaration wins:
// a later override is usually a media query or a per-section variant, and the
// first one is what the theme leads with.
func customProperties(html string) map[string]string {
	declared := map[string]string{}

	for _, block := range styleBlockPattern.FindAllStringSubmatch(html, -1) {
		for _, decl := range customPropertyPattern.FindAllStringSubmatch(block[1], -1) {
			name := strings.ToLower(decl[1])
			value := normalizeColor(decl[2])
			if value == "" {
				continue
			}
			if _, seen := declared[name]; !seen {
				declared[name] = value
			}
		}
	}
	return declared
}

// normalizeColor returns the value when it is a literal CSS colour, or "" when
// it is anything else — a var() reference, a gradient, a length, a font stack.
func normalizeColor(raw string) string {
	value := strings.TrimSpace(raw)
	// A trailing "!important" is a stylesheet concern, not part of the colour.
	if i := strings.Index(strings.ToLower(value), "!important"); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}
	switch {
	case hexColorPattern.MatchString(value):
		return value
	case functionalColorPattern.MatchString(value):
		return value
	case paletteNamedColors[strings.ToLower(value)]:
		return strings.ToLower(value)
	}
	return ""
}
