package basichtml

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// safeURLSchemes is the allow-list of URL schemes permitted in href/src attributes.
// Anything else (javascript:, data:, vbscript:, …) is dropped.
var safeURLSchemes = map[string]bool{
	"http": true, "https": true, "mailto": true, "tel": true, "ftp": true,
}

// isSafeURLValue reports whether an href/src value is safe to keep. It defeats
// common obfuscations by decoding HTML entities and stripping whitespace/control
// characters (which browsers ignore inside a scheme) before checking the scheme
// against an allow-list. Relative URLs (no scheme) are permitted (SEC-003).
func isSafeURLValue(v string) bool {
	decoded := html.UnescapeString(v)
	var b strings.Builder
	for _, r := range decoded {
		if r > 0x20 { // drop spaces and C0 control characters
			b.WriteRune(r)
		}
	}
	cleaned := strings.ToLower(b.String())

	i := strings.IndexByte(cleaned, ':')
	if i < 0 {
		return true // no scheme -> relative URL
	}
	scheme := cleaned[:i]
	// A ':' that follows a path/query/fragment delimiter is not a scheme.
	if strings.ContainsAny(scheme, "/?#") {
		return true
	}
	return safeURLSchemes[scheme]
}

// PreserveOptions defines elements to preserve from HTML processing
type PreserveOptions struct {
	Classes []string // CSS classes to preserve
	IDs     []string // Element IDs to preserve
}

// Sanitizer cleans HTML to basic elements suitable for Shopify/ecommerce platforms
type Sanitizer struct {
	// allowedTags are HTML tags that will be preserved
	allowedTags map[string]bool
	// selfClosingTags don't need closing tags
	selfClosingTags map[string]bool
	// preserveOptions defines elements to preserve from processing
	preserveOptions *PreserveOptions
}

// NewSanitizer creates a new HTML sanitizer
func NewSanitizer() *Sanitizer {
	return newSanitizerWithOptions(nil)
}

// NewSanitizerWithOptions creates a new HTML sanitizer with preserve options
func NewSanitizerWithOptions(preserveOpts *PreserveOptions) *Sanitizer {
	return newSanitizerWithOptions(preserveOpts)
}

// newSanitizerWithOptions is the internal constructor
func newSanitizerWithOptions(preserveOpts *PreserveOptions) *Sanitizer {
	return &Sanitizer{
		allowedTags: map[string]bool{
			// Structure
			"p": true, "br": true, "hr": true,
			// Headers
			"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
			// Formatting
			"strong": true, "b": true, "em": true, "i": true, "u": true,
			"sub": true, "sup": true, "small": true, "mark": true,
			// Lists
			"ul": true, "ol": true, "li": true,
			// Links and media
			"a": true, "img": true,
			// Tables
			"table": true, "thead": true, "tbody": true, "tfoot": true,
			"tr": true, "th": true, "td": true, "caption": true,
			// Quotes and code
			"blockquote": true, "pre": true, "code": true,
			// Semantic
			"article": true, "section": true, "aside": true,
			"figure": true, "figcaption": true,
		},
		selfClosingTags: map[string]bool{
			"br": true, "hr": true, "img": true,
		},
		preserveOptions: preserveOpts,
	}
}

// SanitizePosts sanitizes HTML content in posts
func (s *Sanitizer) SanitizePosts(posts []models.WordPressPost) []models.WordPressPost {
	result := make([]models.WordPressPost, len(posts))
	for i, post := range posts {
		result[i] = post
		result[i].Content.Rendered = s.Sanitize(post.Content.Rendered)
		result[i].Excerpt.Rendered = s.Sanitize(post.Excerpt.Rendered)
	}
	return result
}

