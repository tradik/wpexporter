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
// 1.8.15 made that an opt-in — `--preserve-classes`, which the reporter had no
// reason to guess at — and the export lost the colors again. A silence the
// operator has to know the cure for is still a silence.
//
// So a heading now keeps itself, by default, when it carries a class that means
// something. The line is drawn at boilerplate: `wp-block-heading`,
// `has-text-align-center`, `screen-reader-text` and their kind are what
// WordPress and its blocks stamp on every heading on every site, they say
// nothing a Markdown heading cannot, and keeping those as HTML would turn a
// clean export into a wall of tags for everyone. Anything else — a theme's
// `sc_item_title`, a generated `trx_addons_inline_158836093`, a `text-center`
// from a framework — is styling this format cannot express, and dropping it
// silently is how a headline changes color in migration.
//
// Where the line falls is a property of the site, not of this tool, so it is
// three answers rather than one:
//
//	--preserve-styling auto    (default) keep a heading whose classes mean something
//	--preserve-styling none    convert everything, as 1.8.14 did
//	--preserve-styling all     keep every element carrying a class
//
// and `--preserve-classes` / `--preserve-ids` still name elements exactly,
// wildcards included, on top of whichever mode is in force:
//
//	--preserve-classes 'trx_addons_inline_*'   keep exactly this family

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tradik/wpexporter/internal/rx"
)

// preservedMarker stands in for an element kept as HTML while the rest of the
// conversion runs, in the same NUL-delimited form the list preservation uses:
// no WordPress content contains a NUL, so nothing else in the pipeline can
// match or mangle it.
const preservedMarker = "\x00wpx-keep-%d\x00"

// preserveRules are what this run keeps as HTML: the classes and IDs the
// operator named, and — unless they turned it off — any heading whose classes
// are not boilerplate.
type preserveRules struct {
	classes []string
	ids     []string
	// styledHeadings keeps a heading carrying a class Markdown cannot express.
	// On by default because the loss it prevents is invisible until somebody
	// opens the migrated site and finds the headline in the wrong color (#67).
	styledHeadings bool
	// styledAnything keeps every element that carries a class at all — the
	// answer for a site whose layout is styling from top to bottom, where
	// keeping only the headings would save the color of the headline and lose
	// the section it sits in.
	styledAnything bool
	// ignored are the classes this site considers noise, on top of the ones
	// every WordPress emits. A theme that stamps `sc_title_title` on every
	// heading it has is boilerplate on that site and nowhere else.
	ignored []*regexp.Regexp
}

// Modes of --preserve-styling. Named rather than boolean because two of the
// three answers are not the negation of the other.
const (
	// StylingAuto keeps a heading whose classes are not boilerplate.
	StylingAuto = "auto"
	// StylingNone converts everything, which is the 1.8.14 conversion.
	StylingNone = "none"
	// StylingAll keeps every element that carries a class.
	StylingAll = "all"
)

// StylingModes are the accepted answers, for the flag parser to check against
// so a typo is refused rather than silently read as "none".
var StylingModes = []string{StylingAuto, StylingNone, StylingAll}

// empty reports a rule set with nothing to do, so the whole pass can be skipped
// rather than compiling patterns for a run that named none.
func (r preserveRules) empty() bool {
	return len(r.classes) == 0 && len(r.ids) == 0 && !r.styledHeadings && !r.styledAnything
}

// boilerplateClassRe matches the classes WordPress, its block editor and its
// alignment and color utilities stamp on headings everywhere. They describe
// nothing a Markdown heading is missing, so a heading wearing only these
// converts as it always has.
var boilerplateClassRe = regexp.MustCompile(`^(wp-block-[\w-]+|has-[\w-]+|is-[\w-]+|` +
	`align[\w-]*|screen-reader-text|entry-title|post-title|page-title|widget-title|` +
	`section-title|title|heading|subtitle|sr-only)$`)

// boilerplate reports a class that says nothing a Markdown heading is missing.
// The built-in list covers what WordPress and its blocks stamp everywhere;
// a site whose theme adds its own noise names it with --boilerplate-classes,
// because one tool's idea of meaningless cannot fit every theme ever written.
func (r preserveRules) boilerplate(class string) bool {
	if boilerplateClassRe.MatchString(class) {
		return true
	}

	for _, ignored := range r.ignored {
		if ignored != nil && ignored.MatchString(class) {
			return true
		}
	}

	return false
}

// headingTagRe matches an opening heading tag and captures its attributes.
var headingTagRe = regexp.MustCompile(`(?is)^<h[1-6]\b([^>]*)>$`)

// classValueRe reads a class attribute's value.
var classValueRe = regexp.MustCompile(`(?is)\bclass\s*=\s*["']([^"']*)["']`)

// keeps reports whether this run's styling mode holds on to an element, given
// its whole opening tag.
func (r preserveRules) keeps(openTag string) bool {
	switch {
	case r.styledAnything:
		return classValueRe.MatchString(openTag)
	case r.styledHeadings:
		return r.keepsItsStyling(openTag)
	default:
		return false
	}
}

// keepsItsStyling reports a heading whose classes carry something a `##` cannot.
func (r preserveRules) keepsItsStyling(openTag string) bool {
	attrs := headingTagRe.FindStringSubmatch(openTag)
	if attrs == nil {
		return false
	}

	classes := classValueRe.FindStringSubmatch(attrs[1])
	if classes == nil {
		return false
	}

	for _, class := range strings.Fields(classes[1]) {
		if !r.boilerplate(class) {
			return true
		}
	}

	return false
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

		named := matchesAny(attrs, matchers)
		styled := rules.keeps(html[start : offset+tag[1]])

		if !named && !styled {
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

	// Compiled once per tag name rather than once per element: this runs for
	// every kept element of every document, and HTML has a small closed set of
	// names to build it from.
	pattern := rx.Get(`(?is)<(/?)` + regexp.QuoteMeta(name) + `\b[^>]*>`)

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

	return rx.Get(`(?is)\b` + attribute + `\s*=\s*["'][^"']*\b` +
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

// compileClassPatterns turns the operator's names into matchers, wildcards
// included, skipping anything empty so a stray comma cannot match everything.
func compileClassPatterns(names []string) []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, 0, len(names))

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		patterns = append(patterns, rx.Get(`^`+wildcardPattern(name)+`$`))
	}

	return patterns
}
