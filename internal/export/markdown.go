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
	return htmlToMarkdownKeeping(input, preserveRules{})
}

// htmlToMarkdownKeeping converts the same way, leaving the elements the
// operator named as the HTML they came in as — a heading whose class is where
// the theme's color lives has nowhere to put it in Markdown (#67).
func htmlToMarkdownKeeping(input string, keep preserveRules) string {
	md := mdScriptStyleRe.ReplaceAllString(input, "")

	// Before the first rule rewrites anything: an element kept whole has to be
	// out of the way of every one of them, not just the one that would have
	// converted its outer tag.
	md, preservedElements := preserveElements(md, keep)

	// Before any delimiter is written: a space inside <strong> becomes a space
	// inside `**`, and a closing run preceded by whitespace closes nothing in
	// CommonMark, so the reader is shown the asterisks (#50).
	md = normalizeEmphasisSpacing(md)

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

	// A fence owns its lines and is longer than anything inside it, so a <pre>
	// a plugin ships inside a <div> cannot take the rest of the page with it
	// (#69). The tag-by-tag rules below still catch a stray half of a <pre>,
	// which is malformed markup rather than a block.
	md = convertPreBlocks(md)
	md = mdPreOpenRe.ReplaceAllString(md, "\n```\n")
	md = mdPreCloseRe.ReplaceAllString(md, "\n```\n")
	md = mdCodeOpenRe.ReplaceAllString(md, "`")
	md = mdCodeCloseRe.ReplaceAllString(md, "`")

	md = mdBlockquoteOpen.ReplaceAllString(md, "\n\n> ")
	md = mdBlockquoteClose.ReplaceAllString(md, "\n\n")

	// Lists are converted as blocks, innermost first, so an ordered list stays
	// ordered and a nested one keeps its own kind (#39). The tag-by-tag rules
	// below still catch a stray item outside any list, which is malformed
	// markup rather than a list, and reads best as a bullet.
	md, preservedLists := convertLists(md)
	md = mdListWrapOpenRe.ReplaceAllString(md, "")
	md = mdListWrapCloseRe.ReplaceAllString(md, "\n")
	md = mdLiOpenRe.ReplaceAllString(md, "- ")
	md = mdLiCloseRe.ReplaceAllString(md, "\n")

	md = mdParaOpenRe.ReplaceAllString(md, "")
	md = mdParaCloseRe.ReplaceAllString(md, "\n\n")
	md = mdBrRe.ReplaceAllString(md, "\n")

	md = mdBlankLinesRe.ReplaceAllString(md, "\n\n")
	md = dedentOutsideCodeFences(md)
	md = strings.TrimSpace(md)

	// UTF-8 output, so typographic entities are noise; HTML-significant ones stay
	// encoded (decoding them would turn escaped markup into live markup).
	md = decodeTypographicEntities(md)

	// A document that ends inside a fence takes the rest of the page with it —
	// and on a generator's index, the next document too (#69).
	md = closeDanglingFence(md)

	// Last, so nothing above rewrites the markup of a list Markdown cannot
	// number — a lettered, roman or reversed one, which travels as HTML — or of
	// an element the operator asked to keep.
	md = restorePreservedLists(md, preservedLists)

	return restorePreserved(md, preservedElements)
}

// dedentOutsideCodeFences strips the leading whitespace of every line that is
// not inside a fenced code block (issue #26).
//
// Page builders — Elementor, WPBakery, Divi — indent their nested markup with
// tabs, and `</p>` above converts to a blank line, which ends the surrounding
// HTML block per CommonMark. Every following builder line then starts with a
// tab, i.e. four columns of indentation, so the renderer reads `</div>` as an
// INDENTED CODE BLOCK and prints the closing tags to the visitor as monospaced
// text. Removing the indentation keeps those lines HTML blocks instead.
//
// Two kinds of line keep their indentation: a fenced block's contents (from
// `<pre>`, where whitespace is the content) and a list line, where the
// indentation attaches a nested list or a continuation to its parent item
// (#39). Everything else that is indented was indented by the source HTML's
// pretty-printer, and loses nothing by being straightened.
func dedentOutsideCodeFences(md string) string {
	lines := strings.Split(md, "\n")
	inFence := false

	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") {
			// The fence marker itself must sit at column 0 to open or close.
			lines[i] = trimmed
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if isListLine(trimmed) && line != trimmed {
			// Indented list content: normalise tabs to the spaces CommonMark
			// counts, and leave the depth alone.
			lines[i] = strings.ReplaceAll(line[:len(line)-len(trimmed)], "\t", "    ") + trimmed
			continue
		}
		lines[i] = trimmed
	}

	return strings.Join(lines, "\n")
}

// listLineRe matches a Markdown list marker: a bullet, or a number followed by
// a dot or a bracket.
var listLineRe = regexp.MustCompile(`^(?:[-*+] |\d+[.)] )`)

// isListLine reports whether a line carries a list marker, and so whether its
// indentation is structure rather than the source's pretty-printing.
func isListLine(trimmed string) bool {
	return listLineRe.MatchString(trimmed)
}
