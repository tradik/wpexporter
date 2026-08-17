package export

// Elements Markdown has nowhere to put (#67).
//
// A heading's classes are dropped on the way to `##`, and on themes that emit
// one generated class per element — `trx_addons_inline_158836093`, with a
// stylesheet rule to match — the heading's color goes with them. The
// stylesheets migrate fine; there is simply nothing left for them to match, so
// the front page's headline turns from the theme's brown to plain black while
// another headline two sections down keeps its color by accident, because that
// one happens to style its inner span as well.
//
// Markdown allows raw HTML, so the remedy is to leave such an element alone.
// What it must not do is decide for itself which classes matter: a Gutenberg
// site carries `wp-block-heading` on every heading it has, and keeping those as
// HTML would turn a clean Markdown export into a wall of tags for every user of
// this tool. So the operator names them, with the flags that already exist for
// exactly this — `--preserve-classes` and `--preserve-ids`, wildcards included,
// which until now only applied to `--flat-html` and `--basic-html`.
//
//	--preserve-classes 'trx_addons_inline_*'   keep what the theme colors
//	--preserve-classes '*'                     keep every element that has a class
//
// Named nothing, an export converts exactly as it did before.

import (
	"fmt"
	"regexp"
	"strings"
)

// preservedMarker stands in for an element kept as HTML while the rest of the
// conversion runs, in the same NUL-delimited form the list preservation uses:
// no WordPress content contains a NUL, so nothing else in the pipeline can
// match or mangle it.
const preservedMarker = "\x00wpx-keep-%d\x00"

// preserveRules are the classes and IDs the operator asked to keep. The zero
// value keeps nothing, which is what nearly every run wants.
type preserveRules struct {
	classes []string
	ids     []string
}

// empty reports a rule set with nothing to do, so the whole pass can be skipped
// rather than compiling patterns for a run that named none.
func (r preserveRules) empty() bool {
	return len(r.classes) == 0 && len(r.ids) == 0
}

// openTagRe finds an element's opening tag. Attributes are captured so the
// class and id can be read without parsing the whole document.
var openTagRe = regexp.MustCompile(`(?is)<([a-z][a-z0-9]*)\b([^>]*)>`)

// voidElements close themselves, so a match on one keeps the tag alone rather
// than searching for an end that never comes.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"source": true, "track": true, "wbr": true,
}

// preserveElements swaps every element the rules name for a marker, and returns
// the raw HTML to be put back after the conversion has run.
func preserveElements(html string, rules preserveRules) (string, []string) {
	if rules.empty() {
		return html, nil
	}

	matchers := attributeMatchers(rules)

	var preserved []string

	for offset := 0; offset < len(html); {
		tag := openTagRe.FindStringSubmatchIndex(html[offset:])
		if tag == nil {
			break
		}

		start := offset + tag[0]
		name := html[offset+tag[2] : offset+tag[3]]
		attrs := html[offset+tag[4] : offset+tag[5]]

		if !matchesAny(attrs, matchers) {
			offset = start + 1

			continue
		}

		end := elementEnd(html, name, start, offset+tag[1])
		marker := fmt.Sprintf(preservedMarker, len(preserved))
		preserved = append(preserved, html[start:end])
		html = html[:start] + marker + html[end:]
		offset = start + len(marker)
	}

	return html, preserved
}

// restorePreserved puts the untouched HTML back where its marker sits.
func restorePreserved(md string, preserved []string) string {
	for i, block := range preserved {
		md = strings.ReplaceAll(md, fmt.Sprintf(preservedMarker, i), block)
	}

	return md
}

// elementEnd finds where an element closes, counting nested tags of the same
// name so a <div> holding another <div> is kept whole. Markup that never closes
// keeps the opening tag alone rather than swallowing the rest of the document.
func elementEnd(html, name string, start, afterOpen int) int {
	if voidElements[strings.ToLower(name)] || strings.HasSuffix(html[start:afterOpen], "/>") {
		return afterOpen
	}

	pattern := regexp.MustCompile(`(?is)<(/?)` + regexp.QuoteMeta(name) + `\b[^>]*>`)

	depth := 1
	for offset := afterOpen; offset < len(html); {
		tag := pattern.FindStringSubmatchIndex(html[offset:])
		if tag == nil {
			break
		}

		if html[offset+tag[2]:offset+tag[3]] == "/" {
			depth--
			if depth == 0 {
				return offset + tag[1]
			}
		} else {
			depth++
		}

		offset += tag[1]
	}

	return afterOpen
}

// attributeMatchers compiles one pattern per named class or ID. Wildcards are
// supported because the classes worth keeping are often generated — a theme
// emits `trx_addons_inline_158836093` per element, and no operator can list
// those in advance.
func attributeMatchers(rules preserveRules) []*regexp.Regexp {
	matchers := make([]*regexp.Regexp, 0, len(rules.classes)+len(rules.ids))

	for _, class := range rules.classes {
		if pattern := attributeMatcher("class", class); pattern != nil {
			matchers = append(matchers, pattern)
		}
	}

	for _, id := range rules.ids {
		if pattern := attributeMatcher("id", id); pattern != nil {
			matchers = append(matchers, pattern)
		}
	}

	return matchers
}

// attributeMatcher builds the pattern for one named value of one attribute.
func attributeMatcher(attribute, value string) *regexp.Regexp {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	return regexp.MustCompile(`(?is)\b` + attribute + `\s*=\s*["'][^"']*\b` +
		wildcardPattern(value) + `\b[^"']*["']`)
}

// wildcardPattern turns a shell-style name into a regular expression, so
// `trx_addons_inline_*` matches whatever number the theme generated.
func wildcardPattern(value string) string {
	parts := strings.Split(value, "*")
	for i, part := range parts {
		parts[i] = regexp.QuoteMeta(part)
	}

	return strings.Join(parts, `[^"'\s]*`)
}

// matchesAny reports whether an opening tag's attributes carry anything the
// operator named.
func matchesAny(attrs string, matchers []*regexp.Regexp) bool {
	for _, matcher := range matchers {
		if matcher.MatchString(attrs) {
			return true
		}
	}

	return false
}
