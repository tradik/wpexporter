package export

// "Continue reading" in a language nobody listed.
//
// A theme appends a read-more link to every generated excerpt, and the excerpt
// is not the post: exported as content it becomes a line of chrome at the end of
// every summary on the migrated site. The link was recognized by its WordPress
// `more-link` class or by its text, matched against seven phrases in six
// European languages — which is a list that will never be finished. A Japanese
// site says 続きを読む, a Greek one Διαβάστε περισσότερα, and a theme author is
// free to write "→" and nothing else.
//
// Two things make this answerable without the list:
//
//   - **Structure.** WordPress's own read-more link points at the post it ends,
//     carries `rel="bookmark"` or an `aria-label`, and holds a handful of words
//     at most. None of that is language.
//   - **Repetition.** A theme writes the same string at the end of every
//     excerpt it generates. Across a site's posts, the trailing link text that
//     recurs *is* the read-more phrase, whatever language it is in and whether
//     or not anyone has heard of it.
//
// The phrase list stays as a seed, because the first post of a one-post export
// has nothing to compare itself against, and `--read-more-phrases` names one
// outright for a site that wants no guessing at all.

import (
	"strings"
	"unicode/utf8"
)

// readMoreMaxRunes is how long a trailing link may be and still be chrome.
// "Continue reading »" is 18, "Διαβάστε περισσότερα" is 20; an excerpt ending
// in a real link to a real article is usually longer, and counted in characters
// so the limit means the same thing in every alphabet.
const readMoreMaxRunes = 40

// readMoreSeenTimes is how many excerpts must end in the same words before that
// string is taken to be the theme's, rather than a coincidence of two posts
// linking to the same place.
const readMoreSeenTimes = 3

// readMoreArrows are what a theme writes instead of words, or after them. A
// trailing link that is nothing but these is chrome in any language.
var readMoreArrows = []string{"→", "»", "›", "▸", "⟶", ">>", "...", "…", "->"}

// structuralReadMoreAttrs are the attributes WordPress and its themes put on
// the link they generate. They are markup, so they are the same everywhere.
var structuralReadMoreAttrs = []string{"more-link", "read-more", "readmore", "rel=\"bookmark\""}

// readMoreVocabulary is what one export learned about one site's theme.
//
// The zero value knows only the seeded phrases and the structural marks, which
// is what a single-document conversion has to work with.
type readMoreVocabulary struct {
	learned map[string]bool
	seeded  []string
}

// newReadMoreVocabulary reads a site's own excerpts and learns the phrase its
// theme appends, whatever language it is in.
//
// Counting rather than assuming: a string ending three excerpts is the theme
// speaking, while a string ending one is that post's last sentence, which is
// content and must survive.
func newReadMoreVocabulary(excerpts []string, configured []string) readMoreVocabulary {
	vocabulary := readMoreVocabulary{learned: map[string]bool{}, seeded: seedPhrases(configured)}

	counts := map[string]int{}

	for _, excerpt := range excerpts {
		text := trailingAnchorText(excerpt)
		if text == "" || utf8.RuneCountInString(text) > readMoreMaxRunes {
			continue
		}

		counts[text]++
	}

	for text, seen := range counts {
		if seen >= readMoreSeenTimes {
			vocabulary.learned[text] = true
		}
	}

	return vocabulary
}

// seedPhrases is the starting vocabulary: what the operator named, or the
// handful of phrases common enough to be worth guessing at on a site with too
// few excerpts to learn from.
func seedPhrases(configured []string) []string {
	if len(configured) > 0 {
		phrases := make([]string, 0, len(configured))
		for _, phrase := range configured {
			if phrase = strings.ToLower(strings.TrimSpace(phrase)); phrase != "" {
				phrases = append(phrases, phrase)
			}
		}

		return phrases
	}

	return readMorePhrases
}

// isReadMore reports whether a trailing anchor is theme-generated chrome.
//
// Structure first, because it is the same in every language; then what this
// site's own excerpts revealed; then the seeded phrases, which are the last
// resort rather than the method.
func (v readMoreVocabulary) isReadMore(attrs, inner string) bool {
	if structuralReadMore(attrs) {
		return true
	}

	text := strings.ToLower(strings.TrimSpace(plainText(inner)))
	if text == "" {
		return false
	}

	if v.learned[text] {
		return true
	}

	if arrowOnly(text) {
		return true
	}

	for _, phrase := range v.seeded {
		if strings.HasPrefix(text, phrase) {
			return true
		}
	}

	return false
}

// structuralReadMore reads the marks a theme leaves on the link itself.
func structuralReadMore(attrs string) bool {
	lowered := strings.ToLower(attrs)

	for _, mark := range structuralReadMoreAttrs {
		if strings.Contains(lowered, mark) {
			return true
		}
	}

	return false
}

// arrowOnly reports a link that is punctuation rather than words — "→", "»",
// "…" — which no language makes into a sentence.
func arrowOnly(text string) bool {
	for _, arrow := range readMoreArrows {
		text = strings.ReplaceAll(text, arrow, "")
	}

	return strings.TrimSpace(text) == ""
}

// trailingAnchorText is the words of the anchor closing a string, or "" when it
// does not end in one.
func trailingAnchorText(rendered string) string {
	loc := trailingAnchorPattern.FindStringSubmatchIndex(rendered)
	if loc == nil {
		return ""
	}

	return strings.ToLower(strings.TrimSpace(plainText(rendered[loc[4]:loc[5]])))
}
