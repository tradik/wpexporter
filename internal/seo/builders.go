package seo

// Pages that are not empty and hold nothing (#63).
//
// `--crawl-content` fired only on a body the API served as empty, and the
// pages it was recommended for are not empty: a King Composer front page is
// several kilobytes of `kc-elm` wrappers with a headline somewhere inside it.
// So the remedy the export's own warning names — "try --assisted-crawl
// --crawl-content to take the rendered page instead" — reached five pages of
// twenty and none of the ones the warning was about.
//
// The layout only exists while the plugin renders it. What the REST API stores
// is the instruction to render, and exported as-is it becomes forty nested divs
// in a single column: no grid, no cards, no prices. The difference between a
// client saying "that is my site" and "where is everything".
//
// The test is therefore what the body amounts to rather than how long it is: an
// ordinary page carries hundreds of characters of text per container element,
// and a builder shell carries a handful. A known builder's class prefix raises
// the threshold rather than being the whole rule, because the shape matters
// more than the brand — the next builder is not on anyone's list.
//
// This is deliberately not the same question as `--skip-empty-content` asks. A
// builder page is worth crawling and is not worth discarding, so the emptiness
// test that filter uses is left exactly as it was.

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/tradik/wpexporter/internal/rx"
)

// builderClassRe matches the class prefixes the page builders stamp on their
// wrappers: King Composer, WPBakery, Divi, Elementor, Beaver Builder, Bricks,
// Fusion, Oxygen, Themify. Each is the visible half of a plugin that renders at
// request time.
//
// The match is anchored inside a class attribute rather than taken anywhere in
// the body. Now that one of these names decides the question by itself, a page
// that merely mentions `elementor-` in a link or a comment must not be re-read
// from the network on the strength of it.
var builderClassRe = regexp.MustCompile(`(?i)class\s*=\s*["'][^"']*\b(kc-elm|kc_row|kc_column|` +
	`vc_row|vc_column|wpb_|et_pb_|elementor-element|elementor-section|elementor-widget|` +
	`fl-builder|fl-row|fl-module|brxe-|fusion-builder|fusion-fullwidth|oxy-|ct-section|` +
	`themify_builder)`)

// containerTagRe counts the elements a builder nests to make a layout.
var containerTagRe = regexp.MustCompile(`(?i)<(div|section|article|aside|header|footer)\b`)

// htmlTagRe and htmlCommentRe strip the markup so what a reader would see can
// be measured.
var (
	htmlTagRe     = regexp.MustCompile(`(?is)<[^>]*>`)
	htmlCommentRe = regexp.MustCompile(`(?is)<!--.*?-->`)
)

// shellTextPerContainer is the ratio that catches a builder nobody has heard
// of, in visible characters per container element — characters rather than
// bytes, so the question is the same one on every alphabet. A page of prose runs
// into the hundreds; the reported front page, forty wrappers around one
// headline, is under ten.
const shellTextPerContainer = 20

// minContainers keeps an ordinary short page out of the ratio test. A page with
// four divs and a sentence in it is a page, not a shell.
const minContainers = 6

// Modes of --crawl-content-mode. Which pages are worth re-reading is a property
// of the site: a shop built entirely in a builder wants every page taken from
// the rendered HTML, a site with two odd pages wants only those, and an
// operator who already knows which would rather say so than guess which rule
// fired (#63).
const (
	// CrawlAuto re-reads the pages whose stored body is empty or is a page
	// builder's scaffolding.
	CrawlAuto = "auto"
	// CrawlEmpty re-reads only the pages the API served empty, as 1.8.14 did.
	CrawlEmpty = "empty"
	// CrawlAlways re-reads every page.
	CrawlAlways = "always"
)

// CrawlModes are the accepted answers, so a typo is refused rather than
// silently read as the narrowest one.
var CrawlModes = []string{CrawlAuto, CrawlEmpty, CrawlAlways}

// needsRenderedContent reports whether the crawler should fetch the page the
// visitor sees instead of trusting what the API stored.
func (c *Crawler) needsRenderedContent(content string) bool {
	switch c.config.CrawlContentMode {
	case CrawlAlways:
		return true
	case CrawlEmpty:
		return isContentEmpty(content)
	default:
		return isContentEmpty(content) || c.storedAsBuilderMarkup(content)
	}
}

// storedAsBuilderMarkup reports a body that renders to nothing without the
// plugin that builds it.
//
// A recognized builder's class is the whole answer where it appears. 1.8.15
// treated it as evidence toward a ratio instead, and the reporter's front page
// — forty `kc-elm` wrappers, none of its three sections in the export — cleared
// the ratio and was walked past again (#63). What that class means is that the
// stored body is an instruction to render, and an instruction is never the
// page: whatever text sits between the wrappers, the grid, the cards and the
// prices are produced at request time and are not in there.
//
// Everything else is caught by shape, because the next builder is on nobody's
// list: many containers and almost no text renders to nothing whatever it is
// called.
func (c *Crawler) storedAsBuilderMarkup(content string) bool {
	if builderClassRe.MatchString(content) || c.siteBuilderClass(content) {
		return true
	}

	containers := len(containerTagRe.FindAllString(content, -1))
	if containers < minContainers {
		return false
	}

	return visibleTextLength(content)/containers < shellTextPerContainer
}

// visibleTextLength is how much of the body a reader would actually see, with
// markup, comments and entity padding removed.
func visibleTextLength(content string) int {
	text := htmlCommentRe.ReplaceAllString(content, "")
	text = htmlTagRe.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	// Characters, not bytes: 20 bytes is 20 letters of English and under 7 of
	// Japanese, so a byte budget silently asks a different question of every
	// alphabet — and never of the English one.
	return utf8.RuneCountInString(strings.Join(strings.Fields(text), " "))
}

// siteBuilderClass matches the prefixes this operator named for their own site.
//
// The built-in list is the builders anyone would recognize, and the next
// builder is on nobody's list — a theme with its own layout shortcodes emits
// markup no less unreadable for being unknown. --builder-classes is how a site
// says which its is, and it is matched inside a class attribute like the rest.
func (c *Crawler) siteBuilderClass(content string) bool {
	if c.config == nil {
		return false
	}

	for _, prefix := range c.config.BuilderClasses {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}

		if rx.Get(`(?i)class\s*=\s*["'][^"']*\b` + wildcardClassPattern(prefix)).MatchString(content) {
			return true
		}
	}

	return false
}

// wildcardClassPattern turns a shell-style name into a regular expression, and
// treats a bare prefix as one: `kc-elm` names the family `kc-elm…`, which is
// what an operator reading their own markup means by it.
func wildcardClassPattern(prefix string) string {
	parts := strings.Split(prefix, "*")
	for i, part := range parts {
		parts[i] = regexp.QuoteMeta(part)
	}

	return strings.Join(parts, `[^"'\s]*`)
}
