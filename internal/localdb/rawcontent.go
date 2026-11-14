package localdb

import (
	"regexp"
	"strings"
)

// blockComment matches a block editor delimiter: <!-- wp:… --> and
// <!-- /wp:… -->. The markup of static blocks between them stays.
var blockComment = regexp.MustCompile(`<!--\s*/?wp:[^>]*?-->`)

// shortcodePattern finds a shortcode tag such as [gallery ids="1,2"].
var shortcodePattern = regexp.MustCompile(`\[/?[a-zA-Z][a-zA-Z0-9_-]*(\s[^\]]*)?\]`)

// paragraphBreak is a blank line, which separates paragraphs.
var paragraphBreak = regexp.MustCompile(`\n\s*\n`)

// blockTagPattern is a line that starts with an element wpautop leaves
// unwrapped.
var blockTagPattern = regexp.MustCompile(`(?i)^<(?:` + strings.Join([]string{
	"p", "div", "h[1-6]", "ul", "ol", "li", "table", "thead", "tbody", "tr", "td", "th", "blockquote", "pre",
	"figure", "figcaption", "section", "article", "aside", "header", "footer", "nav", "hr", "form", "iframe",
	"script", "style", "dl", "dt", "dd", "address", "details", "summary",
}, "|") + `)[\s>/]`)

// CleanRawContent turns a body as stored into something closer to what the
// site renders, for documents the crawl could not read and for comments:
//
//  1. block editor delimiters are removed, the static markup between them kept;
//  2. a body with no paragraph markup gets a simplified wpautop — paragraphs at
//     blank lines, single newlines as <br />, block-level lines left alone;
//  3. shortcodes stay exactly as written (HasShortcode counts them).
//
// It is not a port of wpautop, and does not try to be.
func CleanRawContent(raw string) string {
	content := strings.TrimSpace(blockComment.ReplaceAllString(raw, ""))
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if content == "" || strings.Contains(strings.ToLower(content), "<p") {
		return content
	}

	var out []string
	for _, para := range paragraphBreak.Split(content, -1) {
		// Never empty: the content was trimmed, and a run of blank lines is
		// one separator.
		para = strings.TrimSpace(para)
		if blockTagPattern.MatchString(para) {
			out = append(out, para)
			continue
		}
		out = append(out, "<p>"+strings.ReplaceAll(para, "\n", "<br />\n")+"</p>")
	}
	return strings.Join(out, "\n")
}

// HasShortcode reports whether content still holds a shortcode.
func HasShortcode(content string) bool {
	return shortcodePattern.MatchString(content)
}
