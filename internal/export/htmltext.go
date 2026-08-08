package export

import (
	"fmt"
	"html"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	// entityPattern matches a named or numeric HTML entity.
	entityPattern = regexp.MustCompile(`&#?[0-9A-Za-z]+;`)
	// htmlTagPattern matches any HTML tag.
	htmlTagPattern = regexp.MustCompile(`<[^>]*>`)
	// whitespacePattern matches any run of whitespace.
	whitespacePattern = regexp.MustCompile(`\s+`)
	// trailingAnchorPattern captures the attributes and inner markup of the anchor
	// that closes a string, plus any closing tags that wrap it — WordPress emits
	// the read-more link inside the excerpt's own <p>.
	trailingAnchorPattern = regexp.MustCompile(`(?is)\s*<a\b([^>]*)>(.*?)</a>((?:\s*</[a-zA-Z][a-zA-Z0-9]*>)*)\s*$`)
	// imgTagPattern matches a complete img tag.
	imgTagPattern = regexp.MustCompile(`(?is)<img\b[^>]*?/?>`)
	// attrPattern matches one quoted name="value" attribute.
	attrPattern = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*("[^"]*"|'[^']*')`)
	// digitsPattern matches a run of digits.
	digitsPattern = regexp.MustCompile(`^\d+$`)
)

// htmlSignificantRunes must survive entity decoding: turning `&lt;script&gt;`
// back into a live tag would resurrect markup the source deliberately escaped.
var htmlSignificantRunes = map[string]bool{"<": true, ">": true, "&": true, `"`: true, "'": true}

// readMorePhrases open the "continue reading" link WordPress themes append to a
// generated excerpt.
var readMorePhrases = []string{
	"continue reading", "read more", "czytaj dalej", "weiterlesen",
	"lire la suite", "leer más", "continua a leggere",
}

// wpInternalClassPrefixes are WordPress presentation classes that refer to the
// old theme's stylesheet rather than to the content.
var wpInternalClassPrefixes = []string{
	"wp-image-", "size-", "attachment-", "align", "wp-post-image", "wp-block-",
}

// droppedImageAttrs are browser hints a static site generator emits itself.
var droppedImageAttrs = map[string]bool{"loading": true, "decoding": true, "sizes": true}

// htmlAttr is one attribute of a tag, with its value already entity-decoded.
type htmlAttr struct {
	name  string
	value string
}

// decodeTypographicEntities decodes HTML entities to their UTF-8 characters,
// leaving the five HTML-significant ones encoded.
//
// Exported files are UTF-8, so `&#8211;` and `&hellip;` are noise that survives
// into the rendered page. `&lt;` and friends are not noise — they are the
// source's own escaping, and decoding them would turn escaped markup into live
// markup.
func decodeTypographicEntities(s string) string {
	return entityPattern.ReplaceAllStringFunc(s, func(entity string) string {
		decoded := html.UnescapeString(entity)

		// Reject anything that did not collapse to a single character. Go's
		// unescaper honors HTML5 legacy entities that need no semicolon, so
		// "&notanentity;" would otherwise come back as "¬anentity;".
		if decoded == entity || utf8.RuneCountInString(decoded) != 1 {
			return entity
		}

		if htmlSignificantRunes[decoded] {
			return entity
		}

		return decoded
	})
}

// plainText reduces rendered HTML to plain text: tags stripped, entities
// decoded, whitespace collapsed. Suitable for a meta description.
func plainText(rendered string) string {
	stripped := htmlTagPattern.ReplaceAllString(rendered, " ")
	decoded := html.UnescapeString(stripped)

	return strings.TrimSpace(whitespacePattern.ReplaceAllString(decoded, " "))
}

// plainTextExcerpt turns a rendered WordPress excerpt into plain text fit for a
// description field: the theme's "continue reading" chrome is removed first,
// since it is navigation rather than content and otherwise lands in
// `<meta name="description">`.
func plainTextExcerpt(rendered string) string {
	return plainText(stripReadMoreAnchor(rendered))
}

// stripReadMoreAnchor removes a trailing "Continue reading →" link. An excerpt
// that legitimately ends in a link keeps it — only anchors that look like
// read-more chrome are removed.
func stripReadMoreAnchor(rendered string) string {
	loc := trailingAnchorPattern.FindStringSubmatchIndex(rendered)
	if loc == nil {
		return rendered
	}

	attrs := rendered[loc[2]:loc[3]]
	inner := rendered[loc[4]:loc[5]]
	closingTags := rendered[loc[6]:loc[7]]

	if !isReadMoreAnchor(attrs, inner) {
		return rendered
	}

	// The wrapping tags the anchor sat inside are content structure, not chrome.
	return strings.TrimSpace(rendered[:loc[0]]) + closingTags
}

