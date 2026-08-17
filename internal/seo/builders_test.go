package seo

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// kingComposerFrontPage is the body #63 was reported with, shortened: the
// wrappers a plugin renders a layout from, and one headline inside them. The
// built site shows the words in a single column with no grid, no cards and no
// prices, because the layout only exists while the plugin is running.
func kingComposerFrontPage() string {
	var b strings.Builder

	b.WriteString(`<section class="kc-elm kc-css-2251604 kc_row">`)
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, `<div class="kc-row-container kc-container"><div class="kc-wrap-columns">`+
			`<div class="kc-elm kc-css-%d kc_column kc_col-sm-4"></div></div></div>`, i)
	}
	b.WriteString(`<h1>WITAJ W SKLEPIE INTERNETOWYM</h1></section>`)

	return b.String()
}

// TestBuilderShellIsCrawled: the failure. These pages are several kilobytes
// long, so the emptiness test passed them over and --crawl-content reached five
// pages of twenty — none of them the ones the export's own warning was about.
func TestBuilderShellIsCrawled(t *testing.T) {
	shell := kingComposerFrontPage()

	assert.False(t, isContentEmpty(shell), "it is not empty; that was the whole problem")
	assert.True(t, storedAsBuilderMarkup(shell))
	assert.True(t, needsRenderedContent(shell))
}

// TestOrdinaryPageIsNotCrawled: the cost of the fix, paid by every page that
// does not need it. Crawling a page whose stored body is the page replaces good
// content with scraped HTML, so prose in a wrapper must not look like a shell.
func TestOrdinaryPageIsNotCrawled(t *testing.T) {
	prose := strings.Repeat(
		`<div class="entry"><p>Zapraszamy do naszego sklepu z artykulami dla dzieci. `+
			`Oferujemy wozki, foteliki i akcesoria od sprawdzonych producentow.</p></div>`, 8)

	assert.False(t, storedAsBuilderMarkup(prose))
	assert.False(t, needsRenderedContent(prose))
}

// TestGutenbergPageIsNotAShell: the case that would break most exports if the
// rule were "has containers". Block markup is full of wrappers and full of
// text, and it stores the page rather than an instruction to build one.
func TestGutenbergPageIsNotAShell(t *testing.T) {
	blocks := strings.Repeat(
		`<div class="wp-block-group"><p class="wp-block-paragraph">`+
			`Nasza firma dziala na rynku od 1998 roku i obsluguje klientow w calej Polsce.</p></div>`, 10)

	assert.False(t, needsRenderedContent(blocks))
}

// TestAShortPageIsAPage: a handful of divs and a sentence is a page, not
// scaffolding. Without the floor, every short page on every site would be
// re-fetched.
func TestAShortPageIsAPage(t *testing.T) {
	short := `<div><p>Kontakt: biuro@example.com</p></div><div><p>Telefon: 22 000 00 00</p></div>`

	assert.False(t, storedAsBuilderMarkup(short))
}

// TestKnownBuilderNeedsNoRatio: what reopened #63. 1.8.15 treated the class as
// evidence toward a text-per-container threshold, and the reporter's front page
// cleared the threshold and was walked past again — forty kc-elm wrappers, and
// none of the three sections the live page shows. What the class means is that
// the stored body is an instruction to render, and an instruction is never the
// page, whatever text happens to sit between the wrappers.
func TestKnownBuilderNeedsNoRatio(t *testing.T) {
	wordy := `<div class="kc-elm kc_row"><div class="kc_column">` +
		strings.Repeat(`<p>Opis produktu, ktory zajmuje sporo miejsca w zapisanym ciele strony. `, 20) +
		`</div></div>`

	assert.True(t, storedAsBuilderMarkup(wordy), "the class decides; the ratio does not get a vote")
	assert.True(t, needsRenderedContent(wordy))
}

// TestBuilderNameOutsideAClassIsNotEvidence: the guard the decisive rule needs.
// A page mentioning a builder in a link or a comment must not be re-read from
// the network on the strength of the word.
func TestBuilderNameOutsideAClassIsNotEvidence(t *testing.T) {
	mention := `<div class="entry"><p>` +
		strings.Repeat(`Zbudowalismy te strone we wtyczce elementor-pro i jestesmy zadowoleni. `, 6) +
		`<a href="https://elementor.com/elementor-section/">wiecej</a></p></div>`

	assert.False(t, storedAsBuilderMarkup(mention))
}

// TestUnknownBuilderIsCaughtByShape: the list of class prefixes is evidence,
// not the rule. The next builder is on nobody's list, and a body that is all
// containers and no text renders to nothing whatever it is called.
func TestUnknownBuilderIsCaughtByShape(t *testing.T) {
	unknown := strings.Repeat(`<div class="zz-layout-cell"><div class="zz-inner"></div></div>`, 12)

	assert.False(t, builderClassRe.MatchString(unknown), "nothing here is recognized")
	assert.True(t, storedAsBuilderMarkup(unknown), "and it is still a shell")
}

// TestEmptyContentStillCounts: the case that always worked keeps working, and
// the two questions stay separate — --skip-empty-content must not start
// discarding builder pages, which are worth crawling and not worth dropping.
func TestEmptyContentStillCounts(t *testing.T) {
	assert.True(t, needsRenderedContent(""))
	assert.True(t, needsRenderedContent("<p>&nbsp;</p>"))

	shell := kingComposerFrontPage()
	assert.False(t, isContentEmpty(shell),
		"the filter's question is unchanged: a builder page is not discarded")
}

// TestVisibleTextIgnoresMarkupAndComments: what a reader sees is the measure.
// Gutenberg ships block comments in every body, and counting those as text
// would make a shell look like a page.
func TestVisibleTextIgnoresMarkupAndComments(t *testing.T) {
	assert.Equal(t, len("Hello world"),
		visibleTextLength(`<!-- wp:paragraph --><p class="x">Hello   world</p><!-- /wp:paragraph -->`))
	assert.Zero(t, visibleTextLength(`<div class="a"><span>&nbsp;</span></div>`))
}
