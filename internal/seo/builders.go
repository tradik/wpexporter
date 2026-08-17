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
// wrappers. Each is the visible half of a plugin that renders at request time:
// King Composer, WPBakery, Divi, Elementor, Beaver Builder, Bricks, Fusion,
// Oxygen, Themify.
var builderClassRe = regexp.MustCompile(`(?i)\b(kc-elm|kc_row|kc_column|vc_row|wpb_|et_pb_|` +
	`elementor-|fl-builder|fl-row|brxe-|fusion-builder|fusion-fullwidth|oxy-|ct-section|` +
	`tb_|themify_builder)`)

// containerTagRe counts the elements a builder nests to make a layout.
var containerTagRe = regexp.MustCompile(`(?i)<(div|section|article|aside|header|footer)\b`)

// htmlTagRe and htmlCommentRe strip the markup so what a reader would see can
// be measured.
var (
	htmlTagRe     = regexp.MustCompile(`(?is)<[^>]*>`)
	htmlCommentRe = regexp.MustCompile(`(?is)<!--.*?-->`)
)

// Thresholds in characters of visible text per container element. A page of
// prose runs into the hundreds; the reported front page, with forty wrappers
// around one headline, is under ten. The known-builder figure is the more
// generous of the two: a wrapper carrying a recognized class is evidence in
// itself, so less is asked of the ratio.
const (
	builderTextPerContainer = 60
	shellTextPerContainer   = 20
	// minContainers keeps an ordinary short page out of it. A page with four
	// divs and a sentence in it is a page, not a shell.
	minContainers = 6
)

// needsRenderedContent reports whether the crawler should fetch the page the
// visitor sees instead of trusting what the API stored.
//
// Two cases: a body the API served empty, which is what this always caught, and
// a body that is a page builder's scaffolding, which is what it did not (#63).
func needsRenderedContent(content string) bool {
	return isContentEmpty(content) || storedAsBuilderMarkup(content)
}

// storedAsBuilderMarkup reports a body that renders to nothing without its
// plugin: many containers, almost no text.
func storedAsBuilderMarkup(content string) bool {
	containers := len(containerTagRe.FindAllString(content, -1))
	if containers < minContainers {
		return false
	}

	perContainer := visibleTextLength(content) / containers

	if builderClassRe.MatchString(content) {
		return perContainer < builderTextPerContainer
	}

	return perContainer < shellTextPerContainer
}

// visibleTextLength is how much of the body a reader would actually see, with
// markup, comments and entity padding removed.
func visibleTextLength(content string) int {
	text := htmlCommentRe.ReplaceAllString(content, "")
	text = htmlTagRe.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	return len(strings.Join(strings.Fields(text), " "))
}