// Sanitize cleans HTML content, keeping only basic allowed tags
func (s *Sanitizer) Sanitize(html string) string {
	if html == "" {
		return ""
	}

	// Step 0: Extract and preserve elements with specific classes/IDs
	var preserved []string
	html, preserved = s.extractPreservedElements(html)

	// Step 1: Remove script and style tags with their content
	html = s.removeTagWithContent(html, "script")
	html = s.removeTagWithContent(html, "style")
	html = s.removeTagWithContent(html, "noscript")
	html = s.removeTagWithContent(html, "iframe")
	html = s.removeTagWithContent(html, "svg")

	// Step 2: Remove comments
	commentPattern := regexp.MustCompile(`<!--[\s\S]*?-->`)
	html = commentPattern.ReplaceAllString(html, "")

	// Step 3: Process all tags
	html = s.processTags(html)

	// Step 4: Restore preserved elements
	html = s.restorePreservedElements(html, preserved)

	// Step 5: Clean up whitespace
	html = s.cleanWhitespace(html)

	return html
}

// extractPreservedElements extracts elements with preserved classes/IDs and replaces them with placeholders
func (s *Sanitizer) extractPreservedElements(html string) (string, []string) {
	if s.preserveOptions == nil || (len(s.preserveOptions.Classes) == 0 && len(s.preserveOptions.IDs) == 0) {
		return html, nil
	}

	var preserved []string
	result := html

	// Extract elements by class
	for _, class := range s.preserveOptions.Classes {
		if class == "" {
			continue
		}
		// Convert wildcard pattern to regex (e.g., klaviyo-form-* -> klaviyo-form-[^"'\s]*)
		classPattern := wildcardToRegex(class)
		// Match elements with the specified class (handles class being anywhere in the class attribute)
		pattern := regexp.MustCompile(`(?is)(<[^>]*\bclass\s*=\s*["'][^"']*\b` +
			classPattern + `\b[^"']*["'][^>]*>[\s\S]*?</[^>]+>)`)
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			idx := len(preserved)
			preserved = append(preserved, match)
			return fmt.Sprintf("___PRESERVE_%d___", idx)
		})
	}

	// Extract elements by ID
	for _, id := range s.preserveOptions.IDs {
		if id == "" {
			continue
		}
		// Convert wildcard pattern to regex
		idPattern := wildcardToRegex(id)
		// Match elements with the specified ID
		pattern := regexp.MustCompile(`(?is)(<[^>]*\bid\s*=\s*["']` +
			idPattern + `["'][^>]*>[\s\S]*?</[^>]+>)`)
		result = pattern.ReplaceAllStringFunc(result, func(match string) string {
			idx := len(preserved)
			preserved = append(preserved, match)
			return fmt.Sprintf("___PRESERVE_%d___", idx)
		})
	}

	return result, preserved
}

// wildcardToRegex converts a wildcard pattern (with *) to a regex pattern
func wildcardToRegex(pattern string) string {
	if !strings.Contains(pattern, "*") {
		return regexp.QuoteMeta(pattern)
	}
	// Split by *, quote each part, join with regex wildcard
	parts := strings.Split(pattern, "*")
	for i, part := range parts {
		parts[i] = regexp.QuoteMeta(part)
	}
	return strings.Join(parts, `[^"'\s]*`)
}

// restorePreservedElements restores the preserved elements from placeholders
func (s *Sanitizer) restorePreservedElements(html string, preserved []string) string {
	if len(preserved) == 0 {
		return html
	}

	result := html
	for i, elem := range preserved {
		placeholder := fmt.Sprintf("___PRESERVE_%d___", i)
		result = strings.ReplaceAll(result, placeholder, elem)
	}

	return result
}

// removeTagWithContent removes a tag and all its content
func (s *Sanitizer) removeTagWithContent(html, tag string) string {
	pattern := regexp.MustCompile(`(?is)<` + tag + `[^>]*>[\s\S]*?</` + tag + `>`)
	return pattern.ReplaceAllString(html, "")
}

