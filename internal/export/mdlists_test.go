package export

// Lists keep their kind (#39). A recipe's method is an <ol> and its ingredients
// are a <ul>; exported as bullets they became the same thing, and "step 3" —
// which the text itself refers to — stopped existing.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recipeBody is the shape from the issue: ingredients unordered, method
// ordered, both plain WordPress markup.
const recipeBody = `<ul><li>250g unsalted butter at room temperature</li><li>200g sugar</li></ul>` +
	`<ol><li>Preheat the oven to 180°C.</li><li>Put the butter in a bowl and beat it.</li></ol>`

// TestOrderedListKeepsItsNumbers: the fix. The two lists are now distinguishable
// in the export, which is what every consumer downstream depends on.
func TestOrderedListKeepsItsNumbers(t *testing.T) {
	out := htmlToMarkdown(recipeBody)

	assert.Contains(t, out, "- 250g unsalted butter at room temperature")
	assert.Contains(t, out, "- 200g sugar")
	assert.Contains(t, out, "1. Preheat the oven to 180°C.")
	assert.Contains(t, out, "2. Put the butter in a bowl and beat it.")

	assert.NotContains(t, out, "- Preheat", "the method is ordered, not a set of suggestions")
}

// TestNestedListKeepsEachLevelsKind: a bulleted aside inside a numbered step is
// two lists, not one. The indentation is what keeps the inner one attached.
func TestNestedListKeepsEachLevelsKind(t *testing.T) {
	out := htmlToMarkdown(`<ol><li>Beat the butter.<ul><li>by hand</li><li>with a mixer</li></ul></li>` +
		`<li>Fold in the flour.</li></ol>`)

	assert.Contains(t, out, "1. Beat the butter.")
	assert.Contains(t, out, "\n   - by hand")
	assert.Contains(t, out, "\n   - with a mixer")
	assert.Contains(t, out, "2. Fold in the flour.")
}

// TestListStartIsCarriedOver: a list that starts at 5 says so because the four
// before it are elsewhere on the page. Renumbering it from 1 is a different
// document.
func TestListStartIsCarriedOver(t *testing.T) {
	out := htmlToMarkdown(`<ol start="5"><li>Fifth step</li><li>Sixth step</li></ol>`)

	assert.Contains(t, out, "5. Fifth step")
	assert.Contains(t, out, "6. Sixth step")
}

// TestUnnumberableListKeepsItsHTML: Markdown has no syntax for a lettered,
// roman or reversed list. Numbering it 1, 2, 3 would state something the page
// does not, so the markup travels instead — valid inside Markdown, and honest.
func TestUnnumberableListKeepsItsHTML(t *testing.T) {
	for _, attrs := range []string{` type="a"`, ` type="i"`, ` reversed`} {
		out := htmlToMarkdown(`<ol` + attrs + `><li>alpha</li><li>beta</li></ol>`)

		assert.Contains(t, out, "<ol"+attrs+">", "attrs %q", attrs)
		assert.Contains(t, out, "<li>alpha</li>")
		assert.NotContains(t, out, "1. alpha")
	}

	// type="1" is what Markdown already writes, so it converts.
	assert.Contains(t, htmlToMarkdown(`<ol type="1"><li>one</li></ol>`), "1. one")
}

// TestListItemKeepsItsMarkup: an item is not plain text — it carries links and
// emphasis, and an exporter that reads only the text between the tags drops
// them.
func TestListItemKeepsItsMarkup(t *testing.T) {
	out := htmlToMarkdown(`<ol><li>Preheat to <strong>180°C</strong>, see ` +
		`<a href="/tin/">the tin</a>.</li></ol>`)

	assert.Contains(t, out, "1. Preheat to **180°C**, see <a href=\"/tin/\">the tin</a>.")
}

// TestListsSurviveMalformedMarkup: WordPress content is not always well formed.
// A missing </li> costs nothing, and an unclosed list is left as it was rather
// than half-rewritten.
func TestListsSurviveMalformedMarkup(t *testing.T) {
	missingItemEnd := htmlToMarkdown(`<ol><li>one<li>two</ol>`)
	assert.Contains(t, missingItemEnd, "1. one")
	assert.Contains(t, missingItemEnd, "2. two")

	unclosed := htmlToMarkdown(`<ol><li>dangling`)
	assert.Contains(t, unclosed, "dangling", "content is never lost to a broken tag")

	assert.Equal(t, "", strings.TrimSpace(htmlToMarkdown(`<ul></ul>`)),
		"a list with no items is nothing to write")
}

// TestGutenbergListBlocksConvert: the block editor's lists carry classes, which
// is what issue #21 was about; they must still convert as lists.
func TestGutenbergListBlocksConvert(t *testing.T) {
	out := htmlToMarkdown(`<ul class="wp-block-list"><li>chocolate</li></ul>` +
		`<ol class="wp-block-list"><li>Melt</li></ol>`)

	assert.Contains(t, out, "- chocolate")
	assert.Contains(t, out, "1. Melt")
	assert.NotContains(t, out, "wp-block")
}

// TestDedentKeepsListStructure: the dedent pass exists to straighten a page
// builder's pretty-printing (#26). It must not straighten the indentation that
// holds a nested list to its parent item.
func TestDedentKeepsListStructure(t *testing.T) {
	dedented := dedentOutsideCodeFences("1. step\n   - nested\n\t- tabbed\n        deep continuation")

	require.Contains(t, dedented, "\n   - nested")
	assert.Contains(t, dedented, "\n    - tabbed", "a tab is four columns to CommonMark")
	assert.Contains(t, dedented, "\ndeep continuation", "an indented non-list line is still straightened")
}

// TestListStartIgnoresRubbish: an unreadable start attribute is not a reason to
// drop the list; it numbers from one, as an <ol> without the attribute does.
func TestListStartIgnoresRubbish(t *testing.T) {
	assert.Equal(t, 1, listStart(``))
	assert.Equal(t, 1, listStart(` start="not-a-number"`))
	assert.Equal(t, 5, listStart(` start=5`))
	assert.Equal(t, 1, listStart(` start="99999999999999999999999"`))
}
