package export

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/internal/config"
)

// themedHeading is the markup #67 was reported with: four classes, the last of
// them generated per element, and a stylesheet rule matching only that one.
const themedHeading = `<h2 class="sc_item_title sc_title_title trx_addons_inline_158836093">` +
	`<span class="sc_item_title_text">Delivering value</span></h2>`

// TestHeadingKeepsTheClassThatColorsIt: the failure. The <h2> and its classes
// go, the inner <span> survives, and the migrated page renders the site's main
// headline in the body color while a headline two sections down keeps the
// theme's by accident — two identical headings, two different results.
func TestHeadingKeepsTheClassThatColorsIt(t *testing.T) {
	// What reopened this: 1.8.15 made the remedy an opt-in the reporter had no
	// reason to guess at, and the headline lost its color again. A heading
	// wearing a class that means something now keeps itself.
	byDefault := htmlToMarkdownKeeping(themedHeading, preserveRules{styledHeadings: true})
	assert.Contains(t, byDefault, "trx_addons_inline_158836093")
	assert.NotContains(t, byDefault, "## ")

	kept := htmlToMarkdownKeeping(themedHeading, preserveRules{classes: []string{"trx_addons_inline_*"}})
	assert.Contains(t, kept, `class="sc_item_title sc_title_title trx_addons_inline_158836093"`,
		"the element travels as the HTML it arrived as")
	assert.Contains(t, kept, "Delivering value")
	assert.NotContains(t, kept, "## ", "and is not converted as well as kept")
}

// TestPreservedElementIsUntouchedByEveryRule: a kept element is out of the way
// of the whole pipeline, not just of the rule that would have converted its
// outer tag. A <strong> inside it must not become ** while the tags around it
// stay HTML.
func TestPreservedElementIsUntouchedByEveryRule(t *testing.T) {
	in := `<div class="keep-me"><ul><li><strong>bold</strong></li></ul><pre>code</pre></div>` +
		`<p>After</p>`

	out := htmlToMarkdownKeeping(in, preserveRules{classes: []string{"keep-me"}})

	assert.Contains(t, out, `<div class="keep-me"><ul><li><strong>bold</strong></li></ul><pre>code</pre></div>`)
	assert.NotContains(t, out, "**bold**")
	assert.NotContains(t, out, "```")
	assert.Contains(t, out, "After", "and the document around it converts as usual")
}

// TestPreservedElementKeepsItsNesting: an element holding another of the same
// name must be kept whole. Matched to the first closing tag instead, the outer
// half of a nested <div> would be left behind as a stray tag.
func TestPreservedElementKeepsItsNesting(t *testing.T) {
	in := `<div class="outer keep"><div class="inner"><p>Text</p></div></div>`

	out := htmlToMarkdownKeeping(in, preserveRules{classes: []string{"keep"}})

	assert.Equal(t, in, strings.TrimSpace(out))
}

// TestPreserveByID: the other half of the pair of flags, and the reason the
// rules are one type rather than two code paths.
func TestPreserveByID(t *testing.T) {
	in := `<section id="hero-2024"><h1>Title</h1></section><p>Body</p>`

	out := htmlToMarkdownKeeping(in, preserveRules{ids: []string{"hero-*"}})

	assert.Contains(t, out, `<section id="hero-2024"><h1>Title</h1></section>`)
	assert.Contains(t, out, "Body")
}

// TestPreserveNothingChangesNothing: the compatibility pin. Every export this
// tool has written was produced with no rules at all, and the empty rule set
// must be exactly that conversion — not a re-run of the pipeline through a
// preservation pass that happens to match nothing.
func TestPreserveNothingChangesNothing(t *testing.T) {
	in := `<h2 class="wp-block-heading">Title</h2><p>Body <strong>bold</strong></p>` +
		`<ul class="wp-block-list"><li>one</li></ul>`

	assert.Equal(t, htmlToMarkdown(in), htmlToMarkdownKeeping(in, preserveRules{}))
	assert.Equal(t, htmlToMarkdown(in), htmlToMarkdownKeeping(in, preserveRules{classes: []string{""}}))
	assert.Equal(t, htmlToMarkdown(in), htmlToMarkdownKeeping(in, preserveRules{classes: []string{"absent"}}))
}

// TestPreserveEverythingStyled: a theme generates a class per element, so the
// operator cannot list them; `*` is how they say "keep whatever carries one".
func TestPreserveEverythingStyled(t *testing.T) {
	in := `<h2 class="a">Kept</h2><h3>Converted</h3>`

	out := htmlToMarkdownKeeping(in, preserveRules{classes: []string{"*"}})

	assert.Contains(t, out, `<h2 class="a">Kept</h2>`)
	assert.Contains(t, out, "### Converted", "a heading with nothing to lose still converts")
}

// TestPreservedVoidElementHasNoEnd: an <img> never closes, and searching for
// its closing tag would keep the rest of the document as HTML.
func TestPreservedVoidElementHasNoEnd(t *testing.T) {
	in := `<img class="logo" src="/a.png"><p>Body</p><h2>Heading</h2>`

	out := htmlToMarkdownKeeping(in, preserveRules{classes: []string{"logo"}})

	assert.Contains(t, out, `<img class="logo" src="/a.png">`)
	assert.Contains(t, out, "## Heading", "the document after it still converts")
}

// TestPreservedElementNeverClosed: malformed markup keeps the tag alone rather
// than swallowing the page. Page builders emit unclosed elements, and the
// alternative here is an export whose body is one HTML block.
func TestPreservedElementNeverClosed(t *testing.T) {
	in := `<div class="keep"><p>Inside</p><h2>Heading</h2>`

	out := htmlToMarkdownKeeping(in, preserveRules{classes: []string{"keep"}})

	assert.Contains(t, out, `<div class="keep">`)
	assert.Contains(t, out, "## Heading")
}

