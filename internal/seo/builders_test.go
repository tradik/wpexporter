package seo

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/internal/config"
)

// autoCrawler asks the default question: empty, or a builder's scaffolding.
func autoCrawler(t *testing.T) *Crawler {
	t.Helper()

	return NewCrawler(&config.Config{CrawlContentMode: CrawlAuto})
}

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
	assert.True(t, autoCrawler(t).storedAsBuilderMarkup(shell))
	assert.True(t, autoCrawler(t).needsRenderedContent(shell))
}

// TestOrdinaryPageIsNotCrawled: the cost of the fix, paid by every page that
// does not need it. Crawling a page whose stored body is the page replaces good
// content with scraped HTML, so prose in a wrapper must not look like a shell.
func TestOrdinaryPageIsNotCrawled(t *testing.T) {
	prose := strings.Repeat(
		`<div class="entry"><p>Zapraszamy do naszego sklepu z artykulami dla dzieci. `+
			`Oferujemy wozki, foteliki i akcesoria od sprawdzonych producentow.</p></div>`, 8)

	assert.False(t, autoCrawler(t).storedAsBuilderMarkup(prose))
	assert.False(t, autoCrawler(t).needsRenderedContent(prose))
}

// TestGutenbergPageIsNotAShell: the case that would break most exports if the
// rule were "has containers". Block markup is full of wrappers and full of
// text, and it stores the page rather than an instruction to build one.
func TestGutenbergPageIsNotAShell(t *testing.T) {
	blocks := strings.Repeat(
		`<div class="wp-block-group"><p class="wp-block-paragraph">`+
			`Nasza firma dziala na rynku od 1998 roku i obsluguje klientow w calej Polsce.</p></div>`, 10)

	assert.False(t, autoCrawler(t).needsRenderedContent(blocks))
}

// TestAShortPageIsAPage: a handful of divs and a sentence is a page, not
// scaffolding. Without the floor, every short page on every site would be
// re-fetched.
func TestAShortPageIsAPage(t *testing.T) {
	short := `<div><p>Kontakt: biuro@example.com</p></div><div><p>Telefon: 22 000 00 00</p></div>`

	assert.False(t, autoCrawler(t).storedAsBuilderMarkup(short))
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

	assert.True(t, autoCrawler(t).storedAsBuilderMarkup(wordy), "the class decides; the ratio does not get a vote")
	assert.True(t, autoCrawler(t).needsRenderedContent(wordy))
}

// TestBuilderNameOutsideAClassIsNotEvidence: the guard the decisive rule needs.
// A page mentioning a builder in a link or a comment must not be re-read from
// the network on the strength of the word.
func TestBuilderNameOutsideAClassIsNotEvidence(t *testing.T) {
	mention := `<div class="entry"><p>` +
		strings.Repeat(`Zbudowalismy te strone we wtyczce elementor-pro i jestesmy zadowoleni. `, 6) +
		`<a href="https://elementor.com/elementor-section/">wiecej</a></p></div>`

	assert.False(t, autoCrawler(t).storedAsBuilderMarkup(mention))
}

// TestUnknownBuilderIsCaughtByShape: the list of class prefixes is evidence,
// not the rule. The next builder is on nobody's list, and a body that is all
// containers and no text renders to nothing whatever it is called.
func TestUnknownBuilderIsCaughtByShape(t *testing.T) {
	unknown := strings.Repeat(`<div class="zz-layout-cell"><div class="zz-inner"></div></div>`, 12)

	assert.False(t, builderClassRe.MatchString(unknown), "nothing here is recognized")
	assert.True(t, autoCrawler(t).storedAsBuilderMarkup(unknown), "and it is still a shell")
}

