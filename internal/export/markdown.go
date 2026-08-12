package export

import (
	"regexp"
	"strings"
)

// Attribute-aware HTML→Markdown substitutions. The previous implementation used
// literal string replacement (`<h2>` → `## `), which matched only bare tags. For
// Gutenberg content — `<h2 class="wp-block-heading">`, `<p class="wp-block-paragraph">`,
// `<ul class="wp-block-list">` — it left the opening tag in place while still
// converting the closing tag, producing orphaned opening tags. Every CommonMark
// renderer then treats those lines as raw HTML blocks and renders the `**`/`- `
// markers literally (issue #21). Matching the opening tag regardless of attributes
// converts both ends, so no orphan survives.
//
// Self-contained elements (`<img>`, `<figure>`, `<a>`, …) are deliberately left as
// HTML: a complete tag is valid inside Markdown, the SSG format keeps images as
// cleaned HTML, and media URLs inside them are localized separately by the rewriter.
var (
	mdScriptStyleRe   = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(?:script|style)\s*>`)
	mdHeadingOpenRe   = regexp.MustCompile(`(?i)<h([1-6])\b[^>]*>`)
	mdHeadingCloseRe  = regexp.MustCompile(`(?i)</h[1-6]\s*>`)
	mdStrongOpenRe    = regexp.MustCompile(`(?i)<(?:strong|b)\b[^>]*>`)
	mdStrongCloseRe   = regexp.MustCompile(`(?i)</(?:strong|b)\s*>`)
	mdEmOpenRe        = regexp.MustCompile(`(?i)<(?:em|i)\b[^>]*>`)
	mdEmCloseRe       = regexp.MustCompile(`(?i)</(?:em|i)\s*>`)
	mdPreOpenRe       = regexp.MustCompile(`(?i)<pre\b[^>]*>`)
	mdPreCloseRe      = regexp.MustCompile(`(?i)</pre\s*>`)
	mdCodeOpenRe      = regexp.MustCompile(`(?i)<code\b[^>]*>`)
	mdCodeCloseRe     = regexp.MustCompile(`(?i)</code\s*>`)
	mdBlockquoteOpen  = regexp.MustCompile(`(?i)<blockquote\b[^>]*>`)
	mdBlockquoteClose = regexp.MustCompile(`(?i)</blockquote\s*>`)
	mdListWrapOpenRe  = regexp.MustCompile(`(?i)<(?:ul|ol)\b[^>]*>`)
	mdListWrapCloseRe = regexp.MustCompile(`(?i)</(?:ul|ol)\s*>`)
	mdLiOpenRe        = regexp.MustCompile(`(?i)<li\b[^>]*>`)
	mdLiCloseRe       = regexp.MustCompile(`(?i)</li\s*>`)
	mdParaOpenRe      = regexp.MustCompile(`(?i)<p\b[^>]*>`)
	mdParaCloseRe     = regexp.MustCompile(`(?i)</p\s*>`)
	mdBrRe            = regexp.MustCompile(`(?i)<br\b[^>]*>`)
	mdBlankLinesRe    = regexp.MustCompile(`\n{3,}`)
)

// htmlToMarkdown converts the block and inline elements WordPress emits into
// Markdown, attribute-aware so Gutenberg block tags convert cleanly and leave no
// orphaned opening tag behind (issue #21). Self-contained HTML (images, figures,
// anchors) is passed through unchanged.
func htmlToMarkdown(input string) string {
	md := mdScriptStyleRe.ReplaceAllString(input, "")

	// Headings: prefix a blank line so a heading that follows inline content starts
	// its own block; the final blank-line collapse removes the extras.
	md = mdHeadingOpenRe.ReplaceAllStringFunc(md, func(tag string) string {
		level := int(mdHeadingOpenRe.FindStringSubmatch(tag)[1][0] - '0')
		return "\n\n" + strings.Repeat("#", level) + " "
	})
	md = mdHeadingCloseRe.ReplaceAllString(md, "\n\n")

	md = mdStrongOpenRe.ReplaceAllString(md, "**")
	md = mdStrongCloseRe.ReplaceAllString(md, "**")
	md = mdEmOpenRe.ReplaceAllString(md, "*")
	md = mdEmCloseRe.ReplaceAllString(md, "*")

	md = mdPreOpenRe.ReplaceAllString(md, "```\n")
	md = mdPreCloseRe.ReplaceAllString(md, "\n```")
	md = mdCodeOpenRe.ReplaceAllString(md, "`")
	md = mdCodeCloseRe.ReplaceAllString(md, "`")

	md = mdBlockquoteOpen.ReplaceAllString(md, "\n\n> ")
	md = mdBlockquoteClose.ReplaceAllString(md, "\n\n")

	md = mdListWrapOpenRe.ReplaceAllString(md, "")
	md = mdListWrapCloseRe.ReplaceAllString(md, "\n")
	md = mdLiOpenRe.ReplaceAllString(md, "- ")
	md = mdLiCloseRe.ReplaceAllString(md, "\n")

	md = mdParaOpenRe.ReplaceAllString(md, "")
	md = mdParaCloseRe.ReplaceAllString(md, "\n\n")
	md = mdBrRe.ReplaceAllString(md, "\n")

	md = mdBlankLinesRe.ReplaceAllString(md, "\n\n")
	md = strings.TrimSpace(md)

	// UTF-8 output, so typographic entities are noise; HTML-significant ones stay
	// encoded (decoding them would turn escaped markup into live markup).
	return decodeTypographicEntities(md)
}
