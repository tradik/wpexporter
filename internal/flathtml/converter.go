package flathtml

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// PreserveOptions defines elements to preserve from HTML processing
type PreserveOptions struct {
	Classes []string // CSS classes to preserve
	IDs     []string // Element IDs to preserve
}

// Converter converts HTML content to Markdown format
type Converter struct {
	rules           []ConversionRule
	customRules     []config.FlatHTMLRule
	preserveOptions *PreserveOptions
}

// ConversionRule defines a conversion pattern (internal)
type ConversionRule struct {
	Pattern     *regexp.Regexp
	Replacement string
	Handler     func(match []string) string
}

// NewConverter creates a new HTML to Markdown converter with default rules
func NewConverter() *Converter {
	c := &Converter{
		rules:       make([]ConversionRule, 0),
		customRules: make([]config.FlatHTMLRule, 0),
	}
	c.initDefaultRules()
	return c
}

// NewConverterWithRules creates a converter with custom rules
func NewConverterWithRules(customRules []config.FlatHTMLRule) *Converter {
	c := &Converter{
		rules:       make([]ConversionRule, 0),
		customRules: customRules,
	}
	c.initCustomRules()
	c.initDefaultRules()
	return c
}

// NewConverterWithOptions creates a converter with custom rules and preserve options
func NewConverterWithOptions(customRules []config.FlatHTMLRule, preserveOpts *PreserveOptions) *Converter {
	c := &Converter{
		rules:           make([]ConversionRule, 0),
		customRules:     customRules,
		preserveOptions: preserveOpts,
	}
	c.initCustomRules()
	c.initDefaultRules()
	return c
}

// initCustomRules initializes user-defined custom rules
func (c *Converter) initCustomRules() {
	for _, rule := range c.customRules {
		if rule.Class == "" && rule.Tag == "" {
			continue
		}

		var patternStr string
		if rule.Class != "" && rule.Tag != "" {
			// Match specific tag with class
			patternStr = `(?is)<` + regexp.QuoteMeta(rule.Tag) + `[^>]*class\s*=\s*["'][^"']*` +
				regexp.QuoteMeta(rule.Class) + `[^"']*["'][^>]*>([^<]*)</` + regexp.QuoteMeta(rule.Tag) + `>`
		} else if rule.Class != "" {
			// Match any tag with class
			patternStr = `(?is)<[^>]*class\s*=\s*["'][^"']*` + regexp.QuoteMeta(rule.Class) + `[^"']*["'][^>]*>([^<]*)</[^>]+>`
		} else {
			// Match specific tag without class requirement
			patternStr = `(?is)<` + regexp.QuoteMeta(rule.Tag) + `[^>]*>([^<]*)</` + regexp.QuoteMeta(rule.Tag) + `>`
		}

		pattern, err := regexp.Compile(patternStr)
		if err != nil {
			continue
		}

		markdownTemplate := rule.Markdown
		c.rules = append(c.rules, ConversionRule{
			Pattern: pattern,
			Handler: func(match []string) string {
				if len(match) < 2 {
					return match[0]
				}
				content := strings.TrimSpace(match[1])
				return strings.ReplaceAll(markdownTemplate, "{content}", content)
			},
		})
	}
}

