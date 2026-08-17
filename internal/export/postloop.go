package export

// Pages whose content is a post loop (#41).
//
// A site's /blog/ is often a WordPress *page* whose body, in the editor, is a
// page-builder element that renders the archive: Fusion's [fusion_blog],
// Elementor's Posts widget, a block query, a shortcode. The REST API serves
// what is stored, and what is stored is the element — the listing itself is
// produced at render time by the plugin, and never reaches the export.
//
// Such a page exports as a few hundred bytes that say nothing, which is worse
// than empty: it collides with the target's own listing. A generator told to
// build the archive at /blog/ finds a migrated page already sitting there, the
// page wins, and the operator gets an empty blog with no error anywhere.
//
// So the page is exported — it has a title, an address and possibly an
// introduction worth keeping — and it says what it is. The detection is a short
// list of markers that names what it matched, so a wrong guess is visible in the
// front matter and in the report rather than silently changing what is written.

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/tradik/wpexporter/pkg/models"
)

// postLoopMarkers are the elements that render an archive rather than store
// one. Each is matched literally, case-insensitively, against the raw body.
//
// These are identifiers rather than words — a shortcode name, a block name, a
// CSS class — so they read the same on a Polish site as on an English one, and
// nothing here needs translating. What they do need is company: every theme
// that ships its own listing element writes its own name for it, and the next
// one is on nobody's list. `--post-loop-markers` adds this site's.
var postLoopMarkers = []string{
	"fusion_blog",            // Avada / Fusion Builder
	"elementor-widget-posts", // Elementor Posts widget
	"elementor-widget-archive-posts",
	"wp:query",       // Gutenberg Query Loop block
	"wp-block-query", // …as rendered
	"[blog",          // generic shortcodes
	"[posts",
	"[recent_posts",
	"[ajax_load_more",
	"td_block_",     // Newspaper / tagDiv
	"vc_basic_grid", // WPBakery post grid
	"et_pb_blog",    // Divi blog module
}

// postLoopTextBudget is how many visible characters a page may carry and still
// count as "generated, not stored".
//
// Characters rather than bytes: a budget counted in bytes is a different budget
// on every alphabet. 400 bytes is 400 letters of English, 300 of Polish and 133
// of Japanese — so the same listing page would be judged three ways depending
// on the language it is written in, and the sites that suffer are never the
// English ones.
//
// Not zero: a listing page usually has a heading and a line of introduction
// above the loop, and that copy is worth migrating. Well below the length of a
// page that actually says something, so a real article that happens to mention
// a shortcode in passing is not mislabelled.
const postLoopTextBudget = 400

var (
	// shortcodeRe strips [shortcodes] before the remaining text is measured:
	// the marker itself is not content.
	shortcodeRe = regexp.MustCompile(`\[[^\]]*\]`)
	// tagRe strips markup, leaving the words a reader would see.
	tagRe = regexp.MustCompile(`(?s)<[^>]*>`)
	// scriptStyleRe drops script and style bodies, which are not visible text.
	scriptStyleRe = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(?:script|style)\s*>`)
)

// postLoopHint names the post-loop element a page's body consists of, or ""
// when the page is ordinary content.
//
// Both halves have to hold: a known marker, and a body with little else in it.
// An article about Elementor mentioning the Posts widget keeps its own content
// and is not a listing.
func postLoopHint(content string) string {
	return postLoopHintWith(content, nil)
}

// postLoopHintWith asks the same question, with this site's own listing
// elements added to the recognized ones.
func postLoopHintWith(content string, extra []string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	lower := strings.ToLower(content)

	matched := ""
	for _, marker := range allPostLoopMarkers(extra) {
		if strings.Contains(lower, marker) {
			matched = marker
			break
		}
	}

	if matched == "" {
		return ""
	}

	if utf8.RuneCountInString(visibleText(content)) > postLoopTextBudget {
		return ""
	}

	return matched
}

// visibleText reduces a body to the words a reader would see, so the measure is
// of content rather than of markup.
func visibleText(content string) string {
	text := scriptStyleRe.ReplaceAllString(content, " ")
	text = shortcodeRe.ReplaceAllString(text, " ")
	text = tagRe.ReplaceAllString(text, " ")
	text = decodeTypographicEntities(text)

	return strings.TrimSpace(strings.Join(strings.Fields(text), " "))
}

// notePostLoopPages reports every page that renders an archive rather than
// storing one, and records the same lines in metadata.json — a console scrolls
// away, and this is the one warning that changes what the operator configures
// on the other side.
func (e *Exporter) notePostLoopPages(data *models.ExportData) {
	for i := range data.Pages {
		hint := postLoopHintWith(data.Pages[i].Content.Rendered, e.postLoopMarkers())
		if hint == "" {
			continue
		}

		notice := postLoopNotice(data.Pages[i].Link, hint)
		data.Stats.PostLoopPages = append(data.Stats.PostLoopPages, notice)

		if !e.config.Quiet {
			fmt.Println(notice)
		}
	}
}

// writePostLoopFrontMatter states, in the document itself, that its body is a
// listing element. A target that understands the key can point its own archive
// here; one that does not ignores two lines.
func (e *Exporter) writePostLoopFrontMatter(builder *strings.Builder, post models.WordPressPost, contentType string) {
	if contentType != "page" {
		return
	}

	hint := postLoopHintWith(post.Content.Rendered, e.postLoopMarkers())
	if hint == "" {
		return
	}

	builder.WriteString("lists: posts\n")
	fmt.Fprintf(builder, "lists_hint: %q\n", hint)
}

// postLoopNotice is the line the export prints for such a page. It names the
// address, because the point is what the operator should do with it: point the
// generator's listing there instead of migrating a page over it.
func postLoopNotice(link, hint string) string {
	address := link
	if address == "" {
		address = "(no link)"
	}

	return "Warning: " + address + " renders a post loop (" + hint + ") — its content is " +
		"generated, not stored. Point your generator's listing at this URL rather " +
		"than migrating the page."
}

// allPostLoopMarkers is the recognized listing elements plus this site's own.
//
// Additive rather than replacing: a site running Elementor and a theme with its
// own blog shortcode has both, and asking the operator to re-list what the tool
// already knows is how a list goes stale.
func allPostLoopMarkers(extra []string) []string {
	if len(extra) == 0 {
		return postLoopMarkers
	}

	markers := make([]string, 0, len(postLoopMarkers)+len(extra))
	markers = append(markers, postLoopMarkers...)

	for _, marker := range extra {
		if marker = strings.ToLower(strings.TrimSpace(marker)); marker != "" {
			markers = append(markers, marker)
		}
	}

	return markers
}

// postLoopMarkers is what this site named for its own theme, if anything.
func (e *Exporter) postLoopMarkers() []string {
	if e.config == nil {
		return nil
	}

	return e.config.PostLoopMarkers
}
