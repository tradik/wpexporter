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
// of, in characters of visible text per container element. A page of prose runs
// into the hundreds; the reported front page, forty wrappers around one
// headline, is under ten.
const shellTextPerContainer = 20

// minContainers keeps an ordinary short page out of the ratio test. A page with
// four divs and a sentence in it is a page, not a shell.
const minContainers = 6

// needsRenderedContent reports whether the crawler should fetch the page the
// visitor sees instead of trusting what the API stored.
//
// Two cases: a body the API served empty, which is what this always caught, and
// a body that is a page builder's scaffolding, which is what it did not (#63).
func needsRenderedContent(content string) bool {
	return isContentEmpty(content) || storedAsBuilderMarkup(content)
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
func storedAsBuilderMarkup(content string) bool {
	if builderClassRe.MatchString(content) {
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

	return len(strings.Join(strings.Fields(text), " "))
}