// initDefaultRules initializes the default conversion rules
func (c *Converter) initDefaultRules() {
	// Bricks Builder specific classes
	c.rules = append(c.rules,
		// brxe-heading with data-level attribute
		ConversionRule{
			Pattern: regexp.MustCompile(`(?is)<[^>]*class\s*=\s*["'][^"']*brxe-heading[^"']*["'][^>]*data-level\s*=\s*["']?(\d)["']?[^>]*>([^<]*)</[^>]+>`),
			Handler: func(match []string) string {
				if len(match) < 3 {
					return match[0]
				}
				level := match[1]
				text := strings.TrimSpace(match[2])
				prefix := strings.Repeat("#", parseInt(level))
				return prefix + " " + text + "\n\n"
			},
		},
		// brxe-heading without data-level (default to h2)
		ConversionRule{
			Pattern: regexp.MustCompile(`(?is)<[^>]*class\s*=\s*["'][^"']*brxe-heading[^"']*["'][^>]*>([^<]*)</[^>]+>`),
			Handler: func(match []string) string {
				if len(match) < 2 {
					return match[0]
				}
				text := strings.TrimSpace(match[1])
				return "## " + text + "\n\n"
			},
		},
		// brxe-text (paragraph)
		ConversionRule{
			Pattern: regexp.MustCompile(`(?is)<[^>]*class\s*=\s*["'][^"']*brxe-text[^"']*["'][^>]*>([^<]+)</[^>]+>`),
			Handler: func(match []string) string {
				if len(match) < 2 {
					return match[0]
				}
				text := strings.TrimSpace(match[1])
				return text + "\n\n"
			},
		},
		// brxe-list items
		ConversionRule{
			Pattern: regexp.MustCompile(`(?is)<[^>]*class\s*=\s*["'][^"']*brxe-list[^"']*["'][^>]*>(.*?)</[^>]+>`),
			Handler: func(match []string) string {
				if len(match) < 2 {
					return match[0]
				}
				return convertList(match[1])
			},
		},
		// brxe-image
		ConversionRule{
			Pattern: regexp.MustCompile(`(?is)<[^>]*class\s*=\s*["'][^"']*brxe-image[^"']*["'][^>]*>` +
				`.*?<img[^>]*src\s*=\s*["']([^"']+)["'][^>]*(?:alt\s*=\s*["']([^"']*)["'])?[^>]*>.*?</[^>]+>`),
			Handler: func(match []string) string {
				if len(match) < 2 {
					return match[0]
				}
				src := match[1]
				alt := ""
				if len(match) > 2 {
					alt = match[2]
				}
				return "![" + alt + "](" + src + ")\n\n"
			},
		},
	)

	// Standard HTML heading conversions
	for i := 1; i <= 6; i++ {
		level := i
		prefix := strings.Repeat("#", level)
		tagNum := string(rune('0' + i))
		c.rules = append(c.rules, ConversionRule{
			Pattern: regexp.MustCompile(`(?is)<h` + tagNum + `[^>]*>([^<]*)</h` + tagNum + `>`),
			Handler: func(match []string) string {
				if len(match) < 2 {
					return match[0]
				}
				text := strings.TrimSpace(match[1])
				return prefix + " " + text + "\n\n"
			},
		})
	}

	// Paragraph
	c.rules = append(c.rules, ConversionRule{
		Pattern: regexp.MustCompile(`(?is)<p[^>]*>([^<]+)</p>`),
		Handler: func(match []string) string {
			if len(match) < 2 {
				return match[0]
			}
			text := strings.TrimSpace(match[1])
			if text == "" {
				return ""
			}
			return text + "\n\n"
		},
	})

	// Bold
	c.rules = append(c.rules, ConversionRule{
		Pattern:     regexp.MustCompile(`(?is)<(?:strong|b)[^>]*>([^<]+)</(?:strong|b)>`),
		Replacement: "**$1**",
	})

	// Italic
	c.rules = append(c.rules, ConversionRule{
		Pattern:     regexp.MustCompile(`(?is)<(?:em|i)[^>]*>([^<]+)</(?:em|i)>`),
		Replacement: "*$1*",
	})

	// Links
	c.rules = append(c.rules, ConversionRule{
		Pattern: regexp.MustCompile(`(?is)<a[^>]*href\s*=\s*["']([^"']+)["'][^>]*>([^<]+)</a>`),
		Handler: func(match []string) string {
			if len(match) < 3 {
				return match[0]
			}
			return "[" + match[2] + "](" + match[1] + ")"
		},
	})

	// Images - src before alt
	c.rules = append(c.rules, ConversionRule{
		Pattern: regexp.MustCompile(`(?is)<img[^>]*src\s*=\s*["']([^"']+)["'][^>]*alt\s*=\s*["']([^"']*)["'][^>]*/?>`),
		Handler: func(match []string) string {
			if len(match) < 3 {
				return match[0]
			}
			return "![" + match[2] + "](" + match[1] + ")\n\n"
		},
	})

	// Images - alt before src
	c.rules = append(c.rules, ConversionRule{
		Pattern: regexp.MustCompile(`(?is)<img[^>]*alt\s*=\s*["']([^"']*)["'][^>]*src\s*=\s*["']([^"']+)["'][^>]*/?>`),
		Handler: func(match []string) string {
			if len(match) < 3 {
				return match[0]
			}
			return "![" + match[1] + "](" + match[2] + ")\n\n"
		},
	})

	// Images - src only (no alt)
	c.rules = append(c.rules, ConversionRule{
		Pattern: regexp.MustCompile(`(?is)<img[^>]*src\s*=\s*["']([^"']+)["'][^>]*/?>`),
		Handler: func(match []string) string {
			if len(match) < 2 {
				return match[0]
			}
			return "![" + "](" + match[1] + ")\n\n"
		},
	})

	// Unordered list
	c.rules = append(c.rules, ConversionRule{
		Pattern: regexp.MustCompile(`(?is)<ul[^>]*>(.*?)</ul>`),
		Handler: func(match []string) string {
			if len(match) < 2 {
				return match[0]
			}
			return convertList(match[1])
		},
	})

	// Ordered list
	c.rules = append(c.rules, ConversionRule{
		Pattern: regexp.MustCompile(`(?is)<ol[^>]*>(.*?)</ol>`),
		Handler: func(match []string) string {
			if len(match) < 2 {
				return match[0]
			}
			return convertOrderedList(match[1])
		},
	})

	// Blockquote
	c.rules = append(c.rules, ConversionRule{
		Pattern: regexp.MustCompile(`(?is)<blockquote[^>]*>([^<]+)</blockquote>`),
		Handler: func(match []string) string {
			if len(match) < 2 {
				return match[0]
			}
			lines := strings.Split(strings.TrimSpace(match[1]), "\n")
			var result []string
			for _, line := range lines {
				result = append(result, "> "+strings.TrimSpace(line))
			}
			return strings.Join(result, "\n") + "\n\n"
		},
	})

	// Code block
	c.rules = append(c.rules, ConversionRule{
		Pattern: regexp.MustCompile(`(?is)<pre[^>]*><code[^>]*>([^<]+)</code></pre>`),
		Handler: func(match []string) string {
			if len(match) < 2 {
				return match[0]
			}
			return "```\n" + strings.TrimSpace(match[1]) + "\n```\n\n"
		},
	})

	// Inline code
	c.rules = append(c.rules, ConversionRule{
		Pattern:     regexp.MustCompile(`(?is)<code[^>]*>([^<]+)</code>`),
		Replacement: "`$1`",
	})

	// Horizontal rule
	c.rules = append(c.rules, ConversionRule{
		Pattern:     regexp.MustCompile(`(?is)<hr[^>]*/?\s*>`),
		Replacement: "\n---\n\n",
	})

	// Line break
	c.rules = append(c.rules, ConversionRule{
		Pattern:     regexp.MustCompile(`(?is)<br[^>]*/?\s*>`),
		Replacement: "\n",
	})
}

