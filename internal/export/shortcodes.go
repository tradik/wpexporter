package export

// Shortcodes the REST API did not expand (#47).
//
// A plugin that renders on the front end and not in the REST context leaves its
// shortcode in `content.rendered`, and the export wrote it into the document.
// The migrated page then shows a reader the source text of a plugin call —
// `[osm_map_v3 map_center=&#8221;35.849,14.571&#8243; …]`, mangled entities and
// all — where the site rendered a map. Counted across six unrelated migrations:
// 113 leaks in three of them, an events calendar, an image plugin, a gallery, a
// newsletter form and a map.
//
// A leftover shortcode is never content a reader should see. Removing it leaves
// a gap the operator can fill; leaving it shows markup to visitors. Both are
// losses, and only one of them is visible on the published site — so the markup
// goes, and the export says what it removed, with counts, rather than letting
// anybody find out later.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

var (
	// shortcodeOpenRe matches the opening of a shortcode: [name] or
	// [name attr="…"].
	//
	// Three or more characters and a letter first, so a footnote marker or an
	// editorial `[1]` is not mistaken for a plugin call. Whether the match is
	// really a shortcode is decided below — RE2 has no backreferences, so a
	// paired shortcode cannot be written as one pattern anyway, and the body
	// between [gallery] and [/gallery] is found by looking for the closing tag
	// the opening one names.
	shortcodeOpenRe = regexp.MustCompile(`(?i)\[([a-z][a-z0-9_-]{2,})((?:\s[^\]]*)?)\]`)
	// markdownLinkTailRe recognizes the "(" that makes a bracket a link label.
	// Eating `[text](url)` would be a worse bug than the one being fixed.
	markdownLinkTailRe = regexp.MustCompile(`^\s*\(`)
)

// knownProse are bracketed words that are ordinary editorial marks rather than
// plugin calls, and would otherwise match the pattern.
var knownProse = map[string]bool{
	"sic": true, "etc": true, "and": true, "the": true, "more": true,
	"citation": true, "note": true, "author": true, "editor": true,
}

// shortcodeLeak is one shortcode removed from one document.
type shortcodeLeak struct {
	Name     string
	Document string
}

// stripShortcodes removes the shortcodes a plugin never expanded, and returns
// the names of what it removed. Content with none comes back untouched.
func stripShortcodes(content string) (string, []string) {
	if !strings.Contains(content, "[") {
		return content, nil
	}

	var (
		rebuilt strings.Builder
		removed []string
		cursor  int
	)

	for _, match := range shortcodeOpenRe.FindAllStringSubmatchIndex(content, -1) {
		if match[0] < cursor {
			continue // inside a body already removed
		}

		name := strings.ToLower(content[match[2]:match[3]])
		attrs := content[match[4]:match[5]]

		if !isShortcode(name, attrs, content[match[1]:], standsAlone(content, match[0], match[1])) {
			continue
		}

		rebuilt.WriteString(content[cursor:match[0]])
		cursor = match[1]

		// [gallery]…[/gallery] takes its body with it: the body is the
		// plugin's arguments, not the page's prose.
		closing := "[/" + name + "]"
		if end := strings.Index(strings.ToLower(content[cursor:]), closing); end >= 0 {
			cursor += end + len(closing)
		}

		removed = append(removed, name)
	}

	rebuilt.WriteString(content[cursor:])

	// Removing a block can leave the blank lines that surrounded it back to
	// back, which a Markdown reader turns into a gap nobody wrote.
	return mdBlankLinesRe.ReplaceAllString(rebuilt.String(), "\n\n"), removed
}

// isShortcode decides whether a bracketed name is a plugin call rather than
// something a person wrote.
//
// A plugin call carries attributes, has an underscore in its name, closes
// itself later in the document, or stands alone as a whole block — which is how
// `[postimages]` in a paragraph of its own differs from `[sic]` in the middle of
// a sentence. `[text](url)` is a Markdown link, and eating its label would be a
// worse bug than the one being fixed.
func isShortcode(name, attrs, tail string, alone bool) bool {
	if knownProse[name] || markdownLinkTailRe.MatchString(tail) {
		return false
	}

	if strings.TrimSpace(attrs) != "" || strings.Contains(name, "_") {
		return true
	}

	if strings.Contains(strings.ToLower(tail), "[/"+name+"]") {
		return true
	}

	return alone
}

// standsAlone reports whether nothing but markup and whitespace sits on either
// side of the bracket — a paragraph that holds only a plugin call, rather than
// a word inside a sentence.
func standsAlone(content string, start, end int) bool {
	before := strings.TrimRight(content[:start], " \t\r\n")
	after := strings.TrimLeft(content[end:], " \t\r\n")

	leftClear := before == "" || strings.HasSuffix(before, ">")
	rightClear := after == "" || strings.HasPrefix(after, "<")

	return leftClear && rightClear
}

// stripPostShortcodes cleans one document and records what it lost.
func (e *Exporter) stripPostShortcodes(post *models.WordPressPost, document string) {
	cleaned, removed := stripShortcodes(post.Content.Rendered)
	if len(removed) == 0 {
		return
	}

	post.Content.Rendered = cleaned

	for _, name := range removed {
		e.shortcodeLeaks = append(e.shortcodeLeaks, shortcodeLeak{Name: name, Document: document})
	}
}

// reportShortcodes states what was removed, heaviest first: the biggest number
// is a plugin whose output is missing from every page that used it.
//
// The count is the point. "45 [eo_events]" tells an operator that a calendar
// has to be rebuilt on the other side; silence tells them nothing until a
// visitor finds the gap.
func (e *Exporter) reportShortcodes(data *models.ExportData) {
	if len(e.shortcodeLeaks) == 0 {
		return
	}

	counts := map[string]int{}
	for _, leak := range e.shortcodeLeaks {
		counts[leak.Name]++
	}

	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}

	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}

		return names[i] < names[j]
	})

	lines := make([]string, 0, len(names)+1)
	lines = append(lines, fmt.Sprintf(
		"Removed %d unexpanded shortcode(s) — their plugins do not render over REST, "+
			"so the export carried their source text, not their output:", len(e.shortcodeLeaks)))

	for _, name := range names {
		lines = append(lines, fmt.Sprintf("  [%-24s %d", name+"]", counts[name]))
	}

	data.Stats.RemovedShortcodes = lines

	if !e.config.Quiet {
		for _, line := range lines {
			fmt.Println(line)
		}
	}
}
