package export

// Lists that keep their kind (#39).
//
// Every list used to leave the exporter as bullets: `<li>` became "- " and the
// `<ul>`/`<ol>` around it was deleted, so an ordered list and an unordered one
// were indistinguishable afterwards. For a recipe, a tutorial or an assembly
// guide the numbers *are* the content — "step 3" is a reference, and a bulleted
// method reads as a set of unordered suggestions. Nothing downstream could
// recover the distinction, and no count-based check could notice: the same
// items arrive, with the same words.
//
// Lists are therefore converted structurally rather than tag by tag: innermost
// first, so each level keeps its own kind, and with the ordering attributes
// Markdown can express (`start`) carried over. The ones it cannot — `type="a"`,
// `reversed` — keep their HTML, which is valid in Markdown and says more than a
// number that would be wrong.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// listTagRe matches either end of a list. The innermost block is found by
	// walking these in order rather than by one pattern: RE2 has neither
	// backreferences nor lookahead, so "a list containing no list" cannot be
	// written as a regex — and a nested list must be converted before the list
	// that holds it.
	listTagRe = regexp.MustCompile(`(?is)<(/?)(ul|ol)\b([^>]*)>`)
	// listItemOpenRe finds where each item begins; the text runs to the next
	// item or the end of the list, so an omitted </li> loses nothing.
	listItemOpenRe = regexp.MustCompile(`(?is)<li\b[^>]*>`)
	listItemEndRe  = regexp.MustCompile(`(?is)</li\s*>`)
	// startAttrRe reads <ol start="5">.
	startAttrRe = regexp.MustCompile(`(?is)\bstart\s*=\s*["']?(-?\d+)`)
	// unnumberableRe marks the ordering Markdown has no syntax for: a letter or
	// roman marker, or a list counting down.
	unnumberableRe = regexp.MustCompile(`(?is)\breversed\b|\btype\s*=\s*["']?[aAiI]`)
	// listBlankLineRe collapses the blank lines inside one item: a blank line
	// would end the list and orphan everything under it.
	listBlankLineRe = regexp.MustCompile(`\n\s*\n+`)
)

// preservedListMarker stands in for a list kept as HTML while the rest of the
// conversion runs. NUL-delimited because no WordPress content contains one, so
// nothing else in the pipeline can match or mangle it.
//
// Named a marker rather than a token because it is neither: gosec reads
// "token" as a credential (G101), and a false positive in a security report
// costs more attention than the word is worth.
const preservedListMarker = "\x00wpx-list-%d\x00"

// convertLists rewrites the content's lists into Markdown and returns, with it,
// the raw blocks that must be restored verbatim afterwards.
func convertLists(html string) (string, []string) {
	var preserved []string

	for {
		block, found := innermostList(html)
		if !found {
			break
		}

		var replacement string
		if block.kind == "ol" && unnumberableRe.MatchString(block.attrs) {
			// A letter, roman or reversed list: Markdown would renumber it 1, 2,
			// 3 and say something the page does not. The HTML stays.
			preserved = append(preserved, html[block.start:block.end])
			replacement = fmt.Sprintf(preservedListMarker, len(preserved)-1)
		} else {
			replacement = renderList(block.kind, block.attrs, html[block.bodyStart:block.bodyEnd])
		}

		html = html[:block.start] + replacement + html[block.end:]
	}

	return html, preserved
}

// listBlock is one list found in the content, by byte offsets.
type listBlock struct {
	kind      string
	attrs     string
	start     int
	end       int
	bodyStart int
	bodyEnd   int
}

// innermostList finds a list holding no other list — an opening tag whose very
// next list tag is a closing one. Converting that first, repeatedly, means a
// nested list is already Markdown by the time its parent item is written.
//
// Unbalanced markup simply yields no pair, and the content is left alone: half
// a list rewritten would be worse than one left as HTML.
func innermostList(html string) (listBlock, bool) {
	tags := listTagRe.FindAllStringSubmatchIndex(html, -1)

	for i := 0; i+1 < len(tags); i++ {
		opening, next := tags[i], tags[i+1]
		isOpening := opening[2] == opening[3] // the "/" group matched nothing
		nextIsClosing := next[2] != next[3]

		if !isOpening || !nextIsClosing {
			continue
		}

		return listBlock{
			kind:      strings.ToLower(html[opening[4]:opening[5]]),
			attrs:     html[opening[6]:opening[7]],
			start:     opening[0],
			end:       next[1],
			bodyStart: opening[1],
			bodyEnd:   next[0],
		}, true
	}

	return listBlock{}, false
}

// restorePreservedLists puts the untouched HTML lists back.
func restorePreservedLists(md string, preserved []string) string {
	for i, block := range preserved {
		md = strings.ReplaceAll(md, fmt.Sprintf(preservedListMarker, i), block)
	}

	return md
}

// renderList turns one list's items into Markdown lines.
func renderList(kind, attrs, body string) string {
	items := listItems(body)
	if len(items) == 0 {
		return ""
	}

	number := 1
	if kind == "ol" {
		number = listStart(attrs)
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		marker := "- "
		if kind == "ol" {
			marker = strconv.Itoa(number) + ". "
			number++
		}

		lines = append(lines, renderItem(marker, item))
	}

	// Blank lines around the block, so a list following a paragraph starts one.
	return "\n\n" + strings.Join(lines, "\n") + "\n\n"
}

// renderItem writes one item: the marker, then every continuation line indented
// to the marker's width, which is what keeps a nested list attached to its
// parent item rather than starting a list of its own.
func renderItem(marker, item string) string {
	item = listBlankLineRe.ReplaceAllString(strings.TrimSpace(item), "\n")

	lines := strings.Split(item, "\n")
	indent := strings.Repeat(" ", len(marker))

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if i == 0 {
			lines[i] = marker + line
			continue
		}
		lines[i] = indent + line
	}

	return strings.Join(lines, "\n")
}

// listItems splits a list body into its items. An item runs from its opening
// tag to the next one or to the end, so markup inside it — a link, bold text,
// an already-converted nested list — travels with it, and a missing `</li>`
// costs nothing.
func listItems(body string) []string {
	opens := listItemOpenRe.FindAllStringIndex(body, -1)
	if len(opens) == 0 {
		return nil
	}

	items := make([]string, 0, len(opens))
	for i, open := range opens {
		end := len(body)
		if i+1 < len(opens) {
			end = opens[i+1][0]
		}

		text := body[open[1]:end]
		if closing := listItemEndRe.FindStringIndex(text); closing != nil {
			text = text[:closing[0]]
		}

		if trimmed := strings.TrimSpace(text); trimmed != "" {
			items = append(items, trimmed)
		}
	}

	return items
}

// listStart reads where an ordered list begins. A list that starts at 5 says so
// because the four before it are elsewhere; renumbering from 1 would be a
// different document.
func listStart(attrs string) int {
	match := startAttrRe.FindStringSubmatch(attrs)
	if match == nil {
		return 1
	}

	start, err := strconv.Atoi(match[1])
	if err != nil {
		return 1
	}

	return start
}