// TestEmptyContentStillCounts: the case that always worked keeps working, and
// the two questions stay separate — --skip-empty-content must not start
// discarding builder pages, which are worth crawling and not worth dropping.
func TestEmptyContentStillCounts(t *testing.T) {
	assert.True(t, autoCrawler(t).needsRenderedContent(""))
	assert.True(t, autoCrawler(t).needsRenderedContent("<p>&nbsp;</p>"))

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

// TestCrawlModesAnswerDifferentSites: one rule cannot fit a shop built entirely
// in a page builder and a site with two odd pages. `empty` is the 1.8.14
// question, `always` is the operator who already knows how their site is built
// and would rather not guess which heuristic fired (#63).
func TestCrawlModesAnswerDifferentSites(t *testing.T) {
	shell := kingComposerFrontPage()
	prose := `<div class="entry"><p>` + strings.Repeat("Zwykla tresc strony. ", 30) + `</p></div>`

	onlyEmpty := NewCrawler(&config.Config{CrawlContentMode: CrawlEmpty})
	assert.False(t, onlyEmpty.needsRenderedContent(shell), "the 1.8.14 question, still askable")
	assert.True(t, onlyEmpty.needsRenderedContent(""))

	always := NewCrawler(&config.Config{CrawlContentMode: CrawlAlways})
	assert.True(t, always.needsRenderedContent(prose))
	assert.True(t, always.needsRenderedContent(shell))

	auto := autoCrawler(t)
	assert.True(t, auto.needsRenderedContent(shell))
	assert.False(t, auto.needsRenderedContent(prose))
}

// TestSiteNamesItsOwnBuilder: the next builder is on nobody's list. A theme
// with its own layout shortcodes emits markup no less unreadable for being
// unknown, and the operator reading it can say what it is called.
func TestSiteNamesItsOwnBuilder(t *testing.T) {
	markup := `<div class="zzbuild-row"><div class="zzbuild-col">` +
		strings.Repeat(`<span>Tekst sekcji ktory zajmuje troche miejsca. </span>`, 20) +
		`</div></div>`

	assert.False(t, autoCrawler(t).storedAsBuilderMarkup(markup), "nothing recognizes it yet")

	named := NewCrawler(&config.Config{CrawlContentMode: CrawlAuto, BuilderClasses: []string{"zzbuild-*"}})
	assert.True(t, named.storedAsBuilderMarkup(markup))

	// A bare prefix names the family, which is what an operator reading their
	// own markup means by it.
	prefix := NewCrawler(&config.Config{CrawlContentMode: CrawlAuto, BuilderClasses: []string{"zzbuild-", "  "}})
	assert.True(t, prefix.storedAsBuilderMarkup(markup))
}

// TestSiteNamesItsOwnContentArea: a theme can call its content area anything,
// and the built-in selector list is the set of names that happen to be common
// rather than the set that exists. Without a way to name it, such a page falls
// through to the whole body — chrome included.
func TestSiteNamesItsOwnContentArea(t *testing.T) {
	for _, spelling := range []string{"section.kc-main", ".kc-main", "#kc-main", "section"} {
		selectors := NewCrawler(&config.Config{ContentSelectors: []string{spelling}}).siteContentSelectors()
		assert.Len(t, selectors, 1, "spelling %q", spelling)
	}

	assert.Empty(t, NewCrawler(&config.Config{ContentSelectors: []string{"", "   "}}).siteContentSelectors())
	assert.Empty(t, NewCrawler(&config.Config{}).siteContentSelectors())
}

// TestNamedContentAreaWins: what the operator named is tried before the guesses,
// because they are looking at the markup and this tool is not.
func TestNamedContentAreaWins(t *testing.T) {
	html := `<html><body><div id="content">Theme chrome that a guess would take instead.</div>` +
		`<section class="kc-main"><p>The words the visitor actually reads on this page.</p></section>` +
		`</body></html>`

	named := NewCrawler(&config.Config{ContentSelectors: []string{"section.kc-main"}})
	assert.Contains(t, named.extractMainContent(html), "the visitor actually reads")

	guessing := NewCrawler(&config.Config{})
	assert.Contains(t, guessing.extractMainContent(html), "Theme chrome")
}