// processTags processes all HTML tags, keeping only allowed ones
func (s *Sanitizer) processTags(html string) string {
	// Match any HTML tag (opening, closing, or self-closing)
	tagPattern := regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)\s*([^>]*)>`)

	return tagPattern.ReplaceAllStringFunc(html, func(match string) string {
		// Parse the tag
		parts := tagPattern.FindStringSubmatch(match)
		if len(parts) < 3 {
			return ""
		}

		isClosing := parts[1] == "/"
		tagName := strings.ToLower(parts[2])
		attributes := parts[3]

		// Check if tag is allowed
		if !s.allowedTags[tagName] {
			// Not allowed - check if it's a block element that should become a paragraph break
			if s.isBlockElement(tagName) {
				if isClosing {
					return "\n"
				}
				return "\n"
			}
			// Inline elements - just remove the tag, keep content
			return ""
		}

		// Tag is allowed - sanitize attributes
		cleanAttrs := s.sanitizeAttributes(tagName, attributes)

		// Build clean tag
		if isClosing {
			return "</" + tagName + ">"
		}

		if s.selfClosingTags[tagName] {
			if cleanAttrs != "" {
				return "<" + tagName + " " + cleanAttrs + " />"
			}
			return "<" + tagName + " />"
		}

		if cleanAttrs != "" {
			return "<" + tagName + " " + cleanAttrs + ">"
		}
		return "<" + tagName + ">"
	})
}

// isBlockElement checks if a tag is typically a block-level element
func (s *Sanitizer) isBlockElement(tag string) bool {
	blockTags := map[string]bool{
		"div": true, "section": true, "article": true, "aside": true,
		"header": true, "footer": true, "nav": true, "main": true,
		"address": true, "details": true, "dialog": true, "fieldset": true,
		"form": true, "hgroup": true,
	}
	return blockTags[tag]
}

// sanitizeAttributes keeps only safe attributes for a given tag
func (s *Sanitizer) sanitizeAttributes(tag, attrs string) string {
	if attrs == "" {
		return ""
	}

	// Define allowed attributes per tag
	allowedAttrs := map[string][]string{
		"a":     {"href", "title", "target", "rel"},
		"img":   {"src", "alt", "title", "width", "height"},
		"table": {"border", "cellpadding", "cellspacing", "width"},
		"td":    {"colspan", "rowspan", "width", "align", "valign"},
		"th":    {"colspan", "rowspan", "width", "align", "valign", "scope"},
		"tr":    {"align", "valign"},
		"ol":    {"type", "start"},
		"ul":    {"type"},
		"li":    {"value"},
	}

	allowed, hasSpecific := allowedAttrs[tag]
	if !hasSpecific {
		// No specific attributes allowed for this tag
		return ""
	}

	// Parse and filter attributes
	var result []string
	attrPattern := regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	matches := attrPattern.FindAllStringSubmatch(attrs, -1)

	for _, match := range matches {
		attrName := strings.ToLower(match[1])

		// Check if attribute is allowed
		isAllowed := false
		for _, a := range allowed {
			if a == attrName {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			continue
		}

		// Get attribute value
		attrValue := match[2]
		if attrValue == "" {
			attrValue = match[3]
		}
		if attrValue == "" {
			attrValue = match[4]
		}

		// Drop URL-bearing attributes whose scheme is not on the safe allow-list.
		if (attrName == "href" || attrName == "src") && !isSafeURLValue(attrValue) {
			continue
		}

		result = append(result, attrName+`="`+s.escapeAttr(attrValue)+`"`)
	}

	return strings.Join(result, " ")
}

// escapeAttr escapes special characters in attribute values
func (s *Sanitizer) escapeAttr(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	return value
}

// cleanWhitespace normalizes whitespace in the output
func (s *Sanitizer) cleanWhitespace(html string) string {
	// Replace multiple newlines with double newline
	multiNewline := regexp.MustCompile(`\n{3,}`)
	html = multiNewline.ReplaceAllString(html, "\n\n")

	// Replace multiple spaces with single space
	multiSpace := regexp.MustCompile(`[ \t]+`)
	html = multiSpace.ReplaceAllString(html, " ")

	// Clean up space before/after newlines
	html = regexp.MustCompile(` *\n *`).ReplaceAllString(html, "\n")

	// Trim leading/trailing whitespace
	html = strings.TrimSpace(html)

	return html
}
