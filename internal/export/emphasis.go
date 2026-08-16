package export

// Emphasis that actually closes (#50).
//
// WordPress content is full of `<strong>text </strong>` — the space sits inside
// the tags, which every browser renders without comment. Converted tag for tag
// that becomes `**text **`, and in CommonMark a closing delimiter run preceded
// by whitespace is not right-flanking: it closes nothing, so the reader is shown
// the asterisks instead of bold text.
//
//	Projekt ***bociany.pl ***realizowany jest przez Fundację…
//
// 157 of them across six unrelated migrations, every one printing raw asterisks
// on a published page. The mirror rule catches a leading space the same way,
// and a run with nothing but whitespace inside means nothing in either language.
//
// The whitespace is therefore moved out of the delimiters before conversion,
// where the HTML still says unambiguously which side it belongs to. Nothing is
// lost: the space stays exactly where it was on the page, just outside the
// emphasis rather than inside it.

import (
	"regexp"
	"strings"
)

// emphasisTags are the elements the converter turns into delimiter runs.
var emphasisTags = []string{"strong", "b", "em", "i"}

// emphasisSpacingRe matches one emphasis element, splitting the whitespace at
// each end of its content from the content itself.
//
// One pattern per tag name rather than a backreference, which RE2 does not
// have; the closing tag is literal, so nothing else can close the match.
var emphasisSpacingRe = buildEmphasisPatterns()

func buildEmphasisPatterns() map[string]*regexp.Regexp {
	patterns := make(map[string]*regexp.Regexp, len(emphasisTags))

	for _, tag := range emphasisTags {
		patterns[tag] = regexp.MustCompile(
			`(?is)<` + tag + `\b[^>]*>(\s*)(.*?)(\s*)</` + tag + `\s*>`)
	}

	return patterns
}

// normalizeEmphasisSpacing moves whitespace out of every emphasis element, and
// drops one that holds nothing else.
func normalizeEmphasisSpacing(html string) string {
	for _, tag := range emphasisTags {
		pattern := emphasisSpacingRe[tag]

		html = pattern.ReplaceAllStringFunc(html, func(match string) string {
			parts := pattern.FindStringSubmatch(match)
			leading, content, trailing := parts[1], parts[2], parts[3]

			// `<strong> </strong>` is emphasis around nothing. Keeping it would
			// write `** **`, which is not emphasis in Markdown and is not text
			// either — the space alone is what the page showed.
			if strings.TrimSpace(content) == "" {
				return leading + trailing
			}

			return leading + "<" + tag + ">" + content + "</" + tag + ">" + trailing
		})
	}

	return html
}
