package seo

import "testing"

// elementorPage is what a page built with Elementor over a classic theme
// declares: the builder's own globals plus the theme's variables and the block
// editor's presets, all in the document's <style> blocks.
const elementorPage = `<html><head>
<style id="theme-inline-css">
:root{
  --theme-color-accent1: #7b2ff7;
  --theme-color-text: #222733;
  --theme-color-bg_color: #F6F6F6;
  --theme-color-text_link: #a4836d;
  --theme-font-family: Roboto, sans-serif;
}
</style>
<style id="wp-block-library-inline-css">
body{--wp--preset--color--primary: #0693e3;--wp--preset--color--secondary: #cf2e2e;}
</style>
<style id="elementor-frontend-inline-css">
.elementor-kit-7{--e-global-color-primary:#6EC1E4;--e-global-color-secondary:#54595F;--e-global-color-accent:#61CE70;--e-global-color-text:#7A7A7A;}
</style>
</head><body></body></html>`

func TestExtractPalette_ThemeVariablesBeatBuilderDefaults(t *testing.T) {
	palette := ExtractPalette(elementorPage)

	want := map[string]string{
		"primary":    "#7b2ff7", // the theme's own, not Elementor's factory blue
		"secondary":  "#cf2e2e", // no theme variable → the block editor preset
		"accent":     "#61CE70", // only Elementor declares one
		"text":       "#222733",
		"background": "#F6F6F6",
		"link":       "#a4836d",
	}
	for role, value := range want {
		if palette[role] != value {
			t.Errorf("%s = %q, want %q", role, palette[role], value)
		}
	}
	if len(palette) != len(want) {
		t.Errorf("unexpected roles in %v", palette)
	}
}

func TestExtractPalette_IgnoresNonColours(t *testing.T) {
	html := `<style>:root{
		--color-primary: var(--brand);
		--color-secondary: linear-gradient(#fff, #000);
		--color-text: 16px;
		--color-background: #FFF;
		--color-link: rgba(12, 34, 56, 0.5);
	}</style>`

	palette := ExtractPalette(html)

	for _, role := range []string{"primary", "secondary", "text"} {
		if v, ok := palette[role]; ok {
			t.Errorf("%s should have been dropped, got %q", role, v)
		}
	}
	if palette["background"] != "#FFF" {
		t.Errorf("short hex should survive, got %q", palette["background"])
	}
	if palette["link"] != "rgba(12, 34, 56, 0.5)" {
		t.Errorf("functional color should survive, got %q", palette["link"])
	}
}

func TestExtractPalette_OnlyReadsStyleBlocks(t *testing.T) {
	// A custom property in body text or a style attribute is not the theme's
	// palette and must not be picked up.
	html := `<p>--color-primary: #ff0000;</p><div style="--color-primary:#00ff00">x</div>`

	if palette := ExtractPalette(html); palette != nil {
		t.Errorf("nothing outside <style> is a palette, got %v", palette)
	}
}

func TestExtractPalette_FirstDeclarationWins(t *testing.T) {
	// A later override is usually a media query or a section variant; the theme
	// leads with the value it means.
	html := `<style>:root{--color-primary:#111111}
		@media (min-width:900px){:root{--color-primary:#222222}}</style>`

	if got := ExtractPalette(html)["primary"]; got != "#111111" {
		t.Errorf("primary = %q, want the first declaration", got)
	}
}

func TestExtractPalette_NothingDeclared(t *testing.T) {
	if palette := ExtractPalette("<html><head></head></html>"); palette != nil {
		t.Errorf("a page with no stylesheet should yield nothing, got %v", palette)
	}
	// Custom properties that are not colors at all: still nothing.
	if palette := ExtractPalette(`<style>:root{--gap: 12px;}</style>`); palette != nil {
		t.Errorf("non-color properties should yield nothing, got %v", palette)
	}
	// Color properties under names no role maps to: still nothing.
	if palette := ExtractPalette(`<style>:root{--sidebar-shadow: #010203;}</style>`); palette != nil {
		t.Errorf("unmapped names should yield nothing, got %v", palette)
	}
}

func TestNormalizeColor(t *testing.T) {
	cases := map[string]string{
		"#abc":                  "#abc",
		"#AABBCC":               "#AABBCC",
		"#aabbccdd":             "#aabbccdd",
		"  #123456  ":           "#123456",
		"#123456 !important":    "#123456",
		"rgb(1,2,3)":            "rgb(1,2,3)",
		"hsl(210 100% 50%)":     "hsl(210 100% 50%)",
		"WHITE":                 "white",
		"var(--other)":          "",
		"#12345":                "",
		"url(x.png)":            "",
		"":                      "",
		"1px solid #000000":     "",
		"rgb(1,2,3) !important": "rgb(1,2,3)",
	}
	for in, want := range cases {
		if got := normalizeColor(in); got != want {
			t.Errorf("normalizeColor(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestExtractSiteMarketing_CarriesPalette confirms the palette reaches the
// exported marketing block, which is what a migration reads.
func TestExtractSiteMarketing_CarriesPalette(t *testing.T) {
	marketing := extractSiteMarketing(elementorPage, "https://example.com/")

	if marketing.Colors["primary"] != "#7b2ff7" {
		t.Errorf("palette missing from marketing: %+v", marketing.Colors)
	}
	if marketing.IsEmpty() {
		t.Error("a page that declares a palette is not empty marketing")
	}
}
