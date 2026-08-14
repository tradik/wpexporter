package seo

// The palette of a theme that declares no custom properties (#34).
//
// Reading --custom-properties covers block themes and GeneratePress 3.x, and
// misses everything older — which is most of the sites anyone migrates.
// bociany.pl (GeneratePress classic) exported no colors at all, so the migrated
// site arrived in the target theme's defaults. It does have a palette: the
// theme writes it as ordinary rules in the head — body, a, .main-navigation,
// the button classes — and <meta name="theme-color"> already stated the brand
// red.
//
// The only custom properties on such a page are WordPress core's own
// --wp--preset--color--*: Gutenberg's defaults, identical on every site.
// Reading those would be worse than reading nothing, which is why the roles
// below name selectors a theme actually styles and never a core preset.

import (
	"regexp"
	"strings"

	"github.com/tradik/wpexporter/internal/wcag"
)

var (
	// cssRulePattern matches an innermost `selector { declarations }` block.
	// Nesting inside @media leaves the at-rule text on the selector, which
	// ruleSelectors trims off.
	cssRulePattern = regexp.MustCompile(`(?s)([^{}]+)\{([^{}]*)\}`)
)

// ruleRoles maps a role to the selectors a theme styles it with, and the
// property that carries the color. The selectors are matched exactly: a rule
// for `.entry-content a` is about one region of one template, while `a` is the
// theme's link color.
var ruleRoles = []struct {
	role      string
	property  string
	selectors []string
}{
	{"background", "background-color", []string{"body", "html"}},
	{"text", "color", []string{"body"}},
	{"link", "color", []string{"a", "a:link"}},
	{"primary", "background-color", []string{
		".main-navigation", ".site-header", "#masthead", "header", ".navigation",
	}},
	{"accent", "background-color", []string{
		".button", "button", ".wp-block-button__link", "input[type=submit]", "input[type=\"submit\"]",
	}},
}

// minPairContrast is what a background and a text color have to clear to be
// believable as the pair a page renders body copy with.
//
// It is deliberately below the WCAG minimum of 4.5: a theme with genuinely poor
// contrast still has a palette, and the accessibility report is where that is
// called out. Below 3:1 the two rules are not a pair at all — a dark `body`
// color paired with a dark `body` background is two different contexts read as
// one — and a guess is worse than nothing.
const minPairContrast = 3.0

// rulePalette derives the palette from ordinary CSS rules, for a theme that
// declares no custom properties.
func rulePalette(html, brandColor string) map[string]string {
	rules := styleRules(html)
	if len(rules) == 0 && brandColor == "" {
		return nil
	}

	palette := make(map[string]string, len(ruleRoles))
	for _, role := range ruleRoles {
		if value := firstRuleColor(rules, role.selectors, role.property); value != "" {
			palette[role.role] = value
		}
	}

	// theme-color is the brand color by definition, and a classic theme that
	// styles its header with an image rather than a color still declares it.
	if palette["primary"] == "" {
		if _, ok := wcag.Parse(brandColor); ok {
			palette["primary"] = strings.ToLower(strings.TrimSpace(brandColor))
		}
	}

	if !pairIsBelievable(palette) {
		delete(palette, "text")
		delete(palette, "background")
	}

	if len(palette) == 0 {
		return nil
	}

	return palette
}

// pairIsBelievable checks the one pair the rules can disagree about. A palette
// carrying only one of the two is not a pair and is left alone.
func pairIsBelievable(palette map[string]string) bool {
	text, hasText := wcag.Parse(palette["text"])
	background, hasBackground := wcag.Parse(palette["background"])

	if !hasText || !hasBackground {
		return true
	}

	return wcag.ContrastRatio(text, background) >= minPairContrast
}

// cssRule is one parsed rule: the selectors it applies to and its declarations.
type cssRule struct {
	selectors []string
	body      string
}

// styleRules parses the document's own stylesheets into rules. Only <style>
// blocks are read, as with custom properties: an external stylesheet is not
// fetched, and an inline style attribute is one element's business.
func styleRules(html string) []cssRule {
	var rules []cssRule

	for _, block := range styleBlockPattern.FindAllStringSubmatch(html, -1) {
		for _, match := range cssRulePattern.FindAllStringSubmatch(block[1], -1) {
			selectors := ruleSelectors(match[1])
			if len(selectors) == 0 {
				continue
			}

			rules = append(rules, cssRule{selectors: selectors, body: match[2]})
		}
	}

	return rules
}

// ruleSelectors splits a selector list, dropping the at-rule preamble that
// nesting leaves in front of it.
func ruleSelectors(raw string) []string {
	if cut := strings.LastIndexAny(raw, "{}"); cut >= 0 {
		raw = raw[cut+1:]
	}

	var selectors []string
	for _, selector := range strings.Split(raw, ",") {
		selector = strings.ToLower(strings.Join(strings.Fields(selector), " "))
		if selector == "" || strings.HasPrefix(selector, "@") {
			continue
		}

		selectors = append(selectors, selector)
	}

	return selectors
}

// firstRuleColor returns the color the first matching rule declares for the
// property. First wins, as with custom properties: a later rule is usually a
// media query or a per-section variant.
func firstRuleColor(rules []cssRule, selectors []string, property string) string {
	wanted := make(map[string]bool, len(selectors))
	for _, selector := range selectors {
		wanted[selector] = true
	}

	for _, rule := range rules {
		if !matchesAny(rule.selectors, wanted) {
			continue
		}

		if value := ruleDeclaration(rule.body, property); value != "" {
			return value
		}

		// `background: #fff url(…)` carries the color a shorthand states; the
		// longhand above is preferred, and this only runs when it is absent.
		if property == "background-color" {
			if value := ruleDeclaration(rule.body, "background"); value != "" {
				return value
			}
		}
	}

	return ""
}

// matchesAny reports whether a rule targets one of the wanted selectors.
func matchesAny(ruleSelectors []string, wanted map[string]bool) bool {
	for _, selector := range ruleSelectors {
		if wanted[selector] {
			return true
		}
	}

	return false
}

// ruleDeclaration reads one property from a declaration block, exactly: a
// request for "color" never matches "background-color".
func ruleDeclaration(body, property string) string {
	for _, declaration := range strings.Split(body, ";") {
		name, value, found := strings.Cut(declaration, ":")
		if !found || !strings.EqualFold(strings.TrimSpace(name), property) {
			continue
		}

		// A shorthand may carry more than the color (`#fff url(…) no-repeat`);
		// the first token that is a color is the color.
		for _, token := range strings.Fields(value) {
			if color := normalizeColor(token); color != "" {
				return color
			}
		}
	}

	return ""
}