// Convert converts HTML content to Markdown
func (c *Converter) Convert(html string) string {
	result := html

	// Extract and preserve elements with specific classes/IDs
	var preserved []string
	result, preserved = c.extractPreservedElements(result)

	// First, remove unwanted elements
	result = c.cleanHTML(result)

	// Apply conversion rules
	for _, rule := range c.rules {
		if rule.Handler != nil {
			result = rule.Pattern.ReplaceAllStringFunc(result, func(match string) string {
				submatch := rule.Pattern.FindStringSubmatch(match)
				return rule.Handler(submatch)
			})
		} else if rule.Replacement != "" {
			result = rule.Pattern.ReplaceAllString(result, rule.Replacement)
		}
	}

	// Clean up remaining HTML tags
	result = c.stripRemainingTags(result)

	// Restore preserved elements
	result = c.restorePreservedElements(result, preserved)

	// Clean up whitespace
	result = c.normalizeWhitespace(result)

	// Decode HTML entities
	result = decodeHTMLEntities(result)

	return strings.TrimSpace(result)
}

// extractPreservedElements extracts elements with preserved classes/IDs and replaces them with placeholders
func (c *Converter) extractPreservedElements(html string) (string, []string) {
	if c.preserveOptions == nil || (len(c.preserveOptions.Classes) == 0 && len(c.preserveOptions.IDs) == 0) {
		return html, nil
	}

	var preserved []string
	result := html

	// Extract elements by class
	for _, class := range c.preserveOptions.Classes {
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
	for _, id := range c.preserveOptions.IDs {
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
func (c *Converter) restorePreservedElements(html string, preserved []string) string {
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

// ConvertPosts converts HTML content in posts to Markdown
func (c *Converter) ConvertPosts(posts []models.WordPressPost) []models.WordPressPost {
	for i := range posts {
		posts[i].Content.Rendered = c.Convert(posts[i].Content.Rendered)
	}
	return posts
}

// cleanHTML removes script, style, and other unwanted elements
func (c *Converter) cleanHTML(html string) string {
	// Remove script tags
	scriptPattern := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptPattern.ReplaceAllString(html, "")

	// Remove style tags
	stylePattern := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = stylePattern.ReplaceAllString(html, "")

	// Remove comments
	commentPattern := regexp.MustCompile(`(?is)<!--.*?-->`)
	html = commentPattern.ReplaceAllString(html, "")

	// Remove noscript tags
	noscriptPattern := regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	html = noscriptPattern.ReplaceAllString(html, "")

	// Remove empty divs and spans
	emptyDivPattern := regexp.MustCompile(`(?is)<(?:div|span)[^>]*>\s*</(?:div|span)>`)
	for i := 0; i < 3; i++ { // Multiple passes for nested empty elements
		html = emptyDivPattern.ReplaceAllString(html, "")
	}

	// Remove id attributes
	idPattern := regexp.MustCompile(`\s+id\s*=\s*["'][^"']*["']`)
	html = idPattern.ReplaceAllString(html, "")

	return html
}

// stripRemainingTags removes any remaining HTML tags
func (c *Converter) stripRemainingTags(html string) string {
	// Remove remaining opening tags with attributes
	tagPattern := regexp.MustCompile(`<[^>]+>`)
	return tagPattern.ReplaceAllString(html, "")
}

// normalizeWhitespace cleans up excessive whitespace
func (c *Converter) normalizeWhitespace(text string) string {
	// Replace multiple newlines with double newline
	multiNewline := regexp.MustCompile(`\n{3,}`)
	text = multiNewline.ReplaceAllString(text, "\n\n")

	// Remove trailing whitespace from lines
	trailingWS := regexp.MustCompile(`[ \t]+\n`)
	text = trailingWS.ReplaceAllString(text, "\n")

	// Remove leading whitespace from lines (but preserve indentation for lists)
	leadingWS := regexp.MustCompile(`\n[ \t]+([^-*\d])`)
	text = leadingWS.ReplaceAllString(text, "\n$1")

	return text
}

// convertList converts HTML list items to Markdown unordered list
func convertList(listHTML string) string {
	liPattern := regexp.MustCompile(`(?is)<li[^>]*>([^<]+)</li>`)
	matches := liPattern.FindAllStringSubmatch(listHTML, -1)

	var result []string
	for _, match := range matches {
		if len(match) > 1 {
			text := strings.TrimSpace(match[1])
			if text != "" {
				result = append(result, "- "+text)
			}
		}
	}

	if len(result) > 0 {
		return strings.Join(result, "\n") + "\n\n"
	}
	return ""
}

// convertOrderedList converts HTML list items to Markdown ordered list
func convertOrderedList(listHTML string) string {
	liPattern := regexp.MustCompile(`(?is)<li[^>]*>([^<]+)</li>`)
	matches := liPattern.FindAllStringSubmatch(listHTML, -1)

	var result []string
	for i, match := range matches {
		if len(match) > 1 {
			text := strings.TrimSpace(match[1])
			if text != "" {
				result = append(result, fmt.Sprintf("%d. %s", i+1, text))
			}
		}
	}

	if len(result) > 0 {
		return strings.Join(result, "\n") + "\n\n"
	}
	return ""
}

// parseInt safely parses an integer string
func parseInt(s string) int {
	if len(s) == 0 {
		return 2 // default heading level
	}
	n := int(s[0] - '0')
	if n < 1 || n > 6 {
		return 2
	}
	return n
}

// decodeHTMLEntities decodes common HTML entities
func decodeHTMLEntities(s string) string {
	replacements := map[string]string{
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   "\"",
		"&#39;":    "'",
		"&apos;":   "'",
		"&nbsp;":   " ",
		"&ndash;":  "-",
		"&mdash;":  "-",
		"&lsquo;":  "'",
		"&rsquo;":  "'",
		"&ldquo;":  "\"",
		"&rdquo;":  "\"",
		"&hellip;": "...",
		"&copy;":   "(c)",
		"&reg;":    "(R)",
		"&trade;":  "(TM)",
	}

	result := s
	for entity, char := range replacements {
		result = strings.ReplaceAll(result, entity, char)
	}
	return result
}