// TestExporterReadsTheOperatorsRules: the wiring. The flags existed and applied
// to two other conversions; the markdown writer has to be reading the same
// ones, and a nil config must not panic a converter that has no rules anyway.
func TestExporterReadsTheOperatorsRules(t *testing.T) {
	exporter := NewExporter(&config.Config{
		PreserveClasses: []string{"trx_addons_inline_*"},
		PreserveIDs:     []string{"hero"},
	})

	rules := exporter.preserveRules()
	assert.Equal(t, []string{"trx_addons_inline_*"}, rules.classes)
	assert.Equal(t, []string{"hero"}, rules.ids)
	assert.Contains(t, exporter.convertHTMLToMarkdown(themedHeading), "trx_addons_inline_158836093")

	bare := &Exporter{}
	assert.True(t, bare.preserveRules().empty())
	assert.Contains(t, bare.convertHTMLToMarkdown("<h2>Title</h2>"), "## Title")
}

// TestBoilerplateHeadingsStillConvert: the line the default rule is drawn at.
// WordPress and its blocks stamp these on every heading on every site; they say
// nothing a `##` is missing, and keeping them as HTML would turn a clean export
// into a wall of tags for everyone.
func TestBoilerplateHeadingsStillConvert(t *testing.T) {
	styled := preserveRules{styledHeadings: true}

	for _, heading := range []string{
		`<h2>Plain</h2>`,
		`<h2 class="wp-block-heading">Gutenberg</h2>`,
		`<h2 class="wp-block-heading has-text-align-center has-large-font-size">Aligned</h2>`,
		`<h1 class="entry-title">Theme title</h1>`,
		`<h2 class="screen-reader-text">Hidden label</h2>`,
	} {
		out := htmlToMarkdownKeeping(heading, styled)
		assert.NotContains(t, out, "<h", "should convert: %s", heading)
	}
}

// TestHeadingsWorthKeeping: anything that is not boilerplate is styling this
// format cannot express, whoever emitted it.
func TestHeadingsWorthKeeping(t *testing.T) {
	styled := preserveRules{styledHeadings: true}

	for _, heading := range []string{
		`<h2 class="sc_item_title trx_addons_inline_158836093">Theme</h2>`,
		`<h2 class="text-center">Framework</h2>`,
		`<h3 class="wp-block-heading kc-elm">Builder</h3>`,
	} {
		out := htmlToMarkdownKeeping(heading, styled)
		assert.Contains(t, out, "<h", "should be kept: %s", heading)
	}
}

// TestNoPreserveStylingIsTheWayBack: an operator who wants the 1.8.14
// conversion says so, and gets exactly it.
func TestNoPreserveStylingIsTheWayBack(t *testing.T) {
	exporter := NewExporter(&config.Config{PreserveStyling: StylingNone})

	assert.True(t, exporter.preserveRules().empty())
	assert.Equal(t, htmlToMarkdown(themedHeading), exporter.convertHTMLToMarkdown(themedHeading))

	byDefault := NewExporter(&config.Config{})
	assert.True(t, byDefault.preserveRules().styledHeadings)
	assert.False(t, byDefault.preserveRules().styledAnything)
}

// TestStylingModesAnswerDifferentSites: sites differ too much for one rule. A
// site whose whole layout is styling wants everything kept — keeping only the
// headings would save the color of the headline and lose the section it sits
// in — and a site migrating to plain prose wants none of it.
func TestStylingModesAnswerDifferentSites(t *testing.T) {
	// A paragraph, because that is an element the conversion actually rewrites:
	// a <div> passes through as HTML in every mode, so it could not tell them
	// apart.
	body := `<h2 class="sc_item_title">Headline</h2><p class="sc_lead">Lead</p>` +
		`<h3 class="wp-block-heading">Plain</h3>`

	all := NewExporter(&config.Config{PreserveStyling: StylingAll}).preserveRules()
	kept := htmlToMarkdownKeeping(body, all)
	assert.Contains(t, kept, `<p class="sc_lead">Lead</p>`)
	assert.Contains(t, kept, `<h3 class="wp-block-heading">`, "all means all, boilerplate included")

	auto := NewExporter(&config.Config{}).preserveRules()
	headings := htmlToMarkdownKeeping(body, auto)
	assert.Contains(t, headings, `<h2 class="sc_item_title">`)
	assert.NotContains(t, headings, `<p class="sc_lead">`, "auto is about headings")
	assert.Contains(t, headings, "### Plain")

	none := NewExporter(&config.Config{PreserveStyling: StylingNone}).preserveRules()
	assert.Equal(t, htmlToMarkdown(body), htmlToMarkdownKeeping(body, none))
}

// TestSiteNamesItsOwnBoilerplate: one tool's idea of a meaningless class cannot
// fit every theme ever written. A theme stamping sc_title_title on every
// heading it has is noise on that site and nowhere else, and keeping those as
// HTML would be the wall of tags the default rule exists to avoid.
func TestSiteNamesItsOwnBoilerplate(t *testing.T) {
	heading := `<h2 class="sc_item_title sc_title_title">Headline</h2>`

	assert.Contains(t, htmlToMarkdownKeeping(heading,
		NewExporter(&config.Config{}).preserveRules()), "<h2")

	named := NewExporter(&config.Config{BoilerplateClasses: []string{"sc_*", ""}}).preserveRules()
	assert.Contains(t, htmlToMarkdownKeeping(heading, named), "## Headline")
}