// isReadMoreAnchor reports whether an anchor is theme-generated read-more chrome,
// judged by WordPress's `more-link` class or by the link text.
func isReadMoreAnchor(attrs, inner string) bool {
	if strings.Contains(strings.ToLower(attrs), "more-link") {
		return true
	}

	text := strings.ToLower(plainText(inner))
	for _, phrase := range readMorePhrases {
		if strings.HasPrefix(text, phrase) {
			return true
		}
	}

	return false
}

// cleanImages rewrites every img tag in content: a missing alt is filled from the
// media library (WCAG 2.2 SC 1.1.1), a title that merely repeats the filename is
// dropped, WordPress-internal classes are removed, and browser hints the
// generator re-adds itself are stripped.
//
// altByURL maps an image URL — in whichever form the content carries it — to its
// alt text.
func cleanImages(content string, altByURL map[string]string) string {
	return imgTagPattern.ReplaceAllStringFunc(content, func(tag string) string {
		return cleanImageTag(tag, altByURL)
	})
}

// cleanImageTag applies the cleanup to a single img tag.
func cleanImageTag(tag string, altByURL map[string]string) string {
	attrs := parseAttributes(tag)

	var src string
	hasAlt := false

	for _, attr := range attrs {
		switch attr.name {
		case "src":
			src = attr.value
		case "alt":
			hasAlt = strings.TrimSpace(attr.value) != ""
		}
	}

	cleaned := make([]htmlAttr, 0, len(attrs))

	for _, attr := range attrs {
		switch {
		case droppedImageAttrs[attr.name]:
			continue
		case attr.name == "title" && repeatsFilename(attr.value, src):
			continue
		case attr.name == "class":
			kept := keepContentClasses(attr.value)
			if kept == "" {
				continue
			}
			attr.value = kept
		case attr.name == "alt" && !hasAlt:
			if alt := altByURL[src]; alt != "" {
				attr.value = alt
				hasAlt = true
			}
		}

		cleaned = append(cleaned, attr)
	}

	if !hasAlt {
		if alt := altByURL[src]; alt != "" {
			cleaned = append(cleaned, htmlAttr{name: "alt", value: alt})
		}
	}

	return renderImageTag(cleaned)
}

// parseAttributes extracts a tag's quoted attributes in source order, decoding
// each value so it can be re-escaped exactly once on output.
func parseAttributes(tag string) []htmlAttr {
	matches := attrPattern.FindAllStringSubmatch(tag, -1)
	attrs := make([]htmlAttr, 0, len(matches))

	for _, match := range matches {
		quoted := match[2]
		value := quoted[1 : len(quoted)-1]

		attrs = append(attrs, htmlAttr{
			name:  strings.ToLower(match[1]),
			value: html.UnescapeString(value),
		})
	}

	return attrs
}

// renderImageTag rebuilds an img tag from its attributes.
func renderImageTag(attrs []htmlAttr) string {
	var builder strings.Builder

	builder.WriteString("<img")
	for _, attr := range attrs {
		builder.WriteString(fmt.Sprintf(" %s=%q", attr.name, html.EscapeString(attr.value)))
	}
	builder.WriteString(">")

	return builder.String()
}

// keepContentClasses drops WordPress presentation classes, returning the classes
// worth keeping as a space-separated list.
func keepContentClasses(value string) string {
	kept := make([]string, 0, len(strings.Fields(value)))

	for _, class := range strings.Fields(value) {
		if !isWordPressInternalClass(class) {
			kept = append(kept, class)
		}
	}

	return strings.Join(kept, " ")
}

// isWordPressInternalClass reports whether a class is WordPress presentation
// scaffolding rather than authored markup.
func isWordPressInternalClass(class string) bool {
	lower := strings.ToLower(class)
	for _, prefix := range wpInternalClassPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}

	return false
}

// repeatsFilename reports whether a title attribute merely restates the image's
// filename, which carries no information a screen reader or reader can use.
func repeatsFilename(title, src string) bool {
	if title == "" || src == "" {
		return false
	}

	base := path.Base(src)
	stem := strings.TrimSuffix(base, path.Ext(base))

	// Exported files carry an attachment-ID prefix ("391_fran1").
	if idx := strings.Index(stem, "_"); idx > 0 && digitsPattern.MatchString(stem[:idx]) {
		stem = stem[idx+1:]
	}

	return strings.EqualFold(strings.TrimSpace(title), stem)
}
