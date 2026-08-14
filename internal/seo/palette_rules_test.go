package seo

// A classic theme's palette (#34). bociany.pl runs GeneratePress classic: it
// declares no theme custom properties, so the export carried no colors at all
// and the migrated site arrived in the target theme's defaults.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generatePressPage is the shape the issue describes: core's Gutenberg presets
// — identical on every WordPress site — and the theme's real colors in ordinary
// rules.
const generatePressPage = `<html><head>
<meta name="theme-color" content="#dd3333">
<style>
:root{--wp--preset--color--black:#000000;--wp--preset--color--white:#ffffff;
--wp--preset--color--pale-pink:#f78da7;--wp--preset--color--vivid-red:#cf2e2e;}
body{background-color:#ffffff;color:#3a3a3a;font-family:sans-serif}
a{color:#1e73be}
.main-navigation{background-color:#dd3333}
.button, button, input[type="submit"]{background-color:#666666;color:#ffffff}
</style></head><body></body></html>`

// TestRulePaletteReadsAClassicTheme: the roles come from the rules the theme
// actually writes, not from core's presets — which are Gutenberg's defaults and
// say nothing about this site.
func TestRulePaletteReadsAClassicTheme(t *testing.T) {
	palette := ExtractPalette(generatePressPage, "#dd3333")
	require.NotNil(t, palette)

	assert.Equal(t, "#ffffff", palette["background"])
	assert.Equal(t, "#3a3a3a", palette["text"])
	assert.Equal(t, "#1e73be", palette["link"])
	assert.Equal(t, "#dd3333", palette["primary"], "the header color the theme paints its navigation with")
	assert.Equal(t, "#666666", palette["accent"])

	assert.NotContains(t, palette, "secondary", "a role no rule states is absent, not guessed")
	for _, core := range []string{"#f78da7", "#cf2e2e"} {
		assert.NotContains(t, palette, core, "core's presets are not this site's palette")
	}
}

// TestCustomPropertiesStillWin: a theme that declares its palette has said what
// it is, and the rules are not consulted.
func TestCustomPropertiesStillWin(t *testing.T) {
	html := `<html><head><style>
:root{--color-primary:#0a0a0a;--color-text:#111111;}
body{color:#999999}
</style></head></html>`

	palette := ExtractPalette(html, "")
	assert.Equal(t, "#0a0a0a", palette["primary"])
	assert.Equal(t, "#111111", palette["text"])
}

// TestBrandColorStandsInForPrimary: a theme whose header is an image declares
// no navigation color, but <meta name="theme-color"> is the brand color by
// definition.
func TestBrandColorStandsInForPrimary(t *testing.T) {
	html := `<html><head><style>body{background-color:#ffffff;color:#222222}</style></head></html>`

	palette := ExtractPalette(html, "#dd3333")
	assert.Equal(t, "#dd3333", palette["primary"])

	none := ExtractPalette(html, "not-a-color")
	assert.NotContains(t, none, "primary", "a brand color that is not a color is not one")
}

// TestDisagreeingRulesEmitNothing: dark text on a dark body is two contexts read
// as one pair. A guess about the site's colors is worse than saying nothing.
func TestDisagreeingRulesEmitNothing(t *testing.T) {
	html := `<html><head><style>body{background-color:#101010;color:#151515}a{color:#1e73be}</style></head></html>`

	palette := ExtractPalette(html, "")
	assert.NotContains(t, palette, "text")
	assert.NotContains(t, palette, "background")
	assert.Equal(t, "#1e73be", palette["link"], "the roles that do not depend on the pair survive")
}

// TestRulePaletteIgnoresForeignRules: a rule for one region of one template is
// not the theme's link color, and a page with nothing to say yields nothing.
func TestRulePaletteIgnoresForeignRules(t *testing.T) {
	html := `<html><head><style>.entry-content a{color:#ff0000}.widget{color:#00ff00}</style></head></html>`
	assert.Nil(t, ExtractPalette(html, ""))

	assert.Nil(t, ExtractPalette(`<html><head><style>body{font-size:16px}</style></head></html>`, ""))
	assert.Nil(t, ExtractPalette(`<html><head></head><body></body></html>`, ""))
}

// TestRuleDeclarationReadsShorthandAndExactNames: `background: #fff url(…)`
// carries a color, and a request for "color" never answers with
// "background-color".
func TestRuleDeclarationReadsShorthandAndExactNames(t *testing.T) {
	html := `<html><head><style>body{background:#fafafa url(bg.png) no-repeat;color:#202020}</style></head></html>`

	palette := ExtractPalette(html, "")
	assert.Equal(t, "#fafafa", palette["background"])
	assert.Equal(t, "#202020", palette["text"])

	assert.Equal(t, "", ruleDeclaration("background-color:#000", "color"))
	assert.Equal(t, "", ruleDeclaration("color", "color"), "a declaration with no value is not one")
}

// TestStyleRulesTrimAtRulePreamble: a rule nested in @media is still the rule
// it looks like, and the at-rule text in front of it is not a selector.
func TestStyleRulesTrimAtRulePreamble(t *testing.T) {
	html := `<html><head><style>@media (min-width:900px){body{color:#123456;background-color:#ffffff}}</style></head></html>`

	palette := ExtractPalette(html, "")
	assert.Equal(t, "#123456", palette["text"])

	// An at-rule of its own is not a selector, and neither is the empty string a
	// trailing comma leaves behind.
	withAtRules := `<html><head><style>@font-face{font-family:x;color:#ff0000}
body, {color:#202020;background-color:#ffffff}</style></head></html>`

	fromRules := ExtractPalette(withAtRules, "")
	assert.Equal(t, "#202020", fromRules["text"])
	assert.Equal(t, "#ffffff", fromRules["background"])
}
